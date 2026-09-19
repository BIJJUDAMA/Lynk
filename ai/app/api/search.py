"""Internal API routes for AI semantic and hybrid search."""

from ai.pipelines.search.hybrid import (
    SearchResultItem,
    get_search_pipeline,
)
from fastapi import APIRouter, Request
from pydantic import BaseModel, Field

router = APIRouter(prefix="/internal/v1/search", tags=["search"])


class SearchRequest(BaseModel):
    query: str = Field(
        ..., min_length=1, max_length=500, description="Natural language search query"
    )
    entity_type: str = Field(
        default="job",
        max_length=50,
        description="Entity type to search: 'job' or 'profile'",
    )
    limit: int = Field(default=20, ge=1, le=100, description="Max results to return")


class SearchResponse(BaseModel):
    results: list[SearchResultItem] = Field(
        default_factory=list, description="Ranked list of search results"
    )
    total: int = Field(..., description="Total number of results returned")
    model_name: str = Field(..., description="Embedding model used for query vector")
    model_version: str = Field(..., description="Model version")


@router.post("", response_model=SearchResponse)
async def hybrid_search(payload: SearchRequest, request: Request) -> SearchResponse:
    """Execute hybrid semantic and lexical search across platform entities."""
    pool = getattr(request.app.state, "db_pool", None)
    pipeline = get_search_pipeline(db_pool=pool)

    query = payload.query.strip()
    if not query:
        return SearchResponse(
            results=[],
            total=0,
            model_name=pipeline.embedding_provider.model_name,
            model_version=pipeline.embedding_provider.model_version,
        )

    results = await pipeline.search(
        query=query,
        entity_type=payload.entity_type,
        limit=payload.limit,
    )

    return SearchResponse(
        results=results,
        total=len(results),
        model_name=pipeline.embedding_provider.model_name,
        model_version=pipeline.embedding_provider.model_version,
    )
