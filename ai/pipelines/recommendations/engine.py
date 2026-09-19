"""Member Recommendations Engine.

Generates personalized skill suggestions, profile improvement tips, and relevant
opportunity matches for campus marketplace members using skill co-occurrence,
academic domain mapping, and heuristic profile completeness rules.
"""

import json
import logging
from typing import Any

from ai.app.middleware.run_tracker import track_ai_run
from pydantic import BaseModel, Field

logger = logging.getLogger("lynk-ai.pipelines.recommendations")


class RecommendationItem(BaseModel):
    """An individual personalized recommendation item."""

    type: str = Field(
        ...,
        description="Category of recommendation: 'skill', 'profile', or 'opportunity'",
    )
    title: str = Field(..., description="Short, descriptive headline or skill name")
    reason: str = Field(
        ...,
        description="Transparent, human-readable rationale explaining why this was recommended",
    )
    confidence: float = Field(
        ...,
        ge=0.0,
        le=1.0,
        description="Confidence score between 0.0 and 1.0",
    )
    metadata: dict[str, Any] = Field(
        default_factory=dict,
        description="Structured context (e.g. source skills, categories, form targets)",
    )


# Curated skill co-occurrence knowledge base
# Maps combinations of canonical skills to target skill recommendations
SKILL_CO_OCCURRENCE_RULES: list[dict[str, Any]] = [
    {
        "required_all": ["python", "machine learning"],
        "recommendations": [
            {
                "title": "PyTorch",
                "reason": "Frequently required alongside Python and Machine Learning in campus AI research and student startup projects.",
                "confidence": 0.92,
                "category": "Data & AI",
            },
            {
                "title": "scikit-learn",
                "reason": "Standard statistical learning library commonly paired with Python in data science job listings.",
                "confidence": 0.88,
                "category": "Data & AI",
            },
            {
                "title": "Docker",
                "reason": "Essential for containerizing machine learning models and deploying reproducible AI pipelines.",
                "confidence": 0.82,
                "category": "DevOps & Cloud",
            },
            {
                "title": "TensorFlow",
                "reason": "Widely used production deep learning framework frequently paired with Python.",
                "confidence": 0.80,
                "category": "Data & AI",
            },
        ],
    },
    {
        "required_all": ["react", "typescript"],
        "recommendations": [
            {
                "title": "Next.js",
                "reason": "Leading full-stack React framework widely preferred for campus startup web applications.",
                "confidence": 0.94,
                "category": "Frontend",
            },
            {
                "title": "Tailwind CSS",
                "reason": "Standard utility CSS framework frequently paired with React and TypeScript interfaces.",
                "confidence": 0.90,
                "category": "Frontend",
            },
            {
                "title": "Node.js",
                "reason": "Server-side runtime commonly paired with React and TypeScript for full-stack delivery.",
                "confidence": 0.82,
                "category": "Backend",
            },
        ],
    },
    {
        "required_all": ["go", "postgresql"],
        "recommendations": [
            {
                "title": "Docker",
                "reason": "Essential containerization tool for Go and PostgreSQL backend services.",
                "confidence": 0.88,
                "category": "DevOps & Cloud",
            },
            {
                "title": "Redis",
                "reason": "Standard in-memory cache layer frequently paired with Go and PostgreSQL architectures.",
                "confidence": 0.84,
                "category": "Databases & Storage",
            },
            {
                "title": "gRPC",
                "reason": "High-performance microservice communication protocol widely adopted with Go services.",
                "confidence": 0.82,
                "category": "Backend",
            },
        ],
    },
    {
        "required_all": ["python", "data science"],
        "recommendations": [
            {
                "title": "Pandas",
                "reason": "Foundational library for tabular data manipulation and data analysis workflows.",
                "confidence": 0.92,
                "category": "Data & AI",
            },
            {
                "title": "NumPy",
                "reason": "Core numerical computation library universally used in Python scientific computing.",
                "confidence": 0.89,
                "category": "Data & AI",
            },
            {
                "title": "SQL",
                "reason": "Essential database querying language expected of all Python data science practitioners.",
                "confidence": 0.86,
                "category": "Databases & Storage",
            },
        ],
    },
    # Single-skill triggers
    {
        "required_any": ["python"],
        "recommendations": [
            {
                "title": "Machine Learning",
                "reason": "High-growth domain with extensive campus gig demand for Python developers.",
                "confidence": 0.78,
                "category": "Data & AI",
            },
            {
                "title": "PostgreSQL",
                "reason": "Robust relational database standard for Python web backends and data pipelines.",
                "confidence": 0.74,
                "category": "Databases & Storage",
            },
            {
                "title": "Docker",
                "reason": "Valuable tool for packaging and deploying Python applications reliably.",
                "confidence": 0.72,
                "category": "DevOps & Cloud",
            },
        ],
    },
    {
        "required_any": ["react"],
        "recommendations": [
            {
                "title": "TypeScript",
                "reason": "Strongly typed JavaScript superset standard in professional React development.",
                "confidence": 0.88,
                "category": "Frontend",
            },
            {
                "title": "Next.js",
                "reason": "Production React framework with built-in routing, API routes, and SSR.",
                "confidence": 0.86,
                "category": "Frontend",
            },
        ],
    },
    {
        "required_any": ["typescript"],
        "recommendations": [
            {
                "title": "React",
                "reason": "Most popular UI library built with TypeScript in campus startup jobs.",
                "confidence": 0.85,
                "category": "Frontend",
            },
            {
                "title": "Node.js",
                "reason": "Runtime environment for building scalable backend APIs in TypeScript.",
                "confidence": 0.80,
                "category": "Backend",
            },
        ],
    },
    {
        "required_any": ["go", "golang"],
        "recommendations": [
            {
                "title": "PostgreSQL",
                "reason": "Primary relational database paired with Go backend services in production.",
                "confidence": 0.84,
                "category": "Databases & Storage",
            },
            {
                "title": "Docker",
                "reason": "Containerization standard for Go microservice deployment.",
                "confidence": 0.86,
                "category": "DevOps & Cloud",
            },
        ],
    },
    {
        "required_any": ["machine learning"],
        "recommendations": [
            {
                "title": "PyTorch",
                "reason": "Dominant deep learning framework in modern research and applied AI.",
                "confidence": 0.90,
                "category": "Data & AI",
            },
            {
                "title": "Python",
                "reason": "Standard programming language for machine learning engineering.",
                "confidence": 0.94,
                "category": "Programming Languages",
            },
        ],
    },
    {
        "required_any": ["figma", "ui/ux", "ui design"],
        "recommendations": [
            {
                "title": "Wireframing",
                "reason": "Core UX methodology frequently paired with Figma visual design contracts.",
                "confidence": 0.86,
                "category": "Design",
            },
            {
                "title": "Design Systems",
                "reason": "Scalable component architecture highly sought after in campus UI/UX contracts.",
                "confidence": 0.84,
                "category": "Design",
            },
        ],
    },
]

