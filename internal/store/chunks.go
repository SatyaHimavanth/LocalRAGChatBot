package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ChunkRecord struct {
	ID            int64  `json:"id"`
	DocumentID    int64  `json:"documentId"`
	CollectionID  int64  `json:"collectionId"`
	Content       string `json:"content"`
	Summary       string `json:"summary"`
	Ord           int    `json:"ord"`
	Level         int    `json:"level"`
	Role          string `json:"role"`
	ParentOrd     int    `json:"parentOrd"`
	PrevOrd       int    `json:"prevOrd"`
	NextOrd       int    `json:"nextOrd"`
	ChunkHash     string `json:"chunkHash"`
	EmbeddingHash string `json:"embeddingHash"`
	HeadingPath   string `json:"headingPath"`
	UpdatedAt     int64  `json:"updatedAt"`
}

// GetChunkByID returns the chunk row for a primary key.
func GetChunkByID(db *sql.DB, chunkID int64) (*ChunkRecord, error) {
	var c ChunkRecord
	err := db.QueryRow(`
		SELECT id, document_id, collection_id, content, COALESCE(summary,''), ord, COALESCE(level,0), COALESCE(role,''), COALESCE(parent_ord,-1), COALESCE(prev_ord,-1), COALESCE(next_ord,-1), COALESCE(chunk_hash,''), COALESCE(embedding_hash,''), COALESCE(heading_path,''), COALESCE(updated_at, 0)
		FROM chunks WHERE id = ? LIMIT 1`, chunkID).Scan(&c.ID, &c.DocumentID, &c.CollectionID, &c.Content, &c.Summary, &c.Ord, &c.Level, &c.Role, &c.ParentOrd, &c.PrevOrd, &c.NextOrd, &c.ChunkHash, &c.EmbeddingHash, &c.HeadingPath, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetChunkByDocumentAndOrd returns the chunk row for a document/ordinal pair.
func GetChunkByDocumentAndOrd(db *sql.DB, docID int64, ord int) (*ChunkRecord, error) {
	var c ChunkRecord
	err := db.QueryRow(`
		SELECT id, document_id, collection_id, content, COALESCE(summary,''), ord, COALESCE(level,0), COALESCE(role,''), COALESCE(parent_ord,-1), COALESCE(prev_ord,-1), COALESCE(next_ord,-1), COALESCE(chunk_hash,''), COALESCE(embedding_hash,''), COALESCE(heading_path,''), COALESCE(updated_at, 0)
		FROM chunks WHERE document_id = ? AND ord = ? LIMIT 1`, docID, ord).Scan(&c.ID, &c.DocumentID, &c.CollectionID, &c.Content, &c.Summary, &c.Ord, &c.Level, &c.Role, &c.ParentOrd, &c.PrevOrd, &c.NextOrd, &c.ChunkHash, &c.EmbeddingHash, &c.HeadingPath, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetChunksByDocument returns all chunk rows for a document ordered by ord.
func GetChunksByDocument(db *sql.DB, docID int64) ([]ChunkRecord, error) {
	rows, err := db.Query(`
		SELECT id, document_id, collection_id, content, COALESCE(summary,''), ord, COALESCE(level,0), COALESCE(role,''), COALESCE(parent_ord,-1), COALESCE(prev_ord,-1), COALESCE(next_ord,-1), COALESCE(chunk_hash,''), COALESCE(embedding_hash,''), COALESCE(heading_path,''), COALESCE(updated_at, 0)
		FROM chunks WHERE document_id = ? ORDER BY ord ASC`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChunkRecord
	for rows.Next() {
		var c ChunkRecord
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.CollectionID, &c.Content, &c.Summary, &c.Ord, &c.Level, &c.Role, &c.ParentOrd, &c.PrevOrd, &c.NextOrd, &c.ChunkHash, &c.EmbeddingHash, &c.HeadingPath, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteChunk removes a single chunk and its vector/embedding metadata.
func DeleteChunk(db *sql.DB, chunkID int64) error {
	if err := NewSQLiteVectorStore(db).DeleteChunkEmbedding(chunkID); err != nil {
		return err
	}
	_, err := db.Exec(`DELETE FROM chunks WHERE id = ?`, chunkID)
	return err
}

// InsertChunkWithHashes inserts a chunk and persists chunk/embedding hashes.
func InsertChunkWithHashes(db *sql.DB, docID int64, collectionID int64, content string, ord int, chunkHash string, embedding []float32) (int64, error) {
	return InsertChunkWithHierarchy(db, docID, collectionID, content, ord, 0, "leaf", -1, -1, -1, "", nil, chunkHash, embedding)
}

// InsertChunkWithHierarchy inserts a chunk row with hierarchy metadata and optional embedding.
func InsertChunkWithHierarchy(db *sql.DB, docID int64, collectionID int64, content string, ord int, level int, role string, parentOrd, prevOrd, nextOrd int, summary string, headingPath []string, chunkHash string, embedding []float32) (int64, error) {
	now := time.Now().Unix()
	hPath := encodeHeadingPath(headingPath)
	if role == "" {
		role = "leaf"
	}
	res, err := db.Exec(`INSERT INTO chunks (document_id, collection_id, content, summary, ord, level, role, parent_ord, prev_ord, next_ord, chunk_hash, embedding_hash, heading_path, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?)`,
		docID, collectionID, content, summary, ord, level, role, parentOrd, prevOrd, nextOrd, chunkHash, hPath, now)
	if err != nil {
		return 0, err
	}
	chunkID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if len(embedding) == 0 {
		return chunkID, nil
	}
	if err := NewSQLiteVectorStore(db).UpsertChunkEmbedding(chunkID, embedding); err != nil {
		return 0, err
	}
	return chunkID, nil
}

func encodeHeadingPath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	b, err := json.Marshal(path)
	if err != nil {
		return strings.Join(path, " / ")
	}
	return string(b)
}

func decodeHeadingPath(encoded string) []string {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil
	}
	var path []string
	if err := json.Unmarshal([]byte(encoded), &path); err == nil {
		cleaned := make([]string, 0, len(path))
		for _, p := range path {
			p = strings.TrimSpace(p)
			if p != "" {
				cleaned = append(cleaned, p)
			}
		}
		if len(cleaned) > 0 {
			return cleaned
		}
	}
	parts := strings.FieldsFunc(encoded, func(r rune) bool {
		return r == '/' || r == '>' || r == '|' || r == ','
	})
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.Trim(p, "[]"))
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

// GetChunkNeighborhood returns the target chunk plus nearby chunks from the same document.
func GetChunkNeighborhood(db *sql.DB, chunkID int64, radius int) ([]ChunkRecord, error) {
	center, err := GetChunkByID(db, chunkID)
	if err != nil || center == nil {
		return nil, err
	}
	if radius < 0 {
		radius = 0
	}
	all, err := GetChunksByDocument(db, center.DocumentID)
	if err != nil {
		return nil, err
	}
	byOrd := make(map[int]ChunkRecord, len(all))
	for _, c := range all {
		byOrd[c.Ord] = c
	}
	seen := make(map[int64]struct{}, len(all))
	out := make([]ChunkRecord, 0, 2*radius+3)
	add := func(ord int) {
		if ord < 0 {
			return
		}
		c, ok := byOrd[ord]
		if !ok {
			return
		}
		if _, exists := seen[c.ID]; exists {
			return
		}
		seen[c.ID] = struct{}{}
		out = append(out, c)
	}
	add(center.Ord)
	if center.ParentOrd >= 0 {
		add(center.ParentOrd)
	}
	for step := 1; step <= radius; step++ {
		add(center.Ord - step)
		add(center.Ord + step)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Ord == out[j].Ord {
			return out[i].ID < out[j].ID
		}
		return out[i].Ord < out[j].Ord
	})
	return out, nil
}

func hashEmbedding(embedding []float32) string {
	if len(embedding) == 0 {
		return ""
	}
	var b strings.Builder
	for i, v := range embedding {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%.6f", v))
	}
	return HashNormalizedText(b.String())
}

// ChunkSimilarity returns a simple overlap score between chunk hash sets.
func ChunkSimilarity(a, b []ChunkRecord) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := map[string]struct{}{}
	for _, c := range a {
		if c.Role == "summary" {
			continue
		}
		if c.ChunkHash != "" {
			setA[c.ChunkHash] = struct{}{}
		}
	}
	if len(setA) == 0 {
		return 0
	}
	match := 0
	for _, c := range b {
		if c.Role == "summary" {
			continue
		}
		if c.ChunkHash == "" {
			continue
		}
		if _, ok := setA[c.ChunkHash]; ok {
			match++
		}
	}
	denom := maxInt(1, len(setA))
	return float64(match) / float64(denom)
}

// TopChunkHashes returns sorted hashes for duplicate detection.
func TopChunkHashes(chunks []ChunkRecord, limit int) []string {
	out := make([]string, 0, len(chunks))
	for _, c := range chunks {
		if c.Role == "summary" {
			continue
		}
		if c.ChunkHash != "" {
			out = append(out, c.ChunkHash)
		}
	}
	sort.Strings(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
