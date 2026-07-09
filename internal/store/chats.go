package store

import (
	"database/sql"
	"time"
)

type ChatSession struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	CollectionID int64  `json:"collectionId"`
	CreatedAt    int64  `json:"createdAt"`
	Archived     bool   `json:"archived"`
	Pinned       bool   `json:"pinned"`
}

type ChatMessage struct {
	ID        int64  `json:"id"`
	SessionID int64  `json:"sessionId"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"createdAt"`
	Cancelled bool   `json:"cancelled"`
}

func CreateChatSession(db *sql.DB, title string, collectionID int64) (int64, error) {
	res, err := db.Exec("INSERT INTO chat_sessions (title, collection_id, created_at, archived, pinned) VALUES (?, ?, ?, 0, 0)",
		title, collectionID, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetChatSessions(db *sql.DB) ([]ChatSession, error) {
	rows, err := db.Query("SELECT id, title, collection_id, created_at, COALESCE(archived,0), COALESCE(pinned,0) FROM chat_sessions ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []ChatSession
	for rows.Next() {
		var s ChatSession
		if err := rows.Scan(&s.ID, &s.Title, &s.CollectionID, &s.CreatedAt, &s.Archived, &s.Pinned); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func UpdateChatSessionTitle(db *sql.DB, id int64, title string) error {
	_, err := db.Exec("UPDATE chat_sessions SET title = ? WHERE id = ?", title, id)
	return err
}

func ArchiveChatSession(db *sql.DB, id int64) error {
	_, err := db.Exec("UPDATE chat_sessions SET archived = 1 WHERE id = ?", id)
	return err
}

func UnarchiveChatSession(db *sql.DB, id int64) error {
	_, err := db.Exec("UPDATE chat_sessions SET archived = 0 WHERE id = ?", id)
	return err
}

func PinChatSession(db *sql.DB, id int64) error {
	_, err := db.Exec("UPDATE chat_sessions SET pinned = 1 WHERE id = ?", id)
	return err
}

func UnpinChatSession(db *sql.DB, id int64) error {
	_, err := db.Exec("UPDATE chat_sessions SET pinned = 0 WHERE id = ?", id)
	return err
}

func DeleteChatSession(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM chat_messages WHERE session_id = ?", id)
	if err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM chat_sessions WHERE id = ?", id)
	return err
}

func AddChatMessage(db *sql.DB, sessionID int64, role string, content string) (int64, error) {
	res, err := db.Exec("INSERT INTO chat_messages (session_id, role, content, created_at, cancelled) VALUES (?, ?, ?, ?, 0)",
		sessionID, role, content, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func AddCancelledChatMessage(db *sql.DB, sessionID int64, role string, content string) (int64, error) {
	res, err := db.Exec("INSERT INTO chat_messages (session_id, role, content, created_at, cancelled) VALUES (?, ?, ?, ?, 1)",
		sessionID, role, content, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func MarkMessageCancelled(db *sql.DB, msgID int64) error {
	_, err := db.Exec("UPDATE chat_messages SET cancelled = 1 WHERE id = ?", msgID)
	return err
}

func GetChatMessages(db *sql.DB, sessionID int64) ([]ChatMessage, error) {
	rows, err := db.Query("SELECT id, session_id, role, content, created_at, COALESCE(cancelled,0) FROM chat_messages WHERE session_id = ? ORDER BY created_at ASC", sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var m ChatMessage
		var cancelledInt int64
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt, &cancelledInt); err != nil {
			return nil, err
		}
		m.Cancelled = cancelledInt != 0
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
