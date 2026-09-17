"""Tests for AI semantic & hybrid search pipeline and endpoints."""

import math
import pytest
from httpx import ASGITransport, AsyncClient

from ai.app.config import get_settings
from ai.app.main import app
from ai.app.middleware.run_tracker import AIRunRecord
from ai.models.embeddings.provider import BaseEmbeddingProvider, MockEmbeddingProvider
from ai.pipelines.search.hybrid import (
    HybridSearchPipeline,
    SearchResultItem,
    compute_hybrid_score,
    reciprocal_rank_fusion,
)
from ai.pipelines.skills.normalizer import SkillNormalizer


class MockSearchEmbeddingProvider(BaseEmbeddingProvider):
    """Deterministic embedding provider for search tests."""

    def __init__(self) -> None:
        self._dimension = 4
        self._vectors = {
            "Students with PyTorch and computer vision experience": [1.0, 0.0, 0.0, 0.0],
            "Frontend developer with React": [0.0, 1.0, 0.0, 0.0],
            "Computer vision researcher with PyTorch": [0.9, 0.1, 0.0, 0.0],
            "General web developer": [0.0, 0.2, 0.8, 0.0],
        }

    @property
    def model_name(self) -> str:
        return "mock-search-embedder"

    @property
    def model_version(self) -> str:
        return "1.0.0"

    @property
    def dimension(self) -> int:
        return self._dimension

    def embed(self, text: str) -> list[float]:
        vec = self._vectors.get(text, [0.25, 0.25, 0.25, 0.25])
        norm = math.sqrt(sum(x * x for x in vec))
        return [x / norm for x in vec]

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        return [self.embed(t) for t in texts]


def test_query_skill_extraction():
    normalizer = SkillNormalizer()
    query = "Looking for students with PyTorch, Docker, and React experience"
    extracted = normalizer.extract_skills_from_text(query)
    names = {s.canonical_name for s in extracted}
    assert "PyTorch" in names
    assert "Docker" in names
    assert "React" in names


def test_query_embedding_generation():
    provider = MockEmbeddingProvider(dimension=384)
    query = "Need a backend engineer proficient in Go and PostgreSQL"
    embedding = provider.embed(query)
    assert len(embedding) == 384
    norm = math.sqrt(sum(x * x for x in embedding))
    assert abs(norm - 1.0) < 1e-4


def test_hybrid_score_computation():
    # Test combination of semantic cosine sim + text match + skill overlap
    score_high = compute_hybrid_score(
        cosine_similarity=0.92,
        keyword_score=0.85,
        matched_skills_count=2,
        total_query_skills=2,
    )
    score_low = compute_hybrid_score(
        cosine_similarity=0.20,
        keyword_score=0.10,
        matched_skills_count=0,
        total_query_skills=2,
    )
    assert 0.0 <= score_high <= 1.0
    assert 0.0 <= score_low <= 1.0
    assert score_high > score_low


def test_reciprocal_rank_fusion():
    vector_ranks = {"doc1": 1, "doc2": 2, "doc3": 3}
    text_ranks = {"doc2": 1, "doc1": 3, "doc3": 2}

    fused = reciprocal_rank_fusion([vector_ranks, text_ranks], k=60)
    assert len(fused) == 3
    # doc2 ranks #2 in vector and #1 in text -> 1/62 + 1/61 = 0.0161 + 0.0163 = 0.0325
    # doc1 ranks #1 in vector and #3 in text -> 1/61 + 1/63 = 0.0163 + 0.0158 = 0.0322
    assert fused[0][0] == "doc2"
    assert fused[0][1] > fused[1][1]


@pytest.mark.asyncio
async def test_hybrid_search_pipeline_execution():
    provider = MockSearchEmbeddingProvider()
    normalizer = SkillNormalizer()
    runs_sink: list[AIRunRecord] = []

    candidates = [
        {
            "id": "cand-1",
            "title": "Computer vision researcher with PyTorch",
            "description": "Building CV models with PyTorch and Python",
            "skills": ["PyTorch", "Computer Vision", "Python"],
        },
        {
            "id": "cand-2",
            "title": "General web developer",
            "description": "Building websites with HTML and CSS",
            "skills": ["HTML", "CSS"],
        },
    ]

    pipeline = HybridSearchPipeline(
        embedding_provider=provider,
        skill_normalizer=normalizer,
        sink=runs_sink,
    )

    results = await pipeline.search_candidates(
        query="Students with PyTorch and computer vision experience",
        entity_type="profile",
        candidates=candidates,
        limit=10,
    )

    assert len(results) == 2
    top = results[0]
    assert isinstance(top, SearchResultItem)
    assert top.entity_id == "cand-1"
    assert top.score > results[1].score
    assert "PyTorch" in top.matched_skills
    assert "Computer Vision" in top.matched_skills

    # Verify audit log was recorded by track_ai_run
    assert len(runs_sink) == 1
    run = runs_sink[0]
    assert run.feature == "hybrid_search"
    assert run.entity_type == "profile"
    assert run.status == "success"
    assert run.model_name == provider.model_name


@pytest.mark.asyncio
async def test_search_api_unauthorized():
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        # Missing secret
        resp1 = await client.post(
            "/internal/v1/search",
            json={"query": "React engineer", "entity_type": "job"},
        )
        assert resp1.status_code == 401

        # Invalid secret
        resp2 = await client.post(
            "/internal/v1/search",
            headers={"X-Internal-AI-Secret": "wrong-secret"},
            json={"query": "React engineer", "entity_type": "job"},
        )
        assert resp2.status_code == 401


@pytest.mark.asyncio
async def test_search_api_success():
    transport = ASGITransport(app=app)
    settings = get_settings()

    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/search",
            headers={"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET},
            json={
                "query": "Backend developer with Go and Docker",
                "entity_type": "job",
                "limit": 5,
            },
        )
        assert resp.status_code == 200
        data = resp.json()
        assert "results" in data
        assert "total" in data
        assert "model_name" in data
        assert "model_version" in data
        assert isinstance(data["results"], list)


@pytest.mark.asyncio
async def test_search_api_empty_query():
    transport = ASGITransport(app=app)
    settings = get_settings()

    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/search",
            headers={"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET},
            json={"query": "   ", "entity_type": "job"},
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["results"] == []
        assert data["total"] == 0
