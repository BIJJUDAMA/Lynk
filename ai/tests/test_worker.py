"""Unit and integration tests for Asynchronous AI Worker and Job Handlers."""

import uuid
from typing import Any
import pytest

from ai.workers.worker import AIJob, AIWorker
from ai.workers.handlers.embedding_handler import EmbeddingJobHandler
from ai.workers.handlers.resume_handler import ResumeJobHandler
from ai.models.embeddings.provider import MockEmbeddingProvider


class MockJobStore:
    """In-memory mock store simulating PostgreSQL ai_jobs table with FOR UPDATE SKIP LOCKED semantics."""

    def __init__(self, jobs: list[AIJob]) -> None:
        self.jobs = {job.id: job for job in jobs}

    async def fetch_next_job(self) -> AIJob | None:
        for job in self.jobs.values():
            if job.status == "pending":
                job.status = "processing"
                return job
        return None

    async def complete_job(self, job_id: str, result: dict[str, Any] | None = None) -> None:
        if job_id in self.jobs:
            self.jobs[job_id].status = "completed"
            if result:
                self.jobs[job_id].payload.update(result)

    async def fail_job(self, job_id: str, error: str) -> None:
        if job_id in self.jobs:
            job = self.jobs[job_id]
            job.attempts += 1
            job.error = error
            if job.attempts >= job.max_attempts:
                job.status = "dead_letter"
            else:
                job.status = "pending"


@pytest.mark.asyncio
async def test_worker_successful_job_execution():
    """Worker picks up pending job, executes handler, and transitions status to completed."""
    job_id = str(uuid.uuid4())
    test_job = AIJob(
        id=job_id,
        job_type="test_task",
        entity_type="test",
        entity_id="test-1",
        payload={"message": "hello"},
        status="pending",
        attempts=0,
        max_attempts=3,
    )

    store = MockJobStore([test_job])
    executed = []

    async def sample_handler(job: AIJob) -> dict[str, Any]:
        executed.append(job.id)
        return {"processed": True}

    worker = AIWorker(job_store=store)
    worker.register_handler("test_task", sample_handler)

    processed = await worker.process_next_job()

    assert processed is True
    assert len(executed) == 1
    assert executed[0] == job_id
    assert store.jobs[job_id].status == "completed"
    assert store.jobs[job_id].payload.get("processed") is True


@pytest.mark.asyncio
async def test_worker_failure_transitions_to_dead_letter():
    """Failing job increments attempts and transitions to dead_letter on exceeding max_attempts."""
    job_id = str(uuid.uuid4())
    failing_job = AIJob(
        id=job_id,
        job_type="failing_task",
        entity_type="test",
        entity_id="test-2",
        payload={},
        status="pending",
        attempts=2,  # Already attempted twice, next failure should trigger dead_letter
        max_attempts=3,
    )

    store = MockJobStore([failing_job])

    async def failing_handler(job: AIJob) -> None:
        raise ValueError("Simulated handler crash")

    worker = AIWorker(job_store=store)
    worker.register_handler("failing_task", failing_handler)

    processed = await worker.process_next_job()

    assert processed is True
    assert store.jobs[job_id].status == "dead_letter"
    assert store.jobs[job_id].attempts == 3
    assert "Simulated handler crash" in store.jobs[job_id].error


@pytest.mark.asyncio
async def test_embedding_handler_generates_embeddings():
    """EmbeddingJobHandler generates normalized vector embeddings and computes content hash."""
    provider = MockEmbeddingProvider()
    handler = EmbeddingJobHandler(embedding_provider=provider)

    job = AIJob(
        id=str(uuid.uuid4()),
        job_type="generate_embedding",
        entity_type="job",
        entity_id="job-uuid-123",
        payload={"text": "Software engineer building Go microservices"},
        status="processing",
    )

    result = await handler.handle(job)

    assert "embedding" in result
    assert len(result["embedding"]) == 384
    assert "content_hash" in result
    assert len(result["content_hash"]) == 64  # SHA-256


@pytest.mark.asyncio
async def test_resume_handler_extracts_tagged_skills():
    """ResumeJobHandler parses resume text and tags extracted skills with source: ai_resume_parser."""
    handler = ResumeJobHandler()

    resume_text = """
    Jane Doe - Campus Developer
    Experienced in Python, PyTorch, Docker, and PostgreSQL.
    Passionate about Machine Learning research.
    """

    job = AIJob(
        id=str(uuid.uuid4()),
        job_type="parse_resume",
        entity_type="resume",
        entity_id="resume-uuid-456",
        payload={"resume_text": resume_text},
        status="processing",
    )

    result = await handler.handle(job)

    assert "extracted_skills" in result
    skills = result["extracted_skills"]
    assert len(skills) >= 3
    skill_names = [s["skill"] for s in skills]
    assert "Python" in skill_names
    for item in skills:
        assert item["source"] == "ai_resume_parser"


