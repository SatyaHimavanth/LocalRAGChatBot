package store

import (
	"database/sql"
	"fmt"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

type ChunkMetadata struct {
	Title         string `json:"title"`
	SectionPath   string `json:"sectionPath"`
	HeadingLevel  int    `json:"headingLevel"`
	Summary       string `json:"summary"`
	ContentHash   string `json:"contentHash"`
	TokenCount    int    `json:"tokenCount"`
	CharCount     int    `json:"charCount"`
	ParentChunkID int64  `json:"parentChunkId"`
}

type ChunkRecord struct {
	ID            int64  `json:"id"`
	DocumentID    int64  `json:"documentId"`
	CollectionID  int64  `json:"collectionId"`
	Content       string `json:"content"`
	Ord           int    `json:"ord"`
	Title         string `json:"title"`
	SectionPath   string `json:"sectionPath"`
	HeadingLevel  int    `json:"headingLevel"`
	Summary       string `json:"summary"`
	ContentHash   string `json:"contentHash"`
	TokenCount    int    `json:"tokenCount"`
	CharCount     int    `json:"charCount"`
	ParentChunkID int64  `json:"parentChunkId"`
	PrevChunkID   int64  `json:"prevChunkId"`
	NextChunkID   int64  `json:"nextChunkId"`
}

// InsertChunkWithMetadata inserts a chunk and its embedding alongside richer metadata.
func InsertChunkWithMetadata(db *sql.DB, docID int64, collectionID int64, content string, ord int, meta ChunkMetadata, embedding []float32) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	if meta.TokenCount == 0 {
		meta.TokenCount = len(strings.Fields(content))
	}
	if meta.CharCount == 0 {
		meta.CharCount = len([]rune(content))
	}
	res, err := db.Exec(`
        INSERT INTO chunks (
            document_id, collection_id, content, ord,
            title, section_path, heading_level, summary, content_hash,
            token_count, char_count, parent_chunk_id, prev_chunk_id, next_chunk_id
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0)
    `, docID, collectionID, content, ord,
		strings.TrimSpace(meta.Title),
		strings.TrimSpace(meta.SectionPath),
		meta.HeadingLevel,
		strings.TrimSpace(meta.Summary),
		strings.TrimSpace(meta.ContentHash),
		meta.TokenCount,
		meta.CharCount,
		meta.ParentChunkID,
	)
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
	if _, err = db.Exec("INSERT INTO chunks_vec (chunk_id, embedding) VALUES (?, ?)", chunkID, blob); err != nil {
		return 0, err
	}
	return chunkID, nil
}

// UpdateChunkNeighbors refreshes prev/next links for all chunks in a document.
func UpdateChunkNeighbors(db *sql.DB, docID int64) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	rows, err := db.Query(`SELECT id FROM chunks WHERE document_id = ? ORDER BY ord ASC, id ASC`, docID)
	if err != nil {
		return err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i, id := range ids {
		var prevID, nextID int64
		if i > 0 {
			prevID = ids[i-1]
		}
		if i+1 < len(ids) {
			nextID = ids[i+1]
		}
		if _, err := db.Exec(`UPDATE chunks SET prev_chunk_id = ?, next_chunk_id = ? WHERE id = ?`, prevID, nextID, id); err != nil {
			return err
		}
	}
	return nil
}

// GetChunkByID returns a rich chunk record including metadata.
func GetChunkByID(db *sql.DB, chunkID int64) (*ChunkRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var c ChunkRecord
	err := db.QueryRow(`
        SELECT id, document_id, collection_id, content, ord,
               COALESCE(title, ''), COALESCE(section_path, ''), COALESCE(heading_level, 0),
               COALESCE(summary, ''), COALESCE(content_hash, ''), COALESCE(token_count, 0),
               COALESCE(char_count, 0), COALESCE(parent_chunk_id, 0), COALESCE(prev_chunk_id, 0),
               COALESCE(next_chunk_id, 0)
        FROM chunks WHERE id = ?
    `, chunkID).Scan(&c.ID, &c.DocumentID, &c.CollectionID, &c.Content, &c.Ord, &c.Title, &c.SectionPath, &c.HeadingLevel,
		&c.Summary, &c.ContentHash, &c.TokenCount, &c.CharCount, &c.ParentChunkID, &c.PrevChunkID, &c.NextChunkID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetChunksByDocument returns the ordered chunks for a document.
func GetChunksByDocument(db *sql.DB, docID int64) ([]ChunkRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := db.Query(`
        SELECT id, document_id, collection_id, content, ord,
               COALESCE(title, ''), COALESCE(section_path, ''), COALESCE(heading_level, 0),
               COALESCE(summary, ''), COALESCE(content_hash, ''), COALESCE(token_count, 0),
               COALESCE(char_count, 0), COALESCE(parent_chunk_id, 0), COALESCE(prev_chunk_id, 0),
               COALESCE(next_chunk_id, 0)
        FROM chunks WHERE document_id = ? ORDER BY ord ASC, id ASC
    `, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ChunkRecord
	for rows.Next() {
		var c ChunkRecord
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.CollectionID, &c.Content, &c.Ord, &c.Title, &c.SectionPath, &c.HeadingLevel,
			&c.Summary, &c.ContentHash, &c.TokenCount, &c.CharCount, &c.ParentChunkID, &c.PrevChunkID, &c.NextChunkID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
