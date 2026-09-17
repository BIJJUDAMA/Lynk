"""Internal API routes for Member Recommendations."""

from typing import Optional
from fastapi import APIRouter, Request
from pydantic import BaseModel, Field

from ai.pipelines.recommendations.engine import (
    RecommendationEngine,
    RecommendationItem,
    get_recommendation_engine,
)

router = APIRouter(prefix="/internal/v1/recommendations", tags=["recommendations"])


class ProfileRecommendationsRequest(BaseModel):
    """Request payload for profile recommendations."""

    user_id: str = Field(..., description="Unique member user identifier")
    current_skills: list[str] = Field(
        default_factory=list, description="Currently declared skills"
    )
    department: Optional[str] = Field(
        default=None, description="Member's academic department or major"
    )
    bio: Optional[str] = Field(default=None, description="Member's profile bio text")
    portfolio_links: list[str] = Field(
        default_factory=list, description="External portfolio or repository URLs"
    )
    limit: int = Field(default=10, ge=1, le=50, description="Max recommendations to return")


class ProfileRecommendationsResponse(BaseModel):
    """Response payload containing generated recommendations and metadata."""

    recommendations: list[RecommendationItem] = Field(
        default_factory=list, description="Ranked list of recommendation items"
    )
    user_id: str = Field(..., description="Member user identifier")
    model_name: str = Field(..., description="Engine model name")
    model_version: str = Field(..., description="Engine version")


@router.post("/profile", response_model=ProfileRecommendationsResponse)
async def get_profile_recommendations(
    payload: ProfileRecommendationsRequest, request: Request
) -> ProfileRecommendationsResponse:
    """Generate recommendations for a member's profile."""
    pool = getattr(request.app.state, "db_pool", None)
    engine = get_recommendation_engine(db_pool=pool)

    items = await engine.generate_recommendations(
        user_id=payload.user_id,
        current_skills=payload.current_skills,
        department=payload.department,
        bio=payload.bio,
        portfolio_links=payload.portfolio_links,
        limit=payload.limit,
    )

    return ProfileRecommendationsResponse(
        recommendations=items,
        user_id=payload.user_id,
        model_name=engine.model_name,
        model_version=engine.model_version,
    )
