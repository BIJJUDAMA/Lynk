"""ONNX Runtime INT8-quantized sentence embedding inference pipeline.

Falls back to sentence-transformers if the ONNX model file is not present.
"""

import logging
import os

import numpy as np

logger = logging.getLogger(__name__)

MODEL_NAME = "all-MiniLM-L6-v2"
ONNX_MODEL_PATH = os.environ.get("ONNX_MODEL_PATH", "/models/model_quantized.onnx")
EMBEDDING_DIM = 384


def normalize_vector(vec: np.ndarray) -> np.ndarray:
    """L2-normalize a vector to unit length.

    Args:
        vec: A 1-D numpy array.

    Returns:
        The same vector scaled to unit L2 norm. Returns zeros for a zero vector.
    """
    norm = np.linalg.norm(vec)
    if norm == 0.0:
        return vec
    return vec / norm


def mean_pooling(
    token_embeddings: np.ndarray, attention_mask: np.ndarray
) -> np.ndarray:
    """Compute attention-weighted mean of token embeddings."""
    mask_expanded = attention_mask[:, :, np.newaxis].astype(np.float32)
    sum_embeddings = np.sum(token_embeddings * mask_expanded, axis=1)
    sum_mask = np.maximum(np.sum(mask_expanded, axis=1), 1e-9)
    return sum_embeddings / sum_mask


class ONNXEmbedder:
    """Sentence embedding pipeline using ONNX Runtime INT8-quantized model.

    Falls back to sentence-transformers on missing or invalid ONNX file.
    """

    def __init__(self, model_path: str = ONNX_MODEL_PATH) -> None:
        self._session = None
        self._tokenizer = None
        self._fallback = None
        self._load(model_path)

    def _load(self, model_path: str) -> None:
        """Try to load ONNX model; fall back to sentence-transformers on failure."""
        if not os.path.exists(model_path):
            logger.warning(
                "ONNX model not found at %s, using sentence-transformers fallback",
                model_path,
            )
            self._load_fallback()
            return

        try:
            import onnxruntime as ort
            from transformers import AutoTokenizer

            self._session = ort.InferenceSession(
                model_path, providers=["CPUExecutionProvider"]
            )
            self._tokenizer = AutoTokenizer.from_pretrained(MODEL_NAME)
            logger.info("ONNX Runtime session loaded from %s", model_path)
        except Exception as exc:
            logger.warning("Failed to load ONNX model: %s; falling back", exc)
            self._load_fallback()

    def _load_fallback(self) -> None:
        """Load sentence-transformers as CPU fallback."""
        try:
            from sentence_transformers import SentenceTransformer

            self._fallback = SentenceTransformer(MODEL_NAME)
            logger.info("sentence-transformers fallback loaded")
        except Exception as exc:
            logger.error("Both ONNX and sentence-transformers failed to load: %s", exc)

    def embed(self, text: str) -> np.ndarray | None:
        """Embed a single text string.

        Returns a 384-dim unit-norm numpy float32 array, or None on error.
        """
        if self._session is not None and self._tokenizer is not None:
            return self._embed_onnx(text)
        if self._fallback is not None:
            return self._embed_fallback(text)
        return None

    def _embed_onnx(self, text: str) -> np.ndarray | None:
        """Run ONNX inference on a single text."""
        if self._tokenizer is None or self._session is None:
            return None
        try:
            inputs = self._tokenizer(
                text,
                return_tensors="np",
                truncation=True,
                max_length=256,
                padding=True,
            )
            ort_inputs = {
                "input_ids": inputs["input_ids"].astype(np.int64),
                "attention_mask": inputs["attention_mask"].astype(np.int64),
            }
            if "token_type_ids" in inputs:
                ort_inputs["token_type_ids"] = inputs["token_type_ids"].astype(np.int64)

            outputs = self._session.run(None, ort_inputs)
            token_embeddings = outputs[0]
            pooled = mean_pooling(token_embeddings, inputs["attention_mask"])
            return normalize_vector(pooled[0].astype(np.float32))
        except Exception as exc:
            logger.error("ONNX inference failed: %s", exc)
            return None

    def _embed_fallback(self, text: str) -> np.ndarray | None:
        """Run sentence-transformers inference."""
        if self._fallback is None:
            return None
        try:
            vec = self._fallback.encode(text, normalize_embeddings=True)
            return vec.astype(np.float32)
        except Exception as exc:
            logger.error("sentence-transformers inference failed: %s", exc)
            return None

