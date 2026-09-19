"""Tests for Candidate Application Ranking Pipeline and Internal API."""

import pytest
from ai.app.config import get_settings
from ai.app.main import app
from ai.pipelines.ranking.ranker import (
    CandidateRanker,
    CandidateRankInput,
)
from httpx import ASGITransport, AsyncClient


@pytest.mark.asyncio
async def test_ranking_skill_overlap_and_semantic_scoring():
    """Candidate with 100% required skills and aligned bio scores significantly higher

    than candidate with 0% required skills.
    """
    ranker = CandidateRanker()

    job_title = "Backend Go Engineer"
    job_description = "We need a skilled Go developer to build high performance REST APIs and microservices with PostgreSQL."
    job_department = "Computer Science"
    required_skills = ["Go", "PostgreSQL", "Docker"]

    candidates = [
        CandidateRankInput(
            application_id="app-strong-1",
            skills=["Go", "PostgreSQL", "Docker", "Git"],
            bio="Junior computer science student focused on Go backend systems and database indexing.",
            department="Computer Science",
            cover_letter="I have built backend services in Go with PostgreSQL and containerized them using Docker.",
        ),
        CandidateRankInput(
            application_id="app-weak-2",
            skills=["Photoshop", "Illustration"],
            bio="Visual arts student interested in branding and poster design.",
            department="Fine Arts",
            cover_letter="I would like to try software engineering.",
        ),
    ]

    results = await ranker.rank_candidates(
        job_id="job-test-1",
        job_title=job_title,
        job_description=job_description,
        job_department=job_department,
        required_skills=required_skills,
        candidates=candidates,
    )

    assert len(results) == 2
    top = results[0]
    bottom = results[1]

    # Strong candidate should be ranked first
    assert top.application_id == "app-strong-1"
    assert bottom.application_id == "app-weak-2"

    # Score comparison: Strong candidate must score significantly higher
    assert top.score >= 75.0
    assert bottom.score <= 35.0
    assert top.score - bottom.score >= 40.0

    # Skill breakdown assertions
    assert set(top.matched_skills) == {"Go", "PostgreSQL", "Docker"}
    assert len(top.missing_skills) == 0
    assert len(bottom.matched_skills) == 0
    assert set(bottom.missing_skills) == {"Go", "PostgreSQL", "Docker"}

    # Structured reason asserting observable weights
    assert "skills=" in top.reason
    assert "semantic_similarity=" in top.reason
    assert "department_compatibility=" in top.reason
    assert top.confidence > 0.0


@pytest.mark.asyncio
async def test_ranking_fairness_demographics_excluded():
    """Verify that scoring is strictly fair: identical qualifications yield identical scores,

    and non-observable demographic features are excluded.
    """
    ranker = CandidateRanker()

    job_title = "Data Analyst"
    job_description = "Analyzing campus data sets with Python and SQL."
    job_department = "Data Science"
    required_skills = ["Python", "SQL"]

    cand_a = CandidateRankInput(
        application_id="app-a",
        skills=["Python", "SQL"],
        bio="Junior studying data analytics with statistical modeling experience.",
        department="Data Science",
        cover_letter="Excited to analyze campus data.",
    )

    cand_b = CandidateRankInput(
        application_id="app-b",
        skills=["Python", "SQL"],
        bio="Junior studying data analytics with statistical modeling experience.",
        department="Data Science",
        cover_letter="Excited to analyze campus data.",
    )

    results = await ranker.rank_candidates(
        job_id="job-fairness",
        job_title=job_title,
        job_description=job_description,
        job_department=job_department,
        required_skills=required_skills,
        candidates=[cand_a, cand_b],
    )

    assert len(results) == 2
    assert results[0].score == results[1].score
    assert results[0].matched_skills == results[1].matched_skills
    assert results[0].missing_skills == results[1].missing_skills

    # CandidateRankInput schema does not expose or include protected demographic fields
    fields = CandidateRankInput.model_fields.keys()
    assert "gender" not in fields
    assert "race" not in fields
    assert "age" not in fields
    assert "ethnicity" not in fields
    assert "graduation_year" not in fields


@pytest.mark.asyncio
async def test_ranking_api_unauthorized():
    """POST /internal/v1/ranking/candidates rejects requests missing internal secret."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/ranking/candidates",
            json={
                "job_id": "job-1",
                "job_title": "Software Gig",
                "job_description": "Build an app",
                "job_department": "Engineering",
                "required_skills": ["Python"],
                "candidates": [
                    {
                        "application_id": "app-1",
                        "skills": ["Python"],
                        "bio": "Dev",
                        "department": "Engineering",
                    }
                ],
            },
        )
        assert resp.status_code == 401


@pytest.mark.asyncio
async def test_ranking_api_success():
    """POST /internal/v1/ranking/candidates succeeds with valid secret and returns advisory ranking."""
    settings = get_settings()
    headers = {"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET}

    payload = {
        "job_id": "job-api-123",
        "job_title": "Fullstack Web Developer",
        "job_description": "Build responsive React frontend and Node backend.",
        "job_department": "Computer Science",
        "required_skills": ["React", "TypeScript", "Node.js"],
        "candidates": [
            {
                "application_id": "app-101",
                "skills": ["React", "TypeScript", "Node.js"],
                "bio": "Fullstack web developer experienced with modern React and TypeScript.",
                "department": "Computer Science",
                "cover_letter": "I have built several React applications.",
            },
            {
                "application_id": "app-102",
                "skills": ["Python"],
                "bio": "Data researcher.",
                "department": "Mathematics",
                "cover_letter": "I know some Python.",
            },
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/ranking/candidates",
            json=payload,
            headers=headers,
        )
        assert resp.status_code == 200
        data = resp.json()

        assert "results" in data
        assert len(data["results"]) == 2
        assert data["results"][0]["application_id"] == "app-101"
        assert data["results"][0]["score"] > data["results"][1]["score"]
        assert data["model_name"] == "lynk-candidate-ranker"
        assert data["model_version"] == "1.0.0"
        assert data["pipeline_version"] == "ranking-v1"


@pytest.mark.asyncio
async def test_ranking_batch_embedding_vectorization():
    """Verify embed_batch is called for multiple candidates rather than iterative embed calls."""
    from unittest.mock import MagicMock

    from ai.models.embeddings.provider import MockEmbeddingProvider

    mock_provider = MockEmbeddingProvider()
    mock_provider.embed_batch = MagicMock(wraps=mock_provider.embed_batch)

    ranker = CandidateRanker(embedding_provider=mock_provider)

    candidates = [
        CandidateRankInput(
            application_id=f"app-{i}",
            skills=["Python"],
            bio=f"Bio for candidate {i}",
            cover_letter=f"Cover letter for candidate {i}",
        )
        for i in range(5)
    ]

    results = await ranker.rank_candidates(
        job_id="job-batch-test",
        job_title="Software Engineer",
        job_description="Seeking developers",
        job_department="Computer Science",
        required_skills=["Python"],
        candidates=candidates,
    )

    assert len(results) == 5
    # embed_batch should be invoked exactly once for the batch of 5 candidates
    mock_provider.embed_batch.assert_called_once()
    assert len(mock_provider.embed_batch.call_args[0][0]) == 5
