"""Candidate Application Ranking Pipeline.

Computes advisory ranking scores for job applicants based strictly on observable,
non-protected features:
  1. Skill overlap score (weight: 0.45)
  2. Semantic embedding similarity (weight: 0.35)
  3. Domain department compatibility (weight: 0.20)

Strictly FAIR: Protected demographic attributes (gender, race, age, graduation year,
ethnicity) are strictly excluded from all features, schemas, and scoring calculations.
Traces every evaluation run in PostgreSQL ai_runs via track_ai_run.
"""

import asyncio
import logging
from typing import Any

from ai.app.middleware.run_tracker import track_ai_run
from ai.models.embeddings.provider import (
    BaseEmbeddingProvider,
    MockEmbeddingProvider,
    get_embedding_provider,
)
from ai.pipelines.skills.normalizer import SkillNormalizer, get_skill_normalizer
from pydantic import BaseModel, ConfigDict, Field, field_validator

logger = logging.getLogger("lynk-ai.ranking")

STEM_KEYWORDS = {
    "computer science",
    "software engineering",
    "data science",
    "information technology",
    "computer engineering",
    "electrical engineering",
    "mathematics",
    "statistics",
    "physics",
    "engineering",
    "mechanical engineering",
    "cybersecurity",
}

BUSINESS_KEYWORDS = {
    "business",
    "business administration",
    "finance",
    "marketing",
    "accounting",
    "management",
    "economics",
    "entrepreneurship",
}

DESIGN_ARTS_KEYWORDS = {
    "design",
    "graphic design",
    "digital media",
    "fine arts",
    "art",
    "ui/ux",
    "communications",
    "media",
    "journalism",
    "english",
    "humanities",
}


class CandidateRankInput(BaseModel):
    """Observable, non-protected input features for a single candidate application.

    Protected demographic attributes (gender, race, age, graduation year, ethnicity)
    are strictly excluded.
    """

    model_config = ConfigDict(extra="ignore")

    application_id: str = Field(..., description="Application unique identifier")
    skills: list[str] | None = Field(
        default_factory=list, description="Candidate declared skills"
    )
    bio: str | None = Field(default=None, description="Candidate bio text")
    department: str | None = Field(
        default=None, description="Academic department or major"
    )
    cover_letter: str | None = Field(
        default=None, description="Application cover letter"
    )

    @field_validator("skills", mode="before")
    @classmethod
    def _coerce_skills(cls, v: Any) -> list[str]:
        if v is None:
            return []
        return v


class CandidateRankResult(BaseModel):
    """Ranked candidate evaluation result with transparent, observable explanation."""

    application_id: str
    score: float
    matched_skills: list[str] = Field(default_factory=list)
    missing_skills: list[str] = Field(default_factory=list)
    reason: str
    confidence: float


