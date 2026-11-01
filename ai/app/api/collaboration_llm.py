# 100% pure 7-bit ASCII
from typing import Any

from fastapi import APIRouter
from pydantic import BaseModel, Field

from ai.llm.collaboration_client import CollaborationLLMClient
from ai.llm.schemas.collaboration import (
    GeneratedProjectSpec,
    ParsedNaturalQuery,
    ProjectMilestonePlan,
)

router = APIRouter(prefix="/internal/v1", tags=["collaboration_llm"])
client = CollaborationLLMClient()


class NaturalQueryRequest(BaseModel):
    query: str = Field(..., min_length=2)


class ProjectAssistRequest(BaseModel):
    idea: str = Field(..., min_length=3)
    category: str = Field(default="Software")


class MilestonesRequest(BaseModel):
    title: str
    deliverables: list[str]


class RoleAllocationRequest(BaseModel):
    milestone_tasks: list[str]
    teammates: list[dict[str, Any]]


@router.post("/search/natural", response_model=ParsedNaturalQuery)
async def natural_search(payload: NaturalQueryRequest):
    return await client.parse_natural_query(payload.query)


@router.post("/projects/assist", response_model=GeneratedProjectSpec)
async def assist_project(payload: ProjectAssistRequest):
    return await client.create_project_spec(payload.idea, payload.category)


@router.post("/projects/milestones", response_model=ProjectMilestonePlan)
@router.post("/collaboration-llm/milestones", response_model=ProjectMilestonePlan)
async def plan_milestones(payload: MilestonesRequest):
    return await client.plan_project_milestones(payload.title, payload.deliverables)


@router.post("/projects/roles")
async def allocate_roles(payload: RoleAllocationRequest):
    allocations = await client.suggest_role_allocations(
        milestone_tasks=payload.milestone_tasks,
        teammates=payload.teammates,
    )
    return {"allocations": allocations}
