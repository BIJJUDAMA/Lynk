"""Unit tests for Implicit collaborative filtering project recommender."""

from ai.pipelines.recommendations.implicit_recommender import ImplicitRecommender


def test_implicit_recommender_scoring():
    rec = ImplicitRecommender()
    # Seed interactions
    rec.record_interaction("s1", "p1", weight=5.0)  # s1 applied to p1
    rec.record_interaction("s1", "p2", weight=2.0)  # s1 bookmarked p2
    rec.record_interaction("s2", "p1", weight=5.0)  # s2 applied to p1
    rec.record_interaction("s2", "p3", weight=5.0)  # s2 applied to p3

    # Recommend for s1
    recs = rec.recommend_for_student("s1", n=5)
    assert isinstance(recs, list)
    assert len(recs) > 0
    # p3 should be recommended because s2 also interacted with p1
    rec_ids = [r["project_id"] for r in recs]
    assert "p3" in rec_ids
