"""Unit and integration tests for Generative AI Job Draft Generation."""

import httpx
import pytest
from ai.app.config import get_settings
from ai.app.main import app
from ai.llm.client import VLLMClient
from ai.llm.schemas.job_generation import GeneratedJobDraft
from httpx import ASGITransport, AsyncClient


def test_generated_job_draft_schema():
    """GeneratedJobDraft validates title, description, skills, and department."""
    draft = GeneratedJobDraft(
        title="Frontend Web Developer",
        description="Build responsive landing pages using Next.js and Tailwind.",
        required_skills=["Next.js", "Tailwind CSS", "TypeScript"],
        department="Computer Science",
    )
    assert draft.title == "Frontend Web Developer"
    assert "Next.js" in draft.required_skills
    assert draft.department == "Computer Science"


@pytest.mark.asyncio
async def test_vllm_client_fallback_generates_valid_draft():
    """VLLMClient in mock/offline mode produces a well-formed structured draft."""
    client = VLLMClient(base_url="http://mock-offline:8000/v1")
    draft = await client.generate_job_draft(
        idea="Build a React dashboard for robotics club with PostgreSQL backend",
        department="Robotics & Computer Engineering",
    )
    assert isinstance(draft, GeneratedJobDraft)
    assert len(draft.title) > 0
    assert len(draft.description) > 0
    assert len(draft.required_skills) > 0
    assert draft.department == "Robotics & Computer Engineering"


@pytest.mark.asyncio
async def test_job_generation_api_unauthorized():
    """POST /internal/v1/jobs/generate without secret returns 401."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/jobs/generate",
            json={"idea": "Need someone to build a portfolio", "department": "Art"},
        )
        assert resp.status_code == 401


@pytest.mark.asyncio
async def test_job_generation_api_success():
    """POST /internal/v1/jobs/generate with valid secret returns 200 and valid draft."""
    settings = get_settings()
    headers = {"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET}
    payload = {
        "idea": "Need someone to build a PyTorch machine learning classifier",
        "department": "Computer Science",
    }

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/internal/v1/jobs/generate",
            json=payload,
            headers=headers,
        )
        assert resp.status_code == 200
        data = resp.json()
        assert "draft" in data
        draft = data["draft"]
        assert "title" in draft
        assert "description" in draft
        assert "required_skills" in draft
        assert isinstance(draft["required_skills"], list)
        assert data["pipeline_version"] == "generation-v1"


@pytest.mark.asyncio
async def test_temperature_backoff_on_malformed_json(monkeypatch):
    """When attempt 1 returns malformed output, client retries with temperature 0.0."""
    temperatures_seen = []

    async def mock_post(self, url, **kwargs):
        payload = kwargs.get("json", {})
        temp = payload.get("temperature")
        temperatures_seen.append(temp)

        if temp == 0.2:
            return httpx.Response(
                200,
                json={"choices": [{"message": {"content": "Not valid JSON {"}}]},
                request=httpx.Request("POST", url),
            )
        else:
            return httpx.Response(
                200,
                json={
                    "choices": [
                        {
                            "message": {
                                "content": '{"title": "Backend Engineer", "description": "Build high throughput services.", "required_skills": ["Go", "PostgreSQL"], "department": "Computer Science"}'
                            }
                        }
                    ]
                },
                request=httpx.Request("POST", url),
            )

    monkeypatch.setattr(httpx.AsyncClient, "post", mock_post)

    client = VLLMClient(base_url="http://mock-vllm:8000/v1")
    draft = await client.generate_job_draft(
        "Build a Go backend", department="Computer Science"
    )

    assert draft.title == "Backend Engineer"
    assert "Go" in draft.required_skills
    assert temperatures_seen == [0.2, 0.0]


@pytest.mark.asyncio
async def test_malformed_json_primitive_triggers_fallback(monkeypatch):
    """When LLM returns JSON primitive like '[1, 2]', client handles it safely without crashing."""

    async def mock_post(self, url, **kwargs):
        return httpx.Response(
            200,
            json={"choices": [{"message": {"content": "[1, 2, 3]"}}]},
            request=httpx.Request("POST", url),
        )

    monkeypatch.setattr(httpx.AsyncClient, "post", mock_post)

    client = VLLMClient(base_url="http://mock-vllm:8000/v1")
    draft = await client.generate_job_draft(
        "Build a mobile app with Swift", department="Engineering"
    )

    assert isinstance(draft, GeneratedJobDraft)
    assert len(draft.title) > 0


@pytest.mark.asyncio
async def test_job_generation_validation_bounds():
    """Verify GenerateJobDraftRequest rejects invalid idea lengths and department lengths."""
    settings = get_settings()
    headers = {"X-Internal-AI-Secret": settings.INTERNAL_AI_SECRET}
    transport = ASGITransport(app=app)

    async with AsyncClient(transport=transport, base_url="http://test") as client:
        # Idea > 1000 chars
        resp_long_idea = await client.post(
            "/internal/v1/jobs/generate",
            headers=headers,
            json={"idea": "a" * 1001, "department": "Computer Science"},
        )
        assert resp_long_idea.status_code == 422

        # Idea < 3 chars
        resp_short_idea = await client.post(
            "/internal/v1/jobs/generate",
            headers=headers,
            json={"idea": "ab", "department": "Computer Science"},
        )
        assert resp_short_idea.status_code == 422

        # Department > 100 chars
        resp_long_dept = await client.post(
            "/internal/v1/jobs/generate",
            headers=headers,
            json={"idea": "Build a React portfolio", "department": "a" * 101},
        )
        assert resp_long_dept.status_code == 422
