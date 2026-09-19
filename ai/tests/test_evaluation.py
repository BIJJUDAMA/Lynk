"""Comprehensive unit and integration tests for Lynk AI offline evaluation suite."""

import math
from typing import Any

import pytest
from ai.evaluation.harness import EvaluationHarness
from ai.evaluation.metrics import (
    dcg_at_k,
    f1_score,
    mean_reciprocal_rank,
    ndcg_at_k,
    precision_at_k,
    recall_at_k,
    reciprocal_rank,
)
from ai.pipelines.skills.normalizer import SkillNormalizer

# ============================================================================
# Unit Tests: Metrics
# ============================================================================


def test_precision_at_k_basic():
    actual = ["doc1", "doc2", "doc3", "doc4"]
    ground_truth = {"doc1", "doc3"}

    assert precision_at_k(actual, ground_truth, k=1) == pytest.approx(1.0)
    assert precision_at_k(actual, ground_truth, k=2) == pytest.approx(0.5)
    assert precision_at_k(actual, ground_truth, k=3) == pytest.approx(2.0 / 3.0)
    assert precision_at_k(actual, ground_truth, k=4) == pytest.approx(0.5)


def test_precision_at_k_edge_cases():
    actual = ["doc1", "doc2"]
    ground_truth = {"doc1"}

    # k <= 0
    assert precision_at_k(actual, ground_truth, k=0) == 0.0
    assert precision_at_k(actual, ground_truth, k=-5) == 0.0

    # empty actual
    assert precision_at_k([], ground_truth, k=3) == 0.0

    # empty ground truth
    assert precision_at_k(actual, set(), k=2) == 0.0

    # k > len(actual)
    assert precision_at_k(["doc1"], {"doc1"}, k=5) == pytest.approx(0.2)


def test_recall_at_k_basic():
    actual = ["doc1", "doc2", "doc3", "doc4"]
    ground_truth = {"doc1", "doc3", "doc5"}

    assert recall_at_k(actual, ground_truth, k=1) == pytest.approx(1.0 / 3.0)
    assert recall_at_k(actual, ground_truth, k=2) == pytest.approx(1.0 / 3.0)
    assert recall_at_k(actual, ground_truth, k=3) == pytest.approx(2.0 / 3.0)
    assert recall_at_k(actual, ground_truth, k=4) == pytest.approx(2.0 / 3.0)


def test_recall_at_k_edge_cases():
    actual = ["doc1", "doc2"]

    # k <= 0
    assert recall_at_k(actual, {"doc1"}, k=0) == 0.0
    assert recall_at_k(actual, {"doc1"}, k=-1) == 0.0

    # empty ground truth returns 1.0 (no relevant items missed)
    assert recall_at_k(actual, set(), k=2) == 1.0
    assert recall_at_k(actual, [], k=2) == 1.0

    # empty actual with non-empty ground truth returns 0.0
    assert recall_at_k([], {"doc1"}, k=3) == 0.0


def test_reciprocal_rank():
    actual = ["a", "b", "c", "d"]

    # Rank 1
    assert reciprocal_rank(actual, {"a"}) == pytest.approx(1.0)

    # Rank 2
    assert reciprocal_rank(actual, {"b"}) == pytest.approx(0.5)

    # Rank 3
    assert reciprocal_rank(actual, {"c"}) == pytest.approx(1.0 / 3.0)

    # Not found
    assert reciprocal_rank(actual, {"z"}) == 0.0

    # Empty inputs
    assert reciprocal_rank([], {"a"}) == 0.0
    assert reciprocal_rank(actual, set()) == 0.0


def test_mean_reciprocal_rank():
    rankings = [
        (["a", "b", "c"], {"a"}),  # RR = 1.0
        (["a", "b", "c"], {"b"}),  # RR = 0.5
        (["a", "b", "c"], {"c"}),  # RR = 1/3
        (["a", "b", "c"], {"z"}),  # RR = 0.0
    ]
    expected_mrr = (1.0 + 0.5 + (1.0 / 3.0) + 0.0) / 4.0
    assert mean_reciprocal_rank(rankings) == pytest.approx(expected_mrr)

    # Empty rankings list
    assert mean_reciprocal_rank([]) == 0.0


def test_dcg_at_k():
    actual = ["d1", "d2", "d3"]
    relevance = {"d1": 3.0, "d2": 2.0, "d3": 1.0}

    # DCG@3 = (2^3 - 1)/log2(2) + (2^2 - 1)/log2(3) + (2^1 - 1)/log2(4)
    expected_dcg = 7.0 / 1.0 + 3.0 / math.log2(3) + 1.0 / 2.0
    assert dcg_at_k(actual, relevance, k=3) == pytest.approx(expected_dcg)

    # Edge cases
    assert dcg_at_k(actual, relevance, k=0) == 0.0
    assert dcg_at_k(actual, relevance, k=-2) == 0.0
    assert dcg_at_k([], relevance, k=3) == 0.0
    assert dcg_at_k(actual, {}, k=3) == 0.0


