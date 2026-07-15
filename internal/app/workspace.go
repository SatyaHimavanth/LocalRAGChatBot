package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"changeme/internal/store"
)

type workspaceSnapshot struct {
	SessionID       int64    `json:"sessionId"`
	CollectionID    int64    `json:"collectionId"`
	CollectionName  string   `json:"collectionName"`
	Summary         string   `json:"summary"`
	Notes           string   `json:"notes"`
	LastMessageID   int64    `json:"lastMessageId"`
	UpdatedAt       int64    `json:"updatedAt"`
	RecentQuestions []string `json:"recentQuestions"`
	RecentDocuments []string `json:"recentDocuments"`
	LatestAssistant string   `json:"latestAssistant"`
	LatestSignal    string   `json:"latestSignal"`
}

func (s *ChatService) buildWorkspaceSnapshot(sessionID int64) (*workspaceSnapshot, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	chat, err := store.GetChatSessionByID(s.DB, sessionID)
	if err != nil {
		return nil, err
	}
	wm, err := store.GetWorkspaceMemory(s.DB, sessionID)
	if err != nil {
		return nil, err
	}

	var messages []store.ChatMessage
	if msgs, err := store.GetChatMessagesFlat(s.DB, sessionID); err == nil {
		messages = msgs
	}
	var questions []string
	var latestAssistant string
	var latestSignal string
	var lastMessageID int64
	for _, m := range messages {
		if m.ID > lastMessageID {
			lastMessageID = m.ID
		}
		role := strings.TrimSpace(m.Role)
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		if role == "user" {
			questions = append(questions, truncateForMemory(content, 140))
			continue
		}
		if role == "assistant" {
			latestAssistant = truncateForMemory(content, 260)
			latestSignal = buildSignalSummary(m.AgentMetadataJSON)
		}
	}
	if len(questions) > 6 {
		questions = questions[len(questions)-6:]
	}

	var docs []string
	if chat != nil && chat.CollectionID > 0 {
		if records, err := store.GetDocumentsByCollection(s.DB, chat.CollectionID); err == nil {
			for _, d := range records {
				name := strings.TrimSpace(d.Filename)
				if name == "" {
					continue
				}
				label := name
				if d.ChunkCount > 0 {
					label = fmt.Sprintf("%s (%d chunks)", name, d.ChunkCount)
				}
				if summary := strings.TrimSpace(d.Summary); summary != "" {
					label = fmt.Sprintf("%s — %s", label, truncateForMemory(summary, 120))
				}
				docs = append(docs, label)
			}
		}
	}
	if len(docs) > 5 {
		docs = docs[:5]
	}

	return &workspaceSnapshot{
		SessionID: sessionID,
		CollectionID: func() int64 {
			if chat != nil {
				return chat.CollectionID
			}
			return 0
		}(),
		CollectionName: func() string {
			if chat != nil {
				return s.getCollectionName(chat.CollectionID)
			}
			return ""
		}(),
		Summary: strings.TrimSpace(wm.Summary),
		Notes:   strings.TrimSpace(wm.Notes),
		LastMessageID: func() int64 {
			if wm.LastMessageID > 0 {
				return wm.LastMessageID
			}
			return lastMessageID
		}(),
		UpdatedAt:       wm.UpdatedAt,
		RecentQuestions: questions,
		RecentDocuments: docs,
		LatestAssistant: latestAssistant,
		LatestSignal:    latestSignal,
	}, nil
}

func (s *ChatService) workspaceSnapshotToMap(snap *workspaceSnapshot) map[string]any {
	if snap == nil {
		return map[string]any{}
	}
	return map[string]any{
		"sessionId":       snap.SessionID,
		"collectionId":    snap.CollectionID,
		"collectionName":  snap.CollectionName,
		"summary":         snap.Summary,
		"notes":           snap.Notes,
		"lastMessageId":   snap.LastMessageID,
		"updatedAt":       snap.UpdatedAt,
		"recentQuestions": snap.RecentQuestions,
		"recentDocuments": snap.RecentDocuments,
		"latestAssistant": snap.LatestAssistant,
		"latestSignal":    snap.LatestSignal,
		"hasSummary":      snap.Summary != "",
		"hasNotes":        snap.Notes != "",
	}
}

