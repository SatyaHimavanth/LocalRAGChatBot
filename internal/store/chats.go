package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type ChatSession struct {
	ID                   int64  `json:"id"`
	Title                string `json:"title"`
	CollectionID         int64  `json:"collectionId"`
	CurrentLeafMessageID int64  `json:"currentLeafMessageId"`
	CreatedAt            int64  `json:"createdAt"`
	Archived             bool   `json:"archived"`
	Pinned               bool   `json:"pinned"`
}

type ChatMessage struct {
	ID                int64  `json:"id"`
	SessionID         int64  `json:"sessionId"`
	Role              string `json:"role"`
	Content           string `json:"content"`
	CreatedAt         int64  `json:"createdAt"`
	Cancelled         bool   `json:"cancelled"`
	ParentMessageID   int64  `json:"parentMessageId"`
	AgentMetadataJSON string `json:"agentMetadataJson,omitempty"`
}

func CreateChatSession(db *sql.DB, title string, collectionID int64) (int64, error) {
	res, err := db.Exec("INSERT INTO chat_sessions (title, collection_id, current_leaf_message_id, archived, pinned, created_at) VALUES (?, ?, 0, 0, 0, ?)",
		title, collectionID, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetChatSessions(db *sql.DB) ([]ChatSession, error) {
	rows, err := db.Query("SELECT id, title, collection_id, COALESCE(current_leaf_message_id, 0), created_at, COALESCE(archived,0), COALESCE(pinned,0) FROM chat_sessions ORDER BY created_at DESC")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such column") {
			rows, legacyErr := db.Query("SELECT id, title, collection_id, 0, created_at, COALESCE(archived,0), COALESCE(pinned,0) FROM chat_sessions ORDER BY created_at DESC")
			if legacyErr != nil {
				return nil, legacyErr
			}
			defer rows.Close()
			return scanChatSessions(rows)
		}
		return nil, err
	}
	defer rows.Close()
	return scanChatSessions(rows)
}

func scanChatSessions(rows *sql.Rows) ([]ChatSession, error) {
	var sessions []ChatSession
	for rows.Next() {
		var s ChatSession
		if err := rows.Scan(&s.ID, &s.Title, &s.CollectionID, &s.CurrentLeafMessageID, &s.CreatedAt, &s.Archived, &s.Pinned); err != nil {
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
	if _, err := db.Exec("DELETE FROM chat_message_sources WHERE session_id = ?", id); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM chat_session_memory WHERE session_id = ?", id); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM chat_messages WHERE session_id = ?", id); err != nil {
		return err
	}
	_, err := db.Exec("DELETE FROM chat_sessions WHERE id = ?", id)
	return err
}

func getSessionLeafMessageID(db *sql.DB, sessionID int64) (int64, error) {
	var leaf int64
	err := db.QueryRow("SELECT COALESCE(current_leaf_message_id, 0) FROM chat_sessions WHERE id = ?", sessionID).Scan(&leaf)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such column") {
			return 0, nil
		}
		return 0, err
	}
	return leaf, nil
}

func setSessionLeafMessageID(tx *sql.Tx, sessionID, leafMessageID int64) error {
	_, err := tx.Exec("UPDATE chat_sessions SET current_leaf_message_id = ? WHERE id = ?", leafMessageID, sessionID)
	return err
}

