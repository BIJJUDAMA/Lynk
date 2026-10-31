"""Unit tests for ARQ Redis background ML tasks."""

import pytest

from ai.workers.tasks import (
    process_uploaded_resume_task,
    sync_student_vectors_task,
    update_kuzu_graph_edge_task,
)


@pytest.mark.asyncio
async def test_process_uploaded_resume_task():
    ctx = {}
    sample_text = "Junior CS student at MIT skilled in Go, Docker, Python."
    result = await process_uploaded_resume_task(
        ctx, student_id="s123", resume_text=sample_text
    )
    assert result["student_id"] == "s123"
    assert "Go" in result["extracted_skills"]
    assert result["status"] == "success"


@pytest.mark.asyncio
async def test_sync_student_vectors_task():
    ctx = {}
    res = await sync_student_vectors_task(
        ctx, student_id="s123", skills=["Python", "Docker"]
    )
    assert res["status"] == "synced"


@pytest.mark.asyncio
async def test_update_kuzu_graph_edge_task():
    ctx = {}
    res = await update_kuzu_graph_edge_task(
        ctx, student_id="s123", project_id="p456", action="applied"
    )
    assert res["status"] == "edge_created"
