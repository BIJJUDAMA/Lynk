"""Centralized embedding provider interface, concrete implementations, and content hashing."""

import hashlib
import logging
import math
import random
import sys
import threading
from abc import ABC, abstractmethod
from typing import Any, Optional

logger = logging.getLogger(__name__)


def compute_content_hash(text: str) -> str:
    """Compute deterministic SHA-256 content hash with whitespace normalization.

    Args:
        text: Input string to hash.

    Returns:
        64-character lowercase hexadecimal SHA-256 digest.
    """
    normalized = text.strip()
    return hashlib.sha256(normalized.encode("utf-8")).hexdigest()


class BaseEmbeddingProvider(ABC):
    """Abstract base class for Lynk text embedding providers."""

    @property
    @abstractmethod
    def model_name(self) -> str:
        """Return the underlying model name or identifier."""
        pass

    @property
    @abstractmethod
    def model_version(self) -> str:
        """Return the semantic version of the embedding model."""
        pass

    @property
    @abstractmethod
    def dimension(self) -> int:
        """Return the vector dimensionality of output embeddings."""
        pass

    @abstractmethod
    def embed(self, text: str) -> list[float]:
        """Generate a normalized embedding vector for a single text.

        Args:
            text: Text to embed.

        Returns:
            List of floats representing the embedding vector.
        """
        pass

    @abstractmethod
    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        """Generate normalized embedding vectors for a batch of texts.

        Args:
            texts: List of text strings to embed.

        Returns:
            List of float lists matching vector dimension.
        """
        pass


class MockEmbeddingProvider(BaseEmbeddingProvider):
    """Deterministic mock embedding provider generating normalized vectors based on text seed.

    Useful for ultra-fast, reproducible offline testing and development environments
    without internet access or heavyweight ML runtime dependencies.
    """

    def __init__(
        self,
        model_name: str = "all-MiniLM-L6-v2",
        dimension: int = 384,
        model_version: str = "1.0.0",
    ) -> None:
        self._model_name = model_name
        self._dimension = dimension
        self._model_version = model_version

    @property
    def model_name(self) -> str:
        return self._model_name

    @property
    def model_version(self) -> str:
        return self._model_version

    @property
    def dimension(self) -> int:
        return self._dimension

    def embed(self, text: str) -> list[float]:
        """Generate a deterministic, L2-normalized float vector based on text hash seed."""
        normalized = text.strip()
        digest = hashlib.sha256(normalized.encode("utf-8")).digest()
        seed = int.from_bytes(digest[:8], byteorder="big")

        rng = random.Random(seed)
        vec = [rng.gauss(0.0, 1.0) for _ in range(self._dimension)]

        # L2-normalize to unit length
        norm = math.sqrt(sum(x * x for x in vec))
        if norm > 0:
            vec = [float(x / norm) for x in vec]
        return vec

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        """Batch embed texts using deterministic seed mapping."""
        return [self.embed(t) for t in texts]


