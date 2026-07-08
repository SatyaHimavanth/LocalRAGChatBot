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
	created_at INTEGER NOT NULL
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
	created_at INTEGER NOT NULL
);
