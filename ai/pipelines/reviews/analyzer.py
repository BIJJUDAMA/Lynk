"""Review Insights and Aspect Extraction Pipeline.

Extracts canonical collaboration aspects, observable strengths, constructive
improvement recommendations, and recurring reputation patterns from peer reviews.
Aspects evaluated:
  1. technical_ability
  2. timeliness
  3. communication
  4. reliability
  5. code_quality
  6. problem_solving
  7. teamwork

Strictly enforces recurrence threshold: is_recurring is set to True ONLY when
sample_count >= 3.
"""

import logging
import re
from typing import Any, Optional
from pydantic import BaseModel, ConfigDict, Field

from ai.app.middleware.run_tracker import track_ai_run

logger = logging.getLogger("lynk-ai.reviews")

# Lexicon mappings for collaboration aspects
ASPECT_PATTERNS: dict[str, list[str]] = {
    "technical_ability": [
        r"\bcode\b",
        r"\bprogramming\b",
        r"\btechnical\b",
        r"\barchitecture\b",
        r"\bbackend\b",
        r"\bfrontend\b",
        r"\bdatabase\b",
        r"\bapi\b",
        r"\bimplementation\b",
        r"\balgorithm\b",
        r"\bnext\.?js\b",
        r"\breact\b",
        r"\bpython\b",
        r"\bgo\b",
    ],
    "timeliness": [
        r"\bturnaround\s+time\b",
        r"\bturnaround\b",
        r"\bdeadline\b",
        r"\bfast\b",
        r"\bquick\b",
        r"\bon\s+time\b",
        r"\bahead\s+of\s+schedule\b",
        r"\bprompt\b",
        r"\bspeed\b",
        r"\bslow\b",
        r"\bdelay\b",
        r"\blate\b",
    ],
    "communication": [
        r"\bcommunication\b",
        r"\bcommunicative\b",
        r"\bresponsive\b",
        r"\bclear\b",
        r"\bupdate\b",
        r"\bupdates\b",
        r"\barticulate\b",
        r"\bexplained\b",
        r"\bunresponsive\b",
    ],
    "reliability": [
        r"\breliable\b",
        r"\btrustworthy\b",
        r"\bdependable\b",
        r"\bconsistent\b",
        r"\bcommitted\b",
        r"\bresponsible\b",
        r"\bdiligent\b",
    ],
    "code_quality": [
        r"\bcode\s+quality\b",
        r"\bclean\s+code\b",
        r"\breadable\b",
        r"\belegant\b",
        r"\btest\s+coverage\b",
        r"\bdocumented\b",
        r"\bsuperb\s+code\b",
        r"\bwell-structured\b",
        r"\bmaintainable\b",
    ],
    "problem_solving": [
        r"\bproblem\s+solving\b",
        r"\bdebugging\b",
        r"\bcreative\b",
        r"\banalytical\b",
        r"\bsolution\b",
        r"\btroubleshoot\b",
        r"\bresourceful\b",
    ],
    "teamwork": [
        r"\bteamwork\b",
        r"\bcollaborative\b",
        r"\bcollaboration\b",
        r"\bpleasure\s+to\s+work\s+with\b",
        r"\bpartner\b",
        r"\bteam\s+player\b",
        r"\bsupportive\b",
    ],
}

STRENGTH_DESCRIPTIONS: dict[str, str] = {
    "technical_ability": "Superb Technical Skills",
    "timeliness": "Fast Turnaround",
    "communication": "Proactive Communicator",
    "reliability": "Highly Reliable & Dependable",
    "code_quality": "Superb Code Quality",
    "problem_solving": "Resourceful Problem Solver",
    "teamwork": "Exceptional Collaborative Partner",
}

IMPROVEMENT_DESCRIPTIONS: dict[str, str] = {
    "technical_ability": "Deepen domain documentation and technical architecture planning",
    "timeliness": "Ensure timely delivery against deadlines and flag delays early",
    "communication": "Provide frequent, proactive progress updates to project organizers",
    "reliability": "Improve follow-through on project milestone commitments",
    "code_quality": "Increase automated test coverage and adhere to modular style guidelines",
    "problem_solving": "Formulate structured contingency solutions when encountering roadblocks",
    "teamwork": "Engage more actively in team design discussions and peer reviews",
}


class ReviewItemInput(BaseModel):
    """An individual completed review rating and text comment."""

    model_config = ConfigDict(extra="ignore")

    rating: int = Field(..., ge=1, le=5, description="Review rating between 1 and 5 inclusive")
    comment: str = Field(..., description="Qualitative feedback text")


class ReviewAspectInsight(BaseModel):
    """Extracted collaboration aspect insight for a user."""

    model_config = ConfigDict(extra="ignore")

    aspect: str = Field(..., description="Canonical aspect name")
    score: float = Field(..., ge=1.0, le=5.0, description="Aspect score on 1-5 scale")
    confidence: float = Field(..., ge=0.0, le=1.0, description="Confidence score")
    sample_count: int = Field(..., ge=1, description="Number of reviews evaluating this aspect")
    is_recurring: bool = Field(
        default=False, description="True only if sample_count >= 3 with recurring pattern"
    )
    strengths: list[str] = Field(default_factory=list, description="Extracted strength highlights")
    improvements: list[str] = Field(
        default_factory=list, description="Constructive improvement recommendations"
    )


