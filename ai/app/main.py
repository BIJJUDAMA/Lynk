import logging
from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

import asyncpg
from ai.app.api.analytics import router as analytics_router
from ai.app.api.jobs import router as jobs_router
from ai.app.api.moderation import router as moderation_router
from ai.app.api.ranking import router as ranking_router
from ai.app.api.recommendations import router as recommendations_router
from ai.app.api.reviews import router as reviews_router
from ai.app.api.search import router as search_router
from ai.app.api.skills import router as skills_router
from ai.app.config import get_settings
from ai.app.middleware.internal_auth import InternalAuthMiddleware
from ai.app.middleware.tracing import TracingMiddleware
from fastapi import FastAPI
from fastapi.responses import JSONResponse

logger = logging.getLogger("lynk-ai")


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    settings = get_settings()
    app.state.db_pool = None
    try:
        app.state.db_pool = await asyncpg.create_pool(
            dsn=settings.DATABASE_URL,
            min_size=1,
            max_size=5,
            timeout=2.0,
            command_timeout=2.0,
        )
        logger.info("Connected to PostgreSQL database pool")
    except Exception as exc:
        logger.warning(
            "Could not connect to PostgreSQL on startup (running degraded): %s",
            exc,
        )
        app.state.db_pool = None

    yield

    if getattr(app.state, "db_pool", None) is not None:
        await app.state.db_pool.close()
        logger.info("Closed PostgreSQL database pool")


app = FastAPI(
    title="Lynk AI Subsystem",
    lifespan=lifespan,
)

# Middlewares are applied in reverse order of addition:
# InternalAuthMiddleware runs inside TracingMiddleware so 401s still get X-Correlation-ID.
app.add_middleware(InternalAuthMiddleware)
app.add_middleware(TracingMiddleware)


@app.get("/health")
async def health():
    return {"status": "ok", "service": "lynk-ai"}


@app.get("/ready")
async def ready():
    pool = getattr(app.state, "db_pool", None)
    if pool is None:
        return JSONResponse(
            status_code=503,
            content={"status": "not_ready", "database": "disconnected"},
        )
    try:
        async with pool.acquire() as conn:
            await conn.execute("SELECT 1")
        return {"status": "ready", "database": "connected"}
    except Exception:
        return JSONResponse(
            status_code=503,
            content={"status": "not_ready", "database": "disconnected"},
        )


@app.get("/internal/v1/ping")
async def internal_ping():
    return {"status": "pong"}


app.include_router(skills_router)
app.include_router(search_router)
app.include_router(recommendations_router)
app.include_router(ranking_router)
app.include_router(moderation_router)
app.include_router(reviews_router)
app.include_router(analytics_router)
app.include_router(jobs_router)
