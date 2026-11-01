# 100% pure 7-bit ASCII
import hashlib
import json
import logging
import threading
from typing import Any

from cachetools import LRUCache

logger = logging.getLogger(__name__)

# L1 In-Memory Thread-Safe LRU Cache (max 10,000 entries)
_L1_MAX_SIZE = 10000
_L1_CACHE: LRUCache = LRUCache(maxsize=_L1_MAX_SIZE)
_L1_LOCK = threading.Lock()


def clear_l1_cache() -> None:
    """Clear all entries from the in-memory L1 LRU cache."""
    with _L1_LOCK:
        _L1_CACHE.clear()


def compute_content_hash(text: str) -> str:
    """Normalize text and compute a stable SHA-256 hex digest.

    Normalization: lowercase, collapse all whitespace to single spaces, strip.
    Two semantically equivalent strings (differing only in case or whitespace)
    will produce the same hash.
    """
    normalized = " ".join(text.lower().strip().split())
    return hashlib.sha256(normalized.encode("utf-8")).hexdigest()


def _parse_embedding_raw(raw: Any) -> list[float] | None:
    """Safely parse raw embedding value into a list of floats."""
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

    if isinstance(raw, list | tuple):
        try:
            return [float(x) for x in raw]
        except (ValueError, TypeError):
            return None

    return None


async def get_cached_embedding(
    pool: Any, content_hash: str, model_name: str
) -> list[float] | None:
    """Retrieve an embedding from L1 RAM cache or L2 PostgreSQL embedding_cache table.

    Returns a list of floats if found in L1 or L2, or None on cache miss.
    On L2 hit, automatically backfills L1 for subsequent sub-microsecond retrieval.
    """
    key = (content_hash, model_name)

    # 1. Check L1 Memory Cache (fast path, <1us)
    with _L1_LOCK:
        if key in _L1_CACHE:
            return list(_L1_CACHE[key])

    # 2. Check L2 Database Cache
    if pool is None:
        return None

    try:
        row = await pool.fetchrow(
            "SELECT embedding FROM embedding_cache WHERE content_hash = $1 AND model_name = $2",
            content_hash,
            model_name,
        )
    except Exception as exc:
        logger.warning("Failed to fetch cached embedding from database: %s", exc)
        return None

    if not row:
        return None

    try:
        raw = row["embedding"]
    except (KeyError, TypeError, IndexError):
        raw = getattr(row, "embedding", None)

    vec = _parse_embedding_raw(raw)
    if vec is not None:
        with _L1_LOCK:
            _L1_CACHE[key] = vec
        return list(vec)

    return None


async def set_cached_embedding(
    pool: Any,
    content_hash: str,
    model_name: str,
    dimensions: int,
    embedding: Any,
) -> None:
    """Persist an embedding to L1 RAM cache and the L2 embedding_cache PostgreSQL table.

    Uses INSERT ... ON CONFLICT (content_hash, model_name) DO NOTHING for idempotent upserts.
    """
    parsed = _parse_embedding_raw(embedding)
    if parsed is not None:
        key = (content_hash, model_name)
        with _L1_LOCK:
            _L1_CACHE[key] = parsed

    if pool is None:
        return

    if hasattr(embedding, "tolist") and callable(embedding.tolist):
        embedding = embedding.tolist()

    if isinstance(embedding, str):
        vec_str = embedding
    elif isinstance(embedding, list | tuple):
        vec_str = "[" + ",".join(str(float(x)) for x in embedding) + "]"
    elif parsed is not None:
        vec_str = "[" + ",".join(str(float(x)) for x in parsed) + "]"
    else:
        return

    try:
        await pool.execute(
            """
            INSERT INTO embedding_cache (content_hash, model_name, dimensions, embedding)
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (content_hash, model_name) DO NOTHING
            """,
            content_hash,
            model_name,
            dimensions,
            vec_str,
        )
    except Exception as exc:
        logger.warning("Failed to persist embedding to database: %s", exc)


async def get_cached_embeddings_batch(
    pool: Any,
    texts: list[str],
    model_name: str,
    provider: Any,
) -> list[list[float]]:
    """Retrieve embeddings for a batch of texts using 2-tier cache optimization.

    1. Checks L1 RAM cache.
    2. Gathers L1 misses and queries L2 (PostgreSQL) in a single batch query.
    3. Gathers remaining misses and runs provider.embed_batch in a single vectorized pass.
    4. Backfills newly computed vectors to L1 and L2.
    5. Returns vectors preserving the original text ordering.
    """
    if not texts:
        return []

    count = len(texts)
    results: list[list[float] | None] = [None] * count
    hashes = [compute_content_hash(t) for t in texts]

    # 1. L1 RAM lookup
    missing_indices: list[int] = []
    with _L1_LOCK:
        for idx, h in enumerate(hashes):
            key = (h, model_name)
            if key in _L1_CACHE:
                results[idx] = list(_L1_CACHE[key])
            else:
                missing_indices.append(idx)

    if not missing_indices:
        return [r for r in results if r is not None]

    # 2. L2 PostgreSQL lookup for missing indices
    if pool is not None and missing_indices:
        missing_hashes = list({hashes[i] for i in missing_indices})
        try:
            rows = await pool.fetch(
                """
                SELECT content_hash, embedding
                FROM embedding_cache
                WHERE model_name = $1 AND content_hash = ANY($2::text[])
                """,
                model_name,
                missing_hashes,
            )
            l2_hits: dict[str, list[float]] = {}
            for row in rows:
                parsed = _parse_embedding_raw(row["embedding"])
                if parsed:
                    l2_hits[row["content_hash"]] = parsed
                    with _L1_LOCK:
                        _L1_CACHE[(row["content_hash"], model_name)] = parsed

            still_missing: list[int] = []
            for i in missing_indices:
                h = hashes[i]
                if h in l2_hits:
                    results[i] = list(l2_hits[h])
                else:
                    still_missing.append(i)
            missing_indices = still_missing
        except Exception as exc:
            logger.warning("Batch L2 cache lookup failed: %s", exc)

    # 3. Model batch inference for remaining misses
    if missing_indices:
        texts_to_embed = [texts[i] for i in missing_indices]
        new_vectors = provider.embed_batch(texts_to_embed)

        for i, vec in zip(missing_indices, new_vectors, strict=False):
            results[i] = vec
            h = hashes[i]
            # Backfill L1 and L2
            with _L1_LOCK:
                _L1_CACHE[(h, model_name)] = vec
            if pool is not None:
                dim = len(vec)
                vec_str = "[" + ",".join(str(float(x)) for x in vec) + "]"
                try:
                    await pool.execute(
                        """
                        INSERT INTO embedding_cache (content_hash, model_name, dimensions, embedding)
                        VALUES ($1, $2, $3, $4)
                        ON CONFLICT (content_hash, model_name) DO NOTHING
                        """,
                        h,
                        model_name,
                        dim,
                        vec_str,
                    )
                except Exception as exc:
                    logger.debug("Async backfill L2 error: %s", exc)

    return [r for r in results if r is not None]
