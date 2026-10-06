import numpy as np
from ai.pipelines.onnx_embeddings import mean_pooling, normalize_vector


def test_normalize_vector():
    """Normalized vector must have unit L2 norm."""
    vec = np.array([3.0, 4.0])
    norm = normalize_vector(vec)
    assert np.isclose(np.linalg.norm(norm), 1.0)


def test_normalize_vector_zero():
    """Zero vector should not raise; returns the zero vector."""
    vec = np.zeros(5)
    result = normalize_vector(vec)
    assert np.allclose(result, 0.0)


def test_mean_pooling_shape():
    """Mean pooling output shape must be (batch, hidden)."""
    token_embeddings = np.ones((1, 10, 8), dtype=np.float32)
    attention_mask = np.ones((1, 10), dtype=np.float32)
    pooled = mean_pooling(token_embeddings, attention_mask)
    assert pooled.shape == (1, 8)


def test_embed_onnx_dynamic_padding():
    """_embed_onnx must call tokenizer with padding=True instead of max_length."""
    from unittest.mock import MagicMock

    from ai.pipelines.onnx_embeddings import ONNXEmbedder

    embedder = ONNXEmbedder.__new__(ONNXEmbedder)
    mock_session = MagicMock()
    mock_tokenizer = MagicMock()

    mock_inputs = {
        "input_ids": np.ones((1, 8), dtype=np.int64),
        "attention_mask": np.ones((1, 8), dtype=np.int64),
    }
    mock_tokenizer.return_value = mock_inputs
    mock_session.run.return_value = [np.ones((1, 8, 384), dtype=np.float32)]

    embedder._session = mock_session
    embedder._tokenizer = mock_tokenizer

    result = embedder._embed_onnx("test sentence for dynamic padding")
    assert result is not None
    assert result.shape == (384,)
    mock_tokenizer.assert_called_once_with(
        "test sentence for dynamic padding",
        return_tensors="np",
        truncation=True,
        max_length=256,
        padding=True,
    )
