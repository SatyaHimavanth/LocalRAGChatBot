package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Document ingest statuses.
const (
	DocStatusStaging   = "staging"
	DocStatusQueued    = "queued"
	DocStatusEmbedding = "embedding"
	DocStatusReady     = "ready"
	DocStatusFailed    = "failed"
)

// DocumentIngestMetadata stores derived ingest-time metadata.
type DocumentIngestMetadata struct {
	SourceType      string
	SourceSizeBytes int64
	WordCount       int
	LineCount       int
	CharacterCount  int
	ParagraphCount  int
	Title           string
	Summary         string
}

type Document struct {
	ID              int64  `json:"id"`
	CollectionID    int64  `json:"collectionId"`
	Filename        string `json:"filename"`
	Summary         string `json:"summary"`
	Hash            string `json:"hash"`
	Content         string `json:"content"`
	SourceType      string `json:"sourceType"`
	SourceSizeBytes int64  `json:"sourceSizeBytes"`
	WordCount       int    `json:"wordCount"`
	LineCount       int    `json:"lineCount"`
	CharacterCount  int    `json:"characterCount"`
	ParagraphCount  int    `json:"paragraphCount"`
	Title           string `json:"title"`
	CreatedAt       int64  `json:"createdAt"`
	ChunkCount      int    `json:"chunkCount"`
	Status          string `json:"status"`
	ExpectedChunks  int    `json:"expectedChunks"`
	BatchID         string `json:"batchId"`
	ErrorMessage    string `json:"errorMessage"`
	UpdatedAt       int64  `json:"updatedAt"`
}

