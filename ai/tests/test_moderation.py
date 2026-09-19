"""Unit and integration tests for hybrid moderation and spam detection."""

import pytest
from ai.app.config import get_settings
from ai.app.main import app
from ai.models.embeddings.provider import MockEmbeddingProvider
from ai.pipelines.moderation.detector import (
    ModerationCheckInput,
    ModerationDetector,
)
from httpx import ASGITransport, AsyncClient


@pytest.fixture
def mock_detector() -> ModerationDetector:
    """Instantiate a ModerationDetector using deterministic MockEmbeddingProvider."""
    provider = MockEmbeddingProvider()
    return ModerationDetector(embedding_provider=provider)


def test_moderation_clean_unique_content(mock_detector: ModerationDetector):
    """Clean, substantive text with normal velocity should be allowed with low risk."""
    clean_text = (
        "Looking for an experienced student developer to build a modern React "
        "and Go web dashboard for our university lab equipment scheduling system. "
        "We need solid test coverage and clean documentation."
    )
    result = mock_detector.check(
        ModerationCheckInput(
            entity_type="job",
            entity_id="job-clean-1",
            text=clean_text,
            author_id="usr-author-1",
            author_recent_post_count=1,
            recent_submissions=[],
        )
    )

    assert result.decision == "allow"
    assert result.risk_score < 0.30
    assert len(result.signals) == 0
    assert result.confidence >= 0.85


def test_moderation_near_duplicate_description(mock_detector: ModerationDetector):
    """Near duplicate description against recent submissions should flag duplicate_description and risk >= 0.80."""
    original_text = "Looking for a frontend developer to create a landing page with Tailwind CSS and Next.js."
    # Exact or near-identical text submitted recently
    duplicate_text = "Looking for a frontend developer to create a landing page with Tailwind CSS and Next.js. "

    result = mock_detector.check(
        ModerationCheckInput(
            entity_type="job",
            entity_id="job-dup-1",
            text=duplicate_text,
            author_id="usr-spammer-1",
            author_recent_post_count=1,
            recent_submissions=[original_text],
        )
    )

    assert "duplicate_description" in result.signals
    assert result.risk_score >= 0.80
    assert result.decision in ("review", "reject")


def test_moderation_short_content(mock_detector: ModerationDetector):
    """Content with <= 30 characters should trigger short_content signal."""
    short_text = "Need help fast."
    result = mock_detector.check(
        ModerationCheckInput(
            entity_type="job",
            entity_id="job-short-1",
            text=short_text,
            author_id="usr-author-2",
            author_recent_post_count=0,
            recent_submissions=[],
        )
    )

    assert "short_content" in result.signals
    assert result.risk_score >= 0.35


def test_moderation_high_velocity(mock_detector: ModerationDetector):
    """Author posting >= 5 times in 1 hour should trigger high_velocity signal."""
    text = "Campus project seeking a junior software developer for backend Python API development."
    result = mock_detector.check(
        ModerationCheckInput(
            entity_type="job",
            entity_id="job-velo-1",
            text=text,
            author_id="usr-author-3",
            author_recent_post_count=5,
            recent_submissions=[],
        )
    )

    assert "high_velocity" in result.signals
    assert result.risk_score >= 0.40


def test_moderation_off_platform_payment_patterns(mock_detector: ModerationDetector):
    """Mentions of off-platform payments or external contact should trigger signals."""
    suspicious_text = (
        "Do not apply here on the portal. Message me on telegram @campusdeal or "
        "whatsapp for direct cash wire transfer payment."
    )
    result = mock_detector.check(
        ModerationCheckInput(
            entity_type="job",
            entity_id="job-spam-1",
            text=suspicious_text,
            author_id="usr-author-4",
            author_recent_post_count=0,
            recent_submissions=[],
        )
    )

    assert any(
        s in ("off_platform_payment", "suspicious_contact") for s in result.signals
    )
    assert result.risk_score >= 0.50
    assert result.decision in ("review", "reject")


@pytest.mark.asyncio
async def test_moderation_api_unauthorized():
    """Missing or invalid X-Internal-AI-Secret should return 401."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/moderation/check",
            json={
                "entity_type": "job",
                "entity_id": "job-1",
                "text": "Valid job description for student developer.",
                "author_id": "usr-1",
            },
        )
        assert resp.status_code == 401


@pytest.mark.asyncio
async def test_moderation_api_success():
    """Valid request with internal secret should return 200 and valid ModerationResult."""
    settings = get_settings()
    headers = {"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET}
    payload = {
        "entity_type": "job",
        "entity_id": "job-api-1",
        "text": "Looking for research assistant in computational genomics with Python experience.",
        "author_id": "usr-prof-1",
        "author_recent_post_count": 0,
        "recent_submissions": [],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/moderation/check",
            json=payload,
            headers=headers,
        )
        assert resp.status_code == 200
        data = resp.json()
        assert "risk_score" in data
        assert "decision" in data
        assert data["decision"] == "allow"
        assert "signals" in data
        assert data["confidence"] >= 0.8
        assert data["pipeline_version"] == "moderation-v1"
