import json
from unittest.mock import AsyncMock, MagicMock
import pytest
from httpx import ASGITransport, AsyncClient

from ai.app.main import app
from ai.app.config import get_settings
from ai.models.embeddings.provider import BaseEmbeddingProvider
from ai.pipelines.skills.normalizer import (
    SkillNormalizer,
    NormalizedSkillResult,
    get_skill_normalizer,
)
from ai.app.api.skills import (
    NormalizeSkillRequest,
    NormalizeSkillResponse,
    ExtractSkillsRequest,
    ExtractSkillsResponse,
)


class MockSemanticEmbeddingProvider(BaseEmbeddingProvider):
    """Test embedding provider that assigns high cosine similarity to semantic test pairs."""

    def __init__(self) -> None:
        self._dimension = 4
        # Orthogonal basis for skills
        self._vectors = {
            "Computer Vision": [1.0, 0.0, 0.0, 0.0],
            "Deep Convolutional Networks": [0.85, 0.15, 0.0, 0.0],
            "Machine Learning": [0.0, 1.0, 0.0, 0.0],
            "React": [0.0, 0.0, 1.0, 0.0],
            "Python": [0.0, 0.0, 0.0, 1.0],
        }

    @property
    def model_name(self) -> str:
        return "mock-semantic"

    @property
    def model_version(self) -> str:
        return "1.0.0"

    @property
    def dimension(self) -> int:
        return self._dimension

    def embed(self, text: str) -> list[float]:
        # Return known vector or zero vector
        if text in self._vectors:
            vec = self._vectors[text]
            # normalize
            import math
            norm = math.sqrt(sum(x * x for x in vec))
            return [x / norm for x in vec]
        return [0.0, 0.0, 0.0, 0.0]

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        return [self.embed(t) for t in texts]


def test_exact_alias_match():
    normalizer = SkillNormalizer()
    result = normalizer.normalize("ReactJS")

    assert isinstance(result, NormalizedSkillResult)
    assert result.original_skill == "ReactJS"
    assert result.canonical_name == "React"
    assert result.confidence == 1.0
    assert result.match_method == "exact_alias"
    assert result.category == "Frontend"


def test_exact_alias_match_other_skills():
    normalizer = SkillNormalizer()

    test_cases = [
        ("golang", "Go", "Backend"),
        ("postgres", "PostgreSQL", "Database"),
        ("python3", "Python", "Backend / Data"),
        ("ts", "TypeScript", "Frontend / Fullstack"),
        ("nlp", "Natural Language Processing", "Data & AI"),
        ("ml", "Machine Learning", "Data & AI"),
    ]

    for alias, expected_canonical, expected_category in test_cases:
        res = normalizer.normalize(alias)
        assert res.canonical_name == expected_canonical
        assert res.confidence == 1.0
        assert res.match_method == "exact_alias"
        assert res.category == expected_category


def test_fuzzy_match():
    normalizer = SkillNormalizer()
    result = normalizer.normalize("ComputerVision")

    assert isinstance(result, NormalizedSkillResult)
    assert result.canonical_name == "Computer Vision"
    assert result.confidence >= 0.9
    assert result.match_method == "fuzzy"
    assert result.category == "Data & AI"


def test_embedding_similarity_match():
    provider = MockSemanticEmbeddingProvider()
    normalizer = SkillNormalizer(embedding_provider=provider)
    result = normalizer.normalize("Deep Convolutional Networks")

    assert isinstance(result, NormalizedSkillResult)
    assert result.canonical_name == "Computer Vision"
    assert result.confidence >= 0.75
    assert result.match_method == "embedding"
    assert result.category == "Data & AI"


def test_unknown_string_fallback():
    normalizer = SkillNormalizer()
    arbitrary_str = "CompletelyUnknownWidget99X"
    result = normalizer.normalize(arbitrary_str)

    assert isinstance(result, NormalizedSkillResult)
    assert result.original_skill == arbitrary_str
    assert result.canonical_name == arbitrary_str
    assert result.confidence < 0.5
    assert result.match_method == "fallback"
    assert result.category is None


def test_extract_skills_from_text():
    normalizer = SkillNormalizer()
    sample_text = (
        "We are hiring a backend engineer skilled in Golang and PostgreSQL. "
        "Familiarity with ReactJS, Docker, and Machine Learning is required."
    )

    extracted = normalizer.extract_skills_from_text(sample_text)
    assert isinstance(extracted, list)
    canonical_names = [s.canonical_name for s in extracted]

    assert "Go" in canonical_names
    assert "PostgreSQL" in canonical_names
    assert "React" in canonical_names
    assert "Docker" in canonical_names
    assert "Machine Learning" in canonical_names


@pytest.mark.asyncio
async def test_api_normalize_missing_secret():
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/internal/v1/skills/normalize",
            json={"skill": "ReactJS"},
        )
        assert response.status_code == 401


@pytest.mark.asyncio
async def test_api_normalize_valid_secret():
    transport = ASGITransport(app=app)
    settings = get_settings()
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/internal/v1/skills/normalize",
            headers={"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET},
            json={"skill": "ReactJS"},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["original_skill"] == "ReactJS"
        assert data["canonical_name"] == "React"
        assert data["confidence"] == 1.0
        assert data["match_method"] == "exact_alias"
        assert data["category"] == "Frontend"


@pytest.mark.asyncio
async def test_api_extract_valid_secret():
    transport = ASGITransport(app=app)
    settings = get_settings()
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/internal/v1/skills/extract",
            headers={"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET},
            json={
                "text": "Seeking a developer experienced with Python, Docker, and Next.js."
            },
        )
        assert response.status_code == 200
        data = response.json()
        assert "extracted_skills" in data
        names = [s["canonical_name"] for s in data["extracted_skills"]]
        assert "Python" in names
        assert "Docker" in names
        assert "Next.js" in names


@pytest.mark.asyncio
async def test_api_extract_missing_secret():
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/internal/v1/skills/extract",
            json={"text": "Any text here"},
        )
        assert response.status_code == 401


