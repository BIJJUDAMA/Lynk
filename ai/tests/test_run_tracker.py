import asyncio
import json
from unittest.mock import AsyncMock, MagicMock
import pytest

from ai.app.middleware.run_tracker import (
    AIRunRecord,
    track_ai_run,
    compute_run_input_hash,
)


@pytest.mark.asyncio
async def test_successful_run_captures_all_fields():
    """Test 1: Successful run captures input hash, output JSON, confidence, latency_ms >= 0, and status='success'."""
    raw_input = {"query": "Machine Learning Engineer", "campus": "Main"}

    async with track_ai_run(
        feature="skill_normalization",
        entity_type="job",
        entity_id="job-123",
        model_name="skill-normalizer",
        model_version="1.0.0",
        pipeline_version="skills-v1",
        prompt_version="prompt-v2",
        input_data=raw_input,
    ) as tracker:
        await asyncio.sleep(0.01)  # small delay to ensure measurable wall-clock latency
        tracker.set_output({"canonical_name": "Machine Learning", "confidence": 0.96})
        tracker.set_confidence(0.96)

    record = tracker.record
    assert isinstance(record, AIRunRecord)
    assert tracker.last_run == record
    assert record.feature == "skill_normalization"
    assert record.entity_type == "job"
    assert record.entity_id == "job-123"
    assert record.model_name == "skill-normalizer"
    assert record.model_version == "1.0.0"
    assert record.prompt_version == "prompt-v2"
    assert record.pipeline_version == "skills-v1"
    assert len(record.input_hash) == 64  # valid SHA-256 hex digest
    assert record.output_json == {"canonical_name": "Machine Learning", "confidence": 0.96}
    assert record.confidence == 0.96
    assert record.latency_ms > 0
    assert record.status == "success"
    assert record.error is None


@pytest.mark.asyncio
async def test_failed_run_captures_error_and_reraises():
    """Test 2: Failed run (raising an error) captures error message, status='failed', and re-raises exception."""
    tracker_ref = None

    with pytest.raises(RuntimeError) as exc_info:
        async with track_ai_run(
            feature="moderation",
            entity_type="post",
            entity_id="post-456",
            model_name="moderation-guard",
            model_version="0.2.1",
            pipeline_version="mod-v1",
            input_data="Inappropriate content sample",
        ) as tracker:
            tracker_ref = tracker
            raise RuntimeError("Inference engine timeout")

    assert "Inference engine timeout" in str(exc_info.value)
    assert tracker_ref is not None
    record = tracker_ref.record
    assert record.status == "failed"
    assert "Inference engine timeout" in (record.error or "")
    assert record.latency_ms >= 0


@pytest.mark.asyncio
async def test_persists_to_asyncpg_pool():
    """Test 3: Persists to mock or live asyncpg pool via parameterized INSERT query into ai_runs."""
    mock_conn = AsyncMock()
    mock_conn.execute = AsyncMock(return_value="INSERT 0 1")

    # Setup context manager for acquire()
    mock_pool = MagicMock()
    mock_acquire_cm = AsyncMock()
    mock_acquire_cm.__aenter__.return_value = mock_conn
    mock_acquire_cm.__aexit__.return_value = None
    mock_pool.acquire.return_value = mock_acquire_cm

    async with track_ai_run(
        feature="job_search_ranking",
        entity_type="candidate",
        entity_id="cand-789",
        model_name="bi-encoder-ranker",
        model_version="2.1.0",
        pipeline_version="ranker-v2",
        prompt_version=None,
        input_data={"candidate_id": "cand-789", "skills": ["Go", "React"]},
        db_pool=mock_pool,
    ) as tracker:
        tracker.set_output({"rank": 1, "score": 0.98})
        tracker.set_confidence(0.98)

    mock_pool.acquire.assert_called_once()
    mock_conn.execute.assert_awaited_once()

    call_args = mock_conn.execute.call_args[0]
    query = call_args[0]
    params = call_args[1:]

    assert "INSERT INTO ai_runs" in query
    assert "feature" in query
    assert "input_hash" in query
    assert params[0] == "job_search_ranking"
    assert params[1] == "candidate"
    assert params[2] == "cand-789"
    assert params[3] == "bi-encoder-ranker"
    assert params[4] == "2.1.0"
    assert params[5] is None  # prompt_version
    assert params[6] == "ranker-v2"
    assert len(params[7]) == 64  # input_hash
    # output_json parameter serialized as json
    assert json.loads(params[8]) == {"rank": 1, "score": 0.98}
    assert params[9] == 0.98  # confidence
    assert isinstance(params[10], int)  # latency_ms
    assert params[11] == "success"  # status
    assert params[12] is None  # error