func (s *ChatService) GetWorkspaceMemory(sessionID int64) (map[string]any, error) {
	snap, err := s.buildWorkspaceSnapshot(sessionID)
	if err != nil {
		return nil, err
	}
	return s.workspaceSnapshotToMap(snap), nil
}

func (s *ChatService) UpdateWorkspaceNotes(sessionID int64, notes string) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	snap, err := s.buildWorkspaceSnapshot(sessionID)
	if err != nil {
		return err
	}
	return store.UpsertWorkspaceMemory(s.DB, sessionID, snap.Summary, strings.TrimSpace(notes), snap.LastMessageID)
}

func (s *ChatService) RefreshWorkspaceMemory(sessionID int64) (map[string]any, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	snap, err := s.buildWorkspaceSnapshot(sessionID)
	if err != nil {
		return nil, err
	}
	if err := store.UpsertWorkspaceMemory(s.DB, sessionID, buildWorkspaceSummary(snap), snap.Notes, snap.LastMessageID); err != nil {
		return nil, err
	}
	updated, err := s.buildWorkspaceSnapshot(sessionID)
	if err != nil {
		return nil, err
	}
	return s.workspaceSnapshotToMap(updated), nil
}

func (s *ChatService) refreshWorkspaceMemoryForTurn(sessionID int64) {
	if s.DB == nil {
		return
	}
	if snap, err := s.buildWorkspaceSnapshot(sessionID); err == nil {
		_ = store.UpsertWorkspaceMemory(s.DB, sessionID, buildWorkspaceSummary(snap), snap.Notes, snap.LastMessageID)
	}
}

func buildWorkspaceSummary(snap *workspaceSnapshot) string {
	if snap == nil {
		return ""
	}
	var lines []string
	if snap.CollectionName != "" {
		lines = append(lines, fmt.Sprintf("Collection: %s", snap.CollectionName))
	}
	if len(snap.RecentQuestions) > 0 {
		lines = append(lines, "Recent questions:")
		for i, q := range snap.RecentQuestions {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, q))
		}
	}
	if snap.LatestAssistant != "" {
		assistantLine := "Latest assistant reply: " + snap.LatestAssistant
		if snap.LatestSignal != "" {
			assistantLine += " (" + snap.LatestSignal + ")"
		}
		lines = append(lines, assistantLine)
	}
	if len(snap.RecentDocuments) > 0 {
		lines = append(lines, "Recent documents:")
		for _, d := range snap.RecentDocuments {
			lines = append(lines, "- "+d)
		}
	}
	if len(lines) == 0 {
		return "No workspace memory yet."
	}
	return strings.Join(lines, "\n")
}

func buildSignalSummary(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return ""
	}
	var parts []string
	if effort, _ := meta["evidenceEffort"].(string); effort != "" {
		parts = append(parts, "effort "+effort)
	}
	if src, ok := asInt(meta["sourceCount"]); ok && src > 0 {
		parts = append(parts, fmt.Sprintf("%d sources", src))
	}
	if score, ok := asFloat(meta["verificationScore"]); ok && score > 0 {
		parts = append(parts, fmt.Sprintf("verified %.0f%%", score*100))
	}
	if verdict, _ := meta["verificationVerdict"].(string); verdict != "" {
		parts = append(parts, verdict)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func truncateForMemory(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit-1] + "…"
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func (s *ChatService) recentUserQuestions(sessionID int64, limit int) ([]string, error) {
	msgs, err := store.GetChatMessagesFlat(s.DB, sessionID)
	if err != nil {
		return nil, err
	}
	var questions []string
	for _, m := range msgs {
		if strings.TrimSpace(m.Role) != "user" {
			continue
		}
		q := truncateForMemory(m.Content, 160)
		if q != "" {
			questions = append(questions, q)
		}
	}
	if len(questions) > limit {
		questions = questions[len(questions)-limit:]
	}
	return questions, nil
}

func sortStringsCaseInsensitive(values []string) []string {
	out := append([]string(nil), values...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}
