"""Unit tests for Skill Gap & Learning Pathway Analyzer."""

from ai.pipelines.skills.gap_analyzer import SkillGapAnalyzer


def test_skill_gap_analysis():
    analyzer = SkillGapAnalyzer()
    student_skills = ["Python", "Pandas"]
    project_required_skills = ["Python", "PyTorch", "Docker"]

    result = analyzer.analyze_gap(student_skills, project_required_skills)

    assert "Python" in result.ready_skills
    assert "PyTorch" in result.missing_skills
    assert "Docker" in result.missing_skills
    assert len(result.learning_pathway) > 0
    assert result.readiness_percentage == 33.3  # 1 out of 3
