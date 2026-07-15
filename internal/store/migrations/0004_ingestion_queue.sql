-- 0005_ingestion_queue.sql
-- Durable ingestion queue and logs.

CREATE TABLE IF NOT EXISTS embeddings (
    id INTEGER PRIMARY KEY,
    chunk_id INTEGER NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    model TEXT NOT NULL DEFAULT '',
    dims INTEGER NOT NULL DEFAULT 0,
    embedding_hash TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS ingestion_jobs (
    id INTEGER PRIMARY KEY,
    batch_id TEXT NOT NULL,
    collection_id INTEGER NOT NULL REFERENCES collections(id),
    status TEXT NOT NULL DEFAULT 'queued',
    stage TEXT NOT NULL DEFAULT 'loading',
    progress_pct INTEGER NOT NULL DEFAULT 0,
    total_documents INTEGER NOT NULL DEFAULT 0,
    completed_documents INTEGER NOT NULL DEFAULT 0,
    failed_documents INTEGER NOT NULL DEFAULT 0,
    message TEXT NOT NULL DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    paused_at INTEGER NOT NULL DEFAULT 0,
    resumed_at INTEGER NOT NULL DEFAULT 0,
    cancelled_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS ingestion_logs (
    id INTEGER PRIMARY KEY,
    job_id INTEGER NOT NULL REFERENCES ingestion_jobs(id) ON DELETE CASCADE,
    document_id INTEGER NOT NULL DEFAULT 0,
    collection_id INTEGER NOT NULL DEFAULT 0,
    batch_id TEXT NOT NULL DEFAULT '',
    level TEXT NOT NULL DEFAULT 'info',
    stage TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    memory_bytes INTEGER NOT NULL DEFAULT 0,
    throughput REAL NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(hash);
CREATE INDEX IF NOT EXISTS idx_documents_collection_filename ON documents(collection_id, filename);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_chunks_document_ord ON chunks(document_id, ord);
CREATE INDEX IF NOT EXISTS idx_chunks_chunk_hash ON chunks(chunk_hash);
CREATE INDEX IF NOT EXISTS idx_embeddings_chunk_hash ON embeddings(embedding_hash);
CREATE INDEX IF NOT EXISTS idx_ingestion_jobs_collection_status ON ingestion_jobs(collection_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ingestion_logs_job_created ON ingestion_logs(job_id, created_at DESC);
