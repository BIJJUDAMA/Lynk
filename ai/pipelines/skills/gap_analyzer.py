"""Skill Gap & Learning Pathway Analyzer.

100% pure 7-bit ASCII
Identifies matching ready skills vs. missing skills for a student and project,
and suggests curated next learning milestones.
"""

from pydantic import BaseModel, Field

# Curated skill progression milestones
SKILL_PROGRESSIONS = {
    "pytorch": "Deep learning library commonly paired with Python; explore tutorials on PyTorch tensors and basic CNNs.",
    "docker": "Containerization standard; learn how to write a Dockerfile and spin up multi-container docker-compose setups.",
    "next.js": "React framework for production web apps; explore Server Components, App Router routing, and SSR.",
    "postgresql": "Relational database; practice writing indexing strategies, joins, and ACID transactions.",
    "go": "High-concurrency systems language; explore goroutines, channels, and standard library HTTP servers.",
    "figma": "Collaborative design standard; practice component variants, auto-layout, and prototyping user flows.",
}


class SkillGapResult(BaseModel):
    """Evaluation breakdown of student readiness for a target project."""

    ready_skills: list[str] = Field(
        default_factory=list, description="Skills the student already possesses"
    )
    missing_skills: list[str] = Field(
        default_factory=list,
        description="Skills required by project that student lacks",
    )
    readiness_percentage: float = Field(
        ge=0.0, le=100.0, description="Readiness score from 0.0 to 100.0"
    )
    learning_pathway: list[dict[str, str]] = Field(
        default_factory=list, description="Curated learning recommendations"
    )


class SkillGapAnalyzer:
    """Analyzes skill differences and produces actionable learning roadmaps."""

    def analyze_gap(
        self,
        student_skills: list[str],
        project_required_skills: list[str],
    ) -> SkillGapResult:
        """Compare student capabilities against project requirements."""
        student_set = {s.strip().lower() for s in student_skills}

        ready = []
        missing = []

        for req in project_required_skills:
            if req.strip().lower() in student_set:
                ready.append(req)
            else:
                missing.append(req)

        total_req = max(len(project_required_skills), 1)
        readiness = round((len(ready) / total_req) * 100.0, 1)

        pathway = []
        for miss in missing:
            key = miss.strip().lower()
            advice = SKILL_PROGRESSIONS.get(
                key,
                f"Essential project requirement; explore documentation and practical starter tutorials for {miss}.",
            )
            pathway.append({"skill": miss, "guidance": advice})

        return SkillGapResult(
            ready_skills=ready,
            missing_skills=missing,
            readiness_percentage=readiness,
            learning_pathway=pathway,
        )
