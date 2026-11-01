# 100% pure 7-bit ASCII
import uuid
from unittest.mock import AsyncMock, MagicMock

import pytest
from fastapi.testclient import TestClient

from ai.app.main import app


@pytest.fixture
def client():
    return TestClient(app)


def test_submit_feedback_endpoint_success(client):
    """Submitting feedback returns HTTP 201 with feedback_id."""
    payload = {
        "target_type": "job_draft",
        "target_id": "draft-101",
        "feedback_type": "thumbs_up",
        "rating": 5.0,
        "comment": "Accurate skills and budget estimation",
        "metadata": {"edit_distance": 0},
        "user_id": "student-42",
    }
    response = client.post("/feedback", json=payload)
    assert response.status_code == 201
    data = response.json()
    assert data["status"] == "recorded"
    assert "feedback_id" in data
    assert uuid.UUID(data["feedback_id"])  # Valid UUID


def test_submit_feedback_with_run_id(client):
    """Feedback with valid run_id links to ai_runs."""
    run_id = str(uuid.uuid4())
    payload = {
        "target_type": "recommendation",
        "target_id": "rec-202",
        "feedback_type": "accepted",
        "run_id": run_id,
        "rating": 1.0,
    }
    response = client.post("/feedback", json=payload)
    assert response.status_code == 201
    data = response.json()
    assert data["status"] == "recorded"


def test_get_feedback_summary_fallback_without_db(client):
    """Feedback summary gracefully returns default structure when db_pool is None."""
    response = client.get("/feedback/summary?target_type=job_draft")
    assert response.status_code == 200
    data = response.json()
    assert data["target_type"] == "job_draft"
    assert "total_feedback" in data
    assert "positive_ratio" in data


@pytest.mark.asyncio
async def test_feedback_persists_to_pool():
    """Feedback endpoint executes SQL insert against db_pool."""
    from ai.app.api.feedback import FeedbackSubmission, submit_feedback

    mock_pool = AsyncMock()
    mock_pool.execute = AsyncMock(return_value="INSERT 0 1")

    mock_request = MagicMock()
    mock_request.app.state.db_pool = mock_pool

    submission = FeedbackSubmission(
        target_type="collaborator",
        target_id="cand-99",
        feedback_type="thumbs_down",
        rating=1.0,
        comment="Mismatched graduation year",
    )

    res = await submit_feedback(submission, mock_request)
    assert res.status == "recorded"
    mock_pool.execute.assert_awaited_once()
    args = mock_pool.execute.call_args[0]
    assert "INSERT INTO ai_feedback" in args[0]
    assert args[2] is None  # run_uuid
    assert args[3] == "collaborator"
    assert args[4] == "cand-99"
    assert args[5] == "thumbs_down"
    assert args[6] == 1.0
