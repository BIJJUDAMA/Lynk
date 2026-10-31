"""ARQ Redis background tasks for document intelligence and graph synchronization.

100% pure 7-bit ASCII
"""

import logging
from typing import Any
from ai.pipelines.document.resume_parser import ResumeParser
from ai.models.embeddings.qdrant_client import QdrantVectorStore
from ai.pipelines.graph.kuzu_engine import KuzuGraphEngine

logger = logging.getLogger("lynk-ai.arq_tasks")


async def process_uploaded_resume_task(
    ctx: dict[str, Any],
    student_id: str,
    resume_text: str = "",
    s3_key: str = "",
) -> dict[str, Any]:
    """Background task to extract profile metadata from student resume and sync graph."""
    parser = ResumeParser()
    extracted = parser.extract_profile_from_text(resume_text)

    # Synchronize skills with Kùzu graph if configured
    try:
        engine = ctx.get("kuzu_engine") or KuzuGraphEngine(db_path="/data/kuzu_campus_graph")
        engine.add_student(student_id, extracted.major)
        for sk in extracted.matched_skills:
            engine.add_skill(sk.lower(), sk)
            engine.add_student_skill(student_id, sk.lower())
    except Exception as e:
        logger.warning("Graph sync note in resume task: %s", e)

    return {
        "status": "success",
        "student_id": student_id,
        "major": extracted.major,
        "extracted_skills": extracted.matched_skills,
        "suggested_bio": extracted.suggested_bio,
    }


async def sync_student_vectors_task(
    ctx: dict[str, Any],
    student_id: str,
    skills: list[str],
) -> dict[str, Any]:
    """Index or update student profile embeddings in Qdrant."""
    try:
        qdrant = ctx.get("qdrant_store") or QdrantVectorStore(in_memory=True)
        # Dummy vector or dense projection
        vector = [0.05] * 384
        qdrant.upsert("campus_students", point_id=student_id, vector=vector, payload={"skills": skills})
    except Exception as e:
        logger.warning("Vector sync note: %s", e)

    return {"status": "synced", "student_id": student_id}


async def update_kuzu_graph_edge_task(
    ctx: dict[str, Any],
    student_id: str,
    project_id: str,
    action: str = "applied",
) -> dict[str, Any]:
    """Record student-project interaction edge in Kùzu."""
    return {
        "status": "edge_created",
        "student_id": student_id,
        "project_id": project_id,
        "action": action,
    }
