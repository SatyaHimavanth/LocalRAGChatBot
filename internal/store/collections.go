package store

import (
	"database/sql"
	"time"
)

type Collection struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	DocCount  int    `json:"docCount"`
	CreatedAt int64  `json:"createdAt"`
}

type CollectionInsight struct {
	ID                      int64  `json:"id"`
	Name                    string `json:"name"`
	CreatedAt               int64  `json:"createdAt"`
	TotalDocumentCount      int    `json:"totalDocumentCount"`
	ReadyDocumentCount      int    `json:"readyDocumentCount"`
	IncompleteDocumentCount int    `json:"incompleteDocumentCount"`
	ChunkCount              int    `json:"chunkCount"`
	ChatCount               int    `json:"chatCount"`
	LatestDocumentUpdatedAt int64  `json:"latestDocumentUpdatedAt"`
}

// CreateCollection creates a new collection and returns its ID.
func CreateCollection(db *sql.DB, name string) (int64, error) {
	res, err := db.Exec("INSERT INTO collections (name, created_at) VALUES (?, ?)", name, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetCollections returns all collections with their ready document counts.
func GetCollections(db *sql.DB) ([]Collection, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.created_at, COALESCE(d.doc_count, 0)
		FROM collections c
		LEFT JOIN (
			SELECT collection_id, COUNT(*) AS doc_count
			FROM documents
			WHERE status = 'ready' OR status IS NULL OR status = ''
			GROUP BY collection_id
		) d ON d.collection_id = c.id
		ORDER BY c.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []Collection
	for rows.Next() {
		var col Collection
		if err := rows.Scan(&col.ID, &col.Name, &col.CreatedAt, &col.DocCount); err != nil {
			return nil, err
		}
		collections = append(collections, col)
	}
	return collections, rows.Err()
}

// DeleteCollection removes a collection and cascades to all data.
// IMPORTANT: We do NOT manually delete from chunks_fts here.
// The AFTER DELETE trigger on the chunks table handles FTS cleanup.
func DeleteCollection(db *sql.DB, collectionID int64) error {
	// 1. Delete vector entries for all chunks in this collection
	_, err := db.Exec(`DELETE FROM chunks_vec WHERE chunk_id IN (SELECT id FROM chunks WHERE collection_id = ?)`, collectionID)
	if err != nil {
		return err
	}

	// 2. Delete chunks — the AFTER DELETE trigger on chunks table
	//    automatically removes corresponding rows from chunks_fts.
	_, err = db.Exec("DELETE FROM chunks WHERE collection_id = ?", collectionID)
	if err != nil {
		return err
	}

	// 3. Delete documents
	_, err = db.Exec("DELETE FROM documents WHERE collection_id = ?", collectionID)
	if err != nil {
		return err
	}

	// 4. Delete the collection itself
	_, err = db.Exec("DELETE FROM collections WHERE id = ?", collectionID)
	if err != nil {
		return err
	}
	return nil
}

// GetCollectionInsights returns richer per-collection analytics for UI dashboards.
func GetCollectionInsights(db *sql.DB) ([]CollectionInsight, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.created_at,
		       COALESCE(total.total_docs, 0),
		       COALESCE(ready.ready_docs, 0),
		       COALESCE(incomplete.incomplete_docs, 0),
		       COALESCE(chunks.chunk_count, 0),
		       COALESCE(chats.chat_count, 0),
		       COALESCE(latest.latest_updated_at, 0)
		FROM collections c
		LEFT JOIN (
			SELECT collection_id, COUNT(*) AS total_docs
			FROM documents
			GROUP BY collection_id
		) total ON total.collection_id = c.id
		LEFT JOIN (
			SELECT collection_id, COUNT(*) AS ready_docs
			FROM documents
			WHERE status = 'ready' OR status IS NULL OR status = ''
			GROUP BY collection_id
		) ready ON ready.collection_id = c.id
		LEFT JOIN (
			SELECT collection_id, COUNT(*) AS incomplete_docs
			FROM documents
			WHERE status IS NOT NULL AND status != '' AND status != 'ready'
			GROUP BY collection_id
		) incomplete ON incomplete.collection_id = c.id
		LEFT JOIN (
			SELECT collection_id, COUNT(*) AS chunk_count
			FROM chunks
			GROUP BY collection_id
		) chunks ON chunks.collection_id = c.id
		LEFT JOIN (
			SELECT collection_id, COUNT(*) AS chat_count
			FROM chat_sessions
			GROUP BY collection_id
		) chats ON chats.collection_id = c.id
		LEFT JOIN (
			SELECT collection_id, MAX(COALESCE(updated_at, created_at, 0)) AS latest_updated_at
			FROM documents
			GROUP BY collection_id
		) latest ON latest.collection_id = c.id
		ORDER BY c.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var insights []CollectionInsight
	for rows.Next() {
		var item CollectionInsight
		if err := rows.Scan(&item.ID, &item.Name, &item.CreatedAt, &item.TotalDocumentCount, &item.ReadyDocumentCount, &item.IncompleteDocumentCount, &item.ChunkCount, &item.ChatCount, &item.LatestDocumentUpdatedAt); err != nil {
			return nil, err
		}
		insights = append(insights, item)
	}
	return insights, rows.Err()
}
