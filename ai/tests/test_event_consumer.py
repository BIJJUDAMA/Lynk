# 100% pure 7-bit ASCII
from unittest.mock import MagicMock

import pytest

from ai.models.embeddings.provider import MockEmbeddingProvider
from ai.workers.event_consumer import RabbitMQEventConsumer


@pytest.fixture
def mock_qdrant():
    client = MagicMock()
    client.upsert_point = MagicMock()
    client.delete_point = MagicMock()
    return client


@pytest.fixture
def mock_provider():
    return MockEmbeddingProvider(dimension=16)


@pytest.fixture
def mock_kuzu():
    engine = MagicMock()
    engine.add_student = MagicMock()
    engine.add_skill_edge = MagicMock()
    return engine


@pytest.mark.asyncio
async def test_process_project_created_event(mock_qdrant, mock_provider):
    """Project creation event generates vector and indexes into campus_projects."""
    consumer = RabbitMQEventConsumer(
        qdrant_client=mock_qdrant,
        embedding_provider=mock_provider,
    )

    payload = {
        "id": "proj-uuid-123",
        "title": "Autonomous Drone Delivery",
        "description": "Building embedded systems with ROS and C++",
        "required_skills": ["C++", "ROS", "Python"],
        "category": "Robotics",
        "department": "Engineering",
    }

    result = await consumer.process_event("project.created", payload)

    assert result["status"] == "indexed"
    assert result["project_id"] == "proj-uuid-123"
    assert result["dim"] == 16

    mock_qdrant.upsert_point.assert_called_once()
    call_args = mock_qdrant.upsert_point.call_args[1]
    assert call_args["collection_name"] == "campus_projects"
    assert call_args["point_id"] == "proj-uuid-123"
    assert len(call_args["vector"]) == 16
    assert call_args["payload"]["title"] == "Autonomous Drone Delivery"


@pytest.mark.asyncio
async def test_process_project_deleted_event(mock_qdrant, mock_provider):
    """Project deletion removes point from Qdrant vector index."""
    consumer = RabbitMQEventConsumer(
        qdrant_client=mock_qdrant,
        embedding_provider=mock_provider,
    )

    payload = {"id": "proj-uuid-456", "action": "delete"}
    result = await consumer.process_event("project.deleted", payload)

    assert result["status"] == "deleted"
    assert result["project_id"] == "proj-uuid-456"
    mock_qdrant.delete_point.assert_called_once_with(
        collection_name="campus_projects", point_id="proj-uuid-456"
    )


@pytest.mark.asyncio
async def test_process_student_updated_event(mock_qdrant, mock_provider, mock_kuzu):
    """Student profile update updates vector and synchronizes Kuzu graph edges."""
    consumer = RabbitMQEventConsumer(
        qdrant_client=mock_qdrant,
        embedding_provider=mock_provider,
        kuzu_engine=mock_kuzu,
    )

    payload = {
        "student_id": "stud-789",
        "bio": "CS junior passionate about distributed systems",
        "department": "Computer Science",
        "graduation_year": 2026,
        "skills": ["Go", "Distributed Systems", "Kubernetes"],
    }

    result = await consumer.process_event("student.updated", payload)

    assert result["status"] == "indexed"
    assert result["student_id"] == "stud-789"
    mock_qdrant.upsert_point.assert_called_once()
    mock_kuzu.add_student.assert_called_once_with(
        student_id="stud-789", department="Computer Science", grad_year=2026
    )
    assert mock_kuzu.add_skill_edge.call_count == 3


@pytest.mark.asyncio
async def test_unhandled_routing_key(mock_qdrant, mock_provider):
    """Unhandled routing keys are safely logged and ignored."""
    consumer = RabbitMQEventConsumer(
        qdrant_client=mock_qdrant,
        embedding_provider=mock_provider,
    )

    result = await consumer.process_event("unrelated.billing.event", {"id": 1})
    assert result["status"] == "ignored"
    mock_qdrant.upsert_point.assert_not_called()


@pytest.mark.asyncio
async def test_consumer_close_graceful():
    """Graceful close handles uninitialized or closed channel cleanly."""
    consumer = RabbitMQEventConsumer()
    await consumer.close()  # No exceptions raised
