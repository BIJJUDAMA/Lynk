# 100% pure 7-bit ASCII
import asyncio
import json
import logging
from typing import Any

import aio_pika

from ai.app.config import get_settings
from ai.models.embeddings.provider import BaseEmbeddingProvider, get_embedding_provider
from ai.models.embeddings.qdrant_client import QdrantVectorStore

logger = logging.getLogger("lynk-ai.event_consumer")

QUEUE_NAME = "tasks.ai.embeddings"
EXCHANGE_EVENTS = "lynk.events"
EXCHANGE_DLX = "lynk.dlx"
ROUTING_KEY_DLQ = "tasks.ai.dead"


class RabbitMQEventConsumer:
    """Asynchronous RabbitMQ consumer bridging platform events directly to vector & graph stores."""

    def __init__(
        self,
        amqp_url: str | None = None,
        qdrant_client: QdrantVectorStore | None = None,
        embedding_provider: BaseEmbeddingProvider | None = None,
        kuzu_engine: Any | None = None,
        db_pool: Any | None = None,
    ) -> None:
        settings = get_settings()
        self.amqp_url = amqp_url or settings.RABBITMQ_URL
        self.qdrant = qdrant_client
        self.provider = embedding_provider
        self.kuzu = kuzu_engine
        self.db_pool = db_pool

        self._connection: aio_pika.abc.AbstractRobustConnection | None = None
        self._channel: aio_pika.abc.AbstractChannel | None = None
        self._is_running = False

    def _ensure_clients(self) -> None:
        """Lazily initialize provider and vector clients if not injected."""
        if self.provider is None:
            self.provider = get_embedding_provider()
        if self.qdrant is None:
            self.qdrant = QdrantVectorStore()

    async def process_event(
        self, routing_key: str, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Route domain events to corresponding vector embedding and graph synchronization handlers."""
        self._ensure_clients()
        routing = routing_key.lower()

        logger.info(
            "Processing AI task event: key=%s, payload_keys=%s",
            routing_key,
            list(payload.keys()),
        )

        # 1. Project Domain Events (project.created, project.updated, project.deleted)
        if (
            "project" in routing
            or routing.startswith("ai.embedding.project")
            or payload.get("entity_type") == "project"
        ):
            return await self._handle_project_event(routing, payload)

        # 2. Student Domain Events (student.updated, student.registered, ai.embedding.student)
        elif (
            "student" in routing
            or "user" in routing
            or routing.startswith("ai.embedding.student")
            or payload.get("entity_type") == "student"
        ):
            return await self._handle_student_event(routing, payload)

        # 3. Resume Events
        elif (
            "resume" in routing
            or routing.startswith("ai.embedding.resume")
            or payload.get("entity_type") == "resume"
        ):
            return await self._handle_resume_event(routing, payload)

        logger.info("Ignored unhandled routing key: %s", routing_key)
        return {"status": "ignored", "routing_key": routing_key}

    async def _handle_project_event(
        self, routing: str, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Handle project vector indexing and deletions."""
        project_id = str(payload.get("id") or payload.get("project_id") or "")
        if not project_id:
            logger.warning("Project event missing project_id")
            return {"status": "skipped", "reason": "missing_id"}

        action = str(payload.get("action") or "").lower()
        if action == "delete" or routing.endswith(".deleted"):
            if self.qdrant:
                self.qdrant.delete_point(
                    collection_name="campus_projects", point_id=project_id
                )
            logger.info("Deleted project vector point: %s", project_id)
            return {"status": "deleted", "project_id": project_id}

        # Build comprehensive semantic text for project embedding
        title = str(payload.get("title") or "")
        desc = str(payload.get("description") or "")
        skills = payload.get("required_skills") or payload.get("skills") or []
        skills_str = " ".join(skills) if isinstance(skills, list) else str(skills)

        semantic_text = f"{title}. {desc}. Required skills: {skills_str}".strip()
        if not semantic_text and not title:
            return {"status": "skipped", "reason": "empty_content"}

        vector = self.provider.embed(semantic_text)  # type: ignore

        metadata = {
            "title": title,
            "category": payload.get("category", "General"),
            "required_skills": skills if isinstance(skills, list) else [skills_str],
            "department": payload.get("department", "All"),
            "status": payload.get("status", "open"),
        }

        if self.qdrant:
            self.qdrant.upsert_point(
                collection_name="campus_projects",
                point_id=project_id,
                vector=vector,
                payload=metadata,
            )

        logger.info("Upserted project vector: id=%s", project_id)
        return {"status": "indexed", "project_id": project_id, "dim": len(vector)}

    async def _handle_student_event(
        self, routing: str, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Handle student profile vector embedding and Kuzu campus graph synchronization."""
        student_id = str(payload.get("id") or payload.get("student_id") or "")
        if not student_id:
            return {"status": "skipped", "reason": "missing_student_id"}

        bio = str(payload.get("bio") or "")
        skills = payload.get("skills") or []
        department = str(payload.get("department") or "")
        grad_year = payload.get("graduation_year", 2026)

        skills_str = " ".join(skills) if isinstance(skills, list) else str(skills)
        semantic_text = (
            f"Department: {department}. Skills: {skills_str}. Bio: {bio}".strip()
        )

        vector = self.provider.embed(semantic_text)  # type: ignore

        if self.qdrant:
            self.qdrant.upsert_point(
                collection_name="campus_students",
                point_id=student_id,
                vector=vector,
                payload={
                    "department": department,
                    "graduation_year": grad_year,
                    "skills": skills if isinstance(skills, list) else [skills_str],
                },
            )

        # Update Kuzu graph edge if engine is attached
        if self.kuzu is not None and isinstance(skills, list):
            try:
                self.kuzu.add_student(
                    student_id=student_id,
                    department=department,
                    grad_year=int(grad_year or 2026),
                )
                for s in skills:
                    skill_name = str(s).strip()
                    if skill_name:
                        self.kuzu.add_skill_edge(
                            student_id=student_id,
                            skill_name=skill_name,
                            weight=1.0,
                        )
            except Exception as exc:
                logger.warning(
                    "Kuzu edge sync failed for student %s: %s", student_id, exc
                )

        logger.info("Upserted student profile vector: id=%s", student_id)
        return {"status": "indexed", "student_id": student_id, "dim": len(vector)}

    async def _handle_resume_event(
        self, routing: str, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Handle asynchronous resume skill extraction request."""
        resume_id = str(payload.get("resume_id") or payload.get("id") or "")
        logger.info("Resume event received for indexing: %s", resume_id)
        return {"status": "processed", "resume_id": resume_id}

    async def start_consuming(self, stop_event: asyncio.Event | None = None) -> None:
        """Connect to RabbitMQ, declare dead-letter routing, and consume messages until stopped."""
        self._ensure_clients()
        self._is_running = True

        logger.info("Connecting to RabbitMQ at %s", self.amqp_url)
        self._connection = await aio_pika.connect_robust(self.amqp_url)
        self._channel = await self._connection.channel()
        await self._channel.set_qos(prefetch_count=10)

        # Declare main topic exchange
        exchange = await self._channel.declare_exchange(
            EXCHANGE_EVENTS, aio_pika.ExchangeType.TOPIC, durable=True
        )

        # Declare Dead Letter Exchange
        await self._channel.declare_exchange(
            EXCHANGE_DLX, aio_pika.ExchangeType.DIRECT, durable=True
        )

        # Declare consumer queue with dead letter configuration
        queue = await self._channel.declare_queue(
            QUEUE_NAME,
            durable=True,
            arguments={
                "x-dead-letter-exchange": EXCHANGE_DLX,
                "x-dead-letter-routing-key": ROUTING_KEY_DLQ,
            },
        )

        # Bind topic routing keys
        for key in ["ai.embedding.*", "project.*", "student.*", "resume.*"]:
            await queue.bind(exchange, routing_key=key)

        logger.info(
            "RabbitMQ Event Consumer listening on '%s' with DLQ '%s'",
            QUEUE_NAME,
            ROUTING_KEY_DLQ,
        )

        async with queue.iterator() as queue_iter:
            async for message in queue_iter:
                if not self._is_running:
                    break

                # aio-pika process automatically rejects without requeue if exception escapes
                async with message.process(requeue=False):
                    try:
                        body_data = json.loads(message.body.decode("utf-8"))
                        routing_key = message.routing_key or ""
                        await self.process_event(routing_key, body_data)
                    except Exception as exc:
                        logger.error(
                            "Failed to process message (routing to DLX): %s", exc
                        )
                        raise  # Triggers reject(requeue=False) -> routes to lynk.dlx

                if stop_event and stop_event.is_set():
                    break

    async def close(self) -> None:
        """Gracefully close channel and connection."""
        self._is_running = False
        if self._channel and not self._channel.is_closed:
            await self._channel.close()
        if self._connection and not self._connection.is_closed:
            await self._connection.close()
        logger.info("RabbitMQ Event Consumer closed gracefully")
