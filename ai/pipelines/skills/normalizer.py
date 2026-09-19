"""Canonical skill taxonomy, multi-stage extraction, and normalization pipeline."""

import logging
import re
from dataclasses import dataclass, field
from difflib import SequenceMatcher
from functools import lru_cache

from ai.models.embeddings.provider import BaseEmbeddingProvider, get_embedding_provider

logger = logging.getLogger(__name__)


@dataclass
class CanonicalSkill:
    """Canonical skill definition in the Lynk taxonomy."""

    skill_id: str
    canonical_name: str
    category: str
    aliases: list[str] = field(default_factory=list)


@dataclass
class NormalizedSkillResult:
    """Normalized skill result with audit metadata."""

    original_skill: str
    canonical_name: str
    confidence: float
    match_method: str  # "exact_alias", "fuzzy", "embedding", "fallback"
    category: str | None = None
    skill_id: str | None = None


CANONICAL_TAXONOMY: list[CanonicalSkill] = [
    CanonicalSkill(
        skill_id="react",
        canonical_name="React",
        category="Frontend",
        aliases=["reactjs", "react.js", "react-js"],
    ),
    CanonicalSkill(
        skill_id="python",
        canonical_name="Python",
        category="Backend / Data",
        aliases=["python3", "python 3", "py"],
    ),
    CanonicalSkill(
        skill_id="go",
        canonical_name="Go",
        category="Backend",
        aliases=["golang"],
    ),
    CanonicalSkill(
        skill_id="typescript",
        canonical_name="TypeScript",
        category="Frontend / Fullstack",
        aliases=["ts"],
    ),
    CanonicalSkill(
        skill_id="postgresql",
        canonical_name="PostgreSQL",
        category="Database",
        aliases=["postgres", "psql"],
    ),
    CanonicalSkill(
        skill_id="docker",
        canonical_name="Docker",
        category="DevOps",
        aliases=["containerization", "docker-compose"],
    ),
    CanonicalSkill(
        skill_id="machine-learning",
        canonical_name="Machine Learning",
        category="Data & AI",
        aliases=["ml"],
    ),
    CanonicalSkill(
        skill_id="computer-vision",
        canonical_name="Computer Vision",
        category="Data & AI",
        aliases=["cv"],
    ),
    CanonicalSkill(
        skill_id="natural-language-processing",
        canonical_name="Natural Language Processing",
        category="Data & AI",
        aliases=["nlp"],
    ),
    CanonicalSkill(
        skill_id="fastapi",
        canonical_name="FastAPI",
        category="Backend",
        aliases=["fast-api"],
    ),
    CanonicalSkill(
        skill_id="tailwind-css",
        canonical_name="Tailwind CSS",
        category="Frontend",
        aliases=["tailwind", "tailwindcss"],
    ),
    CanonicalSkill(
        skill_id="next-js",
        canonical_name="Next.js",
        category="Frontend",
        aliases=["nextjs", "next.js"],
    ),
    CanonicalSkill(
        skill_id="pytorch",
        canonical_name="PyTorch",
        category="Data & AI",
        aliases=["torch"],
    ),
    CanonicalSkill(
        skill_id="kubernetes",
        canonical_name="Kubernetes",
        category="DevOps",
        aliases=["k8s"],
    ),
    CanonicalSkill(
        skill_id="git",
        canonical_name="Git",
        category="Tools & VCS",
        aliases=["github", "gitlab"],
    ),
    CanonicalSkill(
        skill_id="node-js",
        canonical_name="Node.js",
        category="Backend",
        aliases=["nodejs", "node.js", "node"],
    ),
    CanonicalSkill(
        skill_id="java",
        canonical_name="Java",
        category="Backend",
        aliases=["java8", "java11", "java17", "java21"],
    ),
    CanonicalSkill(
        skill_id="cpp",
        canonical_name="C++",
        category="Systems",
        aliases=["cpp", "cplusplus"],
    ),
    CanonicalSkill(
        skill_id="c",
        canonical_name="C",
        category="Systems",
        aliases=[],
    ),
    CanonicalSkill(
        skill_id="rust",
        canonical_name="Rust",
        category="Systems",
        aliases=[],
    ),
    CanonicalSkill(
        skill_id="sql",
        canonical_name="SQL",
        category="Database",
        aliases=[],
    ),
    CanonicalSkill(
        skill_id="mongodb",
        canonical_name="MongoDB",
        category="Database",
        aliases=["mongo"],
    ),
    CanonicalSkill(
        skill_id="redis",
        canonical_name="Redis",
        category="Database",
        aliases=[],
    ),
    CanonicalSkill(
        skill_id="linux",
        canonical_name="Linux",
        category="DevOps",
        aliases=["unix", "bash"],
    ),
    CanonicalSkill(
        skill_id="aws",
        canonical_name="AWS",
        category="Cloud",
        aliases=["amazon web services"],
    ),
    CanonicalSkill(
        skill_id="html",
        canonical_name="HTML",
        category="Frontend",
        aliases=["html5"],
    ),
    CanonicalSkill(
        skill_id="css",
        canonical_name="CSS",
        category="Frontend",
        aliases=["css3"],
    ),
    CanonicalSkill(
        skill_id="graphql",
        canonical_name="GraphQL",
        category="Backend",
        aliases=[],
    ),
    CanonicalSkill(
        skill_id="tensorflow",
        canonical_name="TensorFlow",
        category="Data & AI",
        aliases=["tf"],
    ),
    CanonicalSkill(
        skill_id="pandas",
        canonical_name="Pandas",
        category="Data & AI",
        aliases=[],
    ),
    CanonicalSkill(
        skill_id="numpy",
        canonical_name="NumPy",
        category="Data & AI",
        aliases=[],
    ),
]


