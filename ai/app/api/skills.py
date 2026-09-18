"""Internal API routes for skill normalization and text extraction."""

from typing import Optional
from fastapi import APIRouter, Request
from pydantic import BaseModel, Field

from ai.app.middleware.run_tracker import track_ai_run
from ai.pipelines.skills.normalizer import get_skill_normalizer

router = APIRouter(prefix="/internal/v1/skills", tags=["skills"])


class NormalizeSkillRequest(BaseModel):
    skill: str = Field(
        ..., min_length=1, max_length=200, description="Unstructured skill string to normalize"
    )


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
        ..., min_length=1, max_length=10000, description="Freeform text such as job description or student bio"
    )


class ExtractSkillsResponse(BaseModel):
    extracted_skills: list[NormalizeSkillResponse] = Field(
        default_factory=list,
        description="List of extracted and normalized canonical skills",
    )


@router.post("/normalize", response_model=NormalizeSkillResponse)
async def normalize_skill(
    payload: NormalizeSkillRequest, request: Request
) -> NormalizeSkillResponse:
    """Normalize an individual skill string to canonical taxonomy."""
    pool = getattr(request.app.state, "db_pool", None)
    async with track_ai_run(
        feature="skill_normalization",
        entity_type="skill",
        entity_id=payload.skill[:64],
        model_name="lynk-skill-normalizer",
        model_version="1.0.0",
        pipeline_version="skills-v1",
        input_data={"skill": payload.skill},
        db_pool=pool,
    ) as tracker:
        normalizer = get_skill_normalizer()
        res = normalizer.normalize(payload.skill)
        tracker.set_output(
            {
                "original_skill": res.original_skill,
                "canonical_name": res.canonical_name,
                "confidence": res.confidence,
                "match_method": res.match_method,
                "category": res.category,
                "skill_id": res.skill_id,
            }
        )
        tracker.set_confidence(res.confidence)
        return NormalizeSkillResponse(
            original_skill=res.original_skill,
            canonical_name=res.canonical_name,
            confidence=res.confidence,
            match_method=res.match_method,
            category=res.category,
            skill_id=res.skill_id,
        )


@router.post("/extract", response_model=ExtractSkillsResponse)
async def extract_skills(
    payload: ExtractSkillsRequest, request: Request
) -> ExtractSkillsResponse:
    """Extract and normalize canonical skills from freeform unstructured text."""
    pool = getattr(request.app.state, "db_pool", None)
    async with track_ai_run(
        feature="skill_extraction",
        entity_type="text",
        entity_id=payload.text[:64],
        model_name="lynk-skill-normalizer",
        model_version="1.0.0",
        pipeline_version="skills-v1",
        input_data={"text": payload.text[:500]},
        db_pool=pool,
    ) as tracker:
        normalizer = get_skill_normalizer()
        results = normalizer.extract_skills_from_text(payload.text)
        tracker.set_output({"extracted_count": len(results)})
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
