"""AI Semantic & Hybrid Search Pipeline.

Combines canonical skill matching, full-text lexical ranking, and pgvector cosine
similarity with Reciprocal Rank Fusion (RRF).
"""

import asyncio
import json
import logging
import re
from typing import Any, Callable, Optional, Union
from pydantic import BaseModel, Field

from ai.app.middleware.run_tracker import AIRunRecord, track_ai_run
from ai.models.embeddings.provider import BaseEmbeddingProvider, get_embedding_provider
from ai.pipelines.skills.normalizer import SkillNormalizer, get_skill_normalizer

logger = logging.getLogger("lynk-ai.pipelines.search")


class SearchResultItem(BaseModel):
    """Ranked item returned by hybrid search."""

    entity_id: str = Field(..., description="Unique entity UUID or user ID")
    score: float = Field(..., description="Relevance score between 0.0 and 1.0")
    snippet: str = Field(default="", description="Relevant textual preview or snippet")
    matched_skills: list[str] = Field(
        default_factory=list, description="Canonical skills matched in search"
    )


def compute_hybrid_score(
    cosine_similarity: float,
    keyword_score: float,
    matched_skills_count: int,
    total_query_skills: int,
    w_semantic: float = 0.50,
    w_keyword: float = 0.30,
    w_skills: float = 0.20,
) -> float:
    """Compute normalized hybrid relevance score in [0.0, 1.0]."""
    cos_sim = max(
        0.0,
        min(
            1.0,
            (cosine_similarity + 1.0) / 2.0,
        ),
    )
    kw_score = max(0.0, min(1.0, keyword_score))

    if total_query_skills > 0:
        skill_score = max(0.0, min(1.0, matched_skills_count / total_query_skills))
    else:
        skill_score = 0.0
        w_semantic += w_skills * 0.6
        w_keyword += w_skills * 0.4
        w_skills = 0.0

    score = w_semantic * cos_sim + w_keyword * kw_score + w_skills * skill_score
    return round(max(0.0, min(1.0, score)), 4)


def reciprocal_rank_fusion(
    rankings: list[dict[str, int]],
    k: int = 60,
) -> list[tuple[str, float]]:
    """Combine multiple ranked lists using Reciprocal Rank Fusion (RRF).

    Args:
        rankings: List of dicts mapping entity_id to 1-based rank.
        k: Smoothing constant (default 60).

    Returns:
        List of (entity_id, rrf_score) sorted in descending order of score.
    """
    scores: dict[str, float] = {}
    for rank_dict in rankings:
        for entity_id, rank in rank_dict.items():
            scores[entity_id] = scores.get(entity_id, 0.0) + 1.0 / (k + rank)

    return sorted(scores.items(), key=lambda x: x[1], reverse=True)


