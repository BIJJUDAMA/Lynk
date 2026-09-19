"""Internal API routes for Candidate Application Ranking."""

from ai.pipelines.ranking.ranker import (
    CandidateRankInput,
    CandidateRankResult,
    get_candidate_ranker,
)
from fastapi import APIRouter, Request
from pydantic import BaseModel, Field

router = APIRouter(prefix="/internal/v1/ranking", tags=["ranking"])


class RankCandidatesRequest(BaseModel):
    """Request payload for ranking candidate applications for a job posting."""

    job_id: str = Field(..., description="Unique job posting identifier")
    job_title: str = Field(..., description="Job posting title")
    job_description: str = Field(
        default="", description="Job posting detailed description"
    )
    job_department: str = Field(default="", description="Academic department or field")
    required_skills: list[str] = Field(
        default_factory=list, description="List of required skills"
    )
    candidates: list[CandidateRankInput] = Field(
        default_factory=list, description="List of applicant candidate inputs"
    )
    pipeline_version: str = Field(
        default="ranking-v1", description="Ranking pipeline version"
    )


class RankCandidatesResponse(BaseModel):
    """Response payload containing ranked candidates and metadata."""

    results: list[CandidateRankResult] = Field(
        default_factory=list, description="Ranked candidate evaluation results"
    )
    model_name: str = Field(..., description="Model identifier")
    model_version: str = Field(..., description="Model semantic version")
    pipeline_version: str = Field(..., description="Pipeline semantic version")


@router.post("/candidates", response_model=RankCandidatesResponse)
async def rank_candidates_endpoint(
    payload: RankCandidatesRequest, request: Request
) -> RankCandidatesResponse:
    """Score and rank candidate applications using observable features."""
    pool = getattr(request.app.state, "db_pool", None)
    ranker = get_candidate_ranker(db_pool=pool)

    results = await ranker.rank_candidates(
        job_id=payload.job_id,
        job_title=payload.job_title,
        job_description=payload.job_description,
        job_department=payload.job_department,
        required_skills=payload.required_skills,
        candidates=payload.candidates,
    )

    return RankCandidatesResponse(
        results=results,
        model_name=ranker.model_name,
        model_version=ranker.model_version,
        pipeline_version=payload.pipeline_version or ranker.pipeline_version,
    )
