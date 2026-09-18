"""Internal API routes for Generative AI Job Draft Generation."""

from fastapi import APIRouter, Request
from pydantic import BaseModel, Field

from ai.llm.client import get_vllm_client
from ai.llm.schemas.job_generation import GeneratedJobDraft

router = APIRouter(prefix="/internal/v1/jobs", tags=["jobs"])


class GenerateJobDraftRequest(BaseModel):
    """Request payload for generating a job posting draft from a rough idea."""

    idea: str = Field(
        ..., min_length=3, max_length=1000, description="Rough campus project idea or requirement"
    )
    department: str = Field(
        default="", max_length=100, description="Optional academic department"
    )


class GenerateJobDraftResponse(BaseModel):
    """Response payload containing the structured draft and provenance metadata."""

    draft: GeneratedJobDraft = Field(..., description="Structured job draft")
    model_name: str = Field(..., description="Model identifier used for generation")
    model_version: str = Field(..., description="Model version")
    prompt_version: str = Field(..., description="Prompt template version")
    pipeline_version: str = Field(..., description="Pipeline version")


@router.post("/generate", response_model=GenerateJobDraftResponse)
async def generate_job_draft_endpoint(
    payload: GenerateJobDraftRequest, request: Request
) -> GenerateJobDraftResponse:
    """Generate a structured job posting draft from an unstructured idea."""
    pool = getattr(request.app.state, "db_pool", None)
    client = get_vllm_client(db_pool=pool)

    draft = await client.generate_job_draft(
        idea=payload.idea, department=payload.department
    )

    return GenerateJobDraftResponse(
        draft=draft,
        model_name=client.model_name,
        model_version=client.model_version,
        prompt_version=client.prompt_version,
        pipeline_version=client.pipeline_version,
    )
