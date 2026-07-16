-- 0001_init.sql
CREATE TABLE IF NOT EXISTS collections (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,

    embedding_model TEXT NOT NULL DEFAULT '',
    embedding_dims INTEGER NOT NULL DEFAULT 0,
    vector_backend TEXT NOT NULL DEFAULT 'sqlite-vec',

    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY,

    collection_id INTEGER NOT NULL REFERENCES collections(id),

    filename TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',

    summary TEXT,

    hash TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',

    source_type TEXT NOT NULL DEFAULT '',
    source_size_bytes INTEGER NOT NULL DEFAULT 0,

    word_count INTEGER NOT NULL DEFAULT 0,
    line_count INTEGER NOT NULL DEFAULT 0,
    character_count INTEGER NOT NULL DEFAULT 0,
    paragraph_count INTEGER NOT NULL DEFAULT 0,

    status TEXT NOT NULL DEFAULT 'ready',
    expected_chunks INTEGER NOT NULL DEFAULT 0,
    batch_id TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',

    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS chunks (
    id INTEGER PRIMARY KEY,

    document_id INTEGER NOT NULL REFERENCES documents(id),
    collection_id INTEGER NOT NULL,

    content TEXT NOT NULL,

    ord INTEGER NOT NULL,

    chunk_hash TEXT NOT NULL DEFAULT '',
    embedding_hash TEXT NOT NULL DEFAULT '',

    summary TEXT NOT NULL DEFAULT '',

    level INTEGER NOT NULL DEFAULT 0,
    role TEXT NOT NULL DEFAULT 'leaf',

    parent_ord INTEGER NOT NULL DEFAULT -1,
    prev_ord INTEGER NOT NULL DEFAULT -1,
    next_ord INTEGER NOT NULL DEFAULT -1,

    heading_path TEXT NOT NULL DEFAULT '',

    updated_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS chat_sessions (
	id INTEGER PRIMARY KEY,
	title TEXT,
	collection_id INTEGER NOT NULL DEFAULT 1,
	current_leaf_message_id INTEGER NOT NULL DEFAULT 0,
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
	cancelled INTEGER NOT NULL DEFAULT 0,
	parent_message_id INTEGER NOT NULL DEFAULT 0,
	agent_metadata_json TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_session_parent
ON chat_messages(session_id, parent_message_id, created_at, id);

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


CREATE INDEX IF NOT EXISTS idx_collections_vector_backend
ON collections(vector_backend);

CREATE INDEX IF NOT EXISTS idx_chunks_parent_ord
ON chunks(document_id,parent_ord);

CREATE INDEX IF NOT EXISTS idx_chunks_role
ON chunks(document_id,role,ord);

CREATE INDEX IF NOT EXISTS idx_chunks_level
ON chunks(document_id,level,ord);
