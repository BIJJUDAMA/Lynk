"""Internal API routes for skill normalization and text extraction."""

from typing import Optional
from fastapi import APIRouter
from pydantic import BaseModel, Field

from ai.pipelines.skills.normalizer import get_skill_normalizer

router = APIRouter(prefix="/internal/v1/skills", tags=["skills"])


class NormalizeSkillRequest(BaseModel):
    skill: str = Field(..., description="Unstructured skill string to normalize")


class NormalizeSkillResponse(BaseModel):
    original_skill: str = Field(..., description="Original input skill string")
    canonical_name: str = Field(..., description="Normalized canonical skill name")
    confidence: float = Field(..., description="Match confidence score between 0.0 and 1.0")
    match_method: str = Field(
        ...,
        description="Normalization stage: exact_alias, fuzzy, embedding, or fallback",
    )
    category: Optional[str] = Field(
        default=None, description="Domain category (e.g. Frontend, Data & AI)"
    )
    skill_id: Optional[str] = Field(
        default=None, description="Unique slug or identifier for canonical skill"
    )


class ExtractSkillsRequest(BaseModel):
    text: str = Field(
        ..., description="Freeform text such as job description or student bio"
    )


class ExtractSkillsResponse(BaseModel):
    extracted_skills: list[NormalizeSkillResponse] = Field(
        default_factory=list,
        description="List of extracted and normalized canonical skills",
    )


@router.post("/normalize", response_model=NormalizeSkillResponse)
async def normalize_skill(payload: NormalizeSkillRequest) -> NormalizeSkillResponse:
    """Normalize an individual skill string to canonical taxonomy."""
    normalizer = get_skill_normalizer()
    res = normalizer.normalize(payload.skill)
    return NormalizeSkillResponse(
        original_skill=res.original_skill,
        canonical_name=res.canonical_name,
        confidence=res.confidence,
        match_method=res.match_method,
        category=res.category,
        skill_id=res.skill_id,
    )


@router.post("/extract", response_model=ExtractSkillsResponse)
async def extract_skills(payload: ExtractSkillsRequest) -> ExtractSkillsResponse:
    """Extract and normalize canonical skills from freeform unstructured text."""
    normalizer = get_skill_normalizer()
    results = normalizer.extract_skills_from_text(payload.text)
    return ExtractSkillsResponse(
        extracted_skills=[
            NormalizeSkillResponse(
                original_skill=s.original_skill,
                canonical_name=s.canonical_name,
                confidence=s.confidence,
                match_method=s.match_method,
                category=s.category,
                skill_id=s.skill_id,
            )
            for s in results
        ]
    )
