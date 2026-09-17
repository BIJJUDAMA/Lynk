"""Information retrieval and ranking evaluation metrics.

Provides standard evaluation metrics:
- Precision@K
- Recall@K
- Reciprocal Rank (RR) and Mean Reciprocal Rank (MRR)
- Discounted Cumulative Gain (DCG@K)
- Normalized Discounted Cumulative Gain (NDCG@K)
- F1 Score
"""

import math
from typing import Any, TypeVar

T = TypeVar("T")


def precision_at_k(
    actual: list[T],
    ground_truth: list[T] | set[T],
    k: int,
) -> float:
    """Compute precision of top-k items in actual against ground truth.

    Args:
        actual: Ordered list of retrieved or ranked items.
        ground_truth: Set or list of relevant items.
        k: Number of top items to evaluate. If k <= 0, returns 0.0.

    Returns:
        Fraction of top-k items that are relevant (0.0 to 1.0).
    """
    if k <= 0 or not actual or not ground_truth:
        return 0.0

    gt_set = set(ground_truth)
    top_k = actual[:k]
    hits = sum(1 for item in top_k if item in gt_set)
    return float(hits) / float(k)


def recall_at_k(
    actual: list[T],
    ground_truth: list[T] | set[T],
    k: int,
) -> float:
    """Compute recall of ground truth items captured in top-k actual items.

    Args:
        actual: Ordered list of retrieved or ranked items.
        ground_truth: Set or list of relevant items.
        k: Number of top items to evaluate. If k <= 0, returns 0.0.

    Returns:
        Fraction of relevant items found in top-k (0.0 to 1.0).
        If ground_truth is empty, returns 1.0.
    """
    if k <= 0:
        return 0.0
    if not ground_truth:
        return 1.0
    if not actual:
        return 0.0

    gt_set = set(ground_truth)
    top_k_set = set(actual[:k])
    hits = len(top_k_set & gt_set)
    return float(hits) / float(len(gt_set))


def reciprocal_rank(
    actual: list[T],
    relevant: list[T] | set[T],
) -> float:
    """Compute reciprocal rank for a single ranked list.

    Args:
        actual: Ordered list of retrieved or ranked items.
        relevant: Set or list of relevant items.

    Returns:
        1.0 / (rank of first relevant item, 1-indexed), or 0.0 if not found.
    """
    if not actual or not relevant:
        return 0.0

    rel_set = set(relevant)
    for idx, item in enumerate(actual, start=1):
        if item in rel_set:
            return 1.0 / float(idx)
    return 0.0


def mean_reciprocal_rank(
    rankings: list[Any],
) -> float:
    """Compute Mean Reciprocal Rank (MRR) across multiple query rankings.

    Args:
        rankings: List of tuples (actual, relevant) or lists representing ranked items.

    Returns:
        Mean of reciprocal ranks across queries (0.0 to 1.0).
    """
    if not rankings:
        return 0.0

    rr_values: list[float] = []
    for item in rankings:
        if isinstance(item, (tuple, list)) and len(item) == 2 and isinstance(item[1], (set, list, tuple, frozenset)):
            actual, relevant = item
            rr_values.append(reciprocal_rank(actual, relevant))
        elif isinstance(item, (int, float)):
            rr_values.append(float(item))
        elif isinstance(item, (list, tuple)):
            found = False
            for idx, val in enumerate(item, start=1):
                if val:
                    rr_values.append(1.0 / float(idx))
                    found = True
                    break
            if not found:
                rr_values.append(0.0)
        else:
            rr_values.append(0.0)

    return float(sum(rr_values)) / float(len(rr_values)) if rr_values else 0.0


def dcg_at_k(
    actual: list[T],
    ground_truth_relevance: dict[T, float],
    k: int,
) -> float:
    """Compute Discounted Cumulative Gain at rank k (DCG@K).

    Formula: sum_{i=1}^k (2^{rel_i} - 1) / log2(i + 1)

    Args:
        actual: Ordered list of ranked items.
        ground_truth_relevance: Dict mapping items to graded relevance scores.
        k: Cutoff rank. If k <= 0, returns 0.0.

    Returns:
        DCG score.
    """
    if k <= 0 or not actual or not ground_truth_relevance:
        return 0.0

    top_k = actual[:k]
    dcg = 0.0
    for i, item in enumerate(top_k, start=1):
        rel = float(ground_truth_relevance.get(item, 0.0))
        if rel > 0.0:
            dcg += (2.0 ** rel - 1.0) / math.log2(i + 1)
    return dcg


def ndcg_at_k(
    actual: list[T],
    ground_truth_relevance: dict[T, float],
    k: int,
) -> float:
    """Compute Normalized Discounted Cumulative Gain at rank k (NDCG@K).

    Args:
        actual: Ordered list of ranked items.
        ground_truth_relevance: Dict mapping items to graded relevance scores.
        k: Cutoff rank.

    Returns:
        NDCG score between 0.0 and 1.0. Returns 0.0 if IDCG is 0.0.
    """
    if k <= 0 or not actual or not ground_truth_relevance:
        return 0.0

    actual_dcg = dcg_at_k(actual, ground_truth_relevance, k)
    if actual_dcg <= 0.0:
        return 0.0

    # Ideal ranking: take highest relevance scores up to k
    ideal_relevances = sorted(
        [float(v) for v in ground_truth_relevance.values() if float(v) > 0.0],
        reverse=True,
    )[:k]

    if not ideal_relevances:
        return 0.0

    idcg = sum(
        (2.0 ** rel - 1.0) / math.log2(i + 1)
        for i, rel in enumerate(ideal_relevances, start=1)
    )

    if idcg <= 0.0:
        return 0.0

    return min(1.0, max(0.0, actual_dcg / idcg))


def f1_score(precision: float, recall: float) -> float:
    """Compute F1 score as harmonic mean of precision and recall.

    Formula: 2 * (precision * recall) / (precision + recall)

    Args:
        precision: Precision value.
        recall: Recall value.

    Returns:
        F1 score (0.0 to 1.0). Returns 0.0 if precision + recall == 0.
    """
    if precision <= 0.0 or recall <= 0.0:
        return 0.0
    denom = precision + recall
    if denom <= 0.0:
        return 0.0
    return (2.0 * precision * recall) / denom
