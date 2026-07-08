-- 0003_vec.sql
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_vec USING vec0(
	chunk_id INTEGER PRIMARY KEY,
	embedding float[768]
);
