package store

import (
	"database/sql"
	"time"
)

type WorkspaceMemory struct {
	SessionID     int64  `json:"sessionId"`
	Summary       string `json:"summary"`
	Notes         string `json:"notes"`
	LastMessageID int64  `json:"lastMessageId"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}

func GetWorkspaceMemory(db *sql.DB, sessionID int64) (*WorkspaceMemory, error) {
	row := db.QueryRow(`SELECT session_id, summary, notes, last_message_id, created_at, updated_at FROM workspace_memory WHERE session_id = ?`, sessionID)
	var wm WorkspaceMemory
	if err := row.Scan(&wm.SessionID, &wm.Summary, &wm.Notes, &wm.LastMessageID, &wm.CreatedAt, &wm.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return &WorkspaceMemory{SessionID: sessionID}, nil
		}
		return nil, err
	}
	return &wm, nil
}

func UpsertWorkspaceMemory(db *sql.DB, sessionID int64, summary, notes string, lastMessageID int64) error {
	now := time.Now().Unix()
	_, err := db.Exec(`
        INSERT INTO workspace_memory (session_id, summary, notes, last_message_id, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(session_id) DO UPDATE SET
            summary = excluded.summary,
            notes = excluded.notes,
            last_message_id = excluded.last_message_id,
            updated_at = excluded.updated_at
    `, sessionID, summary, notes, lastMessageID, now, now)
	return err
}

func UpdateWorkspaceNotes(db *sql.DB, sessionID int64, notes string) error {
	now := time.Now().Unix()
	_, err := db.Exec(`
        INSERT INTO workspace_memory (session_id, summary, notes, last_message_id, created_at, updated_at)
        VALUES (?, '', ?, 0, ?, ?)
        ON CONFLICT(session_id) DO UPDATE SET
            notes = excluded.notes,
            updated_at = excluded.updated_at
    `, sessionID, notes, now, now)
	return err
}
