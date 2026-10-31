"""Implicit Collaborative Filtering Project Discovery for Campus Students.

100% pure 7-bit ASCII
Uses Alternating Least Squares (iALS) over sparse user-project interaction matrices
(views = 1.0, bookmarks = 2.0, applications = 5.0) to surface relevant student ventures.
"""

import logging
from typing import Any

logger = logging.getLogger("lynk-ai.recommendations")

try:
    import numpy as np  # noqa: F401
    from implicit.als import AlternatingLeastSquares
    from scipy.sparse import csr_matrix

    IMPLICIT_AVAILABLE = True
except ImportError:
    IMPLICIT_AVAILABLE = False


class ImplicitRecommender:
    """Matrix factorization collaborative filtering recommender for peer projects."""

    def __init__(self, factors: int = 16, iterations: int = 15) -> None:
        self.factors = factors
        self.iterations = iterations
        self.user_to_idx: dict[str, int] = {}
        self.idx_to_user: dict[int, str] = {}
        self.item_to_idx: dict[str, int] = {}
        self.idx_to_item: dict[int, str] = {}
        self.interactions: list[tuple[int, int, float]] = []

        if IMPLICIT_AVAILABLE:
            self.model = AlternatingLeastSquares(
                factors=factors,
                iterations=iterations,
                random_state=42,
            )
        else:
            self.model = None

    def record_interaction(
        self, student_id: str, project_id: str, weight: float = 1.0
    ) -> None:
        """Record or update a student-project interaction weight."""
        if student_id not in self.user_to_idx:
            uidx = len(self.user_to_idx)
            self.user_to_idx[student_id] = uidx
            self.idx_to_user[uidx] = student_id
        u = self.user_to_idx[student_id]

        if project_id not in self.item_to_idx:
            iidx = len(self.item_to_idx)
            self.item_to_idx[project_id] = iidx
            self.idx_to_item[iidx] = project_id
        i = self.item_to_idx[project_id]

        self.interactions.append((u, i, weight))

    def _build_sparse_matrix(self) -> Any:
        """Construct CSR user-item matrix."""
        if not self.interactions:
            return None
        rows = [t[0] for t in self.interactions]
        cols = [t[1] for t in self.interactions]
        data = [t[2] for t in self.interactions]
        num_users = len(self.user_to_idx)
        num_items = len(self.item_to_idx)
        return csr_matrix((data, (rows, cols)), shape=(num_users, num_items))

    def fit(self) -> None:
        """Fit the iALS model on current interaction matrix."""
        mat = self._build_sparse_matrix()
        if mat is not None and self.model is not None:
            self.model.fit(mat)

    def recommend_for_student(
        self, student_id: str, n: int = 5
    ) -> list[dict[str, Any]]:
        """Predict top-N recommended projects for a student."""
        if student_id not in self.user_to_idx or not self.interactions:
            return []

        uidx = self.user_to_idx[student_id]
        mat = self._build_sparse_matrix()

        if self.model and mat is not None:
            self.fit()
            ids, scores = self.model.recommend(
                uidx,
                mat[uidx],
                N=n,
                filter_already_liked_items=True,
            )
            recs = []
            for item_idx, score in zip(ids, scores):
                recs.append(
                    {
                        "project_id": self.idx_to_item[int(item_idx)],
                        "score": round(float(score), 4),
                    }
                )
            return recs
        else:
            # Co-interaction heuristic fallback
            user_items = {i for (u, i, w) in self.interactions if u == uidx}
            other_items_scores: dict[int, float] = {}
            for u, i, w in self.interactions:
                if u != uidx and i not in user_items:
                    other_items_scores[i] = other_items_scores.get(i, 0.0) + w

            sorted_items = sorted(
                other_items_scores.items(), key=lambda x: x[1], reverse=True
            )
            return [
                {
                    "project_id": self.idx_to_item[i],
                    "score": round(score, 4),
                }
                for i, score in sorted_items[:n]
            ]
