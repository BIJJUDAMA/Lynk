"""Asynchronous AI worker polling ai_jobs using SKIP LOCKED with backoff and dead-letter queuing."""

import asyncio
import json
import logging
import signal
import sys
from typing import Any, Callable, Coroutine, Optional

import asyncpg
from pydantic import BaseModel, Field

from ai.app.config import get_settings
from ai.workers.handlers.embedding_handler import EmbeddingJobHandler
from ai.workers.handlers.resume_handler import ResumeJobHandler

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(name)s: %(message)s")
logger = logging.getLogger("ai.worker")


class AIJob(BaseModel):
    """Model representing an asynchronous AI job item."""

    id: str
    job_type: str
    entity_type: str
    entity_id: str
    payload: dict[str, Any] = Field(default_factory=dict)
    status: str = "pending"
    attempts: int = 0
    max_attempts: int = 3
    error: Optional[str] = None
    content_hash: Optional[str] = None


class PostgresJobStore:
    """PostgreSQL-backed job queue store utilizing FOR UPDATE SKIP LOCKED with exponential backoff."""

    def __init__(self, pool: asyncpg.Pool) -> None:
        self.pool = pool

    async def fetch_next_job(self) -> Optional[AIJob]:
        """Claim next pending job eligible by retry_at using SKIP LOCKED to prevent race conditions."""
        query = """
            UPDATE ai_jobs
            SET status = 'processing', started_at = NOW()
            WHERE id = (
                SELECT id FROM ai_jobs
                WHERE status = 'pending'
                  AND (payload->>'retry_at' IS NULL OR (payload->>'retry_at')::timestamptz <= NOW())
                ORDER BY created_at ASC
                FOR UPDATE SKIP LOCKED
                LIMIT 1
            )
            RETURNING id, job_type, entity_type, entity_id, payload, status, attempts, max_attempts, error, content_hash;
        """
        async with self.pool.acquire() as conn:
            row = await conn.fetchrow(query)
            if row is None:
                return None

            raw_payload = row["payload"]
            if isinstance(raw_payload, str):
                try:
                    payload = json.loads(raw_payload)
                except Exception:
                    payload = {}
            elif isinstance(raw_payload, dict):
                payload = raw_payload
            else:
                payload = {}

            return AIJob(
                id=str(row["id"]),
                job_type=row["job_type"],
                entity_type=row["entity_type"],
                entity_id=row["entity_id"],
                payload=payload,
                status=row["status"],
                attempts=row["attempts"],
                max_attempts=row["max_attempts"],
                error=row["error"],
                content_hash=row["content_hash"],
            )

    async def complete_job(self, job_id: str, result: Optional[dict[str, Any]] = None) -> None:
        """Mark job as successfully completed with updated payload."""
        async with self.pool.acquire() as conn:
            await conn.execute(
                """
                UPDATE ai_jobs
                SET status = 'completed',
                    completed_at = NOW(),
                    payload = payload || $2::jsonb
                WHERE id = $1::uuid;
                """,
                job_id,
                json.dumps(result or {}),
            )

    async def fail_job(self, job_id: str, error: str) -> None:
        """Record failure, increment attempts, apply exponential backoff, and mark dead_letter if max attempts reached."""
        async with self.pool.acquire() as conn:
            await conn.execute(
                """
                UPDATE ai_jobs
                SET attempts = attempts + 1,
                    error = $2,
                    status = CASE WHEN attempts + 1 >= max_attempts THEN 'dead_letter' ELSE 'pending' END,
                    payload = CASE
                        WHEN attempts + 1 >= max_attempts THEN payload
                        ELSE jsonb_set(
                            coalesce(payload, '{}'::jsonb),
                            '{retry_at}',
                            to_jsonb(to_char(NOW() + (power(2, attempts + 1) * INTERVAL '1 second'), 'YYYY-MM-DD"T"HH24:MI:SS"Z"'))
                        )
                    END
                WHERE id = $1::uuid;
                """,
                job_id,
                error[:2000],
            )


