package store

import (
	"database/sql"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

type Document struct {
	ID           int64
	CollectionID int64
	Filename     string
	Summary      string
	CreatedAt    int64
}

func CreateCollection(db *sql.DB, name string) (int64, error) {
	res, err := db.Exec("INSERT INTO collections (name, created_at) VALUES (?, ?)", name, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func AddDocument(db *sql.DB, collectionID int64, filename string) (int64, error) {
	res, err := db.Exec("INSERT INTO documents (collection_id, filename, created_at) VALUES (?, ?, ?)", collectionID, filename, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func InsertChunk(db *sql.DB, docID int64, collectionID int64, content string, ord int, embedding []float32) (int64, error) {
	// 1. Insert standard text chunk (automatically triggers FTS5 indexing)
	res, err := db.Exec("INSERT INTO chunks (document_id, collection_id, content, ord) VALUES (?, ?, ?, ?)", docID, collectionID, content, ord)
	if err != nil {
		return 0, err
	}
	chunkID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	// 2. Serialize vector embedding
	blob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return 0, err
	}

	// 3. Store high-performance vector representation
	_, err = db.Exec("INSERT INTO chunks_vec (chunk_id, embedding) VALUES (?, ?)", chunkID, blob)
	if err != nil {
		return 0, err
	}

	return chunkID, nil
}