class SentenceTransformerEmbeddingProvider(BaseEmbeddingProvider):
    """Concrete embedding provider using sentence-transformers with resilient fallback.

    Default model: 'all-MiniLM-L6-v2' (384 dimensions, version '1.0.0').
    Supports CPU / CUDA device selection and automatic fallback to MockEmbeddingProvider
    if weights cannot be loaded or packages are unavailable offline.
    """

    def __init__(
        self,
        model_name: str = "all-MiniLM-L6-v2",
        device: str = "cpu",
        model_version: str = "1.0.0",
        dimension: int = 384,
        fallback_to_mock: bool = True,
    ) -> None:
        self._model_name = model_name
        self._device = device
        self._model_version = model_version
        self._dimension = dimension
        self._fallback_to_mock = fallback_to_mock
        self._model: Any = None
        self._mock_provider: Optional[MockEmbeddingProvider] = None

        self._initialize_model()

    def _initialize_model(self) -> None:
        """Initialize the SentenceTransformer model or fall back to MockEmbeddingProvider."""
        saved_pyarrow = sys.modules.get("pyarrow", None)
        if "pyarrow" not in sys.modules:
            sys.modules["pyarrow"] = None

        try:
            from sentence_transformers import SentenceTransformer  # type: ignore

            self._model = SentenceTransformer(self._model_name, device=self._device)
            logger.info(
                "SentenceTransformer model '%s' initialized on device '%s'",
                self._model_name,
                self._device,
            )
        except Exception as exc:
            if self._fallback_to_mock:
                logger.warning(
                    "SentenceTransformer unavailable (%s). Falling back to MockEmbeddingProvider.",
                    exc,
                )
                self._mock_provider = MockEmbeddingProvider(
                    model_name=self._model_name,
                    dimension=self._dimension,
                    model_version=self._model_version,
                )
            else:
                raise
        finally:
            if saved_pyarrow is None:
                sys.modules.pop("pyarrow", None)
            else:
                sys.modules["pyarrow"] = saved_pyarrow

    @property
    def model_name(self) -> str:
        return self._model_name

    @property
    def model_version(self) -> str:
        return self._model_version

    @property
    def dimension(self) -> int:
        return self._dimension

    @property
    def is_fallback(self) -> bool:
        """True if the provider is currently operating in fallback mock mode."""
        return self._mock_provider is not None

    def embed(self, text: str) -> list[float]:
        """Embed a single text string."""
        if self._mock_provider is not None:
            return self._mock_provider.embed(text)

        raw = self._model.encode(
            text,
            normalize_embeddings=True,
            convert_to_numpy=True,
        )
        return [float(x) for x in raw.tolist()]

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        """Embed a batch of text strings."""
        if not texts:
            return []
        if self._mock_provider is not None:
            return self._mock_provider.embed_batch(texts)

        raw = self._model.encode(
            texts,
            normalize_embeddings=True,
            convert_to_numpy=True,
        )
        return [[float(x) for x in vec] for vec in raw.tolist()]



_PROVIDER_CACHE: dict[tuple[str, tuple[tuple[str, Any], ...]], BaseEmbeddingProvider] = {}
_CACHE_LOCK = threading.Lock()


def _freeze_kwargs(kwargs: dict[str, Any]) -> tuple[tuple[str, Any], ...]:
    """Convert kwargs dict to a sorted, hashable tuple of key-value pairs."""
    items: list[tuple[str, Any]] = []
    for k, v in sorted(kwargs.items()):
        if isinstance(v, list):
            v = tuple(v)
        elif isinstance(v, dict):
            v = _freeze_kwargs(v)
        items.append((k, v))
    return tuple(items)


def clear_provider_cache() -> None:
    """Clear all cached embedding provider singletons."""
    with _CACHE_LOCK:
        _PROVIDER_CACHE.clear()


def get_embedding_provider(
    provider_type: str = "sentence-transformer",
    **kwargs: Any,
) -> BaseEmbeddingProvider:
    """Factory function to acquire a cached singleton embedding provider instance.

    Providers are cached by canonical provider type and configuration kwargs
    so repeated calls return the same singleton instance, eliminating redundant
    PyTorch/transformer model reloads.

    Args:
        provider_type: Type of provider ('sentence-transformer', 'mock').
        **kwargs: Additional arguments passed to provider constructor.

    Returns:
        Cached instance conforming to BaseEmbeddingProvider.

    Raises:
        ValueError: If provider_type is unsupported.
    """
    normalized_type = provider_type.strip().lower()
    if normalized_type in ("sentence-transformer", "sentence_transformer", "st"):
        canonical_type = "sentence-transformer"
    elif normalized_type in ("mock", "test", "deterministic"):
        canonical_type = "mock"
    else:
        raise ValueError(
            f"Unknown embedding provider type: '{provider_type}'. "
            "Supported providers: 'sentence-transformer', 'mock'."
        )

    cache_key = (canonical_type, _freeze_kwargs(kwargs))

    # Fast path: check cache before acquiring lock
    cached_instance = _PROVIDER_CACHE.get(cache_key)
    if cached_instance is not None:
        return cached_instance

    with _CACHE_LOCK:
        # Double-check inside lock
        if cache_key in _PROVIDER_CACHE:
            return _PROVIDER_CACHE[cache_key]

        if canonical_type == "sentence-transformer":
            instance: BaseEmbeddingProvider = SentenceTransformerEmbeddingProvider(**kwargs)
        else:
            instance = MockEmbeddingProvider(**kwargs)

        _PROVIDER_CACHE[cache_key] = instance
        return instance

