# 100% pure 7-bit ASCII
import time
from collections import defaultdict
from threading import Lock

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response

DEFAULT_BUCKETS = (0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0)


class MetricsRegistry:
    def __init__(self):
        self.lock = Lock()
        self.requests_total: dict[tuple[str, str, int], int] = defaultdict(int)
        self.active_requests: int = 0
        self.duration_buckets: dict[float, int] = defaultdict(int)
        self.duration_inf: int = 0
        self.duration_sum: float = 0.0
        self.duration_count: int = 0

    def record(self, method: str, endpoint: str, status: int, duration: float) -> None:
        with self.lock:
            key = (method.upper(), endpoint, status)
            self.requests_total[key] += 1
            self.duration_sum += duration
            self.duration_count += 1
            for b in DEFAULT_BUCKETS:
                if duration <= b:
                    self.duration_buckets[b] += 1
            self.duration_inf += 1

    def generate(self) -> str:
        lines: list[str] = []
        with self.lock:
            # 1. ai_requests_total
            lines.append(
                "# HELP ai_requests_total Total requests processed by AI subsystem."
            )
            lines.append("# TYPE ai_requests_total counter")
            for (method, endpoint, status), count in sorted(
                self.requests_total.items()
            ):
                lines.append(
                    f'ai_requests_total{{method="{method}",endpoint="{endpoint}",status="{status}"}} {count}'
                )

            # 2. ai_active_requests
            lines.append(
                "# HELP ai_active_requests Currently in-flight requests in AI subsystem."
            )
            lines.append("# TYPE ai_active_requests gauge")
            lines.append(f"ai_active_requests {self.active_requests}")

            # 3. ai_request_duration_seconds
            lines.append(
                "# HELP ai_request_duration_seconds Latency of AI subsystem requests in seconds."
            )
            lines.append("# TYPE ai_request_duration_seconds histogram")
            for b in DEFAULT_BUCKETS:
                count = self.duration_buckets[b]
                lines.append(f'ai_request_duration_seconds_bucket{{le="{b}"}} {count}')
            lines.append(
                f'ai_request_duration_seconds_bucket{{le="+Inf"}} {self.duration_inf}'
            )
            lines.append(f"ai_request_duration_seconds_sum {self.duration_sum:.6f}")
            lines.append(f"ai_request_duration_seconds_count {self.duration_count}")

        return "\n".join(lines) + "\n"


registry = MetricsRegistry()


class MetricsMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next) -> Response:
        if request.url.path == "/metrics":
            return await call_next(request)

        with registry.lock:
            registry.active_requests += 1

        start_time = time.perf_counter()
        status_code = 500
        try:
            response: Response = await call_next(request)
            status_code = response.status_code
            return response
        finally:
            duration = time.perf_counter() - start_time
            with registry.lock:
                registry.active_requests = max(0, registry.active_requests - 1)
            # Normalize path (e.g., /health, /ready, /v1/ai/recommendations)
            path = request.url.path
            registry.record(request.method, path, status_code, duration)
