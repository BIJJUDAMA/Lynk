"""Prompt templates for the 4-Pillar Collaboration LLM Client.

100% pure 7-bit ASCII
"""

SYSTEM_COLLABORATION_PROMPT = """You are the Lynk Campus Collaboration Assistant.
Lynk is a student-to-student platform where students find peers, form interdisciplinary teams, and build projects together.
Never adopt a corporate HR or employer hiring tone. Focus entirely on peer collaboration, clear division of responsibilities, and practical student execution.
Always output valid RFC 8259 JSON matching the requested schema.
"""

PARSE_QUERY_TEMPLATE = """Extract structured discovery filters from this student query:
Query: "{query}"

Return JSON:
{{
  "intent": "find_teammate" or "find_project",
  "extracted_skills": ["Skill1", "Skill2"],
  "preferred_department": "Department name or Any"
}}
"""

CREATE_PROJECT_TEMPLATE = """Turn this rough student project thought into a structured project spec:
Idea: "{idea}"
Category: "{category}"

Return JSON matching GeneratedProjectSpec.
"""

PLAN_MILESTONES_TEMPLATE = """Break these project deliverables into a realistic 3-phase student roadmap:
Project: "{title}"
Deliverables: {deliverables}

Return JSON matching ProjectMilestonePlan.
"""
