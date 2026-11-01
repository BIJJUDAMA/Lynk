# 100% pure 7-bit ASCII
import os
import re

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response

TRACEPARENT_REGEX = re.compile(r"^00-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})$")


def generate_trace_id() -> str:
    return os.urandom(16).hex()


def generate_span_id() -> str:
    return os.urandom(8).hex()


class TracingMiddleware(BaseHTTPMiddleware):
    """Extracts or generates W3C TraceContext and X-Correlation-ID for requests."""

    async def dispatch(self, request: Request, call_next) -> Response:
        traceparent = request.headers.get("traceparent")
        raw_correlation_id = request.headers.get("X-Correlation-ID")
        trace_id = None
        parent_span_id = None

        if traceparent:
            match = TRACEPARENT_REGEX.match(traceparent.strip().lower())
            if match:
                trace_id = match.group(1)
                parent_span_id = match.group(2)

        if not trace_id and raw_correlation_id:
            clean_corr = raw_correlation_id.replace("-", "").lower()
            if len(clean_corr) == 32 and all(
                c in "0123456789abcdef" for c in clean_corr
            ):
                trace_id = clean_corr

        if not trace_id:
            trace_id = generate_trace_id()

        correlation_id = raw_correlation_id if raw_correlation_id else trace_id
        span_id = generate_span_id()
        outgoing_traceparent = f"00-{trace_id}-{span_id}-01"

        request.state.trace_id = trace_id
        request.state.span_id = span_id
        request.state.parent_span_id = parent_span_id
        request.state.correlation_id = correlation_id

        response: Response = await call_next(request)
        response.headers["traceparent"] = outgoing_traceparent
        response.headers["X-Trace-ID"] = trace_id
        response.headers["X-Span-ID"] = span_id
        response.headers["X-Correlation-ID"] = correlation_id
        return response
