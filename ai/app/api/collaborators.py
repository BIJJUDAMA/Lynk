# 100% pure 7-bit ASCII
from typing import Any

from fastapi import APIRouter
from pydantic import BaseModel, Field

from ai.pipelines.collaborators.matcher import TeammateMatcher
from ai.pipelines.skills.gap_analyzer import SkillGapAnalyzer, SkillGapResult

router = APIRouter(prefix="/internal/v1", tags=["collaborators"])
matcher = TeammateMatcher()
gap_analyzer = SkillGapAnalyzer()


class MatchRequest(BaseModel):
    team_skills: list[str] = Field(default_factory=list)
    project_missing_skills: list[str] = Field(default_factory=list)
    candidates: list[dict[str, Any]] = Field(default_factory=list)
    limit: int = 10


class GapRequest(BaseModel):
    student_skills: list[str] = Field(default_factory=list)
    project_required_skills: list[str] = Field(default_factory=list)


@router.post("/collaborators/match")
async def match_collaborators(payload: MatchRequest):
    matches = matcher.find_complementary_peers(
        team_skills=payload.team_skills,
        project_missing_skills=payload.project_missing_skills,
        candidates=payload.candidates,
        limit=payload.limit,
    )
    return {"matches": matches}


@router.post("/skills/gap", response_model=SkillGapResult)
@router.post("/collaborators/skill-gap", response_model=SkillGapResult)
async def evaluate_skill_gap(payload: GapRequest):
    return gap_analyzer.analyze_gap(
        student_skills=payload.student_skills,
        project_required_skills=payload.project_required_skills,
    )
