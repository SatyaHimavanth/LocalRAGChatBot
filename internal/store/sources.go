package store

import (
	"database/sql"
)

type SourceChunkRef struct {
	ID             int64   `json:"id"`
	MessageID      int64   `json:"messageId"`
	SessionID      int64   `json:"sessionId"`
	ChunkID        int64   `json:"chunkId"`
	Filename       string  `json:"filename"`
	CollectionID   int64   `json:"collectionId"`
	CollectionName string  `json:"collectionName"`
	Similarity     float64 `json:"similarity"`
	Content        string  `json:"content"`
	RefNumber      int     `json:"refNumber"`
}

// AddMessageSource stores a source chunk reference for an AI message.
func AddMessageSource(db *sql.DB, messageID, sessionID, chunkID int64, filename string, collectionID int64, collectionName string, similarity float64, content string, refNumber int) error {
	_, err := db.Exec(`INSERT INTO chat_message_sources (message_id, session_id, chunk_id, filename, collection_id, collection_name, similarity_score, content, ref_number)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, messageID, sessionID, chunkID, filename, collectionID, collectionName, similarity, content, refNumber)
	return err
}

// GetMessageSources returns all source chunks for a given message.
func GetMessageSources(db *sql.DB, messageID int64) ([]SourceChunkRef, error) {
	rows, err := db.Query(`SELECT id, message_id, session_id, chunk_id, filename, collection_id, collection_name, similarity_score, content, ref_number
		FROM chat_message_sources WHERE message_id = ? ORDER BY ref_number ASC`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []SourceChunkRef
	for rows.Next() {
		var s SourceChunkRef
		if err := rows.Scan(&s.ID, &s.MessageID, &s.SessionID, &s.ChunkID, &s.Filename, &s.CollectionID, &s.CollectionName, &s.Similarity, &s.Content, &s.RefNumber); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

// GetSessionSources returns all source chunks for all messages in a session.
func GetSessionSources(db *sql.DB, sessionID int64) (map[int64][]SourceChunkRef, error) {
	rows, err := db.Query(`SELECT id, message_id, session_id, chunk_id, filename, collection_id, collection_name, similarity_score, content, ref_number
		FROM chat_message_sources WHERE session_id = ? ORDER BY message_id ASC, ref_number ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]SourceChunkRef)
	for rows.Next() {
		var s SourceChunkRef
		if err := rows.Scan(&s.ID, &s.MessageID, &s.SessionID, &s.ChunkID, &s.Filename, &s.CollectionID, &s.CollectionName, &s.Similarity, &s.Content, &s.RefNumber); err != nil {
			return nil, err
		}
		result[s.MessageID] = append(result[s.MessageID], s)
	}
	return result, rows.Err()
}
