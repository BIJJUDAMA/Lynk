"""Qdrant Vector Database Client Wrapper with in-memory test fallback.

100% pure 7-bit ASCII
"""

import logging
import uuid
from typing import Any

logger = logging.getLogger("lynk-ai.qdrant")

try:
    from qdrant_client import QdrantClient
    from qdrant_client.http import models as qmodels

    QDRANT_AVAILABLE = True
except ImportError:
    QDRANT_AVAILABLE = False
    QdrantClient = None  # type: ignore[assignment,misc,no-redef]
    qmodels = None  # type: ignore[assignment,misc,no-redef]


def to_qdrant_id(point_id: str) -> str:
    """Ensure point_id is a valid RFC 4122 UUID string for Qdrant compatibility."""
    try:
        return str(uuid.UUID(point_id))
    except (ValueError, AttributeError):
        # Generate deterministic UUIDv5 from namespace and point_id string
        return str(uuid.uuid5(uuid.NAMESPACE_DNS, f"lynk.campus.{point_id}"))


class QdrantVectorStore:
    """Client for Qdrant vector database supporting collections and ANN cosine search."""

    def __init__(
        self,
        host: str = "localhost",
        port: int = 6333,
        in_memory: bool = False,
    ) -> None:
        self.host = host
        self.port = port
        self.in_memory = in_memory
        self._collections: dict[str, dict[str, Any]] = {}

        if QDRANT_AVAILABLE:
            if in_memory:
                self.client = QdrantClient(":memory:")
            else:
                self.client = QdrantClient(host=host, port=port)
        else:
            self.client = None

    def is_ready(self) -> bool:
        """Check if vector store is ready."""
        if self.in_memory:
            return True
        if self.client:
            try:
                self.client.get_collections()
                return True
            except Exception as e:
                logger.warning("Qdrant healthcheck failed: %s", e)
                return False
        return True

    def init_collection(self, collection_name: str, dim: int = 384) -> None:
        """Initialize collection if not already existing."""
        if self.client and QDRANT_AVAILABLE:
            collections = [c.name for c in self.client.get_collections().collections]
            if collection_name not in collections:
                try:
                    self.client.create_collection(
                        collection_name=collection_name,
                        vectors_config=qmodels.VectorParams(
                            size=dim, distance=qmodels.Distance.COSINE
                        ),
                    )
                except Exception as exc:
                    if "already exists" not in str(exc).lower():
                        logger.warning(
                            "Failed to create collection %s: %s", collection_name, exc
                        )
        else:
            self._collections[collection_name] = {"dim": dim, "points": {}}

    def upsert(
        self,
        collection_name: str,
        point_id: str,
        vector: list[float],
        payload: dict[str, Any] | None = None,
    ) -> None:
        """Upsert a single vector point with payload."""
        data_payload = payload or {}
        # Preserve original entity ID in payload
        data_payload["_entity_id"] = point_id
        qid = to_qdrant_id(point_id)

        if self.client and QDRANT_AVAILABLE:
            self.client.upsert(
                collection_name=collection_name,
                points=[
                    qmodels.PointStruct(
                        id=qid,
                        vector=vector,
                        payload=data_payload,
                    )
                ],
            )
        else:
            if collection_name not in self._collections:
                self.init_collection(collection_name, len(vector))
            self._collections[collection_name]["points"][point_id] = {
                "vector": vector,
                "payload": data_payload,
            }

    def search(
        self,
        collection_name: str,
        query_vector: list[float],
        limit: int = 10,
    ) -> list[dict[str, Any]]:
        """Perform cosine similarity search against collection."""
        if self.client and QDRANT_AVAILABLE:
            # Modern Qdrant 1.10+ uses query_points
            search_response = self.client.query_points(
                collection_name=collection_name,
                query=query_vector,
                limit=limit,
            )
            points = getattr(search_response, "points", search_response)
            return [
                {
                    "id": (hit.payload or {}).get("_entity_id", str(hit.id)),
                    "qdrant_id": str(hit.id),
                    "score": float(hit.score),
                    "payload": hit.payload or {},
                }
                for hit in points
            ]
        else:
            points = self._collections.get(collection_name, {}).get("points", {})
            scored = []
            for pid, data in points.items():
                v = data["vector"]
                dot = sum(a * b for a, b in zip(query_vector, v))
                norm_q = sum(a * a for a in query_vector) ** 0.5
                norm_v = sum(a * a for a in v) ** 0.5
                sim = dot / (norm_q * norm_v) if norm_q and norm_v else 0.0
                scored.append(
                    {
                        "id": pid,
                        "score": round(sim, 4),
                        "payload": data["payload"],
                    }
                )
            scored.sort(key=lambda x: x["score"], reverse=True)
            return scored[:limit]
