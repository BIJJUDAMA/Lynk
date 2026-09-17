"""Internal API routes for Skill Demand Analytics and Forecasting."""

from fastapi import APIRouter, Request
from pydantic import BaseModel, Field

from ai.pipelines.forecasting.demand import (
    SkillDemandForecast,
    SkillDemandForecaster,
    SkillDemandSnapshot,
    SkillHistoricalData,
)

router = APIRouter(prefix="/internal/v1/analytics", tags=["analytics"])


class SkillDemandAnalyticsRequest(BaseModel):
    """Request payload for skill demand analytics and forecasting."""

    period: str = Field(default="monthly", description="Aggregation period (e.g. monthly, weekly)")
    skills_data: list[SkillHistoricalData] = Field(
        default_factory=list, description="Historical posting points per skill"
    )


class SkillDemandAnalyticsResponse(BaseModel):
    """Response payload with aggregated snapshots and forward-looking forecasts."""

    period: str = Field(..., description="Aggregation period")
    snapshots: list[SkillDemandSnapshot] = Field(
        default_factory=list, description="Aggregated demand snapshots"
    )
    forecasts: list[SkillDemandForecast] = Field(
        default_factory=list, description="Forward-looking demand projections"
    )
    model_name: str = Field(..., description="Forecaster model name")
    model_version: str = Field(..., description="Forecaster model version")
    pipeline_version: str = Field(..., description="Pipeline version")


@router.post("/skills", response_model=SkillDemandAnalyticsResponse)
async def analyze_skill_demand_endpoint(
    payload: SkillDemandAnalyticsRequest, request: Request
) -> SkillDemandAnalyticsResponse:
    """Compute demand snapshots and forecasts for specified skills."""
    pool = getattr(request.app.state, "db_pool", None)
    forecaster = SkillDemandForecaster(db_pool=pool)

    snapshots, forecasts = await forecaster.analyze_and_record(
        period=payload.period, skills_data=payload.skills_data
    )

    return SkillDemandAnalyticsResponse(
        period=payload.period,
        snapshots=snapshots,
        forecasts=forecasts,
        model_name=forecaster.model_name,
        model_version=forecaster.model_version,
        pipeline_version=forecaster.pipeline_version,
    )
