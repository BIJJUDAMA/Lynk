"""ARQ Worker configuration and lifecycle settings.

100% pure 7-bit ASCII
"""

import os
from typing import Any

from arq.connections import RedisSettings

from ai.models.embeddings.qdrant_client import QdrantVectorStore
from ai.pipelines.graph.kuzu_engine import KuzuGraphEngine
from ai.workers.tasks import (
    process_uploaded_resume_task,
    sync_student_vectors_task,
    update_kuzu_graph_edge_task,
)


async def startup(ctx: dict[str, Any]) -> None:
    """Initialize shared connections for worker process."""
    ctx["qdrant_store"] = QdrantVectorStore(in_memory=True)
    kuzu_path = os.getenv("KUZU_DB_PATH", "/data/kuzu_worker_graph")
    ctx["kuzu_engine"] = KuzuGraphEngine(db_path=kuzu_path)


async def shutdown(ctx: dict[str, Any]) -> None:
    """Clean up worker connections."""
    pass


class WorkerSettings:
    """ARQ Worker process settings."""

    functions = [
        process_uploaded_resume_task,
        sync_student_vectors_task,
        update_kuzu_graph_edge_task,
    ]
    on_startup = startup
    on_shutdown = shutdown
    redis_settings = RedisSettings(host="redis", port=6379)
    max_jobs = 10
    poll_delay = 0.5
