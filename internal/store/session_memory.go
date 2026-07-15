package store

import (
	"database/sql"
	"strings"
	"time"
)

type ChatSessionMemory struct {
	ID              int64  `json:"id"`
	SessionID       int64  `json:"sessionId"`
	CollectionID    int64  `json:"collectionId"`
	MemoryType      string `json:"memoryType"`
	Summary         string `json:"summary"`
	SourceMessageID int64  `json:"sourceMessageId"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}

func UpsertChatSessionMemory(db *sql.DB, sessionID, collectionID int64, memoryType, summary string, sourceMessageID int64) error {
	memoryType = strings.TrimSpace(memoryType)
	if memoryType == "" {
		memoryType = "rolling_summary"
	}
	summary = strings.TrimSpace(summary)
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO chat_session_memory (session_id, collection_id, memory_type, summary, source_message_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, memory_type) DO UPDATE SET
			collection_id = excluded.collection_id,
			summary = excluded.summary,
			source_message_id = excluded.source_message_id,
			updated_at = excluded.updated_at`, sessionID, collectionID, memoryType, summary, sourceMessageID, now, now)
	return err
}

func GetLatestChatSessionMemory(db *sql.DB, sessionID int64, memoryType string) (*ChatSessionMemory, error) {
	memoryType = strings.TrimSpace(memoryType)
	if memoryType == "" {
		memoryType = "rolling_summary"
	}
	var m ChatSessionMemory
	err := db.QueryRow(`SELECT id, session_id, collection_id, memory_type, summary, source_message_id, created_at, updated_at
		FROM chat_session_memory WHERE session_id = ? AND memory_type = ? LIMIT 1`, sessionID, memoryType).Scan(&m.ID, &m.SessionID, &m.CollectionID, &m.MemoryType, &m.Summary, &m.SourceMessageID, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func ListChatSessionMemory(db *sql.DB, sessionID int64) ([]ChatSessionMemory, error) {
	rows, err := db.Query(`SELECT id, session_id, collection_id, memory_type, summary, source_message_id, created_at, updated_at
		FROM chat_session_memory WHERE session_id = ? ORDER BY updated_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatSessionMemory
	for rows.Next() {
		var m ChatSessionMemory
		if err := rows.Scan(&m.ID, &m.SessionID, &m.CollectionID, &m.MemoryType, &m.Summary, &m.SourceMessageID, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func DeleteChatSessionMemory(db *sql.DB, sessionID int64) error {
	_, err := db.Exec(`DELETE FROM chat_session_memory WHERE session_id = ?`, sessionID)
	return err
}