class ReviewAnalyzer:
    """Aspect-based sentiment and reputation analyzer for peer reviews."""

    def __init__(self, db_pool: Optional[Any] = None) -> None:
        self.db_pool = db_pool
        self.model_name = "lynk-review-analyzer"
        self.model_version = "1.0.0"
        self.pipeline_version = "reviews-v1"

    def analyze(
        self, user_id: str, reviews: list[ReviewItemInput]
    ) -> list[ReviewAspectInsight]:
        """Analyze reviews and extract structured aspect insights synchronously."""
        if not reviews:
            return []

        # Map aspect -> list of (rating, comment)
        aspect_matches: dict[str, list[ReviewItemInput]] = {k: [] for k in ASPECT_PATTERNS}

        for rev in reviews:
            comment_lower = rev.comment.lower()
            for aspect, patterns in ASPECT_PATTERNS.items():
                if any(re.search(pat, comment_lower) for pat in patterns):
                    aspect_matches[aspect].append(rev)

        insights: list[ReviewAspectInsight] = []

        for aspect, matched_reviews in aspect_matches.items():
            if not matched_reviews:
                continue

            sample_count = len(matched_reviews)
            # Calculate aspect score from ratings
            ratings = [r.rating for r in matched_reviews]
            mean_score = round(sum(ratings) / len(ratings), 1)

            # Recurrence invariant: is_recurring is True ONLY when sample_count >= 3
            is_recurring = sample_count >= 3

            strengths: list[str] = []
            improvements: list[str] = []

            if mean_score >= 4.0:
                strengths.append(STRENGTH_DESCRIPTIONS.get(aspect, f"Strong {aspect.replace('_', ' ').title()}"))
            elif mean_score <= 3.0:
                improvements.append(IMPROVEMENT_DESCRIPTIONS.get(aspect, f"Improve {aspect.replace('_', ' ')}"))

            confidence = round(min(0.98, 0.75 + 0.07 * min(3, sample_count)), 2)

            insights.append(
                ReviewAspectInsight(
                    aspect=aspect,
                    score=mean_score,
                    confidence=confidence,
                    sample_count=sample_count,
                    is_recurring=is_recurring,
                    strengths=strengths,
                    improvements=improvements,
                )
            )

        # Sort by sample count descending, then score descending
        insights.sort(key=lambda item: (item.sample_count, item.score), reverse=True)
        return insights

    async def analyze_and_record(
        self, user_id: str, reviews: list[ReviewItemInput]
    ) -> list[ReviewAspectInsight]:
        """Analyze reviews, trace run in ai_runs, and persist to review_insights in PostgreSQL."""
        async with track_ai_run(
            feature="review_insights",
            entity_type="user",
            entity_id=user_id,
            model_name=self.model_name,
            model_version=self.model_version,
            pipeline_version=self.pipeline_version,
            input_data={
                "user_id": user_id,
                "review_count": len(reviews),
            },
            db_pool=self.db_pool,
        ) as tracker:
            insights = self.analyze(user_id=user_id, reviews=reviews)
            tracker.set_output(
                {
                    "insights_count": len(insights),
                    "recurring_count": sum(1 for item in insights if item.is_recurring),
                }
            )
            tracker.set_confidence(insights[0].confidence if insights else 1.0)

            # Persist to review_insights table if db_pool is available
            if self.db_pool is not None:
                try:
                    async with self.db_pool.acquire() as conn:
                        for item in insights:
                            await conn.execute(
                                """
                                INSERT INTO review_insights (
                                    user_id, aspect, score, confidence, sample_count, is_recurring, strengths, improvements, updated_at
                                ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
                                ON CONFLICT (user_id, aspect) DO UPDATE SET
                                    score = EXCLUDED.score,
                                    confidence = EXCLUDED.confidence,
                                    sample_count = EXCLUDED.sample_count,
                                    is_recurring = EXCLUDED.is_recurring,
                                    strengths = EXCLUDED.strengths,
                                    improvements = EXCLUDED.improvements,
                                    updated_at = NOW();
                                """,
                                user_id,
                                item.aspect,
                                item.score,
                                item.confidence,
                                item.sample_count,
                                item.is_recurring,
                                item.strengths,
                                item.improvements,
                            )
                except Exception as exc:
                    logger.warning("Failed to persist review insights for user %s: %s", user_id, exc)

            return insights


_analyzer_instance: Optional[ReviewAnalyzer] = None


def get_review_analyzer(db_pool: Optional[Any] = None) -> ReviewAnalyzer:
    """Acquire or initialize ReviewAnalyzer singleton."""
    global _analyzer_instance
    if _analyzer_instance is None:
        _analyzer_instance = ReviewAnalyzer(db_pool=db_pool)
    elif db_pool is not None and _analyzer_instance.db_pool is None:
        _analyzer_instance.db_pool = db_pool
    return _analyzer_instance
