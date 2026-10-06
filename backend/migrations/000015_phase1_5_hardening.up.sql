-- 1. Embedding Cache Table for Content-Hash Deduplication (SHA-256) with Composite PK
CREATE TABLE IF NOT EXISTS embedding_cache (
    content_hash VARCHAR(64) NOT NULL,
    model_name VARCHAR(64) NOT NULL,
    dimensions INTEGER NOT NULL,
    embedding vector(384) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (content_hash, model_name)
);

CREATE INDEX IF NOT EXISTS idx_embedding_cache_created 
ON embedding_cache (model_name, created_at DESC);

-- 2. Transactional Outbox Table for Asynchronous Event Dispatching
CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(128) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 5,
    last_error TEXT,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending_events 
ON outbox_events (next_retry_at, created_at) 
WHERE status IN ('pending', 'failed');
