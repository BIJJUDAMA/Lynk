"""AI Run Tracking & Model Versioning System.

Captures immutable audit logs for every AI operation (ranking, search, moderation,
generation, skill extraction) into the ai_runs PostgreSQL table.
"""

import asyncio
import functools
import inspect
import json
import logging
import time
from typing import Any, Callable, Optional, Union
from pydantic import BaseModel, Field

from ai.models.embeddings import compute_content_hash

logger = logging.getLogger("lynk-ai.run_tracker")


class AIRunRecord(BaseModel):
    """Pydantic model representing an audit log entry in the ai_runs table."""

    feature: str
    entity_type: str
    entity_id: str
    model_name: str
    model_version: str
    prompt_version: Optional[str] = None
    pipeline_version: str
    input_hash: str
    output_json: dict = Field(default_factory=dict)
    confidence: Optional[float] = None
    latency_ms: int = 0
    status: str = "success"
    error: Optional[str] = None


def compute_run_input_hash(input_data: Any) -> str:
    """Compute deterministic SHA-256 hash for run input data with normalization.

    Handles strings, structured dictionaries (with sorted keys), lists,
    Pydantic models, or arbitrary serializable objects.
    """
    if isinstance(input_data, (dict, list)):
        serialized = json.dumps(input_data, sort_keys=True, default=str)
    elif isinstance(input_data, str):
        serialized = input_data
    elif hasattr(input_data, "model_dump"):
        serialized = json.dumps(input_data.model_dump(), sort_keys=True, default=str)
    elif hasattr(input_data, "dict"):
        serialized = json.dumps(input_data.dict(), sort_keys=True, default=str)
    else:
        serialized = str(input_data)
    return compute_content_hash(serialized)