# Department-specific skill recommendations
DEPARTMENT_SKILL_RECOMMENDATIONS: dict[str, list[dict[str, Any]]] = {
    "computer science": [
        {
            "title": "Git",
            "reason": "Fundamental version control tool required for collaborative campus software contracts.",
            "confidence": 0.85,
            "category": "DevOps & Cloud",
        },
        {
            "title": "Docker",
            "reason": "Industry-standard containerization tool frequently required in software engineering roles.",
            "confidence": 0.80,
            "category": "DevOps & Cloud",
        },
    ],
    "data science": [
        {
            "title": "SQL",
            "reason": "Foundational database querying language essential for campus data science contracts.",
            "confidence": 0.88,
            "category": "Databases & Storage",
        },
        {
            "title": "Python",
            "reason": "Core language for data exploration, cleaning, and statistical modeling.",
            "confidence": 0.86,
            "category": "Programming Languages",
        },
    ],
    "design": [
        {
            "title": "Figma",
            "reason": "Leading collaborative product design software used across student design teams.",
            "confidence": 0.88,
            "category": "Design",
        },
    ],
}


class RecommendationEngine:
    """Engine for generating member recommendations."""

    def __init__(self, db_pool: Any | None = None) -> None:
        self.db_pool = db_pool
        self.model_name = "lynk-recommendation-engine"
        self.model_version = "1.0.0"
        self.pipeline_version = "1.0.0"

    async def generate_recommendations(
        self,
        user_id: str,
        current_skills: list[str] | None = None,
        department: str | None = None,
        bio: str | None = None,
        portfolio_links: list[str] | None = None,
        limit: int = 10,
    ) -> list[RecommendationItem]:
        """Generate personalized recommendations for a campus member profile."""
        skills = [s.strip() for s in (current_skills or []) if s.strip()]
        lower_skills = {s.lower() for s in skills}
        bio_text = (bio or "").strip()
        dept_text = (department or "").strip().lower()
        links = portfolio_links if portfolio_links is not None else []

        async with track_ai_run(
            feature="recommendations",
            entity_type="profile",
            entity_id=user_id,
            model_name=self.model_name,
            model_version=self.model_version,
            pipeline_version=self.pipeline_version,
            input_data={
                "user_id": user_id,
                "skills": skills,
                "department": department,
                "bio": bio_text,
                "portfolio_links": links,
            },
            db_pool=self.db_pool,
        ) as tracker:
            results: list[RecommendationItem] = []
            seen_titles: set[str] = set()

            # 1. Profile Completeness Recommendations
            # Bio check
            if not bio_text or len(bio_text) < 50:
                item = RecommendationItem(
                    type="profile",
                    title="Complete your campus bio",
                    reason="Profiles with a bio describing academic background and project interests receive 3x more gig inquiries.",
                    confidence=0.95,
                    metadata={"field": "bio", "action": "add_bio"},
                )
                results.append(item)
                seen_titles.add(item.title.lower())

            # Skills check
            if len(skills) == 0:
                item = RecommendationItem(
                    type="profile",
                    title="Add your technical skills",
                    reason="Listing at least 3 skills allows our matching system to surface relevant campus gigs and projects.",
                    confidence=0.98,
                    metadata={"field": "skills", "action": "add_skills"},
                )
                results.append(item)
                seen_titles.add(item.title.lower())
            elif len(skills) < 3:
                item = RecommendationItem(
                    type="profile",
                    title="Add more skills to your profile",
                    reason="Campus members with 3 or more skills appear in 5x more search queries.",
                    confidence=0.85,
                    metadata={"field": "skills", "action": "expand_skills"},
                )
                results.append(item)
                seen_titles.add(item.title.lower())

            # Portfolio links check
            has_links = len(links) > 0 or any(
                kw in bio_text.lower()
                for kw in (
                    "github.com",
                    "http://",
                    "https://",
                    "portfolio",
                    "gitlab.com",
                )
            )
            if not has_links:
                item = RecommendationItem(
                    type="profile",
                    title="Add portfolio or GitHub links",
                    reason="Showing code repositories or a live design portfolio gives project leaders direct evidence of your work.",
                    confidence=0.88,
                    metadata={"field": "portfolio_links", "action": "add_portfolio"},
                )
                results.append(item)
                seen_titles.add(item.title.lower())

            # Department check
            if not dept_text:
                item = RecommendationItem(
                    type="profile",
                    title="Specify your academic department",
                    reason="Helps department-specific campus recruiters find you for research and teaching assistant gigs.",
                    confidence=0.80,
                    metadata={"field": "department", "action": "set_department"},
                )
                results.append(item)
                seen_titles.add(item.title.lower())

            # 2. Skill Co-occurrence Recommendations
            for rule in SKILL_CO_OCCURRENCE_RULES:
                matches_rule = False
                if "required_all" in rule:
                    if all(req in lower_skills for req in rule["required_all"]):
                        matches_rule = True
                elif "required_any" in rule and any(
                    req in lower_skills for req in rule["required_any"]
                ):
                    matches_rule = True

                if matches_rule:
                    for rec in rule["recommendations"]:
                        title = rec["title"]
                        title_lower = title.lower()
                        # Deduplicate: never recommend skills the member already has
                        if title_lower in lower_skills or title_lower in seen_titles:
                            continue

                        results.append(
                            RecommendationItem(
                                type="skill",
                                title=title,
                                reason=rec["reason"],
                                confidence=rec["confidence"],
                                metadata={
                                    "category": rec.get("category", "General"),
                                    "matched_by": "skill_co_occurrence",
                                },
                            )
                        )
                        seen_titles.add(title_lower)

            # 3. Department-Specific Skill Recommendations
            if dept_text:
                for dept_key, dept_recs in DEPARTMENT_SKILL_RECOMMENDATIONS.items():
                    if dept_key in dept_text:
                        for rec in dept_recs:
                            title = rec["title"]
                            title_lower = title.lower()
                            if (
                                title_lower in lower_skills
                                or title_lower in seen_titles
                            ):
                                continue

                            results.append(
                                RecommendationItem(
                                    type="skill",
                                    title=title,
                                    reason=rec["reason"],
                                    confidence=rec["confidence"],
                                    metadata={
                                        "category": rec.get("category", "Department"),
                                        "matched_by": "department_affinity",
                                    },
                                )
                            )
                            seen_titles.add(title_lower)

            # 4. Opportunity Matching Recommendations
            if any(
                s in lower_skills
                for s in ("python", "machine learning", "pytorch", "tensorflow")
            ):
                opp_title = "Explore AI & Machine Learning Gigs"
                if opp_title.lower() not in seen_titles:
                    results.append(
                        RecommendationItem(
                            type="opportunity",
                            title=opp_title,
                            reason="Your background in Python and Machine Learning matches active campus research and AI prototype roles.",
                            confidence=0.86,
                            metadata={"category": "AI & Research"},
                        )
                    )
                    seen_titles.add(opp_title.lower())

            if any(
                s in lower_skills
                for s in ("react", "typescript", "next.js", "frontend", "javascript")
            ):
                opp_title = "Explore Full-Stack & Web Gigs"
                if opp_title.lower() not in seen_titles:
                    results.append(
                        RecommendationItem(
                            type="opportunity",
                            title=opp_title,
                            reason="Campus startups frequently hire developers proficient in React, TypeScript, and modern web frameworks.",
                            confidence=0.85,
                            metadata={"category": "Web Development"},
                        )
                    )
                    seen_titles.add(opp_title.lower())

            # Sort results by confidence descending
            results.sort(key=lambda x: x.confidence, reverse=True)
            final_items = results[:limit]

            # 5. Optional DB Persistence
            if self.db_pool is not None and len(final_items) > 0:
                try:
                    async with self.db_pool.acquire() as conn:
                        await conn.execute(
                            "UPDATE ai_recommendations SET status = 'archived' WHERE user_id = $1 AND status = 'active'",
                            user_id,
                        )
                        for it in final_items:
                            await conn.execute(
                                "INSERT INTO ai_recommendations ("
                                "user_id, type, title, reason, confidence, metadata, status"
                                ") VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'active')",
                                user_id,
                                it.type,
                                it.title,
                                it.reason,
                                it.confidence,
                                json.dumps(it.metadata or {}),
                            )
                except Exception as exc:
                    logger.warning(
                        "Failed to persist recommendations to ai_recommendations: %s",
                        exc,
                    )

            tracker.set_output({"recommendations_count": len(final_items)})
            tracker.set_confidence(final_items[0].confidence if final_items else 0.0)

            return final_items


def get_recommendation_engine(db_pool: Any | None = None) -> RecommendationEngine:
    """Factory to create a RecommendationEngine instance."""
    return RecommendationEngine(db_pool=db_pool)
