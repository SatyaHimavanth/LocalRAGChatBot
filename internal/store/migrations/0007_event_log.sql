-- 0010_event_log.sql
-- Durable workspace event/audit trail for diagnostics and extensions.

CREATE TABLE IF NOT EXISTS event_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_key TEXT NOT NULL,
    title TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL DEFAULT 'info',
    scope TEXT NOT NULL DEFAULT 'workspace',
    collection_id INTEGER DEFAULT 0,
    chat_id INTEGER DEFAULT 0,
    doc_id INTEGER DEFAULT 0,
    batch_id TEXT DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_event_log_created_at ON event_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_log_event_key ON event_log(event_key);
CREATE INDEX IF NOT EXISTS idx_event_log_scope ON event_log(scope);
CREATE INDEX IF NOT EXISTS idx_event_log_collection ON event_log(collection_id);
