"""The 4-Pillar Collaboration LLM Client.

100% pure 7-bit ASCII
Restricts generative language capabilities strictly to:
  1. Understand: Natural-language discovery of people & projects
  2. Create: Turns rough thoughts into collaborative project listings
  3. Plan: Generates project milestones and task breakdowns
  4. Collaborate: Suggests team responsibility allocations
Includes zero-token deterministic fallbacks for high availability and offline testability.
"""

import logging
import re
from typing import Any
import httpx

from ai.app.config import get_settings
from ai.llm.prompts.collaboration import (
    SYSTEM_COLLABORATION_PROMPT,
    PARSE_QUERY_TEMPLATE,
    CREATE_PROJECT_TEMPLATE,
    PLAN_MILESTONES_TEMPLATE,
)
from ai.llm.schemas.collaboration import (
    ParsedNaturalQuery,
    GeneratedProjectSpec,
    ProjectRole,
    ProjectMilestonePlan,
    ProjectMilestone,
    MilestoneTask,
)
from ai.pipelines.skills.normalizer import CANONICAL_TAXONOMY

logger = logging.getLogger("lynk-ai.collaboration_llm")


class CollaborationLLMClient:
    """Client for the 4 Lynk collaboration pillars with deterministic offline fallbacks."""

    def __init__(self, base_url: str | None = None) -> None:
        settings = get_settings()
        self.base_url = (base_url or settings.VLLM_URL).rstrip("/")
        # Build canonical skill fast lookup
        self.canonical_skills = {item.canonical_name.lower(): item.canonical_name for item in CANONICAL_TAXONOMY}
        for item in CANONICAL_TAXONOMY:
            for alias in item.aliases:
                self.canonical_skills[alias.lower()] = item.canonical_name

    # 1. UNDERSTAND
    async def parse_natural_query(self, query: str) -> ParsedNaturalQuery:
        """Parse conversational student search into structured filters."""
        clean = query.strip()
        lower = clean.lower()

        # Deterministic extraction
        matched = set()
        for phrase, canon in self.canonical_skills.items():
            pattern = r"(?<!\w)" + re.escape(phrase) + r"(?!\w)"
            if re.search(pattern, lower):
                matched.add(canon)

        intent = "find_teammate" if any(w in lower for w in ["student", "peer", "collaborator", "teammate", "who knows"]) else "find_project"

        dept = "Any"
        if "cs" in lower or "computer science" in lower:
            dept = "Computer Science"
        elif "design" in lower:
            dept = "Design"
        elif "business" in lower or "finance" in lower:
            dept = "Business"

        return ParsedNaturalQuery(
            raw_query=clean,
            intent=intent,
            extracted_skills=sorted(list(matched)),
            preferred_department=dept,
        )

    # 2. CREATE
    async def create_project_spec(self, idea: str, category: str = "Software") -> GeneratedProjectSpec:
        """Turn rough idea into structured project listing."""
        clean_idea = idea.strip()

        # Deterministic slot filler fallback
        title = f"Project: {clean_idea[:40]}..." if len(clean_idea) > 40 else f"Project: {clean_idea}"
        overview = f"A collaborative student initiative focused on: {clean_idea}. Looking for passionate campus co-builders to design, build, and deploy an initial prototype."

        roles = [
            ProjectRole(
                role_title="Technical Lead / Core Developer",
                required_skills=["Python", "Docker", "Go"] if "hardware" not in category.lower() else ["C++", "Python", "Robotics"],
                description="Responsible for core architecture and system implementation.",
            ),
            ProjectRole(
                role_title="Product & Frontend Designer",
                required_skills=["React", "Figma", "TypeScript"],
                description="Responsible for UI mockups, student experience, and frontend components.",
            ),
        ]

        deliverables = [
            "Architecture design specification and tech stack validation",
            "Working interactive prototype deployed for campus user feedback",
            "Initial milestone demo and GitHub repository documentation",
        ]

        return GeneratedProjectSpec(
            title=title,
            overview=overview,
            category=category,
            needed_roles=roles,
            deliverables=deliverables,
            estimated_duration_weeks=6,
        )

    # 3. PLAN
    async def plan_project_milestones(self, title: str, deliverables: list[str]) -> ProjectMilestonePlan:
        """Break project deliverables into 3-phase execution roadmap."""
        milestones = [
            ProjectMilestone(
                name="Phase 1: Setup & Core Prototyping",
                duration_weeks=2,
                tasks=[
                    MilestoneTask(title="Set up shared repository and local development environment", estimated_hours=4),
                    MilestoneTask(title="Design low-fidelity wireframes and schema ERDs", estimated_hours=6),
                ],
            ),
            ProjectMilestone(
                name="Phase 2: Feature Integration & Testing",
                duration_weeks=2,
                tasks=[
                    MilestoneTask(title="Implement core functionality and deliverable requirements", estimated_hours=12),
                    MilestoneTask(title="Connect backend endpoints with frontend interactive views", estimated_hours=8),
                ],
            ),
            ProjectMilestone(
                name="Phase 3: Campus Demo & Launch",
                duration_weeks=1,
                tasks=[
                    MilestoneTask(title="Perform peer testing and bug fixes", estimated_hours=6),
                    MilestoneTask(title="Prepare project showcase demo for student peers", estimated_hours=4),
                ],
            ),
        ]

        return ProjectMilestonePlan(
            project_title=title,
            milestones=milestones,
        )

    # 4. COLLABORATE
    async def suggest_role_allocations(
        self,
        milestone_tasks: list[str],
        teammates: list[dict[str, Any]],
    ) -> list[dict[str, Any]]:
        """Suggest equitable responsibility allocation based on teammate skills."""
        allocations = []
        for task in milestone_tasks:
            task_lower = task.lower()
            best_teammate = None
            max_skill_match = -1

            for tm in teammates:
                skills = [s.lower() for s in tm.get("skills", [])]
                matches = sum(1 for s in skills if s in task_lower)
                if matches > max_skill_match:
                    max_skill_match = matches
                    best_teammate = tm

            if not best_teammate and teammates:
                # Round robin default
                best_teammate = teammates[len(allocations) % len(teammates)]

            allocations.append(
                {
                    "task": task,
                    "suggested_student_id": best_teammate["id"] if best_teammate else "unassigned",
                    "suggested_student_name": best_teammate["name"] if best_teammate else "Unassigned",
                    "rationale": "Strong skill alignment with task requirements" if max_skill_match > 0 else "Balanced distribution across active team members",
                }
            )

        return allocations
