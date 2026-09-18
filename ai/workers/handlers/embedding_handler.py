"""Asynchronous handler for generating and indexing vector embeddings."""

import asyncio
import logging
from typing import Any, Optional

from ai.models.embeddings.provider import (
    BaseEmbeddingProvider,
    compute_content_hash,
    get_embedding_provider,
)

logger = logging.getLogger("ai.workers.embedding")


class EmbeddingJobHandler:
    """Processes asynchronous embedding generation requests and indexes vectors in pgvector."""

    def __init__(
        self,
        embedding_provider: Optional[BaseEmbeddingProvider] = None,
        db_pool=None,
    ) -> None:
        self.provider = embedding_provider or get_embedding_provider()
        self.db_pool = db_pool

    async def handle(self, job: Any) -> dict[str, Any]:
        """Generate embedding vector, content hash, and save to ai_embeddings table."""
        payload = job.payload or {}
        text = payload.get("text", "")
        if not text:
            # Fallback to title/description if present
            title = payload.get("title", "")
            description = payload.get("description", "")
            text = f"{title} {description}".strip()

        if not text:
            raise ValueError("Empty text payload for embedding generation")

        content_hash = compute_content_hash(text)
        vector = await asyncio.to_thread(self.provider.embed, text)

        if self.db_pool is not None:
            try:
                async with self.db_pool.acquire() as conn:
                    # Format vector as string for pgvector '[0.1, 0.2, ...]'
                    vec_str = f"[{','.join(f'{x:.6f}' for x in vector)}]"
                    await conn.execute(
                        """
                        INSERT INTO ai_embeddings (
                            entity_type, entity_id, embedding_type, embedding, content_hash, model_name, model_version, created_at, updated_at
                        )
                        VALUES ($1, $2, 'semantic', $3::vector, $4, $5, $6, NOW(), NOW())
                        ON CONFLICT (entity_type, entity_id, embedding_type, model_name, model_version) DO UPDATE SET
                            embedding = EXCLUDED.embedding,
                            content_hash = EXCLUDED.content_hash,
                            updated_at = NOW();
                        """,
                        job.entity_type,
                        job.entity_id,
                        vec_str,
                        content_hash,
                        self.provider.model_name,
                        self.provider.model_version,
                    )
            except Exception as exc:
                logger.error("Failed to persist embedding to PostgreSQL: %s", exc)
                raise

        return {
            "content_hash": content_hash,
            "dimension": len(vector),
            "embedding": vector,
            "model_name": self.provider.model_name,
        }
