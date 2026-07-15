package store

import (
	"database/sql"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// VectorStore abstracts the vector storage backend used by the application.
// The current implementation persists into SQLite + sqlite-vec, but the
// interface keeps higher layers decoupled from that decision.
type VectorStore interface {
	Name() string
	UpsertChunkEmbedding(chunkID int64, embedding []float32) error
	DeleteChunkEmbedding(chunkID int64) error
	DeleteCollectionEmbeddings(collectionID int64) error
	Search(collectionID int64, queryEmbedding []float32, topK int) ([]ScoredChunk, error)
}

type SQLiteVectorStore struct {
	db *sql.DB
}

func NewSQLiteVectorStore(db *sql.DB) *SQLiteVectorStore {
	return &SQLiteVectorStore{db: db}
}

func (s *SQLiteVectorStore) Name() string { return "sqlite-vec" }

func (s *SQLiteVectorStore) UpsertChunkEmbedding(chunkID int64, embedding []float32) error {
	if len(embedding) == 0 {
		return nil
	}
	blob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`INSERT INTO chunks_vec (chunk_id, embedding) VALUES (?, ?)`, chunkID, blob); err != nil {
		return err
	}
	embHash := hashEmbedding(embedding)
	now := time.Now().Unix()
	if _, err := s.db.Exec(`INSERT INTO embeddings (chunk_id, model, dims, embedding_hash, created_at) VALUES (?, '', ?, ?, ?)`, chunkID, len(embedding), embHash, now); err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE chunks SET embedding_hash = ?, updated_at = ? WHERE id = ?`, embHash, now, chunkID)
	return err
}

func (s *SQLiteVectorStore) DeleteChunkEmbedding(chunkID int64) error {
	if _, err := s.db.Exec(`DELETE FROM chunks_vec WHERE chunk_id = ?`, chunkID); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM embeddings WHERE chunk_id = ?`, chunkID); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE chunks SET embedding_hash = '', updated_at = ? WHERE id = ?`, time.Now().Unix(), chunkID)
	return err
}

func (s *SQLiteVectorStore) DeleteCollectionEmbeddings(collectionID int64) error {
	_, err := s.db.Exec(`DELETE FROM chunks_vec WHERE chunk_id IN (SELECT id FROM chunks WHERE collection_id = ?)`, collectionID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM embeddings WHERE chunk_id IN (SELECT id FROM chunks WHERE collection_id = ?)`, collectionID)
	return err
}

func (s *SQLiteVectorStore) Search(collectionID int64, queryEmbedding []float32, topK int) ([]ScoredChunk, error) {
	if len(queryEmbedding) == 0 {
		return nil, nil
	}
	blob, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
		SELECT v.chunk_id, c.content, v.distance
		FROM chunks_vec v
		JOIN chunks c ON c.id = v.chunk_id
		JOIN documents d ON d.id = c.document_id
		WHERE v.embedding MATCH ? AND k = ?
		AND c.collection_id = ?
		AND (d.status = 'ready' OR d.status IS NULL OR d.status = '')
		ORDER BY v.distance
	`, blob, topK, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ScoredChunk
	for rows.Next() {
		var r ScoredChunk
		if err := rows.Scan(&r.ChunkID, &r.Content, &r.Score); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// DeleteCollectionVectors is retained as a narrow compatibility helper so
// collection deletion can cleanly remove all vector rows before chunk deletion.
func DeleteCollectionVectors(db *sql.DB, collectionID int64) error {
	return NewSQLiteVectorStore(db).DeleteCollectionEmbeddings(collectionID)
}
