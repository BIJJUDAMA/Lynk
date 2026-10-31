"""Resume Intelligence and Student Profile Auto-Builder.

100% pure 7-bit ASCII
Extracts major, canonical skills, and suggested bios from student resumes
using layout-aware parsing and rapid token-ratio fuzzy matching against CANONICAL_TAXONOMY.
"""

import logging
import re
from typing import Any
from pydantic import BaseModel, Field

logger = logging.getLogger("lynk-ai.document")

try:
    from rapidfuzz import fuzz, process
    RAPIDFUZZ_AVAILABLE = True
except ImportError:
    RAPIDFUZZ_AVAILABLE = False

from ai.pipelines.skills.normalizer import CANONICAL_TAXONOMY


class ParsedStudentProfile(BaseModel):
    """Structured extraction result from student resume."""
    major: str = Field(default="Undeclared", description="Inferred academic major or department")
    matched_skills: list[str] = Field(default_factory=list, description="Canonical verified skills")
    suggested_bio: str = Field(default="", description="Concise bio summary extracted from resume")
    experience_highlights: list[str] = Field(default_factory=list, description="Key project or role bullets")


MAJOR_KEYWORDS = {
    "Computer Science": [r"\bcomputer science\b", r"\bcs\b", r"\bsoftware engineering\b", r"\bcomputing\b"],
    "Data Science": [r"\bdata science\b", r"\bstatistics\b", r"\bbioinformatics\b"],
    "Electrical Engineering": [r"\belectrical engineering\b", r"\bece\b", r"\beecs\b"],
    "Mechanical Engineering": [r"\bmechanical engineering\b", r"\brobotics\b"],
    "Design": [r"\bdesign\b", r"\bui/ux\b", r"\bgraphic design\b", r"\binteraction design\b", r"\bdigital media\b"],
    "Business": [r"\bbusiness\b", r"\bfinance\b", r"\beconomics\b", r"\bmarketing\b", r"\baccounting\b"],
}


class ResumeParser:
    """Extracts student skills and profile attributes from raw text or PDFs."""

    def __init__(self, fuzzy_threshold: float = 85.0) -> None:
        self.fuzzy_threshold = fuzzy_threshold
        # Precompute canonical names and aliases for quick lookup
        self.canonical_map: dict[str, str] = {}
        self.all_candidates: list[str] = []
        for item in CANONICAL_TAXONOMY:
            canon = item.canonical_name
            self.canonical_map[canon.lower()] = canon
            self.all_candidates.append(canon)
            for alias in item.aliases:
                self.canonical_map[alias.lower()] = canon
                self.all_candidates.append(alias)

    def extract_profile_from_text(self, text: str) -> ParsedStudentProfile:
        """Parse raw text and return structured ParsedStudentProfile."""
        clean_text = text.strip()
        lower_text = clean_text.lower()

        # 1. Infer Major
        inferred_major = "Undeclared"
        for major, patterns in MAJOR_KEYWORDS.items():
            for pat in patterns:
                if re.search(pat, lower_text):
                    inferred_major = major
                    break
            if inferred_major != "Undeclared":
                break

        # 2. Extract Canonical Skills
        matched_canonical = set()

        # Method A: Direct regex boundary matching for canonical names and aliases
        for phrase, canon in self.canonical_map.items():
            pattern = r"(?<!\w)" + re.escape(phrase) + r"(?!\w)"
            if re.search(pattern, lower_text):
                matched_canonical.add(canon)

        # Method B: RapidFuzz token matching on word chunks
        if RAPIDFUZZ_AVAILABLE:
            words = [w.strip(",.;:()[]{}") for w in clean_text.split() if len(w) >= 2]
            for word in words:
                match = process.extractOne(
                    word,
                    self.all_candidates,
                    scorer=fuzz.token_sort_ratio,
                    score_cutoff=self.fuzzy_threshold,
                )
                if match:
                    matched_phrase, score, _ = match
                    canon = self.canonical_map.get(matched_phrase.lower(), matched_phrase)
                    matched_canonical.add(canon)

        # 3. Construct Suggested Bio Summary
        sentences = [s.strip() for s in re.split(r"[\r\n.]+", clean_text) if len(s.strip()) > 15]
        bio_snippets = sentences[:3]
        suggested_bio = ". ".join(bio_snippets)
        if suggested_bio and not suggested_bio.endswith("."):
            suggested_bio += "."

        return ParsedStudentProfile(
            major=inferred_major,
            matched_skills=sorted(list(matched_canonical)),
            suggested_bio=suggested_bio,
            experience_highlights=sentences[3:6] if len(sentences) > 3 else [],
        )
