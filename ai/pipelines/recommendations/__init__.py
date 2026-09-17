"""Recommendation pipeline package for campus member profile recommendations."""

from ai.pipelines.recommendations.engine import (
    RecommendationEngine,
    RecommendationItem,
    get_recommendation_engine,
)

__all__ = [
    "RecommendationEngine",
    "RecommendationItem",
    "get_recommendation_engine",
]
