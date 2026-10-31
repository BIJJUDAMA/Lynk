"""Kuzu Embedded Columnar Graph Database Engine for Lynk Campus Relationships.

100% pure 7-bit ASCII
Persists graph nodes and edges on disk with zero Python dictionary RAM bloat.
Supports openCypher queries for peer finding and skill-tree traversal.
"""

import logging
import os
from typing import Any

logger = logging.getLogger("lynk-ai.kuzu")

try:
    import kuzu

    KUZU_AVAILABLE = True
except ImportError:
    kuzu = None
    KUZU_AVAILABLE = False


class KuzuGraphEngine:
    """Embedded C++ graph database engine backed by Kuzu columnar storage on disk."""

    def __init__(self, db_path: str = "/data/kuzu_campus_graph") -> None:
        self.db_path = db_path
        self._in_memory_fallback: dict[str, Any] = {
            "students": {},
            "skills": {},
            "projects": {},
            "student_skills": set(),
            "project_skills": set(),
        }

        if KUZU_AVAILABLE:
            os.makedirs(os.path.dirname(os.path.abspath(db_path)), exist_ok=True)
            self.db = kuzu.Database(db_path)
            self.conn = kuzu.Connection(self.db)
        else:
            self.db = None
            self.conn = None

    def init_schema(self) -> None:
        """Create Node and Rel tables using openCypher DDL."""
        if not self.conn:
            return

        queries = [
            "CREATE NODE TABLE IF NOT EXISTS Student(id STRING, department STRING, PRIMARY KEY (id))",
            "CREATE NODE TABLE IF NOT EXISTS Skill(id STRING, name STRING, category STRING, PRIMARY KEY (id))",
            "CREATE NODE TABLE IF NOT EXISTS Project(id STRING, title STRING, PRIMARY KEY (id))",
            "CREATE REL TABLE IF NOT EXISTS HasSkill(FROM Student TO Skill)",
            "CREATE REL TABLE IF NOT EXISTS RequiresSkill(FROM Project TO Skill)",
            "CREATE REL TABLE IF NOT EXISTS CollaboratedOn(FROM Student TO Project)",
        ]

        for q in queries:
            try:
                self.conn.execute(q)
            except Exception as e:
                logger.debug("Schema query note (%s): %s", q, e)

    def add_student(self, student_id: str, department: str = "") -> None:
        """Upsert a student node."""
        if self.conn:
            # Check existence or insert
            res = self.conn.execute("MATCH (s:Student {id: $id}) RETURN s.id", {"id": student_id})
            if not res.has_next():
                self.conn.execute(
                    "CREATE (s:Student {id: $id, department: $dept})",
                    {"id": student_id, "dept": department},
                )
        else:
            self._in_memory_fallback["students"][student_id] = {"department": department}

    def add_skill(self, skill_id: str, name: str, category: str = "General") -> None:
        """Upsert a canonical skill node."""
        if self.conn:
            res = self.conn.execute("MATCH (s:Skill {id: $id}) RETURN s.id", {"id": skill_id})
            if not res.has_next():
                self.conn.execute(
                    "CREATE (s:Skill {id: $id, name: $name, category: $cat})",
                    {"id": skill_id, "name": name, "cat": category},
                )
        else:
            self._in_memory_fallback["skills"][skill_id] = {"name": name, "category": category}

    def add_project(self, project_id: str, title: str) -> None:
        """Upsert a project node."""
        if self.conn:
            res = self.conn.execute("MATCH (p:Project {id: $id}) RETURN p.id", {"id": project_id})
            if not res.has_next():
                self.conn.execute(
                    "CREATE (p:Project {id: $id, title: $title})",
                    {"id": project_id, "title": title},
                )
        else:
            self._in_memory_fallback["projects"][project_id] = {"title": title}

    def add_student_skill(self, student_id: str, skill_id: str) -> None:
        """Create HasSkill relationship."""
        if self.conn:
            self.conn.execute(
                """
                MATCH (s:Student {id: $sid}), (sk:Skill {id: $skid})
                MERGE (s)-[:HasSkill]->(sk)
                """,
                {"sid": student_id, "skid": skill_id},
            )
        else:
            self._in_memory_fallback["student_skills"].add((student_id, skill_id))

    def add_project_required_skill(self, project_id: str, skill_id: str) -> None:
        """Create RequiresSkill relationship."""
        if self.conn:
            self.conn.execute(
                """
                MATCH (p:Project {id: $pid}), (sk:Skill {id: $skid})
                MERGE (p)-[:RequiresSkill]->(sk)
                """,
                {"pid": project_id, "skid": skill_id},
            )
        else:
            self._in_memory_fallback["project_skills"].add((project_id, skill_id))

    def get_student_skills(self, student_id: str) -> list[str]:
        """Fetch all skill IDs possessed by student."""
        if self.conn:
            res = self.conn.execute(
                "MATCH (s:Student {id: $sid})-[:HasSkill]->(sk:Skill) RETURN sk.id",
                {"sid": student_id},
            )
            skills = []
            while res.has_next():
                skills.append(res.get_next()[0])
            return skills
        else:
            return [sk for (sid, sk) in self._in_memory_fallback["student_skills"] if sid == student_id]

    def find_complementary_peers(
        self,
        project_id: str,
        current_member_ids: list[str],
        limit: int = 10,
    ) -> list[dict[str, Any]]:
        """Find students who possess skills required by the project that existing members lack."""
        if self.conn:
            query = """
            MATCH (p:Project {id: $pid})-[:RequiresSkill]->(needed:Skill)
            WHERE NOT EXISTS {
                MATCH (m:Student)-[:HasSkill]->(needed)
                WHERE m.id IN $members
            }
            MATCH (candidate:Student)-[:HasSkill]->(needed)
            WHERE NOT candidate.id IN $members
            RETURN candidate.id, candidate.department, count(DISTINCT needed) AS matching_needed_skills
            ORDER BY matching_needed_skills DESC
            LIMIT $limit
            """
            res = self.conn.execute(
                query,
                {
                    "pid": project_id,
                    "members": current_member_ids,
                    "limit": limit,
                },
            )
            peers = []
            while res.has_next():
                row = res.get_next()
                peers.append(
                    {
                        "student_id": row[0],
                        "department": row[1],
                        "matching_skills_count": row[2],
                    }
                )
            return peers
        else:
            # Fallback evaluation
            req_skills = {sk for (pid, sk) in self._in_memory_fallback["project_skills"] if pid == project_id}
            team_skills = {sk for (sid, sk) in self._in_memory_fallback["student_skills"] if sid in current_member_ids}
            missing_skills = req_skills - team_skills

            scored = []
            for sid, info in self._in_memory_fallback["students"].items():
                if sid in current_member_ids:
                    continue
                s_skills = {sk for (s, sk) in self._in_memory_fallback["student_skills"] if s == sid}
                overlap = len(s_skills & missing_skills)
                if overlap > 0:
                    scored.append(
                        {
                            "student_id": sid,
                            "department": info["department"],
                            "matching_skills_count": overlap,
                        }
                    )
            scored.sort(key=lambda x: x["matching_skills_count"], reverse=True)
            return scored[:limit]
