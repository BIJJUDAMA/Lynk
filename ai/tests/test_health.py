import pytest
from httpx import ASGITransport, AsyncClient
from unittest.mock import AsyncMock, MagicMock

from ai.app.main import app, lifespan
from ai.app.config import get_settings


@pytest.mark.asyncio
async def test_health_endpoint():
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/health")
        assert response.status_code == 200
        assert response.json() == {"status": "ok", "service": "lynk-ai"}
        assert "x-correlation-id" in response.headers
        assert len(response.headers["x-correlation-id"]) > 0


@pytest.mark.asyncio
async def test_health_with_custom_correlation_id():
    transport = ASGITransport(app=app)
    custom_id = "test-correlation-id-12345"
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/health", headers={"X-Correlation-ID": custom_id})
        assert response.status_code == 200
        assert response.headers.get("x-correlation-id") == custom_id


@pytest.mark.asyncio
async def test_ready_endpoint_disconnected():
    transport = ASGITransport(app=app)
    app.state.db_pool = None
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/ready")
        assert response.status_code == 503
        assert response.json() == {"status": "not_ready", "database": "disconnected"}


@pytest.mark.asyncio
async def test_ready_endpoint_connected():
    transport = ASGITransport(app=app)
    mock_pool = MagicMock()
    mock_conn = AsyncMock()
    mock_conn.execute.return_value = "SELECT 1"
    mock_pool.acquire.return_value.__aenter__.return_value = mock_conn
    mock_pool.acquire.return_value.__aexit__.return_value = None

    app.state.db_pool = mock_pool
    try:
        async with AsyncClient(transport=transport, base_url="http://test") as client:
            response = await client.get("/ready")
            assert response.status_code == 200
            assert response.json() == {"status": "ready", "database": "connected"}
    finally:
        app.state.db_pool = None


@pytest.mark.asyncio
async def test_ready_endpoint_query_failure():
    transport = ASGITransport(app=app)
    mock_pool = MagicMock()
    mock_conn = AsyncMock()
    mock_conn.execute.side_effect = RuntimeError("Database query failure")
    mock_pool.acquire.return_value.__aenter__.return_value = mock_conn
    mock_pool.acquire.return_value.__aexit__.return_value = None

    app.state.db_pool = mock_pool
    try:
        async with AsyncClient(transport=transport, base_url="http://test") as client:
            response = await client.get("/ready")
            assert response.status_code == 503
            assert response.json() == {"status": "not_ready", "database": "disconnected"}
    finally:
        app.state.db_pool = None


@pytest.mark.asyncio
async def test_internal_ping_missing_secret():
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/internal/v1/ping")
        assert response.status_code == 401
        assert response.json() == {"detail": "Invalid or missing internal AI secret"}


@pytest.mark.asyncio
async def test_internal_ping_invalid_secret():
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get(
            "/internal/v1/ping",
            headers={"X-Internal-AI-Secret": "wrong-secret-value"}
        )
        assert response.status_code == 401
        assert response.json() == {"detail": "Invalid or missing internal AI secret"}


@pytest.mark.asyncio
async def test_internal_ping_valid_secret():
    transport = ASGITransport(app=app)
    settings = get_settings()
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get(
            "/internal/v1/ping",
            headers={"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET}
        )
        assert response.status_code == 200
        assert response.json() == {"status": "pong"}
        assert "x-correlation-id" in response.headers


@pytest.mark.asyncio
async def test_lifespan_degraded_startup():
    async with lifespan(app):
        assert hasattr(app.state, "db_pool")
