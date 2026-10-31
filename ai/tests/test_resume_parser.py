"""Unit tests for Resume Intelligence and Student Profile Auto-Builder."""

from ai.pipelines.document.resume_parser import ResumeParser


def test_resume_parser_student_profile_extraction():
    parser = ResumeParser()
    sample_text = """
    Junior Computer Science student at Berkeley.
    Experienced in full-stack web development and distributed backend architectures.
    Technical Skills: Python, React, Go, Docker, PostgreSQL, TypeScript.
    Built a decentralized peer exchange platform and a campus laundry notification service.
    """
    result = parser.extract_profile_from_text(sample_text)

    assert result.major in ["Computer Science", "Engineering"]
    assert "Python" in result.matched_skills
    assert "React" in result.matched_skills
    assert "Go" in result.matched_skills
    assert "Docker" in result.matched_skills
    assert len(result.suggested_bio) > 20