class AIWorker:
    """Asynchronous worker executing registered AI job handlers."""

    def __init__(
        self,
        job_store=None,
        db_pool: Optional[asyncpg.Pool] = None,
        poll_interval: float = 2.0,
    ) -> None:
        if job_store is not None:
            self.job_store = job_store
        elif db_pool is not None:
            self.job_store = PostgresJobStore(db_pool)
        else:
            self.job_store = None

        self.db_pool = db_pool
        self.poll_interval = poll_interval
        self.handlers: dict[str, Callable[[AIJob], Coroutine[Any, Any, Any]]] = {}
        self.running = False

    def register_handler(
        self, job_type: str, handler: Callable[[AIJob], Coroutine[Any, Any, Any]]
    ) -> None:
        """Register an async handler callable for a specific job_type."""
        self.handlers[job_type] = handler

    async def process_next_job(self) -> bool:
        """Fetch and execute next pending job from the queue. Returns True if job was processed."""
        if self.job_store is None:
            return False

        job = await self.job_store.fetch_next_job()
        if job is None:
            return False

        handler = self.handlers.get(job.job_type)
        if handler is None:
            logger.error("No handler registered for job_type: %s (job_id=%s)", job.job_type, job.id)
            await self.job_store.fail_job(job.id, f"Unregistered handler for job_type: {job.job_type}")
            return True

        logger.info("Executing job: %s (type=%s, entity=%s:%s)", job.id, job.job_type, job.entity_type, job.entity_id)
        try:
            result = await handler(job)
            await self.job_store.complete_job(job.id, result if isinstance(result, dict) else None)
            logger.info("Job completed successfully: %s", job.id)
        except Exception as exc:
            logger.error("Job %s failed with exception: %s", job.id, exc, exc_info=True)
            await self.job_store.fail_job(job.id, str(exc))

        return True

    async def run(self) -> None:
        """Worker event loop polling for jobs until stopped."""
        self.running = True
        logger.info("AI Worker started polling with interval %.1fs", self.poll_interval)
        while self.running:
            try:
                processed = await self.process_next_job()
                if not processed:
                    await asyncio.sleep(self.poll_interval)
            except asyncio.CancelledError:
                break
            except Exception as exc:
                logger.error("Unexpected worker loop error: %s", exc)
                await asyncio.sleep(self.poll_interval)
        logger.info("AI Worker stopped gracefully")

    def stop(self) -> None:
        """Signal worker to terminate polling loop."""
        self.running = False


async def main() -> None:
    """Worker daemon entrypoint when run from CLI or container."""
    settings = get_settings()
    logger.info("Starting AI Worker daemon connecting to %s", settings.DATABASE_URL)

    try:
        pool = await asyncpg.create_pool(dsn=settings.DATABASE_URL, min_size=1, max_size=5)
    except Exception as exc:
        logger.fatal("Failed to connect to PostgreSQL: %s", exc)
        sys.exit(1)

    worker = AIWorker(db_pool=pool, poll_interval=2.0)

    # Register standard handlers
    embedding_handler = EmbeddingJobHandler(db_pool=pool)
    resume_handler = ResumeJobHandler(db_pool=pool)

    worker.register_handler("generate_embedding", embedding_handler.handle)
    worker.register_handler("parse_resume", resume_handler.handle)

    loop = asyncio.get_running_loop()
    stop_event = asyncio.Event()

    def signal_handler():
        logger.info("Received termination signal, shutting down worker...")
        worker.stop()
        stop_event.set()

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, signal_handler)
        except NotImplementedError:
            pass  # Windows signal handlers

    worker_task = asyncio.create_task(worker.run())
    await stop_event.wait()
    await worker_task
    await pool.close()


if __name__ == "__main__":
    asyncio.run(main())
