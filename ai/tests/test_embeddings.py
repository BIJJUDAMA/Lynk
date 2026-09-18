import hashlib
import math
import pytest

from ai.models.embeddings.provider import (
    BaseEmbeddingProvider,
    SentenceTransformerEmbeddingProvider,
    MockEmbeddingProvider,
    compute_content_hash,
    get_embedding_provider,
    clear_provider_cache,
)


def test_content_hash_consistency_and_normalization():
    text_raw = "  Backend Engineer with Go and PostgreSQL experience   \n"
    text_clean = "Backend Engineer with Go and PostgreSQL experience"
    distinct_text = "Frontend Developer with React and Tailwind CSS"

    expected_hash = hashlib.sha256(text_clean.encode("utf-8")).hexdigest()

    hash_raw = compute_content_hash(text_raw)
    hash_clean = compute_content_hash(text_clean)
    hash_distinct = compute_content_hash(distinct_text)

    assert hash_raw == expected_hash
    assert hash_clean == expected_hash
    assert hash_raw == hash_clean
    assert len(hash_raw) == 64
    assert hash_raw != hash_distinct


@pytest.mark.parametrize(
    "provider_type",
    ["mock", "sentence-transformer"],
)
def test_embed_single_vector_shape_and_normalization(provider_type: str):
    provider = get_embedding_provider(provider_type)
    text = "Machine learning engineer building search and embeddings."

    vec = provider.embed(text)

    assert isinstance(vec, list)
    assert len(vec) == 384
    assert len(vec) == provider.dimension
    assert all(isinstance(x, float) for x in vec)

    # Verify L2 normalization (sum of squares ~ 1.0)
    l2_norm = math.sqrt(sum(x * x for x in vec))
    assert abs(l2_norm - 1.0) < 1e-4


@pytest.mark.parametrize(
    "provider_type",
    ["mock", "sentence-transformer"],
)
def test_embed_batch_dimensions(provider_type: str):
    provider = get_embedding_provider(provider_type)
    texts = [
        "First campus project document",
        "Second student freelance assignment",
        "Third verified university portfolio entry",
    ]

    batch = provider.embed_batch(texts)

    assert isinstance(batch, list)
    assert len(batch) == len(texts)

    for vec in batch:
        assert isinstance(vec, list)
        assert len(vec) == provider.dimension
        assert all(isinstance(x, float) for x in vec)
        l2_norm = math.sqrt(sum(x * x for x in vec))
        assert abs(l2_norm - 1.0) < 1e-4


@pytest.mark.parametrize(
    "provider_type",
    ["mock", "sentence-transformer"],
)
def test_single_embed_matches_batch_embed(provider_type: str):
    provider = get_embedding_provider(provider_type)
    target_text = "Full-stack developer with React and PostgreSQL experience"
    distractor_text = "Unrelated campus event coordinator"

    single_vec = provider.embed(target_text)
    batch_vecs = provider.embed_batch([target_text, distractor_text])

    assert len(batch_vecs) == 2
    batch_target_vec = batch_vecs[0]

    assert len(single_vec) == len(batch_target_vec)
    max_diff = max(abs(s - b) for s, b in zip(single_vec, batch_target_vec))
    assert max_diff < 1e-4


@pytest.mark.parametrize(
    "provider_type",
    ["mock", "sentence-transformer"],
)
def test_model_metadata(provider_type: str):
    provider = get_embedding_provider(provider_type)

    assert provider.model_name == "all-MiniLM-L6-v2"
    assert provider.model_version == "1.0.0"
    assert provider.dimension == 384


def test_empty_batch():
    provider = MockEmbeddingProvider()
    assert provider.embed_batch([]) == []


def test_mock_determinism_and_differentiation():
    provider = MockEmbeddingProvider()
    vec1 = provider.embed("Campus gig developer")
    vec2 = provider.embed("Campus gig developer")
    vec3 = provider.embed("Different text altogether")

    assert vec1 == vec2
    assert vec1 != vec3


def test_invalid_provider_type():
    with pytest.raises(ValueError, match="Unknown embedding provider type"):
        get_embedding_provider("unsupported-unknown-provider")


def test_embedding_provider_is_singleton():
    p1 = get_embedding_provider("sentence-transformer")
    p2 = get_embedding_provider("sentence-transformer")
    assert p1 is p2

    p_default = get_embedding_provider()
    assert p_default is p1

    p_alias = get_embedding_provider("sentence_transformer")
    assert p_alias is p1

    m1 = get_embedding_provider("mock")
    m2 = get_embedding_provider("mock")
    assert m1 is m2

    m_kw1 = get_embedding_provider("mock", dimension=256)
    m_kw2 = get_embedding_provider("mock", dimension=256)
    assert m_kw1 is m_kw2
    assert m_kw1 is not m1


def test_clear_provider_cache():
    p1 = get_embedding_provider("mock")
    clear_provider_cache()
    p2 = get_embedding_provider("mock")
    assert p1 is not p2


