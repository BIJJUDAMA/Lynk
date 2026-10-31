"""Teammate Discovery & Complementarity Matching.

100% pure 7-bit ASCII
Calculates skill complement scores to pair students who fill missing project capabilities
without redundant skill overlap, maximizing interdisciplinary collaboration.
"""

import logging
from typing import Any

logger = logging.getLogger("lynk-ai.collaborators")


class TeammateMatcher:
    """Discovers co-builders whose skills complement a project team's composition."""

    def __init__(self) -> None:
        pass

    def find_complementary_peers(
        self,
        team_skills: list[str],
        project_missing_skills: list[str],
        candidates: list[dict[str, Any]],
        limit: int = 10,
    ) -> list[dict[str, Any]]:
        """Rank candidate students by how effectively they fill missing project needs."""
        team_set = {s.lower() for s in team_skills}
        missing_set = {s.lower() for s in project_missing_skills}
        total_missing = max(len(missing_set), 1)

        scored_candidates = []
        for cand in candidates:
            cand_skills = {s.lower() for s in cand.get("skills", [])}

            # How many missing skills does this candidate satisfy?
            covered_missing = cand_skills & missing_set
            coverage_ratio = len(covered_missing) / total_missing

            # Does this candidate introduce completely redundant skills?
            redundant_skills = (cand_skills & team_set) - missing_set
            redundancy_penalty = min(0.3, len(redundant_skills) * 0.05)

            # Complementarity score in [0.0, 1.0]
            complement_score = max(0.0, round(coverage_ratio - redundancy_penalty, 4))

            scored_candidates.append(
                {
                    "id": cand.get("id"),
                    "name": cand.get("name", "Campus Peer"),
                    "department": cand.get("department", "Undeclared"),
                    "complement_score": complement_score,
                    "covered_skills": [
                        s
                        for s in cand.get("skills", [])
                        if s.lower() in covered_missing
                    ],
                    "all_skills": cand.get("skills", []),
                }
            )

        scored_candidates.sort(key=lambda x: x["complement_score"], reverse=True)
        return scored_candidates[:limit]