@pytest.mark.asyncio
async def test_postgres_job_store_fetch_parses_json_string_and_checks_backoff():
    """PostgresJobStore safely parses string payload and query enforces retry_at backoff."""
    from unittest.mock import AsyncMock, MagicMock
    from ai.workers.worker import PostgresJobStore

    mock_pool = MagicMock()
    mock_conn = AsyncMock()
    mock_pool.acquire.return_value.__aenter__.return_value = mock_conn

    job_uuid = uuid.uuid4()
    mock_conn.fetchrow.return_value = {
        "id": job_uuid,
        "job_type": "generate_embedding",
        "entity_type": "job",
        "entity_id": "job-101",
        "payload": '{"text": "Python developer", "retry_at": "2026-01-01T00:00:00Z"}',
        "status": "processing",
        "attempts": 1,
        "max_attempts": 3,
        "error": None,
        "content_hash": "abc123hash",
    }

    store = PostgresJobStore(mock_pool)
    job = await store.fetch_next_job()

    assert job is not None
    assert isinstance(job.payload, dict)
    assert job.payload["text"] == "Python developer"
    assert job.attempts == 1

    # Verify query includes SKIP LOCKED and retry_at condition
    call_query = mock_conn.fetchrow.call_args[0][0]
    assert "FOR UPDATE SKIP LOCKED" in call_query
    assert "retry_at" in call_query


@pytest.mark.asyncio
async def test_postgres_job_store_fail_job_applies_exponential_backoff():
    """PostgresJobStore.fail_job applies power(2, attempts + 1) interval for exponential backoff."""
    from unittest.mock import AsyncMock, MagicMock
    from ai.workers.worker import PostgresJobStore

    mock_pool = MagicMock()
    mock_conn = AsyncMock()
    mock_pool.acquire.return_value.__aenter__.return_value = mock_conn

    store = PostgresJobStore(mock_pool)
    job_id = str(uuid.uuid4())
    await store.fail_job(job_id, "Temporary database timeout")

    assert mock_conn.execute.called
    query = mock_conn.execute.call_args[0][0]
    assert "power(2, attempts + 1)" in query
    assert "retry_at" in query
    assert "dead_letter" in query


@pytest.mark.asyncio
async def test_embedding_handler_db_persistence_matches_unique_constraint():
    """EmbeddingJobHandler executes SQL matching 5-column unique constraint."""
    from unittest.mock import AsyncMock, MagicMock

    mock_pool = MagicMock()
    mock_conn = AsyncMock()
    mock_pool.acquire.return_value.__aenter__.return_value = mock_conn

    handler = EmbeddingJobHandler(embedding_provider=MockEmbeddingProvider(), db_pool=mock_pool)
    job = AIJob(
        id=str(uuid.uuid4()),
        job_type="generate_embedding",
        entity_type="job",
        entity_id="job-uuid-789",
        payload={"text": "PostgreSQL database optimization"},
        status="processing",
    )

    result = await handler.handle(job)
    assert result["dimension"] == 384

    assert mock_conn.execute.called
    query = mock_conn.execute.call_args[0][0]
    assert "ON CONFLICT (entity_type, entity_id, embedding_type, model_name, model_version)" in query
    assert "'semantic'" in query


@pytest.mark.asyncio
async def test_resume_handler_db_persistence_matches_profile_skills_schema():
    """ResumeJobHandler inserts into profile_skills matching schema columns."""
    from unittest.mock import AsyncMock, MagicMock

    mock_pool = MagicMock()
    mock_conn = AsyncMock()
    mock_pool.acquire.return_value.__aenter__.return_value = mock_conn

    handler = ResumeJobHandler(db_pool=mock_pool)
    job = AIJob(
        id=str(uuid.uuid4()),
        job_type="parse_resume",
        entity_type="resume",
        entity_id="user-campus-456",
        payload={"resume_text": "Experienced in React and TypeScript web frontend"},
        status="processing",
    )

    result = await handler.handle(job)
    assert result["count"] >= 2

    assert mock_conn.execute.called
    query = mock_conn.execute.call_args[0][0]
    assert "INSERT INTO profile_skills (user_id, skill_id, confidence, source, updated_at)" in query
    assert "ON CONFLICT (user_id, skill_id) DO UPDATE" in query

