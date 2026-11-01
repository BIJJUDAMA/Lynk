# 100% pure 7-bit ASCII
import logging
import uuid
from typing import Any

from fastapi import APIRouter, Query, Request, status
from pydantic import BaseModel, Field

logger = logging.getLogger("lynk-ai.feedback")

router = APIRouter(prefix="/feedback", tags=["Evaluation & Feedback"])


class FeedbackSubmission(BaseModel):
    """Payload for submitting feedback on AI predictions and generations."""

    target_type: str = Field(
        ...,
        description="Target classification (e.g. job_draft, recommendation, search, collaborator)",
        max_length=50,
    )
    target_id: str = Field(
        ...,
        description="Identifier of the evaluated entity or generation result",
        max_length=128,
    )
    feedback_type: str = Field(
        ...,
        description="Feedback category (e.g. thumbs_up, thumbs_down, rating, accepted, edited)",
        max_length=50,
    )
    run_id: str | None = Field(
        default=None,
        description="Optional foreign key correlating this feedback to a specific ai_runs entry",
    )
    rating: float | None = Field(
        default=None,
        description="Numerical satisfaction score (e.g. 1.0 to 5.0, or 1.0 / 0.0)",
    )
    comment: str | None = Field(
        default=None,
        description="Optional qualitative student or recruiter feedback comments",
        max_length=2000,
    )
    metadata: dict[str, Any] = Field(
        default_factory=dict,
        description="Arbitrary contextual metadata (e.g. edit distance, client info)",
    )
    user_id: str | None = Field(
        default=None,
        description="Identifier of the campus member providing feedback",
        max_length=64,
    )


class FeedbackResponse(BaseModel):
    status: str
    feedback_id: str
    message: str


class FeedbackSummary(BaseModel):
    target_type: str | None
    total_feedback: int
    thumbs_up: int
    thumbs_down: int
    positive_ratio: float
    average_rating: float | None


@router.post(
    "",
    response_model=FeedbackResponse,
    status_code=status.HTTP_201_CREATED,
    summary="Submit user feedback on AI predictions and generations",
)
async def submit_feedback(
    submission: FeedbackSubmission, request: Request
) -> FeedbackResponse:
    """Record student/recruiter feedback for continuous LLMOps evaluation."""
    pool = getattr(request.app.state, "db_pool", None)
    feedback_id = str(uuid.uuid4())

    run_uuid = None
    if submission.run_id:
        try:
            run_uuid = uuid.UUID(submission.run_id)
        except ValueError:
            logger.debug("Non-UUID run_id provided: %s", submission.run_id)
            run_uuid = None

    if pool is not None:
        try:
            await pool.execute(
                """
                INSERT INTO ai_feedback (
                    id, run_id, target_type, target_id, feedback_type,
                    rating, comment, metadata, user_id
                ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
                """,
                uuid.UUID(feedback_id),
                run_uuid,
                submission.target_type,
                submission.target_id,
                submission.feedback_type,
                submission.rating,
                submission.comment,
                submission.metadata,
                submission.user_id,
            )
        except Exception as exc:
            logger.warning("Failed to persist feedback to database: %s", exc)

    logger.info(
        "AI Feedback recorded: target=%s/%s, type=%s, rating=%s",
        submission.target_type,
        submission.target_id,
        submission.feedback_type,
        submission.rating,
    )

    return FeedbackResponse(
        status="recorded",
        feedback_id=feedback_id,
        message="Feedback successfully recorded for model evaluation",
    )


@router.get(
    "/summary",
    response_model=FeedbackSummary,
    summary="Retrieve aggregate quality and evaluation metrics",
)
async def get_feedback_summary(
    request: Request,
    target_type: str | None = Query(
        default=None, description="Filter summary by target type"
    ),
) -> FeedbackSummary:
    """Compute aggregate quality and satisfaction metrics across AI runs."""
    pool = getattr(request.app.state, "db_pool", None)

    if pool is None:
        return FeedbackSummary(
            target_type=target_type,
            total_feedback=0,
            thumbs_up=0,
            thumbs_down=0,
            positive_ratio=1.0,
            average_rating=None,
        )

    try:
        where_clause = "WHERE target_type = $1" if target_type else ""
        args = [target_type] if target_type else []

        row = await pool.fetchrow(
            f"""
            SELECT
                COUNT(*)::int AS total,
                COUNT(*) FILTER (WHERE feedback_type IN ('thumbs_up', 'accepted', 'like'))::int AS positive,
                COUNT(*) FILTER (WHERE feedback_type IN ('thumbs_down', 'rejected', 'dislike'))::int AS negative,
                AVG(rating)::float AS avg_rating
            FROM ai_feedback
            {where_clause}
            """,
            *args,
        )

        total = row["total"] if row and row["total"] else 0
        positive = row["positive"] if row and row["positive"] else 0
        negative = row["negative"] if row and row["negative"] else 0
        avg_rating = (
            round(float(row["avg_rating"]), 2)
            if row and row["avg_rating"] is not None
            else None
        )
        pos_ratio = round(positive / total, 3) if total > 0 else 1.0

        return FeedbackSummary(
            target_type=target_type,
            total_feedback=total,
            thumbs_up=positive,
            thumbs_down=negative,
            positive_ratio=pos_ratio,
            average_rating=avg_rating,
        )
    except Exception as exc:
        logger.warning("Failed to aggregate feedback summary: %s", exc)
        return FeedbackSummary(
            target_type=target_type,
            total_feedback=0,
            thumbs_up=0,
            thumbs_down=0,
            positive_ratio=1.0,
            average_rating=None,
        )
