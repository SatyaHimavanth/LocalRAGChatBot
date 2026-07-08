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

// CreateCollection creates a new collection and returns its ID.
func CreateCollection(db *sql.DB, name string) (int64, error) {
	res, err := db.Exec("INSERT INTO collections (name, created_at) VALUES (?, ?)", name, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetCollections returns all collections with their document counts.
func GetCollections(db *sql.DB) ([]Collection, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.created_at, COALESCE(d.doc_count, 0)
		FROM collections c
		LEFT JOIN (
			SELECT collection_id, COUNT(*) AS doc_count
			FROM documents
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
