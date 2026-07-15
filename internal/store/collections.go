package store

import (
	"database/sql"
	"time"
)

type Collection struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	DocCount       int    `json:"docCount"`
	EmbeddingModel string `json:"embeddingModel"`
	EmbeddingDims  int    `json:"embeddingDims"`
	VectorBackend  string `json:"vectorBackend"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

// CreateCollection creates a new collection and returns its ID.
func CreateCollection(db *sql.DB, name string) (int64, error) {
	return CreateCollectionWithProfile(db, name, "", 0, "sqlite-vec")
}

// CreateCollectionWithProfile creates a collection with an explicit embedding profile.
func CreateCollectionWithProfile(db *sql.DB, name, embeddingModel string, embeddingDims int, vectorBackend string) (int64, error) {
	now := time.Now().Unix()
	if vectorBackend == "" {
		vectorBackend = "sqlite-vec"
	}
	res, err := db.Exec(`INSERT INTO collections (name, embedding_model, embedding_dims, vector_backend, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, name, embeddingModel, embeddingDims, vectorBackend, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetCollections returns all collections with their ready document counts.
func GetCollections(db *sql.DB) ([]Collection, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.created_at, c.updated_at, COALESCE(c.embedding_model,''), COALESCE(c.embedding_dims,0), COALESCE(c.vector_backend,'sqlite-vec'), COALESCE(d.doc_count, 0)
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
		if err := rows.Scan(&col.ID, &col.Name, &col.CreatedAt, &col.UpdatedAt, &col.EmbeddingModel, &col.EmbeddingDims, &col.VectorBackend, &col.DocCount); err != nil {
			return nil, err
		}
		collections = append(collections, col)
	}
	return collections, rows.Err()
}

// GetCollectionByID returns one collection row by id.
func GetCollectionByID(db *sql.DB, collectionID int64) (*Collection, error) {
	var col Collection
	err := db.QueryRow(`
		SELECT c.id, c.name, c.created_at, c.updated_at, COALESCE(c.embedding_model,''), COALESCE(c.embedding_dims,0), COALESCE(c.vector_backend,'sqlite-vec'), COALESCE(d.doc_count, 0)
		FROM collections c
		LEFT JOIN (
			SELECT collection_id, COUNT(*) AS doc_count
			FROM documents
			WHERE status = 'ready' OR status IS NULL OR status = ''
			GROUP BY collection_id
		) d ON d.collection_id = c.id
		WHERE c.id = ?
		LIMIT 1`, collectionID).Scan(&col.ID, &col.Name, &col.CreatedAt, &col.UpdatedAt, &col.EmbeddingModel, &col.EmbeddingDims, &col.VectorBackend, &col.DocCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &col, nil
}

// UpdateCollectionProfile stores the embedding/vector configuration for a collection.
func UpdateCollectionProfile(db *sql.DB, collectionID int64, embeddingModel string, embeddingDims int, vectorBackend string) error {
	if vectorBackend == "" {
		vectorBackend = "sqlite-vec"
	}
	_, err := db.Exec(`UPDATE collections SET embedding_model = ?, embedding_dims = ?, vector_backend = ?, updated_at = ? WHERE id = ?`, embeddingModel, embeddingDims, vectorBackend, time.Now().Unix(), collectionID)
	return err
}

// DeleteCollection removes a collection and cascades to all data.
// IMPORTANT: We do NOT manually delete from chunks_fts here.
// The AFTER DELETE trigger on the chunks table handles FTS cleanup.
func DeleteCollection(db *sql.DB, collectionID int64) error {
	// 1. Delete vector entries for all chunks in this collection
	if err := DeleteCollectionVectors(db, collectionID); err != nil {
		return err
	}

	// 2. Delete chunks — the AFTER DELETE trigger on chunks table
	//    automatically removes corresponding rows from chunks_fts.
	_, err := db.Exec("DELETE FROM chunks WHERE collection_id = ?", collectionID)
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
