"""Hybrid moderation and spam detection pipeline.

Evaluates content submissions across multiple complementary risk vectors:
  1. Content length heuristics (minimum 30 characters)
  2. Author posting velocity in rolling time window
  3. Semantic near-duplicate detection via embedding cosine similarity
  4. Off-platform payment solicitation and suspicious contact pattern recognition

Strictly adheres to transparent audit logging and persistent event tracking in moderation_events.
"""

import logging
import re
from typing import Any

from ai.app.middleware.run_tracker import track_ai_run
from ai.models.embeddings.provider import (
    BaseEmbeddingProvider,
    get_embedding_provider,
)
from pydantic import BaseModel, ConfigDict, Field

logger = logging.getLogger("lynk-ai.moderation")


# Suspicious payment terms indicating off-platform transaction solicitations
OFF_PLATFORM_PAYMENT_PATTERNS = [
    r"\bwire\s+transfer\b",
    r"\bdirect\s+cash\b",
    r"\bcashapp\b",
    r"\bvenmo\b",
    r"\bzelle\b",
    r"\bcrypto\b",
    r"\bbitcoin\b",
    r"\beth\b",
    r"\bpay\s+me\s+outside\b",
    r"\bpay\s+outside\b",
    r"\boutside\s+the\s+portal\b",
    r"\boutside\s+the\s+app\b",
]

# Suspicious external contact terms indicating evasion of platform messaging
SUSPICIOUS_CONTACT_PATTERNS = [
    r"\btelegram\b",
    r"\bwhatsapp\b",
    r"\bt\.me\b",
    r"\bwa\.me\b",
]


class ModerationCheckInput(BaseModel):
    """Input payload for automated moderation check."""

    model_config = ConfigDict(extra="ignore")

    entity_type: str = Field(
        ..., description="Entity type, e.g. 'job' or 'application'"
    )
    entity_id: str = Field(..., description="Entity identifier")
    text: str = Field(..., description="Text content to evaluate")
    author_id: str = Field(..., description="Author campus member identifier")
    author_recent_post_count: int = Field(
        default=0, description="Number of posts made by author in rolling 1-hour window"
    )
    recent_submissions: list[str] = Field(
        default_factory=list,
        description="Recent submission texts by author to evaluate for near-duplicates",
    )


class ModerationResult(BaseModel):
    """Automated moderation evaluation output."""

    model_config = ConfigDict(extra="ignore")

    risk_score: float = Field(
        ..., ge=0.0, le=1.0, description="Aggregate risk score from 0.0 to 1.0"
    )
    decision: str = Field(..., description="'allow', 'review', or 'reject'")
    signals: list[str] = Field(
        default_factory=list, description="List of triggered violation signals"
    )
    confidence: float = Field(
        ..., ge=0.0, le=1.0, description="Confidence score in decision"
    )
    pipeline_version: str = Field(
        default="moderation-v1", description="Pipeline version string"
    )


