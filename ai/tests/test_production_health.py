# 100% pure 7-bit ASCII
"""Unit and integration tests for production model pre-warming and deep readiness probe."""

from unittest.mock import AsyncMock, MagicMock

import pytest
from fastapi.testclient import TestClient

from ai.app.config import get_settings
from ai.app.main import app, lifespan

client = TestClient(app)


def test_liveness_health_endpoint():
    """Verify liveness probe returns 200 OK immediately."""
    resp = client.get("/health")
    assert resp.status_code == 200
    data = resp.json()
    assert data["status"] == "ok"
    assert data["service"] == "lynk-ai"


def test_readiness_probe_structure():
    """Verify deep readiness probe exposes required status and dependencies breakdown."""
    resp = client.get("/ready")
    assert resp.status_code in [200, 503]
    data = resp.json()
    assert "status" in data
    assert "dependencies" in data
    deps = data["dependencies"]
    assert "database" in deps
    assert "redis" in deps
    assert "qdrant" in deps
    assert "kuzu" in deps
    assert "model" in deps


def test_readiness_probe_all_dependencies_healthy():
    """Verify readiness returns 200 OK when all downstream components are healthy."""
    # 1. Mock DB pool
    mock_pool = MagicMock()
    mock_conn = AsyncMock()
    mock_conn.execute.return_value = "SELECT 1"
    mock_pool.acquire.return_value.__aenter__.return_value = mock_conn
    mock_pool.acquire.return_value.__aexit__.return_value = None

    # 2. Mock Redis
    mock_redis = AsyncMock()
    mock_redis.ping.return_value = True

    # 3. Mock Qdrant
    mock_qdrant = MagicMock()
    mock_qdrant.is_ready.return_value = True

    # 4. Mock Kuzu
    mock_kuzu = MagicMock()
    mock_result = MagicMock()
    mock_result.has_next.return_value = True
    mock_kuzu.conn.execute.return_value = mock_result

    # 5. Mock Embedding Provider
    mock_provider = MagicMock()
    mock_provider.dimension = 384

    original_db = getattr(app.state, "db_pool", None)
    original_redis = getattr(app.state, "redis_client", None)
    original_qdrant = getattr(app.state, "qdrant", None)
    original_kuzu = getattr(app.state, "kuzu", None)
    original_model = getattr(app.state, "embedding_provider", None)

    try:
        app.state.db_pool = mock_pool
        app.state.redis_client = mock_redis
        app.state.qdrant = mock_qdrant
        app.state.kuzu = mock_kuzu
        app.state.embedding_provider = mock_provider

        resp = client.get("/ready")
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "ready"
        assert data["database"] == "connected"
        assert data["dependencies"]["database"] == "connected"
        assert data["dependencies"]["redis"] == "connected"
        assert data["dependencies"]["qdrant"] == "connected"
        assert data["dependencies"]["kuzu"] == "connected"
        assert data["dependencies"]["model"] == "loaded"
    finally:
        app.state.db_pool = original_db
        app.state.redis_client = original_redis
        app.state.qdrant = original_qdrant
        app.state.kuzu = original_kuzu
        app.state.embedding_provider = original_model


def test_readiness_probe_when_database_fails():
    """Verify readiness returns 503 when the database pool is unavailable."""
    original_db = getattr(app.state, "db_pool", None)
    try:
        app.state.db_pool = None
        resp = client.get("/ready")
        assert resp.status_code == 503
        data = resp.json()
        assert data["status"] == "not_ready"
        assert data["dependencies"]["database"] == "disconnected"
    finally:
        app.state.db_pool = original_db