class CandidateRanker:
    """Orchestrates candidate scoring, ranking, and transparent explanation generation."""

    def __init__(
        self,
        embedding_provider: BaseEmbeddingProvider | None = None,
        skill_normalizer: SkillNormalizer | None = None,
        db_pool: Any | None = None,
    ) -> None:
        self.embedding_provider = embedding_provider or get_embedding_provider()
        self.skill_normalizer = skill_normalizer or get_skill_normalizer()
        self.db_pool = db_pool
        self.model_name = "lynk-candidate-ranker"
        self.model_version = "1.0.0"
        self.pipeline_version = "ranking-v1"

    def _compute_department_compatibility(
        self, job_dept: str | None, cand_dept: str | None
    ) -> float:
        """Compute compatibility score between job department and candidate department."""
        j_dept = (job_dept or "").strip().lower()
        c_dept = (cand_dept or "").strip().lower()

        if not j_dept or not c_dept:
            return 0.5

        if j_dept == c_dept:
            return 1.0

        # Check cluster compatibility
        if (
            (j_dept in STEM_KEYWORDS and c_dept in STEM_KEYWORDS)
            or (j_dept in BUSINESS_KEYWORDS and c_dept in BUSINESS_KEYWORDS)
            or (j_dept in DESIGN_ARTS_KEYWORDS and c_dept in DESIGN_ARTS_KEYWORDS)
        ):
            return 1.0

        # Partial keyword matching in department names (e.g. "engineering", "computer", "art")
        overlap_keywords = [
            "computer",
            "engineer",
            "data",
            "math",
            "business",
            "design",
            "art",
            "media",
        ]
        if any(w in j_dept and w in c_dept for w in overlap_keywords):
            return 1.0

        # General STEM / Arts background
        if (
            c_dept in STEM_KEYWORDS
            or c_dept in DESIGN_ARTS_KEYWORDS
            or c_dept in BUSINESS_KEYWORDS
            or any(
                w in c_dept
                for w in ["stem", "science", "arts", "engineering", "liberal arts"]
            )
        ):
            return 0.5

        return 0.2

    def _compute_semantic_similarity(
        self,
        job_text: str,
        cand_text: str,
        job_vec: list[float],
        cand_vec: list[float],
    ) -> float:
        """Compute cosine similarity of normalized embeddings with mock fallback resilience."""
        if not job_text.strip() or not cand_text.strip():
            return 0.0

        # Dot product of unit vectors equals cosine similarity
        cos_sim = sum(a * b for a, b in zip(job_vec, cand_vec))
        cos_sim = max(0.0, min(1.0, cos_sim))

        # When running in mock or fallback embedding mode, blend with token overlap proxy
        is_mock = getattr(self.embedding_provider, "is_fallback", False) or isinstance(
            self.embedding_provider, MockEmbeddingProvider
        )
        if is_mock:
            stop_words = {
                "a",
                "an",
                "the",
                "in",
                "on",
                "of",
                "and",
                "or",
                "to",
                "for",
                "with",
                "we",
                "need",
                "i",
                "have",
                "would",
                "like",
                "is",
                "are",
                "this",
                "that",
            }
            job_tokens = {
                w.lower().strip(".,!?:;\"'()") for w in job_text.split()
            } - stop_words
            job_tokens = {w for w in job_tokens if len(w) > 1}
            cand_tokens = {
                w.lower().strip(".,!?:;\"'()") for w in cand_text.split()
            } - stop_words
            cand_tokens = {w for w in cand_tokens if len(w) > 1}

            if job_tokens and cand_tokens:
                overlap = len(job_tokens & cand_tokens) / len(job_tokens)
                token_sim = min(1.0, overlap * 1.5)
                return max(cos_sim, round(token_sim, 4))

        return round(cos_sim, 4)

    async def rank_candidates(
        self,
        job_id: str,
        job_title: str,
        job_description: str,
        job_department: str,
        required_skills: list[str],
        candidates: list[CandidateRankInput],
    ) -> list[CandidateRankResult]:
        """Score and rank candidates based strictly on observable features."""
        if not candidates:
            return []

        async with track_ai_run(
            feature="application_ranking",
            entity_type="job",
            entity_id=job_id,
            model_name=self.model_name,
            model_version=self.model_version,
            pipeline_version=self.pipeline_version,
            input_data={
                "job_id": job_id,
                "job_title": job_title,
                "required_skills": required_skills,
                "candidate_count": len(candidates),
            },
            db_pool=self.db_pool,
        ) as tracker:
            # 1. Embed job text
            job_text = f"{job_title} {job_description}".strip()
            job_vec = (
                await asyncio.to_thread(self.embedding_provider.embed, job_text)
                if job_text
                else []
            )

            # 2. Canonicalize required skills for robust matching
            normalized_req_skills: list[str] = []
            for s in required_skills:
                norm = self.skill_normalizer.normalize(s)
                normalized_req_skills.append(norm.canonical_name.lower())

            # Pre-compute candidate text embeddings in batch
            cand_texts = [
                f"{c.bio or ''} {c.cover_letter or ''}".strip() for c in candidates
            ]
            non_empty_indices = [i for i, t in enumerate(cand_texts) if t]
            cand_vec_map: dict[int, list[float]] = {}
            if non_empty_indices and job_vec:
                batch_texts = [cand_texts[i] for i in non_empty_indices]
                batch_vecs = await asyncio.to_thread(
                    self.embedding_provider.embed_batch, batch_texts
                )
                for idx, vec in zip(non_empty_indices, batch_vecs):
                    cand_vec_map[idx] = vec

            results: list[CandidateRankResult] = []

            for i, cand in enumerate(candidates):
                # Feature 1: Skill overlap score (weight: 0.45)
                cand_skill_norms: set[str] = set()
                for cs in cand.skills:
                    cand_skill_norms.add(
                        self.skill_normalizer.normalize(cs).canonical_name.lower()
                    )
                    cand_skill_norms.add(cs.strip().lower())

                matched_skills: list[str] = []
                missing_skills: list[str] = []

                for orig_req, norm_req in zip(required_skills, normalized_req_skills):
                    if (
                        norm_req in cand_skill_norms
                        or orig_req.strip().lower() in cand_skill_norms
                    ):
                        matched_skills.append(orig_req)
                    else:
                        missing_skills.append(orig_req)

                if required_skills:
                    skill_overlap = len(matched_skills) / len(required_skills)
                else:
                    skill_overlap = 1.0

                # Feature 2: Semantic embedding similarity (weight: 0.35)
                cand_vec = cand_vec_map.get(i)
                if cand_vec and job_vec:
                    semantic_sim = self._compute_semantic_similarity(
                        job_text, cand_texts[i], job_vec, cand_vec
                    )
                else:
                    semantic_sim = 0.0

                # Feature 3: Domain department compatibility (weight: 0.20)
                dept_compat = self._compute_department_compatibility(
                    job_department, cand.department
                )

                # Composite score calculation (0 to 100 scale)
                composite = (
                    0.45 * skill_overlap + 0.35 * semantic_sim + 0.20 * dept_compat
                )
                final_score = round(max(0.0, min(100.0, composite * 100)), 1)

                # Verifiable, transparent explanation citing weights
                reason = (
                    f"Matched skills ({len(matched_skills)}/{len(required_skills)}): "
                    f"{', '.join(matched_skills) if matched_skills else 'none'}. "
                    f"Missing skills: {', '.join(missing_skills) if missing_skills else 'none'}. "
                    f"Observable breakdown: skills={round(skill_overlap * 100, 1)}%, "
                    f"semantic_similarity={round(semantic_sim * 100, 1)}%, "
                    f"department_compatibility={round(dept_compat * 100, 1)}%."
                )

                confidence = round(
                    0.85
                    + (0.10 if len(matched_skills) > 0 else 0.0)
                    + (0.05 if cand.cover_letter else 0.0),
                    2,
                )

                results.append(
                    CandidateRankResult(
                        application_id=cand.application_id,
                        score=final_score,
                        matched_skills=matched_skills,
                        missing_skills=missing_skills,
                        reason=reason,
                        confidence=confidence,
                    )
                )

            # Rank descending by score, then confidence
            results.sort(key=lambda r: (r.score, r.confidence), reverse=True)

            tracker.set_output(
                {
                    "ranked_count": len(results),
                    "top_score": results[0].score if results else 0,
                }
            )
            return results


_ranker_instance: CandidateRanker | None = None


def get_candidate_ranker(db_pool: Any | None = None) -> CandidateRanker:
    """Acquire or initialize CandidateRanker singleton."""
    global _ranker_instance
    if _ranker_instance is None:
        _ranker_instance = CandidateRanker(db_pool=db_pool)
    elif db_pool is not None and _ranker_instance.db_pool is None:
        _ranker_instance.db_pool = db_pool
    return _ranker_instance