class ModerationDetector:
    """Hybrid moderation pipeline assessing textual content, velocity, and duplicates."""

    def __init__(
        self,
        embedding_provider: BaseEmbeddingProvider | None = None,
        db_pool: Any | None = None,
    ) -> None:
        self._provider = embedding_provider
        self.db_pool = db_pool
        self.model_name = "lynk-moderation-detector"
        self.model_version = "1.0.0"
        self.pipeline_version = "moderation-v1"

    @property
    def embedding_provider(self) -> BaseEmbeddingProvider:
        """Acquire or lazily initialize the embedding provider."""
        if self._provider is None:
            self._provider = get_embedding_provider()
        return self._provider

    def _cosine_similarity(self, vec_a: list[float], vec_b: list[float]) -> float:
        """Calculate cosine similarity between two float vectors."""
        if not vec_a or not vec_b or len(vec_a) != len(vec_b):
            return 0.0
        dot = sum(a * b for a, b in zip(vec_a, vec_b))
        norm_a = sum(a * a for a in vec_a) ** 0.5
        norm_b = sum(b * b for b in vec_b) ** 0.5
        if norm_a == 0.0 or norm_b == 0.0:
            return 0.0
        return dot / (norm_a * norm_b)

    def check(self, payload: ModerationCheckInput) -> ModerationResult:
        """Evaluate submission synchronously and return moderation decision."""
        raw_text = payload.text.strip()
        signals: list[str] = []
        base_risk = 0.05

        # 1. Content length heuristic (minimum 30 characters)
        if len(raw_text) <= 30:
            signals.append("short_content")
            base_risk += 0.35

        # 2. Author posting velocity in rolling 1-hour window
        if payload.author_recent_post_count >= 5:
            signals.append("high_velocity")
            base_risk += 0.40

        # 3. Off-platform payment and suspicious contact pattern analysis
        text_lower = raw_text.lower()
        has_payment = any(
            re.search(pat, text_lower) for pat in OFF_PLATFORM_PAYMENT_PATTERNS
        )
        if has_payment:
            signals.append("off_platform_payment")
            base_risk += 0.50

        has_contact = any(
            re.search(pat, text_lower) for pat in SUSPICIOUS_CONTACT_PATTERNS
        )
        if has_contact:
            signals.append("suspicious_contact")
            base_risk += 0.45

        # 4. Semantic near-duplicate detection via embedding cosine similarity
        if payload.recent_submissions and raw_text:
            query_vec = self.embedding_provider.embed(raw_text)
            max_sim = 0.0
            for prev_text in payload.recent_submissions:
                prev_text_clean = prev_text.strip()
                if not prev_text_clean:
                    continue
                prev_vec = self.embedding_provider.embed(prev_text_clean)
                sim = self._cosine_similarity(query_vec, prev_vec)
                if sim > max_sim:
                    max_sim = sim

            # Cosine distance < 0.05 corresponds to cosine similarity > 0.95
            if max_sim > 0.95:
                signals.append("duplicate_description")
                base_risk = max(0.80, base_risk + 0.60)

        risk_score = round(max(0.0, min(1.0, base_risk)), 2)

        # Decision threshold mapping
        if risk_score >= 0.80:
            # High risk: trigger review or reject
            decision = "review" if "duplicate_description" in signals else "reject"
        elif risk_score >= 0.50:
            decision = "review"
        else:
            decision = "allow"

        # Confidence assessment
        confidence = (
            0.95 if not signals else round(min(0.98, 0.85 + 0.04 * len(signals)), 2)
        )

        return ModerationResult(
            risk_score=risk_score,
            decision=decision,
            signals=signals,
            confidence=confidence,
            pipeline_version=self.pipeline_version,
        )

    async def check_and_record(self, payload: ModerationCheckInput) -> ModerationResult:
        """Evaluate submission, track execution in ai_runs, and optionally persist moderation event."""
        async with track_ai_run(
            feature="moderation",
            entity_type=payload.entity_type,
            entity_id=payload.entity_id,
            model_name=self.model_name,
            model_version=self.model_version,
            pipeline_version=self.pipeline_version,
            input_data={
                "entity_type": payload.entity_type,
                "entity_id": payload.entity_id,
                "author_id": payload.author_id,
                "text_length": len(payload.text),
            },
            db_pool=self.db_pool,
        ) as tracker:
            result = self.check(payload)
            tracker.set_output(
                {
                    "decision": result.decision,
                    "risk_score": result.risk_score,
                    "signals": result.signals,
                }
            )
            tracker.set_confidence(result.confidence)

            # Persist domain moderation event if db_pool is available
            if self.db_pool is not None:
                try:
                    async with self.db_pool.acquire() as conn:
                        await conn.execute(
                            """
                            INSERT INTO moderation_events (
                                entity_type, entity_id, risk_score, decision, signals, confidence, pipeline_version
                            ) VALUES ($1, $2, $3, $4, $5, $6, $7)
                            """,
                            payload.entity_type,
                            payload.entity_id,
                            result.risk_score,
                            result.decision,
                            result.signals,
                            result.confidence,
                            result.pipeline_version,
                        )
                except Exception as exc:
                    logger.warning(
                        "Failed to persist moderation event for entity %s: %s",
                        payload.entity_id,
                        exc,
                    )

            return result


_detector_instance: ModerationDetector | None = None


def get_moderation_detector(db_pool: Any | None = None) -> ModerationDetector:
    """Acquire or initialize ModerationDetector singleton."""
    global _detector_instance
    if _detector_instance is None:
        _detector_instance = ModerationDetector(db_pool=db_pool)
    elif db_pool is not None and _detector_instance.db_pool is None:
        _detector_instance.db_pool = db_pool
    return _detector_instance
