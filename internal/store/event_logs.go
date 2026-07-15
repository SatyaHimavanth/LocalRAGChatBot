package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// EventLogEntry captures durable workspace activity for diagnostics.
type EventLogEntry struct {
	ID           int64  `json:"id"`
	EventKey     string `json:"eventKey"`
	Title        string `json:"title"`
	Detail       string `json:"detail"`
	Severity     string `json:"severity"`
	Scope        string `json:"scope"`
	CollectionID int64  `json:"collectionId"`
	ChatID       int64  `json:"chatId"`
	DocID        int64  `json:"docId"`
	BatchID      string `json:"batchId"`
	CreatedAt    int64  `json:"createdAt"`
}

// AddEventLog inserts one event log row.
func AddEventLog(db *sql.DB, eventKey, title, detail, severity, scope string, collectionID, chatID, docID int64, batchID string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	eventKey = strings.TrimSpace(eventKey)
	if eventKey == "" {
		return fmt.Errorf("event key cannot be empty")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = eventKey
	}
	detail = strings.TrimSpace(detail)
	severity = strings.ToLower(strings.TrimSpace(severity))
	if severity == "" {
		severity = "info"
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "workspace"
	}
	batchID = strings.TrimSpace(batchID)
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO event_log (event_key, title, detail, severity, scope, collection_id, chat_id, doc_id, batch_id, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, eventKey, title, detail, severity, scope, collectionID, chatID, docID, batchID, now)
	return err
}

// GetEventLogs returns the most recent event log rows.
func GetEventLogs(db *sql.DB, limit int) ([]EventLogEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.Query(`SELECT id, event_key, title, detail, severity, scope, COALESCE(collection_id,0), COALESCE(chat_id,0), COALESCE(doc_id,0), COALESCE(batch_id,''), created_at
        FROM event_log
        ORDER BY created_at DESC, id DESC
        LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]EventLogEntry, 0, limit)
	for rows.Next() {
		var e EventLogEntry
		if err := rows.Scan(&e.ID, &e.EventKey, &e.Title, &e.Detail, &e.Severity, &e.Scope, &e.CollectionID, &e.ChatID, &e.DocID, &e.BatchID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
