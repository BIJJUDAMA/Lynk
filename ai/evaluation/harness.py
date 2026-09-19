"""Evaluation harness for Lynk AI models and pipelines.

Loads evaluation datasets, runs benchmark suites, and computes performance
metrics for skill normalization, candidate ranking, and search retrieval.
"""

import asyncio
import concurrent.futures
import json
import logging
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from ai.evaluation.metrics import (
    f1_score,
    ndcg_at_k,
    precision_at_k,
    recall_at_k,
    reciprocal_rank,
)

logger = logging.getLogger("lynk-ai.evaluation")

DEFAULT_DATASET_PATH = Path(__file__).parent / "datasets" / "sample_eval.json"


def _run_async_safely(coro: Any) -> Any:
    """Run an async coroutine safely across both sync and async runtime contexts."""
    try:
        loop = asyncio.get_running_loop()
    except RuntimeError:
        loop = None

    if loop and loop.is_running():
        with concurrent.futures.ThreadPoolExecutor(max_workers=1) as executor:
            future = executor.submit(asyncio.run, coro)
            return future.result()
    return asyncio.run(coro)


class EvaluationHarness:
    """Offline benchmark and evaluation harness for the Lynk AI subsystem."""

    def __init__(self, dataset_path: str | Path | None = None) -> None:
        self.dataset_path = Path(dataset_path) if dataset_path else DEFAULT_DATASET_PATH
        self._dataset: dict[str, Any] | None = None

    def load_dataset(self, path: str | Path | None = None) -> dict[str, Any]:
        """Load evaluation dataset from JSON file."""
        target_path = Path(path) if path else self.dataset_path
        if not target_path.exists():
            raise FileNotFoundError(f"Evaluation dataset not found at: {target_path}")

        with open(target_path, encoding="utf-8") as f:
            self._dataset = json.load(f)
        return self._dataset

    @property
    def dataset(self) -> dict[str, Any]:
        """Acquire cached evaluation dataset."""
        if self._dataset is None:
            self.load_dataset()
        return self._dataset  # type: ignore[return-value]

    def evaluate_skill_normalization(
        self,
        normalizer: Any = None,
        skill_dataset: list[dict[str, Any]] | None = None,
    ) -> dict[str, float]:
        """Evaluate skill normalization accuracy, precision, recall, and F1.

        Args:
            normalizer: SkillNormalizer instance or callable. If None, acquires default.
            skill_dataset: List of ground-truth test items with 'raw_input' and 'expected_canonical'.

        Returns:
            Dict containing accuracy, precision, recall, f1, total, and correct counts.
        """
        if normalizer is None:
            from ai.pipelines.skills.normalizer import get_skill_normalizer

            normalizer = get_skill_normalizer()

        dataset = (
            skill_dataset
            if skill_dataset is not None
            else self.dataset.get("skills", [])
        )
        if not dataset:
            return {
                "accuracy": 0.0,
                "precision": 0.0,
                "recall": 0.0,
                "f1": 0.0,
                "total": 0.0,
                "correct": 0.0,
            }

        total = len(dataset)
        correct = 0

        for item in dataset:
            raw_input = item["raw_input"]
            expected = str(item["expected_canonical"]).strip().lower()

            if hasattr(normalizer, "normalize_skill"):
                res = normalizer.normalize_skill(raw_input)
            elif hasattr(normalizer, "normalize"):
                res = normalizer.normalize(raw_input)
            elif callable(normalizer):
                res = normalizer(raw_input)
            else:
                raise TypeError(f"Unsupported normalizer type: {type(normalizer)}")

            if hasattr(res, "canonical_name"):
                predicted = str(res.canonical_name).strip().lower()
            elif isinstance(res, dict) and "canonical_name" in res:
                predicted = str(res["canonical_name"]).strip().lower()
            else:
                predicted = str(res).strip().lower()

            if predicted == expected:
                correct += 1

        accuracy = float(correct) / float(total) if total > 0 else 0.0
        # For standard 1-to-1 classification tasks where every instance has a prediction:
        precision = accuracy
        recall = accuracy
        f1 = f1_score(precision, recall)

        return {
            "accuracy": round(accuracy, 4),
            "precision": round(precision, 4),
            "recall": round(recall, 4),
            "f1": round(f1, 4),
            "total": float(total),
            "correct": float(correct),
        }

    def evaluate_ranking(
        self,
        ranker: Any = None,
        ranking_dataset: list[dict[str, Any]] | None = None,
        k: int = 5,
    ) -> dict[str, float]:
        """Evaluate candidate ranking quality using NDCG@K, MRR, and Precision@K.

        Args:
            ranker: CandidateRanker instance or mock. If None, acquires default.
            ranking_dataset: List of ranking scenarios with 'job' and 'candidates'.
            k: Cutoff rank for NDCG and Precision.

        Returns:
            Dict containing mean ndcg_at_k, mrr, precision_at_k, and recall_at_k.
        """
        if ranker is None:
            from ai.pipelines.ranking.ranker import get_candidate_ranker

            ranker = get_candidate_ranker()

        scenarios = (
            ranking_dataset
            if ranking_dataset is not None
            else self.dataset.get("ranking", [])
        )
        if not scenarios:
            return {
                "ndcg_at_k": 0.0,
                "mrr": 0.0,
                "precision_at_k": 0.0,
                "recall_at_k": 0.0,
                "k": float(k),
                "scenarios_evaluated": 0.0,
            }

        ndcg_scores: list[float] = []
        mrr_scores: list[float] = []
        prec_scores: list[float] = []
        recall_scores: list[float] = []

        for scenario in scenarios:
            job = scenario["job"]
            candidates_raw = scenario["candidates"]

            # Format candidates
            from ai.pipelines.ranking.ranker import CandidateRankInput

            cand_inputs = []
            for c in candidates_raw:
                cid = c.get("id") or c.get("application_id", "")
                if isinstance(c, CandidateRankInput):
                    cand_inputs.append(c)
                elif isinstance(c, dict):
                    cand_inputs.append(
                        CandidateRankInput(
                            application_id=cid,
                            skills=c.get("skills", []),
                            bio=c.get("bio"),
                            department=c.get("department"),
                            cover_letter=c.get("cover_letter"),
                        )
                    )
                else:
                    cand_inputs.append(c)

            # Invoke ranker
            if hasattr(ranker, "rank_candidates"):
                coro_or_res = ranker.rank_candidates(
                    job_id=job.get("id", "test-job"),
                    job_title=job.get("title", ""),
                    job_description=job.get("description", ""),
                    job_department=job.get("department", ""),
                    required_skills=job.get("required_skills", []),
                    candidates=cand_inputs,
                )
                if asyncio.iscoroutine(coro_or_res):
                    results = _run_async_safely(coro_or_res)
                else:
                    results = coro_or_res
            elif callable(ranker):
                coro_or_res = ranker(job=job, candidates=cand_inputs)
                if asyncio.iscoroutine(coro_or_res):
                    results = _run_async_safely(coro_or_res)
                else:
                    results = coro_or_res
            else:
                raise TypeError(f"Unsupported ranker type: {type(ranker)}")

            # Extract ordered actual candidate IDs
            actual_ids: list[str] = []
            for r in results:
                if hasattr(r, "application_id"):
                    actual_ids.append(r.application_id)
                elif isinstance(r, dict) and "application_id" in r:
                    actual_ids.append(r["application_id"])
                elif isinstance(r, dict) and "id" in r:
                    actual_ids.append(r["id"])
                elif hasattr(r, "id"):
                    actual_ids.append(r.id)
                else:
                    actual_ids.append(str(r))

            # Ground truth relevance map
            gt_relevance: dict[str, float] = {}
            for c in candidates_raw:
                cid = c.get("id") or c.get("application_id", "")
                gt_relevance[cid] = float(c.get("relevance", 0.0))

            relevant_ids = {cid for cid, rel in gt_relevance.items() if rel > 0.0}

            ndcg_scores.append(ndcg_at_k(actual_ids, gt_relevance, k=k))
            mrr_scores.append(reciprocal_rank(actual_ids, relevant_ids))
            prec_scores.append(precision_at_k(actual_ids, relevant_ids, k=k))
            recall_scores.append(recall_at_k(actual_ids, relevant_ids, k=k))

        count = float(len(scenarios))
        return {
            "ndcg_at_k": round(sum(ndcg_scores) / count, 4),
            "mrr": round(sum(mrr_scores) / count, 4),
            "precision_at_k": round(sum(prec_scores) / count, 4),
            "recall_at_k": round(sum(recall_scores) / count, 4),
            "k": float(k),
            "scenarios_evaluated": count,
        }

    def run_full_evaluation(
        self,
        normalizer: Any = None,
        ranker: Any = None,
        dataset_path: str | Path | None = None,
    ) -> dict[str, Any]:
        """Execute complete offline evaluation benchmark across all tasks.

        Args:
            normalizer: SkillNormalizer instance. If None, acquires default.
            ranker: CandidateRanker instance. If None, acquires default.
            dataset_path: Path to dataset file. If None, uses default.

        Returns:
            Structured evaluation report with timestamp, sub-task metrics, and summary.
        """
        if dataset_path:
            self.load_dataset(dataset_path)

        skill_metrics = self.evaluate_skill_normalization(normalizer=normalizer)
        ranking_metrics = self.evaluate_ranking(ranker=ranker)

        status = (
            "PASS"
            if skill_metrics["accuracy"] >= 0.8 and ranking_metrics["ndcg_at_k"] >= 0.7
            else "WARN"
        )

        return {
            "timestamp": datetime.now(UTC).isoformat(),
            "dataset_path": str(self.dataset_path),
            "skill_normalization": skill_metrics,
            "ranking": ranking_metrics,
            "summary": {
                "status": status,
                "skill_accuracy": skill_metrics.get("accuracy", 0.0),
                "ranking_ndcg": ranking_metrics.get("ndcg_at_k", 0.0),
                "ranking_mrr": ranking_metrics.get("mrr", 0.0),
            },
        }


if __name__ == "__main__":
    import json

    harness = EvaluationHarness()
    report = harness.run_full_evaluation()
    print(json.dumps(report, indent=2))