func insertDocumentRow(db *sql.DB, collectionID int64, filename, hash, content string, status string, expectedChunks int, batchID string, meta DocumentIngestMetadata) (int64, error) {
	now := time.Now().Unix()
	if status == "" {
		status = DocStatusQueued
	}
	if meta.Title == "" {
		meta.Title = filename
	}
	res, err := db.Exec(`
		INSERT INTO documents (
			collection_id, filename, summary, hash, content, source_type, source_size_bytes,
			word_count, line_count, character_count, paragraph_count, title,
			created_at, status, expected_chunks, batch_id, error_message, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?)
	`, collectionID, filename, meta.Summary, hash, content, meta.SourceType, meta.SourceSizeBytes, meta.WordCount, meta.LineCount, meta.CharacterCount, meta.ParagraphCount, meta.Title, now, status, expectedChunks, batchID, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AddDocument inserts a fully ready document (legacy helper).
func AddDocument(db *sql.DB, collectionID int64, filename, hash, content string) (int64, error) {
	return insertDocumentRow(db, collectionID, filename, hash, content, DocStatusReady, 0, "", DocumentIngestMetadata{})
}

// AddDocumentWithStatus inserts a document with explicit ingest status fields.
func AddDocumentWithStatus(db *sql.DB, collectionID int64, filename, hash, content, status, batchID string, expectedChunks int) (int64, error) {
	return insertDocumentRow(db, collectionID, filename, hash, content, status, expectedChunks, batchID, DocumentIngestMetadata{})
}

// AddDocumentWithMetadata inserts a document and persists derived ingest metadata.
func AddDocumentWithMetadata(db *sql.DB, collectionID int64, filename, hash, content, status, batchID string, expectedChunks int, meta DocumentIngestMetadata) (int64, error) {
	return insertDocumentRow(db, collectionID, filename, hash, content, status, expectedChunks, batchID, meta)
}

func InsertChunk(db *sql.DB, docID int64, collectionID int64, content string, ord int, embedding []float32) (int64, error) {
	return InsertChunkWithHierarchy(db, docID, collectionID, content, ord, 0, "leaf", -1, -1, -1, "", nil, "", embedding)
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

func scanDocumentRows(rows *sql.Rows) ([]Document, error) {
	var docs []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.Hash, &d.Content, &d.SourceType, &d.SourceSizeBytes, &d.WordCount, &d.LineCount, &d.CharacterCount, &d.ParagraphCount, &d.Title, &d.CreatedAt, &d.ChunkCount, &d.Status, &d.ExpectedChunks, &d.BatchID, &d.ErrorMessage, &d.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

func queryDocument(db *sql.DB, query string, args ...any) (*Document, error) {
	var d Document
	err := db.QueryRow(query, args...).Scan(&d.ID, &d.CollectionID, &d.Filename, &d.Summary, &d.Hash, &d.Content, &d.SourceType, &d.SourceSizeBytes, &d.WordCount, &d.LineCount, &d.CharacterCount, &d.ParagraphCount, &d.Title, &d.CreatedAt, &d.ChunkCount, &d.Status, &d.ExpectedChunks, &d.BatchID, &d.ErrorMessage, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// GetDocumentsByCollection returns all documents in a collection with chunk counts.
func GetDocumentsByCollection(db *sql.DB, collectionID int64) ([]Document, error) {
	rows, err := db.Query(`
		SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''),
		       COALESCE(d.source_type,''), COALESCE(d.source_size_bytes,0), COALESCE(d.word_count,0), COALESCE(d.line_count,0), COALESCE(d.character_count,0), COALESCE(d.paragraph_count,0), COALESCE(d.title,''),
		       d.created_at,
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
	return scanDocumentRows(rows)
}

func GetDocumentByHash(db *sql.DB, hash string, collectionID int64) (*Document, error) {
	return queryDocument(db, `SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''),
	       COALESCE(d.source_type,''), COALESCE(d.source_size_bytes,0), COALESCE(d.word_count,0), COALESCE(d.line_count,0), COALESCE(d.character_count,0), COALESCE(d.paragraph_count,0), COALESCE(d.title,''),
	       d.created_at,
	       COALESCE(c.cnt, 0),
	       COALESCE(d.status, 'ready'), COALESCE(d.expected_chunks, 0), COALESCE(d.batch_id, ''),
	       COALESCE(d.error_message, ''), COALESCE(d.updated_at, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.hash = ? AND d.collection_id = ? LIMIT 1`, hash, collectionID)
}

func GetDocumentByHashAny(db *sql.DB, hash string) (*Document, error) {
	return queryDocument(db, `SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''),
	       COALESCE(d.source_type,''), COALESCE(d.source_size_bytes,0), COALESCE(d.word_count,0), COALESCE(d.line_count,0), COALESCE(d.character_count,0), COALESCE(d.paragraph_count,0), COALESCE(d.title,''),
	       d.created_at,
	       COALESCE(c.cnt, 0),
	       COALESCE(d.status, 'ready'), COALESCE(d.expected_chunks, 0), COALESCE(d.batch_id, ''),
	       COALESCE(d.error_message, ''), COALESCE(d.updated_at, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.hash = ? LIMIT 1`, hash)
}

func GetDocumentsByFilename(db *sql.DB, filename string) ([]Document, error) {
	rows, err := db.Query(`
		SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''),
		       COALESCE(d.source_type,''), COALESCE(d.source_size_bytes,0), COALESCE(d.word_count,0), COALESCE(d.line_count,0), COALESCE(d.character_count,0), COALESCE(d.paragraph_count,0), COALESCE(d.title,''),
		       d.created_at,
		       COALESCE(c.cnt, 0),
		       COALESCE(d.status, 'ready'), COALESCE(d.expected_chunks, 0), COALESCE(d.batch_id, ''),
		       COALESCE(d.error_message, ''), COALESCE(d.updated_at, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE LOWER(d.filename) = LOWER(?)
		ORDER BY d.updated_at DESC`, filename)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}

func GetDocumentByID(db *sql.DB, docID int64) (*Document, error) {
	return queryDocument(db, `SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''),
	       COALESCE(d.source_type,''), COALESCE(d.source_size_bytes,0), COALESCE(d.word_count,0), COALESCE(d.line_count,0), COALESCE(d.character_count,0), COALESCE(d.paragraph_count,0), COALESCE(d.title,''),
	       d.created_at,
	       COALESCE(c.cnt, 0),
	       COALESCE(d.status, 'ready'), COALESCE(d.expected_chunks, 0), COALESCE(d.batch_id, ''),
	       COALESCE(d.error_message, ''), COALESCE(d.updated_at, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.id = ?`, docID)
}

func UpdateDocumentContent(db *sql.DB, docID int64, content, hash string) error {
	_, err := db.Exec(`UPDATE documents SET content = ?, hash = ?, updated_at = ? WHERE id = ?`,
		content, hash, time.Now().Unix(), docID)
	return err
}

// UpdateDocumentMetadata updates ingest metadata fields for a document.
func UpdateDocumentMetadata(db *sql.DB, docID int64, meta DocumentIngestMetadata) error {
	_, err := db.Exec(`
		UPDATE documents
		SET summary = ?, source_type = ?, source_size_bytes = ?, word_count = ?, line_count = ?, character_count = ?, paragraph_count = ?, title = ?, updated_at = ?
		WHERE id = ?`,
		meta.Summary, meta.SourceType, meta.SourceSizeBytes, meta.WordCount, meta.LineCount, meta.CharacterCount, meta.ParagraphCount, meta.Title, time.Now().Unix(), docID)
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
		SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''),
		       COALESCE(d.source_type,''), COALESCE(d.source_size_bytes,0), COALESCE(d.word_count,0), COALESCE(d.line_count,0), COALESCE(d.character_count,0), COALESCE(d.paragraph_count,0), COALESCE(d.title,''),
		       d.created_at,
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
	return scanDocumentRows(rows)
}

// GetResumableDocuments returns queued/embedding/failed docs that have content (can resume embed).
func GetResumableDocuments(db *sql.DB, batchID string) ([]Document, error) {
	query := `
		SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''),
		       COALESCE(d.source_type,''), COALESCE(d.source_size_bytes,0), COALESCE(d.word_count,0), COALESCE(d.line_count,0), COALESCE(d.character_count,0), COALESCE(d.paragraph_count,0), COALESCE(d.title,''),
		       d.created_at,
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
	return scanDocumentRows(rows)
}

// DeleteStagingDocuments removes incomplete staging rows left after a crash.
func DeleteStagingDocuments(db *sql.DB) (int64, error) {
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
