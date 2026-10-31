"""Unit tests for Kuzu embedded columnar graph engine."""

import os
import tempfile
from ai.pipelines.graph.kuzu_engine import KuzuGraphEngine


def test_kuzu_engine_schema_and_queries():
    with tempfile.TemporaryDirectory() as tmp_dir:
        db_path = os.path.join(tmp_dir, "test_kuzu_db")
        engine = KuzuGraphEngine(db_path=db_path)
        engine.init_schema()

        # Add students
        engine.add_student("s1", "Computer Science")
        engine.add_student("s2", "Design")
        engine.add_student("s3", "Computer Science")

        # Add skills
        engine.add_skill("sk_python", "Python", "Backend")
        engine.add_skill("sk_react", "React", "Frontend")
        engine.add_skill("sk_figma", "Figma", "Design")

        # Add student skills
        engine.add_student_skill("s1", "sk_python")
        engine.add_student_skill("s2", "sk_figma")
        engine.add_student_skill("s3", "sk_react")

        # Add project requiring Python and Figma
        engine.add_project("p1", "Campus Navigation App")
        engine.add_project_required_skill("p1", "sk_python")
        engine.add_project_required_skill("p1", "sk_figma")

        # Test querying student skills
        s1_skills = engine.get_student_skills("s1")
        assert "sk_python" in s1_skills

        # Test finding complementary peers for project p1
        # Team already has s1 (Python), missing Figma
        peers = engine.find_complementary_peers(project_id="p1", current_member_ids=["s1"])
        assert len(peers) > 0
        assert peers[0]["student_id"] == "s2"  # s2 brings Figma
