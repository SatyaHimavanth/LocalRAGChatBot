-- 0001_init.sql
CREATE TABLE IF NOT EXISTS collections (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS documents (
    id            INTEGER PRIMARY KEY,
    collection_id INTEGER NOT NULL REFERENCES collections(id),
    filename      TEXT NOT NULL,
    summary       TEXT,
    created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
    id            INTEGER PRIMARY KEY,   -- canonical id — fts5 and vec0 both key off this
    document_id   INTEGER NOT NULL REFERENCES documents(id),
    collection_id INTEGER NOT NULL,      -- denormalised for fast collection-scoped filtering
    content       TEXT NOT NULL,
    ord           INTEGER NOT NULL       -- position within the source document
);

CREATE TABLE IF NOT EXISTS chat_sessions (
    id         INTEGER PRIMARY KEY,
    title      TEXT,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id         INTEGER PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES chat_sessions(id),
    role       TEXT NOT NULL,            -- user | assistant
    content    TEXT NOT NULL,
    created_at INTEGER NOT NULL
);