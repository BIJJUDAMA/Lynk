import hashlib
from unittest.mock import AsyncMock

import numpy as np
import pytest
from ai.pipelines.cache import (
    compute_content_hash,
    get_cached_embedding,
    set_cached_embedding,
)


def test_compute_content_hash_deterministic():
    """Content hash must be identical regardless of whitespace and case."""
    text1 = "  Full-Stack Developer with React and Python skills.  \n"
    text2 = "full-stack developer with react and python skills."

    hash1 = compute_content_hash(text1)
    hash2 = compute_content_hash(text2)
    assert hash1 == hash2
    assert len(hash1) == 64


def test_compute_content_hash_length():
    """SHA-256 hex digest is always 64 characters."""
    h = compute_content_hash("hello world")
    assert len(h) == 64


def test_compute_content_hash_is_sha256():
    """Verify hash matches direct SHA-256 computation."""
    text = "hello world"
    expected = hashlib.sha256(text.encode("utf-8")).hexdigest()
    assert compute_content_hash(text) == expected


def test_compute_content_hash_empty():
    """Empty string produces a valid 64-char hash."""
    h = compute_content_hash("")
    assert len(h) == 64


@pytest.mark.asyncio
async def test_get_cached_embedding_none_pool():
    """Returns None immediately when database pool is None."""
    result = await get_cached_embedding(None, "hash123", "model_v1")
    assert result is None


@pytest.mark.asyncio
async def test_get_cached_embedding_cache_miss():
    """Returns None when record is not found in database."""
    mock_pool = AsyncMock()
    mock_pool.fetchrow.return_value = None

    result = await get_cached_embedding(mock_pool, "hash123", "model_v1")
    assert result is None
    mock_pool.fetchrow.assert_awaited_once_with(
        "SELECT embedding FROM embedding_cache WHERE content_hash = $1 AND model_name = $2",
        "hash123",
        "model_v1",
    )


@pytest.mark.asyncio
async def test_get_cached_embedding_none_embedding_value():
    """Returns None when record exists but embedding column is None."""
    mock_pool = AsyncMock()
    mock_pool.fetchrow.return_value = {"embedding": None}

    result = await get_cached_embedding(mock_pool, "hash123", "model_v1")
    assert result is None


@pytest.mark.asyncio
async def test_get_cached_embedding_stringified_json():
    """Safely deserializes stringified JSON vector."""
    mock_pool = AsyncMock()
    mock_pool.fetchrow.return_value = {"embedding": "[0.12, 0.34, 0.56]"}

    result = await get_cached_embedding(mock_pool, "hash123", "model_v1")
    assert result == [0.12, 0.34, 0.56]
    assert all(isinstance(x, float) for x in result)


@pytest.mark.asyncio
async def test_get_cached_embedding_list():
    """Safely handles native Python list."""
    mock_pool = AsyncMock()
    mock_pool.fetchrow.return_value = {"embedding": [0.1, 0.2, 0.3]}

    result = await get_cached_embedding(mock_pool, "hash123", "model_v1")
    assert result == [0.1, 0.2, 0.3]
    assert all(isinstance(x, float) for x in result)


@pytest.mark.asyncio
async def test_get_cached_embedding_tuple():
    """Safely handles tuple of numeric values."""
    mock_pool = AsyncMock()
    mock_pool.fetchrow.return_value = {"embedding": (0.4, 0.5, 0.6)}

    result = await get_cached_embedding(mock_pool, "hash123", "model_v1")
    assert result == [0.4, 0.5, 0.6]
    assert all(isinstance(x, float) for x in result)


@pytest.mark.asyncio
async def test_get_cached_embedding_numpy_array():
    """Safely handles numpy array via tolist()."""
    mock_pool = AsyncMock()
    mock_pool.fetchrow.return_value = {"embedding": np.array([0.7, 0.8, 0.9])}

    result = await get_cached_embedding(mock_pool, "hash123", "model_v1")
    assert result == [0.7, 0.8, 0.9]
    assert all(isinstance(x, float) for x in result)


@pytest.mark.asyncio
async def test_get_cached_embedding_malformed_string():
    """Returns None when string vector cannot be parsed."""
    mock_pool = AsyncMock()
    mock_pool.fetchrow.return_value = {"embedding": "not_a_valid_vector"}

    result = await get_cached_embedding(mock_pool, "hash123", "model_v1")
    assert result is None


@pytest.mark.asyncio
async def test_get_cached_embedding_db_exception():
    """Returns None when fetchrow raises an exception."""
    mock_pool = AsyncMock()
    mock_pool.fetchrow.side_effect = RuntimeError("Database connection reset")

    result = await get_cached_embedding(mock_pool, "hash123", "model_v1")
    assert result is None


@pytest.mark.asyncio
async def test_set_cached_embedding_none_pool():
    """Does not raise when database pool is None."""
    await set_cached_embedding(None, "hash123", "model_v1", 384, [0.1, 0.2])


@pytest.mark.asyncio
async def test_set_cached_embedding_list():
    """Persists vector with vector cast and composite conflict target."""
    mock_pool = AsyncMock()
    mock_pool.execute.return_value = "INSERT 0 1"

    await set_cached_embedding(mock_pool, "hash123", "model_v1", 3, [0.1, 0.2, 0.3])
    mock_pool.execute.assert_awaited_once()

    call_args = mock_pool.execute.call_args[0]
    query = call_args[0]
    content_hash = call_args[1]
    model_name = call_args[2]
    dimensions = call_args[3]
    vec_str = call_args[4]

    assert "$4::vector" in query
    assert "ON CONFLICT (content_hash, model_name) DO NOTHING" in query
    assert content_hash == "hash123"
    assert model_name == "model_v1"
    assert dimensions == 3
    assert vec_str == "[0.1,0.2,0.3]"


@pytest.mark.asyncio
async def test_set_cached_embedding_numpy_array():
    """Handles numpy array serialization in set_cached_embedding."""
    mock_pool = AsyncMock()
    mock_pool.execute.return_value = "INSERT 0 1"

    await set_cached_embedding(
        mock_pool, "hash456", "model_v2", 2, np.array([0.5, 0.6])
    )
    mock_pool.execute.assert_awaited_once()

    call_args = mock_pool.execute.call_args[0]
    assert call_args[4] == "[0.5,0.6]"
