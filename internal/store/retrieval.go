// internal/store/retrieval.go
package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type ScoredChunk struct {
	ChunkID int64
	Content string
	Score   float64
}

// MetadataSearchHit captures a document-level match returned by metadata search.
type MetadataSearchHit struct {
	DocumentID     int64
	ChunkID        int64
	CollectionID   int64
	CollectionName string
	Filename       string
	Title          string
	Summary        string
	Snippet        string
	Score          float64
}

func vectorSearch(db *sql.DB, collectionID int64, queryEmbedding []float32, topK int) ([]ScoredChunk, error) {
	return NewSQLiteVectorStore(db).Search(collectionID, queryEmbedding, topK)
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
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Score == merged[j].Score {
			return merged[i].ChunkID < merged[j].ChunkID
		}
		return merged[i].Score > merged[j].Score
	})

	expanded, err := ExpandChunkNeighborhoods(db, merged, 1)
	if err == nil && len(expanded) > 0 {
		merged = expanded
	}

	if len(merged) > topN {
		merged = merged[:topN]
	}
	return merged, nil
}

// MetadataSearch performs a lightweight document-level search across filenames,
// titles, summaries, content and collection names.
func MetadataSearch(db *sql.DB, collectionID int64, query string, topK int) ([]MetadataSearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	terms := splitSearchTerms(query)
	if len(terms) == 0 {
		return nil, nil
	}
	if topK <= 0 {
		topK = 20
	}

	clauses := []string{"(d.status = 'ready' OR d.status IS NULL OR d.status = '')"}
	args := make([]any, 0, len(terms)*5+2)
	if collectionID > 0 {
		clauses = append(clauses, "d.collection_id = ?")
		args = append(args, collectionID)
	}
	for _, term := range terms {
		like := "%" + term + "%"
		clauses = append(clauses, `(LOWER(d.filename) LIKE ? OR LOWER(COALESCE(d.title,'')) LIKE ? OR LOWER(COALESCE(d.summary,'')) LIKE ? OR LOWER(COALESCE(d.content,'')) LIKE ? OR LOWER(COALESCE(c.name,'')) LIKE ?)`)
		args = append(args, like, like, like, like, like)
	}
	querySQL := fmt.Sprintf(`
		SELECT
			d.id,
			COALESCE(d.collection_id, 0),
			COALESCE(c.name, ''),
			COALESCE(d.filename, ''),
			COALESCE(d.title, ''),
			COALESCE(d.summary, ''),
			COALESCE(d.content, ''),
			COALESCE((SELECT id FROM chunks ch WHERE ch.document_id = d.id ORDER BY ch.ord ASC, ch.id ASC LIMIT 1), 0),
			d.updated_at,
			d.created_at
		FROM documents d
		JOIN collections c ON c.id = d.collection_id
		WHERE %s
		ORDER BY d.updated_at DESC, d.created_at DESC
		LIMIT ?
	`, strings.Join(clauses, " AND "))
	args = append(args, topK)

	rows, err := db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MetadataSearchHit
	for rows.Next() {
		var hit MetadataSearchHit
		var content string
		var updatedAt, createdAt int64
		if err := rows.Scan(&hit.DocumentID, &hit.CollectionID, &hit.CollectionName, &hit.Filename, &hit.Title, &hit.Summary, &content, &hit.ChunkID, &updatedAt, &createdAt); err != nil {
			return nil, err
		}
		hit.Snippet = chooseMetadataSnippet(hit.Title, hit.Summary, content)
		hit.Score = metadataMatchScore(query, hit.Filename, hit.Title, hit.Summary, content, hit.CollectionName)
		if hit.Score <= 0 {
			hit.Score = 0.1
		}
		if updatedAt > 0 && createdAt > 0 && updatedAt >= createdAt {
			hit.Score += 0.01
		}
		out = append(out, hit)
	}
	return out, rows.Err()
}

func splitSearchTerms(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	if len(fields) == 0 {
		return nil
	}
	terms := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, f := range fields {
		f = strings.Trim(f, "\"'()[]{}.,:;!?/\\")
		if f == "" {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		terms = append(terms, f)
	}
	return terms
}

func chooseMetadataSnippet(title, summary, content string) string {
	for _, candidate := range []string{title, summary, content} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if len(candidate) > 240 {
			candidate = candidate[:240] + "..."
		}
		return candidate
	}
	return ""
}

func metadataMatchScore(query, filename, title, summary, content, collectionName string) float64 {
	terms := splitSearchTerms(query)
	if len(terms) == 0 {
		return 0
	}
	fields := []struct {
		text   string
		weight float64
	}{
		{strings.ToLower(filename), 0.35},
		{strings.ToLower(title), 0.30},
		{strings.ToLower(summary), 0.20},
		{strings.ToLower(content), 0.10},
		{strings.ToLower(collectionName), 0.05},
	}
	var score float64
	for _, term := range terms {
		for _, field := range fields {
			if field.text == "" {
				continue
			}
			if strings.Contains(field.text, term) {
				score += field.weight
			}
		}
	}
	normalizer := float64(len(terms))
	if normalizer < 1 {
		normalizer = 1
	}
	score = score / normalizer
	if strings.Contains(strings.ToLower(filename), strings.ToLower(query)) || strings.Contains(strings.ToLower(title), strings.ToLower(query)) {
		score += 0.15
	}
	if score > 1 {
		score = 1
	}
	return score
}

// ExpandChunkNeighborhoods augments scored hits with parent summary and nearby leaf chunks.
func ExpandChunkNeighborhoods(db *sql.DB, hits []ScoredChunk, radius int) ([]ScoredChunk, error) {
	if len(hits) == 0 {
		return nil, nil
	}
	if radius < 0 {
		radius = 0
	}
	seen := make(map[int64]struct{}, len(hits)*3)
	out := make([]ScoredChunk, 0, len(hits)*3)
	add := func(chunk ScoredChunk) {
		if chunk.ChunkID == 0 {
			return
		}
		if _, ok := seen[chunk.ChunkID]; ok {
			return
		}
		seen[chunk.ChunkID] = struct{}{}
		out = append(out, chunk)
	}
	for _, hit := range hits {
		rec, err := GetChunkByID(db, hit.ChunkID)
		if err != nil || rec == nil {
			add(hit)
			continue
		}
		add(hit)
		neighbors, err := GetChunkNeighborhood(db, rec.ID, radius)
		if err != nil {
			continue
		}
		for _, c := range neighbors {
			if c.ID == rec.ID {
				continue
			}
			relScore := hit.Score - 0.001
			relContent := c.Content
			switch {
			case c.Role == "summary":
				relScore = hit.Score - 0.0004
				if strings.TrimSpace(c.Summary) != "" {
					relContent = c.Summary
				}
				if relContent != "" {
					relContent = "[Section summary] " + relContent
				}
			case c.Ord < rec.Ord:
				relContent = "[Previous context] " + relContent
			case c.Ord > rec.Ord:
				relContent = "[Next context] " + relContent
			}
			path := decodeHeadingPath(c.HeadingPath)
			if len(path) > 0 {
				relContent = fmt.Sprintf("%s\n[Heading path] %s", relContent, strings.Join(path, " > "))
			}
			add(ScoredChunk{
				ChunkID: c.ID,
				Content: relContent,
				Score:   relScore,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ChunkID < out[j].ChunkID
		}
		return out[i].Score > out[j].Score
	})
	return out, nil
}
