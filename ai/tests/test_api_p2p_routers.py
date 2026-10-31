# 100% pure 7-bit ASCII
from fastapi.testclient import TestClient

from ai.app.main import app

client = TestClient(app)
AUTH_HEADERS = {"X-Internal-Secret": "lynk-ai-internal-secret-key-2026"}
ALT_AUTH_HEADERS = {"X-Internal-AI-Secret": "lynk-ai-internal-secret-key-2026"}


def test_collaborator_match_endpoint():
    payload = {
        "team_skills": ["React"],
        "project_missing_skills": ["Go", "Docker"],
        "candidates": [
            {"id": "s1", "name": "Bob", "department": "CS", "skills": ["Go", "Docker"]},
            {"id": "s2", "name": "Alice", "department": "CS", "skills": ["React"]},
        ],
    }
    resp = client.post(
        "/internal/v1/collaborators/match", json=payload, headers=AUTH_HEADERS
    )
    assert resp.status_code == 200
    data = resp.json()
    assert len(data["matches"]) > 0
    assert data["matches"][0]["id"] == "s1"


def test_collaborator_match_with_alt_header():
    payload = {
        "team_skills": ["React"],
        "project_missing_skills": ["Go"],
        "candidates": [
            {"id": "s1", "name": "Bob", "department": "CS", "skills": ["Go"]},
        ],
    }
    resp = client.post(
        "/internal/v1/collaborators/match", json=payload, headers=ALT_AUTH_HEADERS
    )
    assert resp.status_code == 200
    data = resp.json()
    assert len(data["matches"]) == 1


def test_skill_gap_endpoint():
    payload = {
        "student_skills": ["Python"],
        "project_required_skills": ["Python", "PyTorch", "Docker"],
    }
    resp = client.post("/internal/v1/skills/gap", json=payload, headers=AUTH_HEADERS)
    assert resp.status_code == 200
    data = resp.json()
    assert "PyTorch" in data["missing_skills"]
    assert data["readiness_percentage"] == 33.3


def test_project_assist_endpoint():
    payload = {
        "idea": "An open source autonomous rover for campus delivery",
        "category": "Hardware",
    }
    resp = client.post(
        "/internal/v1/projects/assist", json=payload, headers=AUTH_HEADERS
    )
    assert resp.status_code == 200
    data = resp.json()
    assert len(data["needed_roles"]) >= 2


def test_project_milestones_endpoint():
    payload = {
        "title": "Autonomous Rover",
        "deliverables": ["ROS integration", "Obstacle avoidance", "Campus demo"],
    }
    resp = client.post(
        "/internal/v1/projects/milestones", json=payload, headers=AUTH_HEADERS
    )
    assert resp.status_code == 200
    data = resp.json()
    assert data["project_title"] == "Autonomous Rover"
    assert len(data["milestones"]) == 3


def test_project_roles_endpoint():
    payload = {
        "milestone_tasks": ["Set up React frontend", "Implement Go API server"],
        "teammates": [
            {"id": "s1", "name": "Bob", "skills": ["React", "TypeScript"]},
            {"id": "s2", "name": "Alice", "skills": ["Go", "PostgreSQL"]},
        ],
    }
    resp = client.post(
        "/internal/v1/projects/roles", json=payload, headers=AUTH_HEADERS
    )
    assert resp.status_code == 200
    data = resp.json()
    assert len(data["allocations"]) == 2


def test_search_natural_endpoint():
    payload = {"query": "looking for a peer who knows figma to design our app"}
    resp = client.post(
        "/internal/v1/search/natural", json=payload, headers=AUTH_HEADERS
    )
    assert resp.status_code == 200
    data = resp.json()
    assert "Figma" in data["extracted_skills"]
    assert data["intent"] == "find_teammate"


def test_resume_parse_endpoint():
    payload = {
        "student_id": "std-123",
        "text": "Junior Computer Science student with experience in Python, Docker, and React.",
    }
    resp = client.post("/internal/v1/resumes/parse", json=payload, headers=AUTH_HEADERS)
    assert resp.status_code == 200
    data = resp.json()
    assert data["major"] == "Computer Science"
    assert "Python" in data["matched_skills"]


def test_p2p_endpoints_unauthorized():
    payload = {"query": "test query"}
    resp = client.post("/internal/v1/search/natural", json=payload)
    assert resp.status_code == 401