def test_readiness_probe_when_redis_fails():
    """Verify readiness returns 503 when Redis ping raises an error."""
    mock_pool = MagicMock()
    mock_conn = AsyncMock()
    mock_conn.execute.return_value = "SELECT 1"
    mock_pool.acquire.return_value.__aenter__.return_value = mock_conn
    mock_pool.acquire.return_value.__aexit__.return_value = None

    mock_redis = AsyncMock()
    mock_redis.ping.side_effect = ConnectionError("Redis unreachable")

    mock_qdrant = MagicMock()
    mock_qdrant.is_ready.return_value = True

    mock_kuzu = MagicMock()
    mock_result = MagicMock()
    mock_result.has_next.return_value = True
    mock_kuzu.conn.execute.return_value = mock_result

    mock_provider = MagicMock()
    mock_provider.dimension = 384

    original_db = getattr(app.state, "db_pool", None)
    original_redis = getattr(app.state, "redis_client", None)
    original_qdrant = getattr(app.state, "qdrant", None)
    original_kuzu = getattr(app.state, "kuzu", None)
    original_model = getattr(app.state, "embedding_provider", None)

    try:
        app.state.db_pool = mock_pool
        app.state.redis_client = mock_redis
        app.state.qdrant = mock_qdrant
        app.state.kuzu = mock_kuzu
        app.state.embedding_provider = mock_provider

        resp = client.get("/ready")
        assert resp.status_code == 503
        data = resp.json()
        assert data["status"] == "not_ready"
        assert data["dependencies"]["redis"] == "disconnected"
    finally:
        app.state.db_pool = original_db
        app.state.redis_client = original_redis
        app.state.qdrant = original_qdrant
        app.state.kuzu = original_kuzu
        app.state.embedding_provider = original_model


def test_readiness_probe_when_qdrant_fails():
    """Verify readiness returns 503 when Qdrant vector store is not ready."""
    mock_pool = MagicMock()
    mock_conn = AsyncMock()
    mock_conn.execute.return_value = "SELECT 1"
    mock_pool.acquire.return_value.__aenter__.return_value = mock_conn
    mock_pool.acquire.return_value.__aexit__.return_value = None

    mock_redis = AsyncMock()
    mock_redis.ping.return_value = True

    mock_qdrant = MagicMock()
    mock_qdrant.is_ready.return_value = False

    mock_kuzu = MagicMock()
    mock_result = MagicMock()
    mock_result.has_next.return_value = True
    mock_kuzu.conn.execute.return_value = mock_result

    mock_provider = MagicMock()
    mock_provider.dimension = 384

    original_db = getattr(app.state, "db_pool", None)
    original_redis = getattr(app.state, "redis_client", None)
    original_qdrant = getattr(app.state, "qdrant", None)
    original_kuzu = getattr(app.state, "kuzu", None)
    original_model = getattr(app.state, "embedding_provider", None)

    try:
        app.state.db_pool = mock_pool
        app.state.redis_client = mock_redis
        app.state.qdrant = mock_qdrant
        app.state.kuzu = mock_kuzu
        app.state.embedding_provider = mock_provider

        resp = client.get("/ready")
        assert resp.status_code == 503
        data = resp.json()
        assert data["status"] == "not_ready"
        assert data["dependencies"]["qdrant"] == "disconnected"
    finally:
        app.state.db_pool = original_db
        app.state.redis_client = original_redis
        app.state.qdrant = original_qdrant
        app.state.kuzu = original_kuzu
        app.state.embedding_provider = original_model


@pytest.mark.asyncio
async def test_lifespan_prewarming_components():
    """Verify lifespan startup executes pre-warming hooks for all core subsystems."""
    async with lifespan(app):
        assert hasattr(app.state, "db_pool")
        assert hasattr(app.state, "kuzu")
        assert hasattr(app.state, "qdrant")
        assert hasattr(app.state, "embedding_provider")


def test_production_config_keys():
    """Verify production settings expose Qdrant, Redis, and Kuzu parameters."""
    settings = get_settings()
    assert hasattr(settings, "QDRANT_HOST")
    assert hasattr(settings, "QDRANT_PORT")
    assert hasattr(settings, "REDIS_URL")
    assert hasattr(settings, "KUZU_DB_PATH")
    assert settings.QDRANT_HOST in {"qdrant", "localhost"}
    assert settings.QDRANT_PORT == 6333
    assert "redis" in settings.REDIS_URL
    assert "kuzu" in settings.KUZU_DB_PATH
