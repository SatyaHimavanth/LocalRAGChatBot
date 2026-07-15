-- 0004_workspace.sql
CREATE TABLE IF NOT EXISTS workspace_memory (
    session_id INTEGER PRIMARY KEY REFERENCES chat_sessions(id),
    summary TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    last_message_id INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
