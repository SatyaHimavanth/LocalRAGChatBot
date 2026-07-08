package store

import (
	"database/sql"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

type Document struct {
	ID           int64  `json:"id"`
	CollectionID int64  `json:"collectionId"`
	Filename     string `json:"filename"`
	Summary      string `json:"summary"`
	CreatedAt    int64  `json:"createdAt"`
}

func AddDocument(db *sql.DB, collectionID int64, filename string) (int64, error) {
	res, err := db.Exec("INSERT INTO documents (collection_id, filename, created_at) VALUES (?, ?, ?)", collectionID, filename, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func InsertChunk(db *sql.DB, docID int64, collectionID int64, content string, ord int, embedding []float32) (int64, error) {
	// 1. Insert standard text chunk (FTS5 trigger auto-indexes this)
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

	// 3. Store vector representation
	_, err = db.Exec("INSERT INTO chunks_vec (chunk_id, embedding) VALUES (?, ?)", chunkID, blob)
	if err != nil {
		return 0, err
	}

	return chunkID, nil
}

// DeleteDocument removes a document and all its chunks.
// IMPORTANT: We do NOT manually delete from chunks_fts here.
// The AFTER DELETE trigger on the chunks table handles FTS cleanup automatically.
func DeleteDocument(db *sql.DB, docID int64) error {
	// 1. Delete vector entries for this document's chunks
	_, err := db.Exec(`DELETE FROM chunks_vec WHERE chunk_id IN (SELECT id FROM chunks WHERE document_id = ?)`, docID)
	if err != nil {
		return err
	}

	// 2. Delete chunks — the AFTER DELETE trigger on chunks table
	//    automatically removes corresponding rows from chunks_fts.
	_, err = db.Exec("DELETE FROM chunks WHERE document_id = ?", docID)
	if err != nil {
		return err
	}

	// 3. Delete document record
	_, err = db.Exec("DELETE FROM documents WHERE id = ?", docID)
	if err != nil {
		return err
	}
	return nil
}

// GetDocumentsByCollection returns all documents in a collection.
func GetDocumentsByCollection(db *sql.DB, collectionID int64) ([]Document, error) {
	rows, err := db.Query("SELECT id, collection_id, filename, COALESCE(summary,''), created_at FROM documents WHERE collection_id = ? ORDER BY created_at DESC", collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.CreatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}