class RunTrackerContext:
    """Async context manager and execution tracker for an individual AI run."""

    def __init__(
        self,
        feature: str,
        entity_type: str,
        entity_id: str,
        model_name: str,
        model_version: str,
        pipeline_version: str,
        input_data: Any = "",
        prompt_version: Optional[str] = None,
        db_pool: Optional[Any] = None,
        sink: Optional[Union[list[AIRunRecord], Callable[[AIRunRecord], Any]]] = None,
        background: bool = False,
    ) -> None:
        self.feature = feature
        self.entity_type = entity_type
        self.entity_id = entity_id
        self.model_name = model_name
        self.model_version = model_version
        self.pipeline_version = pipeline_version
        self.prompt_version = prompt_version
        self.input_data = input_data
        self.db_pool = db_pool
        self.sink = sink
        self.background = background

        input_hash = compute_run_input_hash(input_data)

        self.record = AIRunRecord(
            feature=feature,
            entity_type=entity_type,
            entity_id=entity_id,
            model_name=model_name,
            model_version=model_version,
            prompt_version=prompt_version,
            pipeline_version=pipeline_version,
            input_hash=input_hash,
        )
        self._start_time: float = 0.0
        self.persist_task: Optional[asyncio.Task] = None

    @property
    def last_run(self) -> AIRunRecord:
        """Alias to access the underlying AIRunRecord for test verification."""
        return self.record

    def set_output(self, output: Any) -> None:
        """Set the output JSON payload on the run record."""
        if output is None:
            self.record.output_json = {}
        elif hasattr(output, "model_dump"):
            self.record.output_json = output.model_dump()
        elif hasattr(output, "dict"):
            self.record.output_json = output.dict()
        elif isinstance(output, dict):
            self.record.output_json = output
        else:
            self.record.output_json = {"value": output}

    def set_confidence(self, confidence: Optional[float]) -> None:
        """Set the confidence score between 0.0 and 1.0."""
        self.record.confidence = confidence

    def set_error(self, error: Optional[str]) -> None:
        """Record an error message and set status to failed."""
        self.record.error = error
        if error is not None:
            self.record.status = "failed"

    def set_entity(self, entity_type: str, entity_id: str) -> None:
        """Update entity type and ID if determined dynamically during execution."""
        self.record.entity_type = entity_type
        self.record.entity_id = entity_id

    async def __aenter__(self) -> "RunTrackerContext":
        self._start_time = time.perf_counter()
        return self

    async def __aexit__(
        self,
        exc_type: Optional[type[BaseException]],
        exc_val: Optional[BaseException],
        exc_tb: Optional[Any],
    ) -> bool:
        elapsed_sec = time.perf_counter() - self._start_time
        self.record.latency_ms = max(0, int(elapsed_sec * 1000))

        if exc_type is not None:
            self.record.status = "failed"
            self.record.error = str(exc_val)

        # In-memory sink dispatch
        if self.sink is not None:
            try:
                if isinstance(self.sink, list):
                    self.sink.append(self.record)
                elif callable(self.sink):
                    res = self.sink(self.record)
                    if inspect.isawaitable(res):
                        await res
            except Exception as exc:
                logger.warning("Run tracker sink callback failed: %s", exc)

        # Database persistence
        if self.db_pool is not None:
            if self.background:
                self.persist_task = asyncio.create_task(self._persist())
            else:
                await self._persist()

        # Re-raise exceptions by returning False
        return False

    async def _persist(self) -> None:
        """Asynchronously insert record into ai_runs table."""
        query = (
            "INSERT INTO ai_runs ("
            "feature, entity_type, entity_id, model_name, model_version, "
            "prompt_version, pipeline_version, input_hash, output_json, "
            "confidence, latency_ms, status, error"
            ") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13)"
        )
        try:
            acquire_cm = self.db_pool.acquire()
            if hasattr(acquire_cm, "__aenter__"):
                async with acquire_cm as conn:
                    await conn.execute(
                        query,
                        self.record.feature,
                        self.record.entity_type,
                        self.record.entity_id,
                        self.record.model_name,
                        self.record.model_version,
                        self.record.prompt_version,
                        self.record.pipeline_version,
                        self.record.input_hash,
                        json.dumps(self.record.output_json or {}),
                        self.record.confidence,
                        self.record.latency_ms,
                        self.record.status,
                        self.record.error,
                    )
            else:
                conn = await acquire_cm
                try:
                    await conn.execute(
                        query,
                        self.record.feature,
                        self.record.entity_type,
                        self.record.entity_id,
                        self.record.model_name,
                        self.record.model_version,
                        self.record.prompt_version,
                        self.record.pipeline_version,
                        self.record.input_hash,
                        json.dumps(self.record.output_json or {}),
                        self.record.confidence,
                        self.record.latency_ms,
                        self.record.status,
                        self.record.error,
                    )
                finally:
                    if hasattr(self.db_pool, "release"):
                        await self.db_pool.release(conn)
        except Exception as exc:
            logger.warning("Failed to persist AI run record to ai_runs: %s", exc)

    def __call__(self, func: Callable[..., Any]) -> Callable[..., Any]:
        """Support decorator syntax: @track_ai_run(...)"""
        return tracked_ai_run(
            feature=self.feature,
            entity_type=self.entity_type,
            entity_id=self.entity_id,
            model_name=self.model_name,
            model_version=self.model_version,
            pipeline_version=self.pipeline_version,
            prompt_version=self.prompt_version,
            db_pool=self.db_pool,
            sink=self.sink,
            background=self.background,
        )(func)


# Canonical alias for async context manager
track_ai_run = RunTrackerContext


def tracked_ai_run(
    feature: str,
    entity_type: str,
    entity_id: str,
    model_name: str,
    model_version: str,
    pipeline_version: str,
    prompt_version: Optional[str] = None,
    db_pool: Optional[Any] = None,
    sink: Optional[Union[list[AIRunRecord], Callable[[AIRunRecord], Any]]] = None,
    background: bool = False,
) -> Callable[..., Any]:
    """Decorator to track async functions with AI run tracking."""

    def decorator(func: Callable[..., Any]) -> Callable[..., Any]:
        @functools.wraps(func)
        async def wrapper(*args: Any, **kwargs: Any) -> Any:
            input_data = kwargs if kwargs else (args[0] if args else {})
            async with RunTrackerContext(
                feature=feature,
                entity_type=entity_type,
                entity_id=entity_id,
                model_name=model_name,
                model_version=model_version,
                pipeline_version=pipeline_version,
                input_data=input_data,
                prompt_version=prompt_version,
                db_pool=db_pool,
                sink=sink,
                background=background,
            ) as tracker:
                result = await func(*args, **kwargs)
                tracker.set_output(result)
                return result

        return wrapper

    return decorator
