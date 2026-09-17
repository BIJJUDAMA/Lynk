from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import JSONResponse, Response

from ai.app.config import get_settings


class InternalAuthMiddleware(BaseHTTPMiddleware):
    """Verifies X-Internal-AI-Secret on /internal/... endpoints."""

    async def dispatch(self, request: Request, call_next) -> Response:
        path = request.url.path
        if path.startswith("/internal"):
            settings = get_settings()
            secret = request.headers.get("X-Internal-AI-Secret")
            if not secret or secret != settings.INTERNAL_AI_SECRET:
                return JSONResponse(
                    status_code=401,
                    content={"detail": "Invalid or missing internal AI secret"},
                )
        return await call_next(request)
