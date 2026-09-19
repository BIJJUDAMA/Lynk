"""Internal API routes for Hybrid Moderation and Spam Detection."""

from ai.pipelines.moderation.detector import (
    ModerationCheckInput,
    ModerationResult,
    get_moderation_detector,
)
from fastapi import APIRouter, Request

router = APIRouter(prefix="/internal/v1/moderation", tags=["moderation"])


@router.post("/check", response_model=ModerationResult)
async def check_moderation_endpoint(
    payload: ModerationCheckInput, request: Request
) -> ModerationResult:
    """Evaluate text content for spam, duplicates, and policy violations."""
    pool = getattr(request.app.state, "db_pool", None)
    detector = get_moderation_detector(db_pool=pool)

    return await detector.check_and_record(payload)
