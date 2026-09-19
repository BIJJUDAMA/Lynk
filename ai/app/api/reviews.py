"""Internal API routes for Review Insights and Aspect Extraction."""

from ai.pipelines.reviews.analyzer import (
    ReviewAspectInsight,
    ReviewItemInput,
    get_review_analyzer,
)
from fastapi import APIRouter, Request
from pydantic import BaseModel, Field

router = APIRouter(prefix="/internal/v1/reviews", tags=["reviews"])


class AnalyzeReviewsRequest(BaseModel):
    """Request payload for extracting aspect insights from member reviews."""

    user_id: str = Field(..., description="Target campus member user identifier")
    reviews: list[ReviewItemInput] = Field(
        default_factory=list, description="List of completed reviews to analyze"
    )


class AnalyzeReviewsResponse(BaseModel):
    """Response payload containing aspect insights and recurring reputation patterns."""

    user_id: str = Field(..., description="Target campus member user identifier")
    insights: list[ReviewAspectInsight] = Field(
        default_factory=list, description="List of collaboration aspect evaluations"
    )
    sample_count: int = Field(..., description="Total reviews analyzed")
    model_name: str = Field(..., description="Analyzer model name")
    model_version: str = Field(..., description="Analyzer model version")
    pipeline_version: str = Field(..., description="Pipeline semantic version")


@router.post("/analyze", response_model=AnalyzeReviewsResponse)
async def analyze_reviews_endpoint(
    payload: AnalyzeReviewsRequest, request: Request
) -> AnalyzeReviewsResponse:
    """Analyze completed peer reviews and extract aspect insights and recurring patterns."""
    pool = getattr(request.app.state, "db_pool", None)
    analyzer = get_review_analyzer(db_pool=pool)

    insights = await analyzer.analyze_and_record(
        user_id=payload.user_id, reviews=payload.reviews
    )

    return AnalyzeReviewsResponse(
        user_id=payload.user_id,
        insights=insights,
        sample_count=len(payload.reviews),
        model_name=analyzer.model_name,
        model_version=analyzer.model_version,
        pipeline_version=analyzer.pipeline_version,
    )