@pytest.mark.asyncio
async def test_api_normalize_fallback():
    transport = ASGITransport(app=app)
    settings = get_settings()
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/internal/v1/skills/normalize",
            headers={"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET},
            json={"skill": "CompletelyUnknownFramework123"},
        )
        assert response.status_code == 200
        data = response.json()
        assert data["original_skill"] == "CompletelyUnknownFramework123"
        assert data["canonical_name"] == "CompletelyUnknownFramework123"
        assert data["match_method"] == "fallback"
        assert data["confidence"] < 0.5
        assert data["category"] is None


def test_empty_inputs_edge_cases():
    normalizer = SkillNormalizer()
    res_empty = normalizer.normalize("")
    assert res_empty.match_method == "fallback"
    assert res_empty.confidence == 0.0

    res_spaces = normalizer.normalize("    ")
    assert res_spaces.match_method == "fallback"
    assert res_spaces.confidence == 0.0

    extracted_empty = normalizer.extract_skills_from_text("")
    assert extracted_empty == []

    extracted_no_skills = normalizer.extract_skills_from_text(
        "There are no engineering skills mentioned here."
    )
    assert extracted_no_skills == []


@pytest.mark.asyncio
async def test_skills_validation_bounds():
    """Verify NormalizeSkillRequest and ExtractSkillsRequest reject payloads exceeding bounds."""
    transport = ASGITransport(app=app)
    settings = get_settings()
    headers = {"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET}

    async with AsyncClient(transport=transport, base_url="http://test") as client:
        # Skill > 200 chars
        resp_long_skill = await client.post(
            "/internal/v1/skills/normalize",
            headers=headers,
            json={"skill": "s" * 201},
        )
        assert resp_long_skill.status_code == 422

        # Skill < 1 char
        resp_empty_skill = await client.post(
            "/internal/v1/skills/normalize",
            headers=headers,
            json={"skill": ""},
        )
        assert resp_empty_skill.status_code == 422

        # Text > 10000 chars
        resp_long_text = await client.post(
            "/internal/v1/skills/extract",
            headers=headers,
            json={"text": "t" * 10001},
        )
        assert resp_long_text.status_code == 422


@pytest.mark.asyncio
async def test_skills_telemetry_recording():
    """Verify that /normalize and /extract record AI runs via track_ai_run."""
    transport = ASGITransport(app=app)
    settings = get_settings()
    headers = {"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET}

    mock_conn = AsyncMock()
    mock_conn.execute = AsyncMock(return_value="INSERT 0 1")

    mock_pool = MagicMock()
    mock_acquire_cm = AsyncMock()
    mock_acquire_cm.__aenter__.return_value = mock_conn
    mock_acquire_cm.__aexit__.return_value = None
    mock_pool.acquire.return_value = mock_acquire_cm

    original_pool = getattr(app.state, "db_pool", None)
    app.state.db_pool = mock_pool

    try:
        async with AsyncClient(transport=transport, base_url="http://test") as client:
            # 1. Normalize skill endpoint telemetry
            norm_resp = await client.post(
                "/internal/v1/skills/normalize",
                headers=headers,
                json={"skill": "ReactJS"},
            )
            assert norm_resp.status_code == 200
            assert mock_conn.execute.call_count == 1

            call_args = mock_conn.execute.call_args[0]
            query, params = call_args[0], call_args[1:]
            assert "INSERT INTO ai_runs" in query
            assert params[0] == "skill_normalization"  # feature
            assert params[1] == "skill"  # entity_type
            assert params[2] == "ReactJS"  # entity_id
            assert params[3] == "lynk-skill-normalizer"  # model_name
            assert params[4] == "1.0.0"  # model_version
            assert params[6] == "skills-v1"  # pipeline_version
            assert len(params[7]) == 64  # input_hash
            output_json = json.loads(params[8])
            assert output_json["canonical_name"] == "React"
            assert output_json["confidence"] == 1.0
            assert params[9] == 1.0  # confidence
            assert params[11] == "success"  # status

            # Reset call count
            mock_conn.execute.reset_mock()

            # 2. Extract skills endpoint telemetry
            extract_resp = await client.post(
                "/internal/v1/skills/extract",
                headers=headers,
                json={"text": "Looking for Go and React engineers"},
            )
            assert extract_resp.status_code == 200
            assert mock_conn.execute.call_count == 1

            call_args = mock_conn.execute.call_args[0]
            query, params = call_args[0], call_args[1:]
            assert "INSERT INTO ai_runs" in query
            assert params[0] == "skill_extraction"  # feature
            assert params[1] == "text"  # entity_type
            assert params[2].startswith("Looking for")  # entity_id
            assert params[3] == "lynk-skill-normalizer"  # model_name
            assert params[4] == "1.0.0"  # model_version
            assert params[6] == "skills-v1"  # pipeline_version
            assert len(params[7]) == 64  # input_hash
            output_json = json.loads(params[8])
            assert "extracted_count" in output_json
            assert output_json["extracted_count"] >= 1
            assert params[11] == "success"  # status
    finally:
        app.state.db_pool = original_pool



