// internal/store/retrieval.go
package store

import (
    "database/sql"
    "sort"

    sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

type ScoredChunk struct {
    ChunkID int64
    Content string
    Score   float64
}

func vectorSearch(db *sql.DB, collectionID int64, queryEmbedding []float32, topK int) ([]ScoredChunk, error) {
    // Check sqlite-vec-go-bindings' current docs for the exact serialization
    // helper name — this call shape may differ slightly from what's below.
    blob, err := sqlite_vec.SerializeFloat32(queryEmbedding)
    if err != nil {
        return nil, err
    }
    rows, err := db.Query(`
        SELECT v.chunk_id, c.content, v.distance
        FROM chunks_vec v
        JOIN chunks c ON c.id = v.chunk_id
        WHERE v.embedding MATCH ? AND k = ?
          AND v.chunk_id IN (SELECT id FROM chunks WHERE collection_id = ?)
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
        WHERE chunks_fts MATCH ? AND c.collection_id = ?
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

// HybridSearch merges vector and keyword results with reciprocal rank fusion.
// k=60 is the constant from the original RRF paper — rarely worth tuning.
func HybridSearch(db *sql.DB, collectionID int64, query string, queryEmbedding []float32, topN int) ([]ScoredChunk, error) {
    const k = 60

    vecResults, err := vectorSearch(db, collectionID, queryEmbedding, 20)
    if err != nil {
        return nil, err
    }
    kwResults, err := keywordSearch(db, collectionID, query, 20)
    if err != nil {
        return nil, err
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