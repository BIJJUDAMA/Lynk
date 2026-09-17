"""vLLM and Generative LLM client abstraction with structured validation and offline fallback."""

import json
import logging
import re
from typing import Optional

import httpx
from pydantic import TypeAdapter, ValidationError

from ai.app.config import get_settings
from ai.app.middleware.run_tracker import track_ai_run
from ai.llm.prompts.job_generation import PROMPT_VERSION, SYSTEM_PROMPT, USER_PROMPT_TEMPLATE
from ai.llm.schemas.job_generation import GeneratedJobDraft
from ai.pipelines.skills.normalizer import get_skill_normalizer

logger = logging.getLogger("ai.llm")


class VLLMClient:
    """Client for vLLM OpenAI-compatible endpoints with automated retries and heuristic fallback."""

    def __init__(
        self,
        base_url: Optional[str] = None,
        model_name: Optional[str] = None,
        timeout: float = 15.0,
        db_pool=None,
    ) -> None:
        settings = get_settings()
        self.base_url = (base_url or settings.VLLM_URL).rstrip("/")
        self.model_name = model_name or "meta-llama/Meta-Llama-3-8B-Instruct"
        self.model_version = "1.0.0"
        self.prompt_version = PROMPT_VERSION
        self.pipeline_version = "generation-v1"
        self.timeout = timeout
        self.db_pool = db_pool
        self.normalizer = get_skill_normalizer()

    async def generate_job_draft(
        self, idea: str, department: str = ""
    ) -> GeneratedJobDraft:
        """Generate structured job posting draft from rough idea using vLLM or deterministic fallback."""
        clean_idea = idea.strip()
        dept = department.strip() if department else "General"

        async with track_ai_run(
            feature="job_draft_generation",
            entity_type="job_draft",
            entity_id=clean_idea[:32],
            model_name=self.model_name,
            model_version=self.model_version,
            prompt_version=self.prompt_version,
            pipeline_version=self.pipeline_version,
            input_data={"idea": clean_idea, "department": dept},
            db_pool=self.db_pool,
        ) as tracker:
            draft = await self._call_llm_with_fallback(clean_idea, dept)
            tracker.set_output(draft.model_dump())
            tracker.set_confidence(0.9)
            return draft

    async def _call_llm_with_fallback(self, idea: str, department: str) -> GeneratedJobDraft:
        """Attempt vLLM completion with temperature backoff, or fallback on error."""
        endpoint = f"{self.base_url}/chat/completions"
        user_prompt = USER_PROMPT_TEMPLATE.format(idea=idea, department=department)

        # Attempt 1 (temp 0.2), Attempt 2 (temp 0.0)
        for temp in [0.2, 0.0]:
            try:
                async with httpx.AsyncClient(timeout=self.timeout) as client:
                    resp = await client.post(
                        endpoint,
                        json={
                            "model": self.model_name,
                            "messages": [
                                {"role": "system", "content": SYSTEM_PROMPT},
                                {"role": "user", "content": user_prompt},
                            ],
                            "temperature": temp,
                            "max_tokens": 1024,
                            "response_format": {"type": "json_object"},
                        },
                    )
                    if resp.status_code == 200:
                        data = resp.json()
                        choices = data.get("choices", [])
                        if choices and "message" in choices[0] and "content" in choices[0]["message"]:
                            content = choices[0]["message"]["content"]
                            parsed = self._extract_json(content)
                            if parsed is not None:
                                return TypeAdapter(GeneratedJobDraft).validate_python(parsed)
            except Exception as exc:
                logger.debug("vLLM call attempt failed (temp=%.1f): %s", temp, exc)

        logger.info("vLLM unavailable or invalid output, applying deterministic fallback generation")
        return self._heuristic_fallback(idea, department)

    def _extract_json(self, content: str) -> Optional[dict]:
        """Extract and parse JSON from LLM string output, guaranteeing a dictionary."""
        try:
            val = json.loads(content)
            if isinstance(val, dict):
                return val
        except json.JSONDecodeError:
            pass

        # Match first outermost JSON object {...}
        match = re.search(r"\{.*\}", content, re.DOTALL)
        if match:
            try:
                val = json.loads(match.group(0))
                if isinstance(val, dict):
                    return val
            except json.JSONDecodeError:
                pass
        return None

    def _heuristic_fallback(self, idea: str, department: str) -> GeneratedJobDraft:
        """Generate structured draft deterministically when LLM inference is offline."""
        extracted_skills = self.normalizer.extract_skills_from_text(idea)
        skills = [s.canonical_name for s in extracted_skills]

        idea_lower = idea.lower()
        title = "Campus Project Developer"
        if "ios" in idea_lower or "swift" in idea_lower:
            title = "iOS Application Developer"
            if "Swift" not in skills:
                skills.append("Swift")
            if "iOS" not in skills:
                skills.append("iOS")
        elif "react" in idea_lower and "dashboard" in idea_lower:
            title = "React Dashboard Developer"
            if "React" not in skills:
                skills.append("React")
        elif "machine learning" in idea_lower or "pytorch" in idea_lower or "classifier" in idea_lower:
            title = "Machine Learning Engineer"
            if "Python" not in skills:
                skills.append("Python")
            if "PyTorch" not in skills and "pytorch" in idea_lower:
                skills.append("PyTorch")
        elif "next.js" in idea_lower or "landing" in idea_lower:
            title = "Frontend Web Developer"
            if "Next.js" not in skills:
                skills.append("Next.js")
            if "Tailwind CSS" not in skills and "tailwind" in idea_lower:
                skills.append("Tailwind CSS")
        elif skills:
            title = f"{skills[0]} Software Developer"

        if not skills:
            skills = ["General Development", "Problem Solving"]

        dept = department if department and department.lower() != "general" else "Computer Science"

        description = (
            f"We are seeking a student contributor to help develop: {idea.strip()}. "
            f"Key responsibilities include designing components, writing clean and tested code, "
            f"and delivering working deliverables on schedule."
        )

        return GeneratedJobDraft(
            title=title,
            description=description,
            required_skills=skills,
            department=dept,
        )


_global_vllm_client: Optional[VLLMClient] = None


def get_vllm_client(db_pool=None) -> VLLMClient:
    """Singleton-like accessor for VLLMClient."""
    global _global_vllm_client
    if _global_vllm_client is None:
        _global_vllm_client = VLLMClient(db_pool=db_pool)
    elif db_pool is not None and _global_vllm_client.db_pool is None:
        _global_vllm_client.db_pool = db_pool
    return _global_vllm_client
