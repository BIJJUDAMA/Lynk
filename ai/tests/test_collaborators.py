"""Unit tests for Teammate Complementarity Matcher."""

from ai.pipelines.collaborators.matcher import TeammateMatcher


def test_find_complementary_peers():
    matcher = TeammateMatcher()
    team_skills = ["React", "TypeScript"]
    project_missing_skills = ["Go", "Docker", "PostgreSQL"]

    candidates = [
        {"id": "s1", "name": "Bob", "department": "Computer Science", "skills": ["React", "CSS"]},
        {"id": "s2", "name": "Charlie", "department": "Engineering", "skills": ["Go", "Docker"]},
        {"id": "s3", "name": "Diana", "department": "Data Science", "skills": ["PostgreSQL", "Go"]},
    ]

    results = matcher.find_complementary_peers(
        team_skills=team_skills,
        project_missing_skills=project_missing_skills,
        candidates=candidates,
    )

    assert len(results) >= 2
    # s2 and s3 cover missing skills; s1 duplicates existing React skills
    top_ids = [r["id"] for r in results[:2]]
    assert "s2" in top_ids or "s3" in top_ids
    assert results[0]["complement_score"] > results[-1]["complement_score"]
