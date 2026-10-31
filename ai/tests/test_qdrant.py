"""Unit tests for Qdrant vector store integration."""

from ai.models.embeddings.qdrant_client import QdrantVectorStore


def test_qdrant_vector_store_initialization():
    store = QdrantVectorStore(in_memory=True)
    assert store is not None
    assert store.is_ready()


def test_qdrant_upsert_and_search():
    store = QdrantVectorStore(in_memory=True)
    store.init_collection("campus_students", dim=4)
    store.upsert(
        collection_name="campus_students",
        point_id="s1",
        vector=[0.1, 0.2, 0.3, 0.4],
        payload={"name": "Alice", "skills": ["Python", "Docker"]},
    )
    results = store.search(
        collection_name="campus_students",
        query_vector=[0.1, 0.2, 0.3, 0.4],
        limit=1,
    )
    assert len(results) == 1
    assert results[0]["id"] == "s1"
    assert results[0]["payload"]["name"] == "Alice"
