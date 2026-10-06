import hashlib
import json
import logging
from typing import Any, List, Optional

logger = logging.getLogger(__name__)


def compute_content_hash(text: str) -> str:
    """Normalize text and compute a stable SHA-256 hex digest.

    Normalization: lowercase, collapse all whitespace to single spaces, strip.
    Two semantically equivalent strings (differing only in case or whitespace)
    will produce the same hash.
    """
    normalized = " ".join(text.lower().strip().split())
    return hashlib.sha256(normalized.encode("utf-8")).hexdigest()


async def get_cached_embedding(
    pool, content_hash: str, model_name: str
) -> Optional[List[float]]:
    """Retrieve a cached embedding from the embedding_cache table.

    Returns a list of floats if found, or None on cache miss.
    Handles None pool, stringified JSON vector, list, tuple, or numpy ndarray.
    """
    if pool is None:
        return None

    try:
        row = await pool.fetchrow(
            "SELECT embedding FROM embedding_cache WHERE content_hash = $1 AND model_name = $2",
            content_hash,
            model_name,
        )
    except Exception as exc:
        logger.warning("Failed to fetch cached embedding: %s", exc)
        return None

    if not row:
        return None

    try:
        raw = row["embedding"]
    except (KeyError, TypeError, IndexError):
        raw = getattr(row, "embedding", None)

    if raw is None:
        return None

    if hasattr(raw, "tolist") and callable(raw.tolist):
        raw = raw.tolist()
    elif isinstance(raw, str):
        raw = raw.strip()
        try:
            raw = json.loads(raw)
        except Exception:
            clean = raw.strip("[]{}() \t\n\r")
            if clean:
                try:
                    return [float(x.strip()) for x in clean.split(",") if x.strip()]
                except (ValueError, TypeError):
                    return None
            return None

    if isinstance(raw, (list, tuple)):
        try:
            return [float(x) for x in raw]
        except (ValueError, TypeError):
            return None

    return None


async def set_cached_embedding(
    pool,
    content_hash: str,
    model_name: str,
    dimensions: int,
    embedding: Any,
) -> None:
    """Persist an embedding to the embedding_cache table.

    Uses INSERT ... ON CONFLICT (content_hash, model_name) DO NOTHING for idempotent upserts.
    """
    if pool is None:
        return

    if hasattr(embedding, "tolist") and callable(embedding.tolist):
        embedding = embedding.tolist()

    if isinstance(embedding, str):
        vec_str = embedding
    else:
        vec_str = "[" + ",".join(str(float(x)) for x in embedding) + "]"

    await pool.execute(
        """
        INSERT INTO embedding_cache (content_hash, model_name, dimensions, embedding)
        VALUES ($1, $2, $3, $4::vector)
        ON CONFLICT (content_hash, model_name) DO NOTHING
        """,
        content_hash,
        model_name,
        dimensions,
        vec_str,
    )

