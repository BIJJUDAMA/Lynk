"""Unit and integration tests for review insights and aspect extraction."""

import pytest
from httpx import ASGITransport, AsyncClient

from ai.app.config import get_settings
from ai.app.main import app
from ai.pipelines.reviews.analyzer import (
    ReviewAspectInsight,
    ReviewAnalyzer,
    ReviewItemInput,
)


@pytest.fixture
def analyzer() -> ReviewAnalyzer:
    return ReviewAnalyzer()


def test_aspect_extraction_technical_and_timeliness(analyzer: ReviewAnalyzer):
    """Review mentioning 'superb code quality and great turnaround time' extracts high scores for technical_ability and timeliness."""
    reviews = [
        ReviewItemInput(
            rating=5,
            comment="Superb code quality and great turnaround time! Implemented the entire Next.js feature ahead of schedule.",
        )
    ]

    insights = analyzer.analyze(user_id="usr-123", reviews=reviews)
    aspect_map = {item.aspect: item for item in insights}

    # Verify technical_ability and timeliness are extracted
    assert "technical_ability" in aspect_map or "code_quality" in aspect_map
    assert "timeliness" in aspect_map

    tech_item = aspect_map.get("technical_ability") or aspect_map.get("code_quality")
    time_item = aspect_map["timeliness"]

    assert tech_item.score >= 4.0
    assert time_item.score >= 4.0

    # Single review must have is_recurring: false
    assert tech_item.is_recurring is False
    assert time_item.is_recurring is False
    assert tech_item.sample_count == 1
    assert time_item.sample_count == 1

    # Strengths should be populated
    assert len(tech_item.strengths) > 0 or len(time_item.strengths) > 0


def test_recurring_aspect_when_sample_count_greater_or_equal_3(analyzer: ReviewAnalyzer):
    """3+ consistent reviews for a user set is_recurring: true."""
    reviews = [
        ReviewItemInput(
            rating=5,
            comment="Exceptional turnaround time. Delivered the PR within two hours.",
        ),
        ReviewItemInput(
            rating=5,
            comment="Always on time and fast to respond. Turnaround time was unmatched.",
        ),
        ReviewItemInput(
            rating=5,
            comment="Great speed and turnaround time on the bugfix deliverable.",
        ),
    ]

    insights = analyzer.analyze(user_id="usr-recurring-1", reviews=reviews)
    aspect_map = {item.aspect: item for item in insights}

    assert "timeliness" in aspect_map
    time_item = aspect_map["timeliness"]

    assert time_item.sample_count >= 3
    assert time_item.is_recurring is True
    assert time_item.score >= 4.5


def test_negative_feedback_generates_improvements(analyzer: ReviewAnalyzer):
    """Lower rating reviews generate constructive improvement suggestions."""
    reviews = [
        ReviewItemInput(
            rating=2,
            comment="Delivery was very slow and missed deadlines. Communication was poor.",
        )
    ]

    insights = analyzer.analyze(user_id="usr-improv-1", reviews=reviews)
    aspect_map = {item.aspect: item for item in insights}

    assert "timeliness" in aspect_map or "communication" in aspect_map
    matched = [item for item in insights if item.improvements]
    assert len(matched) > 0


@pytest.mark.asyncio
async def test_reviews_api_unauthorized():
    """POST /internal/v1/reviews/analyze without secret returns 401."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/reviews/analyze",
            json={
                "user_id": "usr-test-1",
                "reviews": [{"rating": 5, "comment": "Great work!"}],
            },
        )
        assert resp.status_code == 401


@pytest.mark.asyncio
async def test_reviews_api_success():
    """POST /internal/v1/reviews/analyze with secret returns 200 and aspect insights."""
    settings = get_settings()
    headers = {"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET}
    payload = {
        "user_id": "usr-api-review-1",
        "reviews": [
            {
                "rating": 5,
                "comment": "Superb code quality and great turnaround time on our campus gig.",
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/reviews/analyze",
            json=payload,
            headers=headers,
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["user_id"] == "usr-api-review-1"
        assert "insights" in data
        assert len(data["insights"]) > 0
        assert data["sample_count"] == 1
        assert data["model_name"] == "lynk-review-analyzer"
        assert data["pipeline_version"] == "reviews-v1"
