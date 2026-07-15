-- 0008_workspace_memory.sql
-- Persistent conversation/workspace memory summaries.

CREATE TABLE IF NOT EXISTS chat_session_memory (
    id INTEGER PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    collection_id INTEGER NOT NULL REFERENCES collections(id),
    memory_type TEXT NOT NULL DEFAULT 'rolling_summary',
    summary TEXT NOT NULL DEFAULT '',
    source_message_id INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(session_id, memory_type)
);

CREATE INDEX IF NOT EXISTS idx_chat_session_memory_session_updated ON chat_session_memory(session_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_session_memory_collection ON chat_session_memory(collection_id, memory_type);
