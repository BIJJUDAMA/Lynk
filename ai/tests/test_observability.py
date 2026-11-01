# 100% pure 7-bit ASCII
import pytest
from httpx import ASGITransport, AsyncClient

from ai.app.main import app


@pytest.mark.asyncio
async def test_metrics_endpoint():
    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as client:
        # Trigger an endpoint to record metrics
        res = await client.get("/health")
        assert res.status_code == 200

        # Now query /metrics
        metrics_res = await client.get("/metrics")
        assert metrics_res.status_code == 200
        assert "text/plain" in metrics_res.headers.get("content-type", "")

        body = metrics_res.text
        assert "ai_requests_total" in body
        assert "ai_active_requests" in body
        assert "ai_request_duration_seconds_bucket" in body
        assert "ai_request_duration_seconds_count" in body


@pytest.mark.asyncio
async def test_traceparent_propagation():
    incoming_trace = "4bf92f3577b34da6a3ce929d0e0e4736"
    incoming_span = "00f067aa0ba902b7"
    incoming_header = f"00-{incoming_trace}-{incoming_span}-01"

    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as client:
        # 1. Request with traceparent
        res = await client.get("/health", headers={"traceparent": incoming_header})
        assert res.status_code == 200
        assert res.headers.get("X-Trace-ID") == incoming_trace
        assert res.headers.get("X-Correlation-ID") == incoming_trace
        outgoing_tp = res.headers.get("traceparent", "")
        assert outgoing_tp.startswith(f"00-{incoming_trace}-")
        assert not outgoing_tp.startswith(f"00-{incoming_trace}-{incoming_span}-")

        # 2. Request without traceparent (auto-generation)
        res_auto = await client.get("/health")
        assert res_auto.status_code == 200
        auto_trace = res_auto.headers.get("X-Trace-ID")
        assert auto_trace is not None
        assert len(auto_trace) == 32
        assert res_auto.headers.get("traceparent") is not None
