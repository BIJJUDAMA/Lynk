"""Unit tests for the 4-Pillar Collaboration LLM Client."""

import pytest
from ai.llm.collaboration_client import CollaborationLLMClient


@pytest.mark.asyncio
async def test_understand_natural_query():
    client = CollaborationLLMClient()
    parsed = await client.parse_natural_query("Looking for a CS major who knows Python and Docker to build a backend API")

    assert "Python" in parsed.extracted_skills
    assert "Docker" in parsed.extracted_skills
    assert parsed.intent in ["find_teammate", "find_project"]


@pytest.mark.asyncio
async def test_create_project_spec():
    client = CollaborationLLMClient()
    spec = await client.create_project_spec(
        idea="A campus drone delivery system for library books",
        category="Hardware & Robotics",
    )

    assert len(spec.title) > 5
    assert len(spec.overview) > 20
    assert len(spec.needed_roles) >= 2
    assert len(spec.deliverables) >= 2


@pytest.mark.asyncio
async def test_plan_project_milestones():
    client = CollaborationLLMClient()
    plan = await client.plan_project_milestones(
        title="Campus Navigation App",
        deliverables=["Figma wireframes", "React Native prototype", "Indoor map routing API"],
    )

    assert len(plan.milestones) == 3
    assert "Phase 1" in plan.milestones[0].name
    assert len(plan.milestones[0].tasks) > 0


@pytest.mark.asyncio
async def test_collaborate_role_allocation():
    client = CollaborationLLMClient()
    allocations = await client.suggest_role_allocations(
        milestone_tasks=["Build backend Go API", "Design Figma screens", "Write Docker Compose"],
        teammates=[
            {"id": "s1", "name": "Alice", "skills": ["Go", "Docker"]},
            {"id": "s2", "name": "Bob", "skills": ["Figma", "UI/UX"]},
        ],
    )

    assert len(allocations) == 3
    # Alice should be suggested for Go or Docker
    alice_tasks = [a["task"] for a in allocations if a["suggested_student_id"] == "s1"]
    assert len(alice_tasks) > 0