class HybridSearchPipeline:
    """Hybrid search engine orchestrating semantic and lexical search."""

    def __init__(
        self,
        db_pool: Optional[Any] = None,
        embedding_provider: Optional[BaseEmbeddingProvider] = None,
        skill_normalizer: Optional[SkillNormalizer] = None,
        sink: Optional[Union[list[AIRunRecord], Callable[[AIRunRecord], Any]]] = None,
        pipeline_version: str = "1.0.0",
    ) -> None:
        self.db_pool = db_pool
        self._provider = embedding_provider
        self._normalizer = skill_normalizer
        self.sink = sink
        self.pipeline_version = pipeline_version

    @property
    def embedding_provider(self) -> BaseEmbeddingProvider:
        if self._provider is None:
            self._provider = get_embedding_provider()
        return self._provider

    @property
    def skill_normalizer(self) -> SkillNormalizer:
        if self._normalizer is None:
            self._normalizer = get_skill_normalizer()
        return self._normalizer

    async def search_candidates(
        self,
        query: str,
        entity_type: str,
        candidates: list[dict[str, Any]],
        limit: int = 20,
    ) -> list[SearchResultItem]:
        """In-memory hybrid search over candidate records."""
        cleaned_query = query.strip()
        if not cleaned_query or not candidates:
            return []

        async with track_ai_run(
            feature="hybrid_search",
            entity_type=entity_type,
            entity_id=entity_type,
            model_name=self.embedding_provider.model_name,
            model_version=self.embedding_provider.model_version,
            pipeline_version=self.pipeline_version,
            input_data={"query": cleaned_query, "entity_type": entity_type, "limit": limit},
            db_pool=self.db_pool,
            sink=self.sink,
        ) as tracker:
            extracted_skills = self.skill_normalizer.extract_skills_from_text(cleaned_query)
            query_skill_names = [s.canonical_name for s in extracted_skills]

            query_vec = self.embedding_provider.embed(cleaned_query)
            query_tokens = set(re.findall(r"\w+", cleaned_query.lower()))

            scored_items: list[SearchResultItem] = []

            for cand in candidates:
                cand_id = str(cand.get("id") or cand.get("entity_id") or "")
                title = str(cand.get("title") or "")
                desc = str(cand.get("description") or cand.get("bio") or "")
                full_text = f"{title} {desc}".strip()

                cand_skills = cand.get("skills") or cand.get("required_skills") or []
                cand_skills_set = {str(s).lower() for s in cand_skills}

                matched = [
                    s for s in query_skill_names
                    if s.lower() in cand_skills_set or s.lower() in full_text.lower()
                ]

                # Semantic similarity
                if "embedding" in cand and cand["embedding"]:
                    cand_vec = cand["embedding"]
                    cos_sim = sum(q * c for q, c in zip(query_vec, cand_vec))
                else:
                    cand_vec = self.embedding_provider.embed(full_text)
                    cos_sim = sum(q * c for q, c in zip(query_vec, cand_vec))

                # Keyword lexical overlap
                cand_tokens = set(re.findall(r"\w+", full_text.lower()))
                overlap = len(query_tokens & cand_tokens)
                kw_score = overlap / max(len(query_tokens), 1)

                score = compute_hybrid_score(
                    cosine_similarity=cos_sim,
                    keyword_score=kw_score,
                    matched_skills_count=len(matched),
                    total_query_skills=len(query_skill_names),
                )

                snippet = desc[:140] if desc else title
                scored_items.append(
                    SearchResultItem(
                        entity_id=cand_id,
                        score=score,
                        snippet=snippet,
                        matched_skills=matched,
                    )
                )

            scored_items.sort(key=lambda x: x.score, reverse=True)
            top_results = scored_items[:limit]

            tracker.set_output({"results_count": len(top_results), "entity_type": entity_type})
            tracker.set_confidence(top_results[0].score if top_results else 0.0)

            return top_results

    async def search(
        self,
        query: str,
        entity_type: str = "job",
        limit: int = 20,
    ) -> list[SearchResultItem]:
        """Execute hybrid search using PostgreSQL pgvector and full-text search."""
        cleaned_query = query.strip()
        if not cleaned_query:
            return []

        async with track_ai_run(
            feature="hybrid_search",
            entity_type=entity_type,
            entity_id=entity_type,
            model_name=self.embedding_provider.model_name,
            model_version=self.embedding_provider.model_version,
            pipeline_version=self.pipeline_version,
            input_data={"query": cleaned_query, "entity_type": entity_type, "limit": limit},
            db_pool=self.db_pool,
            sink=self.sink,
        ) as tracker:
            extracted_skills = self.skill_normalizer.extract_skills_from_text(cleaned_query)
            query_skill_names = [s.canonical_name for s in extracted_skills]

            query_vec = await asyncio.to_thread(self.embedding_provider.embed, cleaned_query)
            query_vec_json = json.dumps(query_vec)

            results: list[SearchResultItem] = []

            if self.db_pool is not None:
                try:
                    if entity_type == "job":
                        sql = """
                            WITH vector_search AS (
                                SELECT
                                    entity_id,
                                    1 - (embedding <=> $1::vector) AS vector_sim,
                                    ROW_NUMBER() OVER (ORDER BY embedding <=> $1::vector ASC) AS vector_rank
                                FROM ai_embeddings
                                WHERE entity_type = 'job'
                                ORDER BY embedding <=> $1::vector ASC
                                LIMIT 100
                            ),
                            text_search AS (
                                SELECT
                                    id::text AS entity_id,
                                    title,
                                    description,
                                    required_skills,
                                    ts_rank_cd(
                                        to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, '')),
                                        plainto_tsquery('english', $2)
                                    ) AS text_rank_score,
                                    ROW_NUMBER() OVER (
                                        ORDER BY ts_rank_cd(
                                            to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, '')),
                                            plainto_tsquery('english', $2)
                                        ) DESC
                                    ) AS text_rank
                                FROM jobs
                                WHERE status = 'open'
                                  AND to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, '')) @@ plainto_tsquery('english', $2)
                                ORDER BY text_rank_score DESC
                                LIMIT 100
                            )
                            SELECT
                                j.id::text AS entity_id,
                                j.title,
                                j.description,
                                j.required_skills,
                                COALESCE(v.vector_sim, 0.0) AS vector_sim,
                                COALESCE(v.vector_rank, 1000) AS vector_rank,
                                COALESCE(t.text_rank_score, 0.0) AS text_score,
                                COALESCE(t.text_rank, 1000) AS text_rank
                            FROM jobs j
                            LEFT JOIN vector_search v ON v.entity_id = j.id::text
                            LEFT JOIN text_search t ON t.entity_id = j.id::text
                            WHERE j.status = 'open'
                              AND (v.entity_id IS NOT NULL OR t.text_rank_score > 0 OR j.required_skills && $3::text[])
                            ORDER BY (COALESCE(v.vector_sim, 0.0) + COALESCE(t.text_rank_score, 0.0)) DESC
                            LIMIT $4;
                        """
                        async with self.db_pool.acquire() as conn:
                            rows = await conn.fetch(
                                sql, query_vec_json, cleaned_query, query_skill_names, limit * 2
                            )
                            for row in rows:
                                ent_id = str(row["entity_id"])
                                title = row["title"] or ""
                                desc = row["description"] or ""
                                req_skills = row["required_skills"] or []
                                req_skills_set = {s.lower() for s in req_skills}

                                matched = [
                                    s for s in query_skill_names
                                    if s.lower() in req_skills_set or s.lower() in (title + " " + desc).lower()
                                ]

                                v_sim = float(row["vector_sim"])
                                t_score = float(row["text_score"])
                                v_rank = int(row["vector_rank"])
                                t_rank = int(row["text_rank"])

                                rrf_score = (1.0 / (60 + v_rank)) + (1.0 / (60 + t_rank))
                                hybrid_score = compute_hybrid_score(
                                    cosine_similarity=v_sim,
                                    keyword_score=min(1.0, t_score),
                                    matched_skills_count=len(matched),
                                    total_query_skills=len(query_skill_names),
                                )
                                combined_score = round(0.5 * hybrid_score + 0.5 * min(1.0, rrf_score * 30), 4)

                                snippet = desc[:140] if desc else title
                                results.append(
                                    SearchResultItem(
                                        entity_id=ent_id,
                                        score=combined_score,
                                        snippet=snippet,
                                        matched_skills=matched,
                                    )
                                )

                    elif entity_type == "profile":
                        sql = """
                            WITH vector_search AS (
                                SELECT
                                    entity_id,
                                    1 - (embedding <=> $1::vector) AS vector_sim,
                                    ROW_NUMBER() OVER (ORDER BY embedding <=> $1::vector ASC) AS vector_rank
                                FROM ai_embeddings
                                WHERE entity_type = 'profile'
                                ORDER BY embedding <=> $1::vector ASC
                                LIMIT 100
                            ),
                            text_search AS (
                                SELECT
                                    user_id AS entity_id,
                                    first_name,
                                    last_name,
                                    bio,
                                    department,
                                    skills,
                                    ts_rank_cd(
                                        to_tsvector('english', coalesce(first_name, '') || ' ' || coalesce(last_name, '') || ' ' || coalesce(bio, '') || ' ' || coalesce(department, '')),
                                        plainto_tsquery('english', $2)
                                    ) AS text_rank_score,
                                    ROW_NUMBER() OVER (
                                        ORDER BY ts_rank_cd(
                                            to_tsvector('english', coalesce(first_name, '') || ' ' || coalesce(last_name, '') || ' ' || coalesce(bio, '') || ' ' || coalesce(department, '')),
                                            plainto_tsquery('english', $2)
                                        ) DESC
                                    ) AS text_rank
                                FROM profiles
                                WHERE to_tsvector('english', coalesce(first_name, '') || ' ' || coalesce(last_name, '') || ' ' || coalesce(bio, '') || ' ' || coalesce(department, '')) @@ plainto_tsquery('english', $2)
                                ORDER BY text_rank_score DESC
                                LIMIT 100
                            )
                            SELECT
                                p.user_id AS entity_id,
                                p.first_name,
                                p.last_name,
                                p.bio,
                                p.skills,
                                COALESCE(v.vector_sim, 0.0) AS vector_sim,
                                COALESCE(v.vector_rank, 1000) AS vector_rank,
                                COALESCE(t.text_rank_score, 0.0) AS text_score,
                                COALESCE(t.text_rank, 1000) AS text_rank
                            FROM profiles p
                            LEFT JOIN vector_search v ON (v.entity_id = p.user_id OR v.entity_id = p.id::text)
                            LEFT JOIN text_search t ON t.entity_id = p.user_id
                            WHERE (v.entity_id IS NOT NULL OR t.text_rank_score > 0 OR p.skills && $3::text[])
                            ORDER BY (COALESCE(v.vector_sim, 0.0) + COALESCE(t.text_rank_score, 0.0)) DESC
                            LIMIT $4;
                        """
                        async with self.db_pool.acquire() as conn:
                            rows = await conn.fetch(
                                sql, query_vec_json, cleaned_query, query_skill_names, limit * 2
                            )
                            for row in rows:
                                ent_id = str(row["entity_id"])
                                fname = row["first_name"] or ""
                                lname = row["last_name"] or ""
                                bio = row["bio"] or ""
                                p_skills = row["skills"] or []
                                p_skills_set = {s.lower() for s in p_skills}

                                matched = [
                                    s for s in query_skill_names
                                    if s.lower() in p_skills_set or s.lower() in bio.lower()
                                ]

                                v_sim = float(row["vector_sim"])
                                t_score = float(row["text_score"])
                                v_rank = int(row["vector_rank"])
                                t_rank = int(row["text_rank"])

                                rrf_score = (1.0 / (60 + v_rank)) + (1.0 / (60 + t_rank))
                                hybrid_score = compute_hybrid_score(
                                    cosine_similarity=v_sim,
                                    keyword_score=min(1.0, t_score),
                                    matched_skills_count=len(matched),
                                    total_query_skills=len(query_skill_names),
                                )
                                combined_score = round(0.5 * hybrid_score + 0.5 * min(1.0, rrf_score * 30), 4)

                                snippet = bio[:140] if bio else f"{fname} {lname}".strip()
                                results.append(
                                    SearchResultItem(
                                        entity_id=ent_id,
                                        score=combined_score,
                                        snippet=snippet,
                                        matched_skills=matched,
                                    )
                                )
                except Exception as exc:
                    logger.warning("PostgreSQL hybrid search query failed: %s", exc)

            results.sort(key=lambda x: x.score, reverse=True)
            final_results = results[:limit]

            tracker.set_output({"results_count": len(final_results), "entity_type": entity_type})
            tracker.set_confidence(final_results[0].score if final_results else 0.0)

            return final_results


def get_search_pipeline(
    db_pool: Optional[Any] = None,
    embedding_provider: Optional[BaseEmbeddingProvider] = None,
    skill_normalizer: Optional[SkillNormalizer] = None,
) -> HybridSearchPipeline:
    """Create a new HybridSearchPipeline instance."""
    return HybridSearchPipeline(
        db_pool=db_pool,
        embedding_provider=embedding_provider,
        skill_normalizer=skill_normalizer,
    )
