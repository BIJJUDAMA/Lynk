"""Internal API routes for text embeddings."""

from ai.models.embeddings.provider import get_embedding_provider
from fastapi import APIRouter, Request
from pydantic import BaseModel, Field

router = APIRouter(prefix="/internal/v1/embeddings", tags=["embeddings"])


class EmbeddingRequest(BaseModel):
    text: str = Field(..., min_length=1, max_length=10000, description="Text to embed")


class EmbeddingResponse(BaseModel):
    embedding: list[float] = Field(..., description="Normalized embedding vector")
    dimension: int = Field(..., description="Embedding dimension")
    model_name: str = Field(..., description="Embedding model name")
    model_version: str = Field(..., description="Model version")


@router.post("", response_model=EmbeddingResponse)
@router.post("/", response_model=EmbeddingResponse)
async def generate_embedding(
    payload: EmbeddingRequest, request: Request
) -> EmbeddingResponse:
    provider = get_embedding_provider()
    vec = provider.embed(payload.text)
    return EmbeddingResponse(
        embedding=vec,
        dimension=len(vec),
        model_name=provider.model_name,
        model_version=provider.model_version,
    )
