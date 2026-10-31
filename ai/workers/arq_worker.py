"""ARQ Worker configuration and lifecycle settings.

100% pure 7-bit ASCII
"""

from typing import Any
from arq.connections import RedisSettings
from ai.workers.tasks import (
    process_uploaded_resume_task,
    sync_student_vectors_task,
    update_kuzu_graph_edge_task,
)
from ai.models.embeddings.qdrant_client import QdrantVectorStore
from ai.pipelines.graph.kuzu_engine import KuzuGraphEngine


async def startup(ctx: dict[str, Any]) -> None:
    """Initialize shared connections for worker process."""
    ctx["qdrant_store"] = QdrantVectorStore(in_memory=True)
    ctx["kuzu_engine"] = KuzuGraphEngine()


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
