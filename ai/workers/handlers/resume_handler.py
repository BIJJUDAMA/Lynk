"""Asynchronous handler for parsing resumes and extracting tagged student skills."""

import logging
from typing import Any, Optional

from ai.pipelines.skills.normalizer import SkillNormalizer, get_skill_normalizer

logger = logging.getLogger("ai.workers.resume")


class ResumeJobHandler:
    """Processes uploaded resume text, extracts canonical skills, and tags source provenance."""

    def __init__(
        self,
        normalizer: Optional[SkillNormalizer] = None,
        db_pool=None,
    ) -> None:
        self.normalizer = normalizer or get_skill_normalizer()
        self.db_pool = db_pool

    async def handle(self, job: Any) -> dict[str, Any]:
        """Extract canonical skills from resume text and tag provenance as ai_resume_parser."""
        payload = job.payload or {}
        resume_text = payload.get("resume_text", "")

        if not resume_text:
            raise ValueError("Empty resume_text payload for resume parsing")

        extracted = self.normalizer.extract_skills_from_text(resume_text)

        tagged_skills = [
            {
                "skill": item.canonical_name,
                "confidence": item.confidence,
                "match_method": item.match_method,
                "category": item.category,
                "source": "ai_resume_parser",
            }
            for item in extracted
        ]

        if self.db_pool is not None and job.entity_id:
            try:
                async with self.db_pool.acquire() as conn:
                    # Insert extracted skills into profile_skills if member profile exists
                    for s in tagged_skills:
                        await conn.execute(
                            """
                            INSERT INTO profile_skills (user_id, skill_id, confidence, source, updated_at)
                            SELECT $1, s.id, $3, 'ai_resume_parser', NOW()
                            FROM skills s
                            WHERE s.canonical_name = $2
                            ON CONFLICT (user_id, skill_id) DO UPDATE SET
                                confidence = EXCLUDED.confidence,
                                source = EXCLUDED.source,
                                updated_at = NOW();
                            """,
                            job.entity_id,
                            s["skill"],
                            s["confidence"],
                        )
            except Exception as exc:
                logger.warning("Failed to link extracted resume skills to profile_skills: %s", exc)

        return {
            "extracted_skills": tagged_skills,
            "count": len(tagged_skills),
            "parser_version": "1.0.0",
        }