@pytest.mark.asyncio
async def test_graceful_handling_of_none_pool_and_db_exception():
    """Test 4: Gracefully handles None db_pool or database exception without breaking execution."""
    # Case A: None db_pool does not crash
    async with track_ai_run(
        feature="test_none_pool",
        entity_type="item",
        entity_id="i1",
        model_name="test-model",
        model_version="1.0",
        pipeline_version="v1",
        input_data="some data",
        db_pool=None,
    ) as tracker:
        tracker.set_output({"result": "ok"})
    assert tracker.record.status == "success"

    # Case B: DB pool raises during acquire or execute
    exploding_pool = MagicMock()
    exploding_cm = AsyncMock()
    exploding_cm.__aenter__.side_effect = ConnectionError("PostgreSQL unreachable")
    exploding_pool.acquire.return_value = exploding_cm

    async with track_ai_run(
        feature="test_db_error",
        entity_type="item",
        entity_id="i2",
        model_name="test-model",
        model_version="1.0",
        pipeline_version="v1",
        input_data="some data",
        db_pool=exploding_pool,
    ) as tracker:
        tracker.set_output({"result": "still_succeeded"})

    assert tracker.record.status == "success"
    assert tracker.record.output_json == {"result": "still_succeeded"}


def test_input_hashing_deterministic():
    """Test 5: Input hashing is deterministic for structured dictionary and string inputs."""
    # Dict key order invariance
    dict_a = {"skills": ["Python", "Docker"], "experience": 4, "role": "Backend"}
    dict_b = {"role": "Backend", "skills": ["Python", "Docker"], "experience": 4}
    assert compute_run_input_hash(dict_a) == compute_run_input_hash(dict_b)

    # String whitespace normalization
    str_a = "  Full-Stack Software Engineer  \n"
    str_b = "Full-Stack Software Engineer"
    assert compute_run_input_hash(str_a) == compute_run_input_hash(str_b)

    # Distinct content produces distinct hash
    str_c = "DevOps Engineer"
    assert compute_run_input_hash(str_a) != compute_run_input_hash(str_c)


@pytest.mark.asyncio
async def test_sink_support():
    """Test optional in-memory sink list and callback."""
    sink_list = []

    async with track_ai_run(
        feature="sink_test",
        entity_type="test",
        entity_id="s1",
        model_name="m1",
        model_version="1.0",
        pipeline_version="p1",
        input_data="sink input",
        sink=sink_list,
    ) as tracker:
        tracker.set_output({"saved": True})

    assert len(sink_list) == 1
    assert sink_list[0].output_json == {"saved": True}

    # Test callable sink
    called_records = []

    async def custom_callback(record):
        called_records.append(record)

    async with track_ai_run(
        feature="sink_test_cb",
        entity_type="test",
        entity_id="s2",
        model_name="m1",
        model_version="1.0",
        pipeline_version="p1",
        input_data="sink input 2",
        sink=custom_callback,
    ) as tracker:
        tracker.set_output({"cb": True})

    assert len(called_records) == 1
    assert called_records[0].output_json == {"cb": True}


@pytest.mark.asyncio
async def test_decorator_syntax():
    """Test decorator usage for track_ai_run and tracked_ai_run."""
    sink_records = []

    @track_ai_run(
        feature="decorated_feature",
        entity_type="item",
        entity_id="item-1",
        model_name="dec-model",
        model_version="1.0.0",
        pipeline_version="dec-v1",
        sink=sink_records,
    )
    async def sample_pipeline(text: str):
        return {"processed": text.upper()}

    res = await sample_pipeline("hello world")
    assert res == {"processed": "HELLO WORLD"}
    assert len(sink_records) == 1
    record = sink_records[0]
    assert record.feature == "decorated_feature"
    assert record.status == "success"
    assert record.output_json == {"processed": "HELLO WORLD"}

