"""Unit and integration tests for skill demand analytics and forecasting."""

import pytest
from httpx import ASGITransport, AsyncClient

from ai.app.config import get_settings
from ai.app.main import app
from ai.pipelines.forecasting.demand import (
    SkillDemandForecaster,
    SkillHistoricalData,
    SkillPostingPoint,
)


@pytest.fixture
def forecaster() -> SkillDemandForecaster:
    return SkillDemandForecaster()


def test_upward_trend_projects_positive_growth_rate(forecaster: SkillDemandForecaster):
    """Given synthetic time series with upward job postings, forecaster projects positive growth rate."""
    data = SkillHistoricalData(
        skill="Python",
        postings=[
            SkillPostingPoint(period="2026-W01", job_count=2, application_count=6, unique_posters=2),
            SkillPostingPoint(period="2026-W02", job_count=4, application_count=12, unique_posters=3),
            SkillPostingPoint(period="2026-W03", job_count=7, application_count=20, unique_posters=5),
            SkillPostingPoint(period="2026-W04", job_count=11, application_count=35, unique_posters=8),
        ],
    )

    forecast = forecaster.forecast_skill_demand(data)

    assert forecast.skill == "Python"
    assert forecast.growth_rate > 0.0
    assert forecast.demand_score > 50.0
    assert forecast.projected_30d_demand > 11.0


def test_aggregation_computes_unique_posters_and_ratio(forecaster: SkillDemandForecaster):
    """Forecaster accurately computes unique posters and application-to-job ratio."""
    data = SkillHistoricalData(
        skill="React",
        postings=[
            SkillPostingPoint(period="2026-M01", job_count=5, application_count=25, unique_posters=4),
            SkillPostingPoint(period="2026-M02", job_count=5, application_count=30, unique_posters=5),
        ],
    )

    snapshot = forecaster.create_snapshot(data, period="monthly")

    assert snapshot.skill == "React"
    assert snapshot.job_count == 10
    assert snapshot.application_count == 55
    assert snapshot.unique_posters == 5  # max or union across points
    assert snapshot.app_to_job_ratio == 5.5
    assert 0.0 <= snapshot.demand_score <= 100.0


@pytest.mark.asyncio
async def test_analytics_api_unauthorized():
    """POST /internal/v1/analytics/skills without secret returns 401."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/analytics/skills",
            json={"skills": ["Python", "Go"], "period": "monthly"},
        )
        assert resp.status_code == 401


@pytest.mark.asyncio
async def test_analytics_api_success():
    """POST /internal/v1/analytics/skills with secret returns 200 and demand metrics."""
    settings = get_settings()
    headers = {"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET}
    payload = {
        "period": "monthly",
        "skills_data": [
            {
                "skill": "TypeScript",
                "postings": [
                    {
                        "period": "2026-01",
                        "job_count": 3,
                        "application_count": 15,
                        "unique_posters": 3,
                    },
                    {
                        "period": "2026-02",
                        "job_count": 6,
                        "application_count": 28,
                        "unique_posters": 5,
                    },
                ],
            }
        ],
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/analytics/skills",
            json=payload,
            headers=headers,
        )
        assert resp.status_code == 200
        data = resp.json()
        assert "snapshots" in data
        assert len(data["snapshots"]) == 1
        assert data["snapshots"][0]["skill"] == "TypeScript"
        assert "forecasts" in data
        assert len(data["forecasts"]) == 1
        assert data["forecasts"][0]["growth_rate"] > 0.0
        assert data["pipeline_version"] == "forecasting-v1"
