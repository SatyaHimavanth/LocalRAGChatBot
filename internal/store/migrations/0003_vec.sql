-- 0003_vec.sql
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_vec USING vec0(
    chunk_id  INTEGER PRIMARY KEY,
    embedding FLOAT[768]   -- match nomic-embed-text-v1.5's output dimension
);