func insertChatMessage(db *sql.DB, sessionID int64, role string, content string, parentMessageID int64, cancelled bool, agentMetadataJSON string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if parentMessageID == 0 {
		parentMessageID, err = getSessionLeafMessageID(db, sessionID)
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
	}

	res, err := tx.Exec(
		`INSERT INTO chat_messages (session_id, role, content, created_at, cancelled, parent_message_id, agent_metadata_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sessionID, role, content, time.Now().Unix(), boolToInt(cancelled), parentMessageID, agentMetadataJSON,
	)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	msgID, err := res.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := setSessionLeafMessageID(tx, sessionID, msgID); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return msgID, nil
}

func AddChatMessage(db *sql.DB, sessionID int64, role string, content string, parentMessageID int64, agentMetadataJSON string) (int64, error) {
	return insertChatMessage(db, sessionID, role, content, parentMessageID, false, agentMetadataJSON)
}

func AddCancelledChatMessage(db *sql.DB, sessionID int64, role string, content string, parentMessageID int64, agentMetadataJSON string) (int64, error) {
	return insertChatMessage(db, sessionID, role, content, parentMessageID, true, agentMetadataJSON)
}

func MarkMessageCancelled(db *sql.DB, msgID int64) error {
	_, err := db.Exec("UPDATE chat_messages SET cancelled = 1 WHERE id = ?", msgID)
	return err
}

func getChatMessageByID(db *sql.DB, sessionID, messageID int64) (*ChatMessage, error) {
	row := db.QueryRow(`SELECT id, session_id, role, content, created_at, COALESCE(cancelled,0), COALESCE(parent_message_id,0), COALESCE(agent_metadata_json,'')
		FROM chat_messages WHERE id = ? AND session_id = ?`, messageID, sessionID)
	var msg ChatMessage
	var cancelledInt int64
	if err := row.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.CreatedAt, &cancelledInt, &msg.ParentMessageID, &msg.AgentMetadataJSON); err != nil {
		return nil, err
	}
	msg.Cancelled = cancelledInt != 0
	return &msg, nil
}

func getLegacyChatMessages(db *sql.DB, sessionID int64) ([]ChatMessage, error) {
	rows, err := db.Query("SELECT id, session_id, role, content, created_at, COALESCE(cancelled,0) FROM chat_messages WHERE session_id = ? ORDER BY created_at ASC, id ASC", sessionID)
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

func GetChatMessages(db *sql.DB, sessionID int64) ([]ChatMessage, error) {
	leafID, err := getSessionLeafMessageID(db, sessionID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such column") {
			return getLegacyChatMessages(db, sessionID)
		}
		return nil, err
	}

	if leafID == 0 {
		var linkedCount int
		err := db.QueryRow("SELECT COUNT(*) FROM chat_messages WHERE session_id = ? AND COALESCE(parent_message_id, 0) <> 0", sessionID).Scan(&linkedCount)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "no such column") {
				return getLegacyChatMessages(db, sessionID)
			}
			return nil, err
		}
		if linkedCount == 0 {
			return getLegacyChatMessages(db, sessionID)
		}

		if err := db.QueryRow("SELECT id FROM chat_messages WHERE session_id = ? ORDER BY id DESC LIMIT 1", sessionID).Scan(&leafID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return []ChatMessage{}, nil
			}
			return nil, err
		}
	}

	var chain []ChatMessage
	seen := map[int64]bool{}
	current := leafID
	for current != 0 {
		if seen[current] {
			break
		}
		seen[current] = true
		msg, err := getChatMessageByID(db, sessionID, current)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			if strings.Contains(strings.ToLower(err.Error()), "no such column") {
				return getLegacyChatMessages(db, sessionID)
			}
			return nil, err
		}
		chain = append(chain, *msg)
		current = msg.ParentMessageID
	}

	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// GetChatBranchOptions returns the user-message variants that share the same
// parent as messageID. It is used to render the branch switcher below an edited
// prompt without exposing unrelated turns.
func GetChatBranchOptions(db *sql.DB, sessionID, messageID int64) ([]ChatMessage, error) {
	msg, err := getChatMessageByID(db, sessionID, messageID)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, session_id, role, content, created_at, COALESCE(cancelled,0), COALESCE(parent_message_id,0), COALESCE(agent_metadata_json,'')
		FROM chat_messages WHERE session_id = ? AND role = 'user' AND COALESCE(parent_message_id,0) = ? ORDER BY created_at ASC, id ASC`, sessionID, msg.ParentMessageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatMessage
	for rows.Next() {
		var option ChatMessage
		var cancelled int64
		if err := rows.Scan(&option.ID, &option.SessionID, &option.Role, &option.Content, &option.CreatedAt, &cancelled, &option.ParentMessageID, &option.AgentMetadataJSON); err != nil {
			return nil, err
		}
		option.Cancelled = cancelled != 0
		out = append(out, option)
	}
	return out, rows.Err()
}

// SelectChatBranch makes messageID's descendant leaf the active conversation
// branch. The recursive walk follows the newest child at each turn, which is
// the branch generated from the selected user prompt.
func SelectChatBranch(db *sql.DB, sessionID, messageID int64) error {
	if _, err := getChatMessageByID(db, sessionID, messageID); err != nil {
		return err
	}
	var leafID int64
	err := db.QueryRow(`WITH RECURSIVE branch(id, depth) AS (
		SELECT id, 0 FROM chat_messages WHERE id = ? AND session_id = ?
		UNION ALL
		SELECT child.id, branch.depth + 1
		FROM chat_messages child JOIN branch ON child.parent_message_id = branch.id
		WHERE child.session_id = ?
	)
	SELECT id FROM branch ORDER BY depth DESC, id DESC LIMIT 1`, messageID, sessionID, sessionID).Scan(&leafID)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE chat_sessions SET current_leaf_message_id = ? WHERE id = ?`, leafID, sessionID)
	return err
}

func boolToInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
