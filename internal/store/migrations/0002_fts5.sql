-- 0002_fts5.sql
-- External content FTS5 table referencing the chunks table
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
	content,
	content=chunks,
	content_rowid=id
);

-- Triggers to keep the FTS5 index in sync with the chunks table
CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
	INSERT INTO chunks_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
	INSERT INTO chunks_fts(chunks_fts, rowid, content) VALUES('delete', old.id, old.content);
END;

CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
	INSERT INTO chunks_fts(chunks_fts, rowid, content) VALUES('delete', old.id, old.content);
	INSERT INTO chunks_fts(rowid, content) VALUES (new.id, new.content);
END;
