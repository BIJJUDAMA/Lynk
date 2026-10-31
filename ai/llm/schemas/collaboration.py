"""Schemas for the 4-Pillar Collaboration LLM Client.

100% pure 7-bit ASCII
"""

from pydantic import BaseModel, Field


class ParsedNaturalQuery(BaseModel):
    """Structured search filters parsed from conversational student text."""

    raw_query: str
    intent: str = Field(
        default="find_project", description="'find_teammate' or 'find_project'"
    )
    extracted_skills: list[str] = Field(default_factory=list)
    preferred_department: str = Field(default="Any")


class ProjectRole(BaseModel):
    """An open collaborator role needed for a project."""

    role_title: str
    required_skills: list[str]
    description: str


class GeneratedProjectSpec(BaseModel):
    """Structured collaborative project listing created from a rough student thought."""

    title: str
    overview: str
    category: str
    needed_roles: list[ProjectRole]
    deliverables: list[str]
    estimated_duration_weeks: int = 4


class MilestoneTask(BaseModel):
    """A discrete task inside a milestone phase."""

    title: str
    estimated_hours: int = 10
    suggested_skill: str = "General"


class ProjectMilestone(BaseModel):
    """A sprint or phase milestone for project execution."""

    name: str
    duration_weeks: int
    tasks: list[MilestoneTask]


class ProjectMilestonePlan(BaseModel):
    """Full 3-stage roadmap for executing a student venture."""

    project_title: str
    milestones: list[ProjectMilestone]
