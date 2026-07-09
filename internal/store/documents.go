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
	Hash         string `json:"hash"`
	Content      string `json:"content"`
	CreatedAt    int64  `json:"createdAt"`
	ChunkCount   int    `json:"chunkCount"`
}

func AddDocument(db *sql.DB, collectionID int64, filename, hash, content string) (int64, error) {
	res, err := db.Exec("INSERT INTO documents (collection_id, filename, hash, content, created_at) VALUES (?, ?, ?, ?, ?)",
		collectionID, filename, hash, content, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func InsertChunk(db *sql.DB, docID int64, collectionID int64, content string, ord int, embedding []float32) (int64, error) {
	res, err := db.Exec("INSERT INTO chunks (document_id, collection_id, content, ord) VALUES (?, ?, ?, ?)", docID, collectionID, content, ord)
	if err != nil {
		return 0, err
	}
	chunkID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	blob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return 0, err
	}

	_, err = db.Exec("INSERT INTO chunks_vec (chunk_id, embedding) VALUES (?, ?)", chunkID, blob)
	if err != nil {
		return 0, err
	}

	return chunkID, nil
}

func DeleteDocument(db *sql.DB, docID int64) error {
	_, err := db.Exec(`DELETE FROM chunks_vec WHERE chunk_id IN (SELECT id FROM chunks WHERE document_id = ?)`, docID)
	if err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM chunks WHERE document_id = ?", docID)
	if err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM documents WHERE id = ?", docID)
	if err != nil {
		return err
	}
	return nil
}

func DeleteDocumentChunks(db *sql.DB, docID int64) error {
	_, err := db.Exec(`DELETE FROM chunks_vec WHERE chunk_id IN (SELECT id FROM chunks WHERE document_id = ?)`, docID)
	if err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM chunks WHERE document_id = ?", docID)
	if err != nil {
		return err
	}
	return nil
}

// GetDocumentsByCollection returns all documents in a collection with chunk counts.
func GetDocumentsByCollection(db *sql.DB, collectionID int64) ([]Document, error) {
	rows, err := db.Query(`
		SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''), d.created_at,
		       COALESCE(c.cnt, 0) AS chunk_count
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.collection_id = ?
		ORDER BY d.created_at DESC
	`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.Hash, &d.Content, &d.CreatedAt, &d.ChunkCount); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

func GetDocumentByHash(db *sql.DB, hash string, collectionID int64) (*Document, error) {
	var d Document
	err := db.QueryRow(`SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''), d.created_at,
	       COALESCE(c.cnt, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.hash = ? AND d.collection_id = ? LIMIT 1`,
		hash, collectionID).Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.Hash, &d.Content, &d.CreatedAt, &d.ChunkCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func GetDocumentByID(db *sql.DB, docID int64) (*Document, error) {
	var d Document
	err := db.QueryRow(`SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''), d.created_at,
	       COALESCE(c.cnt, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.id = ?`, docID).
		Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.Hash, &d.Content, &d.CreatedAt, &d.ChunkCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func UpdateDocumentContent(db *sql.DB, docID int64, content, hash string) error {
	_, err := db.Exec("UPDATE documents SET content = ?, hash = ? WHERE id = ?", content, hash, docID)
	return err
}