def test_ndcg_at_k():
    relevance = {"d1": 3.0, "d2": 2.0, "d3": 1.0}

    # Perfect ranking -> NDCG = 1.0
    perfect_ranking = ["d1", "d2", "d3"]
    assert ndcg_at_k(perfect_ranking, relevance, k=3) == pytest.approx(1.0)

    # Inverted ranking -> 0.0 < NDCG < 1.0
    inverted_ranking = ["d3", "d2", "d1"]
    score = ndcg_at_k(inverted_ranking, relevance, k=3)
    assert 0.0 < score < 1.0

    # Zero relevance -> NDCG = 0.0
    zero_rel = {"d1": 0.0, "d2": 0.0}
    assert ndcg_at_k(["d1", "d2"], zero_rel, k=2) == 0.0

    # Empty inputs
    assert ndcg_at_k([], relevance, k=3) == 0.0
    assert ndcg_at_k(perfect_ranking, {}, k=3) == 0.0
    assert ndcg_at_k(perfect_ranking, relevance, k=0) == 0.0


def test_f1_score():
    assert f1_score(1.0, 1.0) == pytest.approx(1.0)
    assert f1_score(0.8, 0.6) == pytest.approx(2.0 * (0.8 * 0.6) / (0.8 + 0.6))
    assert f1_score(0.0, 0.0) == 0.0
    assert f1_score(1.0, 0.0) == 0.0
    assert f1_score(0.0, 1.0) == 0.0


# ============================================================================
# Integration Tests: Evaluation Harness
# ============================================================================


def test_harness_evaluate_skill_normalization():
    harness = EvaluationHarness()
    normalizer = SkillNormalizer()

    dataset = [
        {"raw_input": "reactjs", "expected_canonical": "React"},
        {"raw_input": "python3", "expected_canonical": "Python"},
        {"raw_input": "golang", "expected_canonical": "Go"},
        {"raw_input": "ts", "expected_canonical": "TypeScript"},
    ]

    metrics = harness.evaluate_skill_normalization(normalizer, dataset)
    assert "accuracy" in metrics
    assert "precision" in metrics
    assert "recall" in metrics
    assert "f1" in metrics
    assert "total" in metrics

    assert metrics["total"] == 4
    assert metrics["accuracy"] == pytest.approx(1.0)
    assert metrics["f1"] == pytest.approx(1.0)


def test_harness_evaluate_ranking_with_mock_ranker():
    class MockRanker:
        async def rank_candidates(self, **kwargs: Any) -> list[Any]:
            # Returns candidates sorted by id
            candidates = kwargs.get("candidates", [])

            class MockResult:
                def __init__(self, app_id: str, score: float):
                    self.application_id = app_id
                    self.score = score

            return [
                MockResult(
                    getattr(c, "application_id", getattr(c, "id", None))
                    if not isinstance(c, dict)
                    else (c.get("id") or c.get("application_id")),
                    100.0 - i * 10,
                )
                for i, c in enumerate(candidates)
            ]

    harness = EvaluationHarness()
    ranking_data = [
        {
            "job": {
                "id": "job-1",
                "title": "Full Stack Dev",
                "required_skills": ["React", "Python"],
                "description": "Building web applications",
                "department": "Engineering",
            },
            "candidates": [
                {
                    "id": "cand-1",
                    "relevance": 3.0,
                    "skills": ["React", "Python"],
                    "bio": "Senior dev",
                },
                {
                    "id": "cand-2",
                    "relevance": 2.0,
                    "skills": ["React"],
                    "bio": "Frontend dev",
                },
                {
                    "id": "cand-3",
                    "relevance": 0.0,
                    "skills": ["Painting"],
                    "bio": "Artist",
                },
            ],
        }
    ]

    ranker = MockRanker()
    metrics = harness.evaluate_ranking(ranker, ranking_data, k=3)
    assert "ndcg_at_k" in metrics
    assert "mrr" in metrics
    assert "precision_at_k" in metrics
    assert "recall_at_k" in metrics

    assert metrics["ndcg_at_k"] == pytest.approx(1.0)
    assert metrics["mrr"] == pytest.approx(1.0)
    assert metrics["precision_at_k"] == pytest.approx(2.0 / 3.0, abs=1e-3)
    assert metrics["recall_at_k"] == pytest.approx(1.0)


def test_harness_load_dataset_and_run_full_evaluation():
    harness = EvaluationHarness()
    # Runs evaluation on bundled sample_eval.json
    results = harness.run_full_evaluation()

    assert "timestamp" in results
    assert "dataset_path" in results
    assert "skill_normalization" in results
    assert "ranking" in results
    assert "summary" in results

    skill_res = results["skill_normalization"]
    assert skill_res["total"] > 0
    assert 0.0 <= skill_res["accuracy"] <= 1.0

    rank_res = results["ranking"]
    assert 0.0 <= rank_res["ndcg_at_k"] <= 1.0
    assert 0.0 <= rank_res["mrr"] <= 1.0
