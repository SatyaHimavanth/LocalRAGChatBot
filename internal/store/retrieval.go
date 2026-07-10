// internal/store/retrieval.go
package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

type ScoredChunk struct {
	ChunkID int64
	Content string
	Score   float64
}

func vectorSearch(db *sql.DB, collectionID int64, queryEmbedding []float32, topK int) ([]ScoredChunk, error) {
	blob, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`
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

func keywordSearch(db *sql.DB, collectionID int64, query string, topK int) ([]ScoredChunk, error) {
	rows, err := db.Query(`
		SELECT c.id, c.content, bm25(chunks_fts) AS score
		FROM chunks_fts
		JOIN chunks c ON c.id = chunks_fts.rowid
		JOIN documents d ON d.id = c.document_id
		WHERE chunks_fts MATCH ? AND c.collection_id = ?
		AND (d.status = 'ready' OR d.status IS NULL OR d.status = '')
		ORDER BY score
		LIMIT ?
	`, query, collectionID, topK)
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

// VectorSearch is an exported wrapper around vectorSearch for use by the Search API.
func VectorSearch(db *sql.DB, collectionID int64, queryEmbedding []float32, topK int) ([]ScoredChunk, error) {
	return vectorSearch(db, collectionID, queryEmbedding, topK)
}

// KeywordSearch is an exported wrapper around keywordSearch for use by the Search API.
func KeywordSearch(db *sql.DB, collectionID int64, query string, topK int) ([]ScoredChunk, error) {
	return keywordSearch(db, collectionID, query, topK)
}

// FallbackSearch performs a simple LIKE-based search when FTS5 is unavailable.
func FallbackSearch(db *sql.DB, collectionID int64, query string, topK int) ([]ScoredChunk, error) {
	// Split query into words for LIKE matching
	words := strings.Fields(query)
	if len(words) == 0 {
		return nil, nil
	}

	// Build LIKE conditions
	likeClauses := make([]string, 0, len(words))
	args := make([]any, 0, len(words)+1)
	args = append(args, collectionID)
	for _, w := range words {
		likeClauses = append(likeClauses, "c.content LIKE ?")
		args = append(args, "%"+w+"%")
	}

	querySQL := fmt.Sprintf(`
		SELECT c.id, c.content, 0.5 AS score
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.collection_id = ? AND (%s)
		AND (d.status = 'ready' OR d.status IS NULL OR d.status = '')
		ORDER BY LENGTH(c.content) ASC
		LIMIT ?
	`, strings.Join(likeClauses, " AND "))
	args = append(args, topK)

	rows, err := db.Query(querySQL, args...)
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

// HybridSearch merges vector and keyword results with reciprocal rank fusion.
func HybridSearch(db *sql.DB, collectionID int64, query string, queryEmbedding []float32, topN int) ([]ScoredChunk, error) {
	const k = 60

	vecResults, err := vectorSearch(db, collectionID, queryEmbedding, 20)
	if err != nil {
		// Non-fatal: continue with keyword-only
		vecResults = nil
	}
	kwResults, err := keywordSearch(db, collectionID, query, 20)
	if err != nil {
		// Try fallback search
		kwResults, err = FallbackSearch(db, collectionID, query, 20)
		if err != nil {
			return nil, err
		}
	}

	scores := map[int64]float64{}
	content := map[int64]string{}
	for rank, r := range vecResults {
		scores[r.ChunkID] += 1.0 / float64(k+rank+1)
		content[r.ChunkID] = r.Content
	}
	for rank, r := range kwResults {
		scores[r.ChunkID] += 1.0 / float64(k+rank+1)
		content[r.ChunkID] = r.Content
	}

	merged := make([]ScoredChunk, 0, len(scores))
	for id, score := range scores {
		merged = append(merged, ScoredChunk{ChunkID: id, Content: content[id], Score: score})
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Score > merged[j].Score })

	if len(merged) > topN {
		merged = merged[:topN]
	}
	return merged, nil
}