class SkillNormalizer:
    """Multi-stage skill normalizer and extractor:

    1. Exact alias lookup (case-insensitive, normalized punctuation).
    2. Fuzzy token / edit-distance sequence matching.
    3. Embedding cosine similarity fallback.
    4. Unmapped fallback.
    """

    def __init__(
        self,
        taxonomy: list[CanonicalSkill] | None = None,
        embedding_provider: BaseEmbeddingProvider | None = None,
        fuzzy_threshold: float = 0.9,
        embedding_threshold: float = 0.75,
    ) -> None:
        self.taxonomy = taxonomy or CANONICAL_TAXONOMY
        self.fuzzy_threshold = fuzzy_threshold
        self.embedding_threshold = embedding_threshold
        self._provider = embedding_provider
        self._canonical_embeddings: dict[str, list[float]] | None = None

        # Build fast alias lookup table
        self._alias_map: dict[str, CanonicalSkill] = {}
        for skill in self.taxonomy:
            # Canonical name lower
            self._alias_map[skill.canonical_name.strip().lower()] = skill
            # Explicit aliases
            for alias in skill.aliases:
                self._alias_map[alias.strip().lower()] = skill

    @property
    def embedding_provider(self) -> BaseEmbeddingProvider:
        """Acquire or lazily initialize the embedding provider."""
        if self._provider is None:
            self._provider = get_embedding_provider()
        return self._provider

    def _ensure_canonical_embeddings(self) -> dict[str, list[float]]:
        """Lazily compute and cache canonical embeddings for stage 3."""
        if self._canonical_embeddings is None:
            self._canonical_embeddings = {}
            for skill in self.taxonomy:
                vec = self.embedding_provider.embed(skill.canonical_name)
                self._canonical_embeddings[skill.canonical_name] = vec
        return self._canonical_embeddings

    def normalize(self, skill: str) -> NormalizedSkillResult:
        """Normalize an unstructured skill string through the 4-stage pipeline."""
        raw = skill.strip()
        if not raw:
            return NormalizedSkillResult(
                original_skill=skill,
                canonical_name=skill,
                confidence=0.0,
                match_method="fallback",
                category=None,
                skill_id=None,
            )

        # Stage 1: Exact alias lookup (case-insensitive, trimmed)
        raw_lower = raw.lower()
        if raw_lower in self._alias_map:
            matched = self._alias_map[raw_lower]
            return NormalizedSkillResult(
                original_skill=raw,
                canonical_name=matched.canonical_name,
                confidence=1.0,
                match_method="exact_alias",
                category=matched.category,
                skill_id=matched.skill_id,
            )

        # Stage 2: Fuzzy token / sequence matching
        best_fuzzy_skill: CanonicalSkill | None = None
        best_fuzzy_ratio: float = 0.0

        for skill_entry in self.taxonomy:
            # Compare with canonical name
            ratio = SequenceMatcher(
                None, raw_lower, skill_entry.canonical_name.lower()
            ).ratio()
            if ratio > best_fuzzy_ratio:
                best_fuzzy_ratio = ratio
                best_fuzzy_skill = skill_entry

            # Also compare against aliases
            for alias in skill_entry.aliases:
                alias_ratio = SequenceMatcher(None, raw_lower, alias.lower()).ratio()
                if alias_ratio > best_fuzzy_ratio:
                    best_fuzzy_ratio = alias_ratio
                    best_fuzzy_skill = skill_entry

        if best_fuzzy_skill is not None and best_fuzzy_ratio >= self.fuzzy_threshold:
            return NormalizedSkillResult(
                original_skill=raw,
                canonical_name=best_fuzzy_skill.canonical_name,
                confidence=round(best_fuzzy_ratio, 4),
                match_method="fuzzy",
                category=best_fuzzy_skill.category,
                skill_id=best_fuzzy_skill.skill_id,
            )

        # Stage 3: Embedding cosine similarity
        canonical_vecs = self._ensure_canonical_embeddings()
        query_vec = self.embedding_provider.embed(raw)

        best_emb_skill: CanonicalSkill | None = None
        best_emb_score: float = -1.0

        for skill_entry in self.taxonomy:
            can_vec = canonical_vecs.get(skill_entry.canonical_name)
            if can_vec:
                dot = sum(q * c for q, c in zip(query_vec, can_vec))
                if dot > best_emb_score:
                    best_emb_score = dot
                    best_emb_skill = skill_entry

        if best_emb_skill is not None and best_emb_score >= self.embedding_threshold:
            return NormalizedSkillResult(
                original_skill=raw,
                canonical_name=best_emb_skill.canonical_name,
                confidence=round(best_emb_score, 4),
                match_method="embedding",
                category=best_emb_skill.category,
                skill_id=best_emb_skill.skill_id,
            )

        # Stage 4: Unmapped fallback
        fallback_conf = 0.0
        if best_emb_score > 0:
            fallback_conf = min(0.49, round(best_emb_score, 4))

        return NormalizedSkillResult(
            original_skill=raw,
            canonical_name=raw,
            confidence=fallback_conf,
            match_method="fallback",
            category=None,
            skill_id=None,
        )

    def extract_skills_from_text(self, text: str) -> list[NormalizedSkillResult]:
        """Extract and normalize canonical skills found in freeform text."""
        if not text or not text.strip():
            return []

        # Build list of candidate phrases: (phrase, is_case_sensitive, canonical_skill)
        candidate_phrases: list[tuple[str, bool, CanonicalSkill]] = []
        for skill in self.taxonomy:
            # Short common English words like "Go" should be matched case-sensitively
            is_case_sensitive = len(skill.canonical_name) <= 2
            candidate_phrases.append((skill.canonical_name, is_case_sensitive, skill))
            for alias in skill.aliases:
                alias_sensitive = len(alias) <= 2 and alias.lower() in ("go", "c")
                candidate_phrases.append((alias, alias_sensitive, skill))

        # Sort candidate phrases by length descending to match longest phrases first
        candidate_phrases.sort(key=lambda x: len(x[0]), reverse=True)

        covered_spans: list[tuple[int, int]] = []
        extracted_results: list[NormalizedSkillResult] = []
        seen_canonical: set[str] = set()

        for phrase, sensitive, _skill in candidate_phrases:
            pattern = rf"(?<![A-Za-z0-9]){re.escape(phrase)}(?![A-Za-z0-9])"
            flags = 0 if sensitive else re.IGNORECASE

            for match in re.finditer(pattern, text, flags):
                start, end = match.span()
                # Check for overlap with already matched longer phrases
                if any(
                    max(start, cov_start) < min(end, cov_end)
                    for cov_start, cov_end in covered_spans
                ):
                    continue

                covered_spans.append((start, end))
                matched_text = match.group(0)
                norm = self.normalize(matched_text)

                if (
                    norm.match_method != "fallback"
                    and norm.canonical_name not in seen_canonical
                ):
                    seen_canonical.add(norm.canonical_name)
                    extracted_results.append(norm)

        return extracted_results

    # Alias for normalization evaluation
    normalize_skill = normalize


@lru_cache(maxsize=1)
def get_skill_normalizer() -> SkillNormalizer:
    """Acquire cached singleton instance of SkillNormalizer."""
    return SkillNormalizer()
