"""Tests for Member Recommendations Engine and Internal API."""

import pytest
from httpx import ASGITransport, AsyncClient

from ai.app.config import get_settings
from ai.app.main import app
from ai.pipelines.recommendations.engine import (
    RecommendationEngine,
    RecommendationItem,
    get_recommendation_engine,
)


@pytest.mark.asyncio
async def test_python_machine_learning_skill_recommendations():
    """Test that a user with Python & Machine Learning receives PyTorch or Docker

    with high confidence and a transparent explanation.
    """
    engine = RecommendationEngine()
    items = await engine.generate_recommendations(
        user_id="usr-ml-123",
        current_skills=["Python", "Machine Learning"],
        department="Computer Science",
        bio="Junior passionate about artificial intelligence and deep learning models.",
    )

    assert len(items) > 0
    titles = [item.title for item in items]
    assert any(t in ("PyTorch", "Docker", "scikit-learn") for t in titles)

    for item in items:
        assert isinstance(item.title, str) and len(item.title) > 0
        assert isinstance(item.reason, str) and len(item.reason) > 0
        assert 0.0 <= item.confidence <= 1.0
        assert item.type in ("skill", "profile", "opportunity")


@pytest.mark.asyncio
async def test_profile_improvement_recommendations_when_empty():
    """Test that profile improvement recommendations are generated when bio

    or portfolio is empty.
    """
    engine = RecommendationEngine()
    items = await engine.generate_recommendations(
        user_id="usr-incomplete-456",
        current_skills=["Python"],
        department=None,
        bio=None,
        portfolio_links=[],
    )

    profile_items = [it for it in items if it.type == "profile"]
    assert len(profile_items) >= 2

    titles = [it.title.lower() for it in profile_items]
    assert any("bio" in t for t in titles)
    assert any("portfolio" in t or "github" in t for t in titles)

    for it in profile_items:
        assert len(it.reason) > 10
        assert 0.0 <= it.confidence <= 1.0


@pytest.mark.asyncio
async def test_skill_deduplication():
    """Test that already declared skills are never recommended back to the member."""
    engine = RecommendationEngine()
    skills = ["Python", "Machine Learning", "PyTorch", "scikit-learn", "Docker"]
    items = await engine.generate_recommendations(
        user_id="usr-advanced-789",
        current_skills=skills,
        department="Computer Science",
        bio="AI researcher with extensive experience in neural networks.",
    )

    skill_recs = [it.title.lower() for it in items if it.type == "skill"]
    for s in skills:
        assert s.lower() not in skill_recs


@pytest.mark.asyncio
async def test_react_typescript_co_occurrence():
    """Test skill co-occurrence for React and TypeScript."""
    engine = RecommendationEngine()
    items = await engine.generate_recommendations(
        user_id="usr-frontend-101",
        current_skills=["React", "TypeScript"],
        department="Software Engineering",
        bio="Frontend web developer building modern applications.",
    )

    titles = [it.title for it in items]
    assert any(t in ("Next.js", "Tailwind CSS") for t in titles)


@pytest.mark.asyncio
async def test_recommendations_api_unauthorized():
    """Test POST /internal/v1/recommendations/profile rejects requests without valid secret."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/recommendations/profile",
            json={"user_id": "usr-1", "current_skills": ["Python"]},
        )
        assert resp.status_code == 401


@pytest.mark.asyncio
async def test_recommendations_api_success():
    """Test POST /internal/v1/recommendations/profile succeeds with X-Internal-AI-Secret."""
    settings = get_settings()
    headers = {"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET}
    payload = {
        "user_id": "usr-api-123",
        "current_skills": ["Python", "Machine Learning"],
        "department": "Computer Science",
        "bio": "Building neural nets and computer vision pipelines.",
        "limit": 5,
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/recommendations/profile",
            json=payload,
            headers=headers,
        )
        assert resp.status_code == 200
        data = resp.json()

        assert data["user_id"] == "usr-api-123"
        assert "recommendations" in data
        assert len(data["recommendations"]) <= 5
        assert len(data["recommendations"]) > 0

        titles = [r["title"] for r in data["recommendations"]]
        assert any(t in ("PyTorch", "Docker", "scikit-learn") for t in titles)
        assert data["model_name"] == "lynk-recommendation-engine"
        assert data["model_version"] == "1.0.0"


@pytest.mark.asyncio
async def test_recommendations_db_persistence_archives_existing():
    """Test that existing active recommendations are archived prior to inserting new ones."""
    from unittest.mock import AsyncMock, MagicMock

    mock_conn = AsyncMock()
    mock_pool = MagicMock()
    mock_pool.acquire.return_value.__aenter__.return_value = mock_conn
    mock_pool.acquire.return_value.__aexit__.return_value = None

    engine = RecommendationEngine(db_pool=mock_pool)
    user_id = "usr-archive-test"
    items = await engine.generate_recommendations(
        user_id=user_id,
        current_skills=["Python", "Machine Learning"],
        department="Computer Science",
        bio="Test bio for recommendations archiving.",
    )

    assert len(items) > 0
    assert mock_conn.execute.call_count >= 2

    # Verify first execute was archiving existing active recommendations
    first_call = mock_conn.execute.call_args_list[0]
    assert "UPDATE ai_recommendations SET status = 'archived'" in first_call[0][0]
    assert first_call[0][1] == user_id

    # Verify subsequent calls inserted new items
    second_call = mock_conn.execute.call_args_list[1]
    assert "INSERT INTO ai_recommendations" in second_call[0][0]
    assert second_call[0][1] == user_id
