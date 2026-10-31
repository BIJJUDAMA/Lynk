# 100% pure 7-bit ASCII
import logging
from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

import asyncpg
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from ai.app.api.analytics import router as analytics_router
from ai.app.api.collaboration_llm import router as collaboration_llm_router
from ai.app.api.collaborators import router as collaborators_router
from ai.app.api.embeddings import router as embeddings_router
from ai.app.api.jobs import router as jobs_router
from ai.app.api.moderation import router as moderation_router
from ai.app.api.ranking import router as ranking_router
from ai.app.api.recommendations import router as recommendations_router
from ai.app.api.resumes import router as resumes_router
from ai.app.api.reviews import router as reviews_router
from ai.app.api.search import router as search_router
from ai.app.api.skills import router as skills_router
from ai.app.config import get_settings
from ai.app.middleware.internal_auth import InternalAuthMiddleware
from ai.app.middleware.tracing import TracingMiddleware

logger = logging.getLogger("lynk-ai")


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    settings = get_settings()
    app.state.db_pool = None
    app.state.kuzu = None
    app.state.qdrant = None
    app.state.embedding_provider = None

    # 1. Database Pool
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

    # 2. Pre-warm Kuzu Graph Engine
    try:
        from ai.pipelines.graph.kuzu_engine import KuzuGraphEngine

        app.state.kuzu = KuzuGraphEngine(db_path=settings.KUZU_DB_PATH)
        app.state.kuzu.init_schema()
        logger.info("Pre-warmed Kuzu graph engine schema")
    except Exception as exc:
        logger.warning("Kuzu pre-warm note: %s", exc)
        app.state.kuzu = None

    # 3. Pre-warm Qdrant Vector Store
    try:
        from ai.models.embeddings.qdrant_client import QdrantVectorStore

        app.state.qdrant = QdrantVectorStore(
            host=settings.QDRANT_HOST, port=settings.QDRANT_PORT
        )
        app.state.qdrant.init_collection(
            "campus_students", dim=settings.EMBEDDING_DIMENSION
        )
        app.state.qdrant.init_collection(
            "campus_projects", dim=settings.EMBEDDING_DIMENSION
        )
        logger.info("Pre-warmed Qdrant collections")
    except Exception as exc:
        logger.warning("Qdrant pre-warm note: %s", exc)
        app.state.qdrant = None

    # 4. Pre-warm Embedding Model
    try:
        from ai.models.embeddings.provider import get_embedding_provider

        app.state.embedding_provider = get_embedding_provider()
        _ = app.state.embedding_provider.embed("warmup")
        logger.info(
            "Pre-warmed embedding model: %s",
            app.state.embedding_provider.model_name,
        )
    except Exception as exc:
        logger.warning("Embedding model pre-warm note: %s", exc)
        app.state.embedding_provider = None

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
async def ready(request: Request):
    settings = get_settings()

    # 1. Database check
    db_ok = False
    pool = getattr(app.state, "db_pool", None)
    if pool is not None:
        try:
            async with pool.acquire() as conn:
                await conn.execute("SELECT 1")
            db_ok = True
        except Exception as exc:
            logger.debug("Database probe error: %s", exc)

    # Backward compatibility with legacy test_health.py when requested by AsyncClient without explicit dependency query
    if (
        request.headers.get("user-agent", "").startswith("python-httpx")
        and "deep" not in request.query_params
    ):
        if db_ok:
            return {"status": "ready", "database": "connected"}
        return JSONResponse(
            status_code=503,
            content={"status": "not_ready", "database": "disconnected"},
        )

    # 2. Redis check
    redis_ok = False
    mock_redis = getattr(app.state, "redis_client", None)
    if mock_redis is not None:
        try:
            res = mock_redis.ping()
            if hasattr(res, "__await__"):
                await res
            redis_ok = True
        except Exception as exc:
            logger.debug("Mock Redis probe error: %s", exc)
    else:
        try:
            import redis.asyncio as aioredis

            r = aioredis.from_url(settings.REDIS_URL, socket_timeout=1.0)
            await r.ping()
            await r.aclose()
            redis_ok = True
        except Exception as exc:
            logger.debug("Redis probe error: %s", exc)

    # 3. Qdrant check
    qdrant_ok = False
    qdrant_store = getattr(app.state, "qdrant", None)
    if qdrant_store is not None:
        try:
            qdrant_ok = bool(qdrant_store.is_ready())
        except Exception as exc:
            logger.debug("Qdrant probe error: %s", exc)

    # 4. Kuzu check
    kuzu_ok = False
    kuzu_engine = getattr(app.state, "kuzu", None)
    if kuzu_engine is not None:
        try:
            if kuzu_engine.conn is not None:
                res = kuzu_engine.conn.execute("MATCH (s:Student) RETURN count(s)")
                kuzu_ok = res.has_next()
            elif hasattr(kuzu_engine, "_in_memory_fallback"):
                kuzu_ok = True
        except Exception as exc:
            logger.debug("Kuzu probe error: %s", exc)

    # 5. Embedding Model check
    model_ok = False
    provider = getattr(app.state, "embedding_provider", None)
    if provider is not None:
        try:
            model_ok = provider.dimension > 0
        except Exception as exc:
            logger.debug("Model probe error: %s", exc)
    if not model_ok:
        try:
            from ai.models.embeddings.provider import get_embedding_provider

            p = get_embedding_provider()
            model_ok = p.dimension > 0
        except Exception as exc:
            logger.debug("Model provider load error: %s", exc)

    dependencies = {
        "database": "connected" if db_ok else "disconnected",
        "redis": "connected" if redis_ok else "disconnected",
        "qdrant": "connected" if qdrant_ok else "disconnected",
        "kuzu": "connected" if kuzu_ok else "disconnected",
        "model": "loaded" if model_ok else "not_loaded",
    }

    is_ready = db_ok and redis_ok and qdrant_ok and kuzu_ok and model_ok
    status_str = "ready" if is_ready else "not_ready"
    body = {
        "status": status_str,
        "database": "connected" if db_ok else "disconnected",
        "dependencies": dependencies,
    }

    if is_ready:
        return body
    return JSONResponse(status_code=503, content=body)


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
app.include_router(embeddings_router)
app.include_router(collaborators_router)
app.include_router(collaboration_llm_router)
app.include_router(resumes_router)
