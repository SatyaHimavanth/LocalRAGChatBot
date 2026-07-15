package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// Document ingest statuses.
const (
	DocStatusStaging   = "staging"
	DocStatusQueued    = "queued"
	DocStatusEmbedding = "embedding"
	DocStatusReady     = "ready"
	DocStatusFailed    = "failed"
)

type Document struct {
	ID             int64  `json:"id"`
	CollectionID   int64  `json:"collectionId"`
	Filename       string `json:"filename"`
	Summary        string `json:"summary"`
	Hash           string `json:"hash"`
	Content        string `json:"content"`
	CreatedAt      int64  `json:"createdAt"`
	ChunkCount     int    `json:"chunkCount"`
	Status         string `json:"status"`
	ExpectedChunks int    `json:"expectedChunks"`
	BatchID        string `json:"batchId"`
	ErrorMessage   string `json:"errorMessage"`
	UpdatedAt      int64  `json:"updatedAt"`
}

// AddDocument inserts a fully ready document (legacy helper).
func AddDocument(db *sql.DB, collectionID int64, filename, hash, content string) (int64, error) {
	now := time.Now().Unix()
	res, err := db.Exec(`
		INSERT INTO documents (collection_id, filename, hash, content, created_at, status, expected_chunks, batch_id, error_message, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, '', '', ?)`,
		collectionID, filename, hash, content, now, DocStatusReady, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AddDocumentWithStatus inserts a document with explicit ingest status fields.
func AddDocumentWithStatus(db *sql.DB, collectionID int64, filename, hash, content, status, batchID string, expectedChunks int) (int64, error) {
	now := time.Now().Unix()
	if status == "" {
		status = DocStatusQueued
	}
	res, err := db.Exec(`
		INSERT INTO documents (collection_id, filename, hash, content, created_at, status, expected_chunks, batch_id, error_message, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', ?)`,
		collectionID, filename, hash, content, now, status, expectedChunks, batchID, now)
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

// HasChunkOrd reports whether a chunk with the given ord already exists for the document.
func HasChunkOrd(db *sql.DB, docID int64, ord int) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(1) FROM chunks WHERE document_id = ? AND ord = ?`, docID, ord).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// CountChunks returns how many chunks exist for a document.
func CountChunks(db *sql.DB, docID int64) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(1) FROM chunks WHERE document_id = ?`, docID).Scan(&n)
	return n, err
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
		       COALESCE(c.cnt, 0) AS chunk_count,
		       COALESCE(d.status, 'ready'), COALESCE(d.expected_chunks, 0), COALESCE(d.batch_id, ''),
		       COALESCE(d.error_message, ''), COALESCE(d.updated_at, 0)
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
		if err := rows.Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.Hash, &d.Content, &d.CreatedAt, &d.ChunkCount,
			&d.Status, &d.ExpectedChunks, &d.BatchID, &d.ErrorMessage, &d.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

func GetDocumentByHash(db *sql.DB, hash string, collectionID int64) (*Document, error) {
	var d Document
	err := db.QueryRow(`SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''), d.created_at,
	       COALESCE(c.cnt, 0),
	       COALESCE(d.status, 'ready'), COALESCE(d.expected_chunks, 0), COALESCE(d.batch_id, ''),
	       COALESCE(d.error_message, ''), COALESCE(d.updated_at, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.hash = ? AND d.collection_id = ? LIMIT 1`,
		hash, collectionID).Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.Hash, &d.Content, &d.CreatedAt, &d.ChunkCount,
		&d.Status, &d.ExpectedChunks, &d.BatchID, &d.ErrorMessage, &d.UpdatedAt)
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
	       COALESCE(c.cnt, 0),
	       COALESCE(d.status, 'ready'), COALESCE(d.expected_chunks, 0), COALESCE(d.batch_id, ''),
	       COALESCE(d.error_message, ''), COALESCE(d.updated_at, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.id = ?`, docID).
		Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.Hash, &d.Content, &d.CreatedAt, &d.ChunkCount,
			&d.Status, &d.ExpectedChunks, &d.BatchID, &d.ErrorMessage, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func UpdateDocumentContent(db *sql.DB, docID int64, content, hash string) error {
	_, err := db.Exec(`UPDATE documents SET content = ?, hash = ?, updated_at = ? WHERE id = ?`,
		content, hash, time.Now().Unix(), docID)
	return err
}

// UpdateDocumentSummary stores an extracted document summary for workspace and analytics views.
func UpdateDocumentSummary(db *sql.DB, docID int64, summary string) error {
	_, err := db.Exec(`UPDATE documents SET summary = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(summary), time.Now().Unix(), docID)
	return err
}

// UpdateDocumentIngest sets status, expected chunks, batch, and optional error for a document.
func UpdateDocumentIngest(db *sql.DB, docID int64, status string, expectedChunks int, batchID, errMsg string) error {
	_, err := db.Exec(`
		UPDATE documents
		SET status = ?, expected_chunks = ?, batch_id = ?, error_message = ?, updated_at = ?
		WHERE id = ?`,
		status, expectedChunks, batchID, errMsg, time.Now().Unix(), docID)
	return err
}

// UpdateDocumentStatus updates only status (and optional error message).
func UpdateDocumentStatus(db *sql.DB, docID int64, status, errMsg string) error {
	_, err := db.Exec(`UPDATE documents SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`,
		status, errMsg, time.Now().Unix(), docID)
	return err
}

// GetIncompleteDocuments returns docs that are not ready (queued/embedding/failed/staging).
func GetIncompleteDocuments(db *sql.DB) ([]Document, error) {
	rows, err := db.Query(`
		SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''), d.created_at,
		       COALESCE(c.cnt, 0),
		       COALESCE(d.status, 'ready'), COALESCE(d.expected_chunks, 0), COALESCE(d.batch_id, ''),
		       COALESCE(d.error_message, ''), COALESCE(d.updated_at, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.status IN (?, ?, ?, ?)
		ORDER BY d.updated_at ASC, d.id ASC
	`, DocStatusStaging, DocStatusQueued, DocStatusEmbedding, DocStatusFailed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.Hash, &d.Content, &d.CreatedAt, &d.ChunkCount,
			&d.Status, &d.ExpectedChunks, &d.BatchID, &d.ErrorMessage, &d.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// GetResumableDocuments returns queued/embedding/failed docs that have content (can resume embed).
func GetResumableDocuments(db *sql.DB, batchID string) ([]Document, error) {
	query := `
		SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''), d.created_at,
		       COALESCE(c.cnt, 0),
		       COALESCE(d.status, 'ready'), COALESCE(d.expected_chunks, 0), COALESCE(d.batch_id, ''),
		       COALESCE(d.error_message, ''), COALESCE(d.updated_at, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.status IN (?, ?, ?) AND LENGTH(TRIM(d.content)) > 0`
	args := []any{DocStatusQueued, DocStatusEmbedding, DocStatusFailed}
	if batchID != "" {
		query += ` AND d.batch_id = ?`
		args = append(args, batchID)
	}
	query += ` ORDER BY d.id ASC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.Hash, &d.Content, &d.CreatedAt, &d.ChunkCount,
			&d.Status, &d.ExpectedChunks, &d.BatchID, &d.ErrorMessage, &d.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// DeleteStagingDocuments removes incomplete staging rows left after a crash.
func DeleteStagingDocuments(db *sql.DB) (int64, error) {
	// Clean vectors/chunks for staging docs first
	_, err := db.Exec(`DELETE FROM chunks_vec WHERE chunk_id IN (
		SELECT c.id FROM chunks c JOIN documents d ON d.id = c.document_id WHERE d.status = ?
	)`, DocStatusStaging)
	if err != nil {
		return 0, err
	}
	_, err = db.Exec(`DELETE FROM chunks WHERE document_id IN (SELECT id FROM documents WHERE status = ?)`, DocStatusStaging)
	if err != nil {
		return 0, err
	}
	res, err := db.Exec(`DELETE FROM documents WHERE status = ?`, DocStatusStaging)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ResetEmbeddingToQueued marks interrupted embedding jobs as queued for resume.
func ResetEmbeddingToQueued(db *sql.DB) error {
	_, err := db.Exec(`UPDATE documents SET status = ?, error_message = '', updated_at = ? WHERE status = ?`,
		DocStatusQueued, time.Now().Unix(), DocStatusEmbedding)
	return err
}

// DiscardIncompleteDocuments deletes all non-ready documents (and their chunks).
func DiscardIncompleteDocuments(db *sql.DB) (int64, error) {
	ids, err := incompleteDocIDs(db)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := DeleteDocument(db, id); err != nil {
			return 0, fmt.Errorf("discarding document %d: %w", id, err)
		}
	}
	return int64(len(ids)), nil
}

func incompleteDocIDs(db *sql.DB) ([]int64, error) {
	rows, err := db.Query(`SELECT id FROM documents WHERE status IN (?, ?, ?, ?)`,
		DocStatusStaging, DocStatusQueued, DocStatusEmbedding, DocStatusFailed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
