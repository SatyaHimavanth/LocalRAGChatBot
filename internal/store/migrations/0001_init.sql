-- 0001_init.sql
CREATE TABLE IF NOT EXISTS collections (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS documents (
	id INTEGER PRIMARY KEY,
	collection_id INTEGER NOT NULL REFERENCES collections(id),
	filename TEXT NOT NULL,
	summary TEXT,
	hash TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	-- Ingest job lifecycle: staging | queued | embedding | ready | failed
	status TEXT NOT NULL DEFAULT 'ready',
	expected_chunks INTEGER NOT NULL DEFAULT 0,
	batch_id TEXT NOT NULL DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	updated_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS chunks (
	id INTEGER PRIMARY KEY,
	document_id INTEGER NOT NULL REFERENCES documents(id),
	collection_id INTEGER NOT NULL,
	content TEXT NOT NULL,
	ord INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_sessions (
	id INTEGER PRIMARY KEY,
	title TEXT,
	collection_id INTEGER NOT NULL DEFAULT 1,
	archived INTEGER NOT NULL DEFAULT 0,
	pinned INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_messages (
	id INTEGER PRIMARY KEY,
	session_id INTEGER NOT NULL REFERENCES chat_sessions(id),
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	cancelled INTEGER NOT NULL DEFAULT 0
);

-- Stores source chunks referenced in AI responses
CREATE TABLE IF NOT EXISTS chat_message_sources (
	id INTEGER PRIMARY KEY,
	message_id INTEGER NOT NULL REFERENCES chat_messages(id),
	session_id INTEGER NOT NULL REFERENCES chat_sessions(id),
	chunk_id INTEGER NOT NULL,
	filename TEXT NOT NULL,
	collection_id INTEGER NOT NULL,
	collection_name TEXT NOT NULL,
	similarity_score REAL NOT NULL DEFAULT 0,
	content TEXT NOT NULL,
	ref_number INTEGER NOT NULL
);
