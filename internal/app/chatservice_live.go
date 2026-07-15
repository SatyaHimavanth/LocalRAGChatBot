package app

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"changeme/internal/agent"
	"changeme/internal/engine"
	"changeme/internal/store"

	llama "github.com/tcpipuk/llama-go"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// â”€â”€â”€ Constants â”€â”€â”€
const (
	// â”€â”€â”€ Context window safety limits â”€â”€â”€
	maxTotalChars   = 12000
	maxHistoryChars = 3000
	maxContextChars = 4000

	// Default context size if we can't detect RAM
	defaultContextSize = 4096

	// Max response tokens â€” generous enough for complete answers,
	// safe enough to prevent runaway generation / OOM
	maxResponseTokens = 1024
)

// getOptimalContextSize returns the best context window size based on
// available system RAM. Override with CHAT_CONTEXT_SIZE env variable.
func getOptimalContextSize() int {
	// 1. Check env var override
	if envStr := os.Getenv("CHAT_CONTEXT_SIZE"); envStr != "" {
		if val, err := strconv.Atoi(envStr); err == nil && val >= defaultContextSize {
			return val
		}
	}

	// 2. Try to detect total system RAM
	totalGB := getTotalRAMGB()

	// 3. Map RAM to safe context size
	//    Rule of thumb: ~512 tokens per GB of RAM beyond OS overhead (2GB)
	//    Clamped between 4096 (minimum useful) and 32768 (very large models)
	switch {
	case totalGB >= 32:
		return 32768
	case totalGB >= 16:
		return 16384
	case totalGB >= 8:
		return 8192
	default:
		return defaultContextSize
	}
}

// getTotalRAMGB returns total system RAM in GB, approximated.
// Returns 0 if detection fails.
func getTotalRAMGB() int {
	switch runtime.GOOS {
	case "linux":
		return getLinuxRAMGB()
	case "darwin":
		return getMacRAMGB()
	case "windows":
		return getWindowsRAMGB()
	default:
		return 0
	}
}

func getLinuxRAMGB() int {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Value is in kB
				if kb, err := strconv.Atoi(parts[1]); err == nil {
					return kb / 1024 / 1024 // kB â†’ GB
				}
			}
		}
	}
	return 0
}

func getMacRAMGB() int {
	return 0 // macOS users should set CHAT_CONTEXT_SIZE env var
}

func getWindowsRAMGB() int {
	return 0 // Windows users should set CHAT_CONTEXT_SIZE env var
}

type ChatService struct {
	Engine *engine.Engine
	DB     *sql.DB
	app    *application.App

	// agent drives persona, memory, and retrieval decisions.
	agent *agent.Agent

	// cancelFuncs tracks active streaming sessions for cancellation
	cancelFuncs map[int64]context.CancelFunc
	cancelMu    sync.Mutex

	// durable ingest worker state (see ingest_jobs.go)
	ingestOnce sync.Once
	ingestRT   *ingestRuntime
}

func (s *ChatService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.app = application.Get()
	s.cancelFuncs = make(map[int64]context.CancelFunc)
	s.agent = agent.New(
		agent.WithPersona(agent.DefaultPersona()),
		agent.WithTools(agent.DefaultTools()),
		agent.WithPlanner(agent.NewHeuristicPlanner()),
	)
	s.CleanupIncompleteOnStartup()
	if s.DB != nil {
		_ = store.EnsureDefaultExtensionHooks(s.DB)
	}
	return nil
}

//  Collection management

func (s *ChatService) CreateCollection(name string) (int64, error) {
	if s.DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("collection name cannot be empty")
	}
	// Check for duplicate collection names
	cols, _ := store.GetCollections(s.DB)
	for _, c := range cols {
		if strings.EqualFold(c.Name, name) {
			return 0, fmt.Errorf("a collection named '%s' already exists", name)
		}
	}
	embeddingModel, embeddingDims, vectorBackend := s.defaultCollectionProfile()
	id, err := store.CreateCollectionWithProfile(s.DB, name, embeddingModel, embeddingDims, vectorBackend)
	if err != nil {
		return 0, fmt.Errorf("creating collection: %w", err)
	}
	s.recordEvent("collection:create", "Collection created", fmt.Sprintf("%s (id %d)", name, id), "info", "collections", id, 0, 0, "")
	return id, nil
}

func (s *ChatService) DeleteCollection(collectionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := store.DeleteCollection(s.DB, collectionID); err != nil {
		return err
	}
	s.recordEvent("collection:delete", "Collection deleted", fmt.Sprintf("Collection %d removed", collectionID), "warn", "collections", collectionID, 0, 0, "")
	return nil
}

// UpdateCollectionProfile stores the embedding/vector profile for a collection.
func (s *ChatService) UpdateCollectionProfile(collectionID int64, embeddingModel string, embeddingDims int, vectorBackend string) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if collectionID <= 0 {
		return fmt.Errorf("invalid collection id")
	}
	if strings.TrimSpace(vectorBackend) == "" {
		vectorBackend = s.defaultCollectionVectorBackend()
	}
	if err := store.UpdateCollectionProfile(s.DB, collectionID, strings.TrimSpace(embeddingModel), embeddingDims, vectorBackend); err != nil {
		return err
	}
	s.recordEvent("collection:update_profile", "Collection profile updated", fmt.Sprintf("Collection %d · %s · %d dims · %s", collectionID, strings.TrimSpace(embeddingModel), embeddingDims, vectorBackend), "info", "collections", collectionID, 0, 0, "")
	return nil
}

// GetExtensionHooks returns the future integration hook registry.
func (s *ChatService) GetExtensionHooks() ([]store.ExtensionHook, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return store.GetExtensionHooks(s.DB)
}

// UpdateExtensionHook persists hook enablement/configuration changes.
func (s *ChatService) UpdateExtensionHook(hookKey string, enabled bool, configJSON string, state string) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := store.UpdateExtensionHook(s.DB, hookKey, enabled, configJSON, state); err != nil {
		return err
	}
	s.recordEvent("extension:update", "Extension hook updated", fmt.Sprintf("%s · enabled=%t · state=%s", hookKey, enabled, state), "info", "extensions", 0, 0, 0, "")
	return nil
}

// ResetExtensionHooks restores the canonical phase-8 hook descriptors.
func (s *ChatService) ResetExtensionHooks() error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := store.ResetExtensionHooks(s.DB); err != nil {
		return err
	}
	s.recordEvent("extension:reset", "Extension hooks reset", "Default phase-8 hook descriptors restored", "info", "extensions", 0, 0, 0, "")
	return nil
}

// GetEventLogs returns the recent workspace audit trail.
func (s *ChatService) GetEventLogs(limit int) ([]store.EventLogEntry, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return store.GetEventLogs(s.DB, limit)
}

func (s *ChatService) recordEvent(eventKey, title, detail, severity, scope string, collectionID, chatID, docID int64, batchID string) {
	if s.DB == nil {
		return
	}
	_ = store.AddEventLog(s.DB, eventKey, title, detail, severity, scope, collectionID, chatID, docID, batchID)
}

func (s *ChatService) DeleteDocument(docID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return store.DeleteDocument(s.DB, docID)
}

func (s *ChatService) GetDocumentsByCollection(collectionID int64) ([]store.Document, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return store.GetDocumentsByCollection(s.DB, collectionID)
}

func (s *ChatService) GetCollections() ([]store.Collection, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	cols, err := store.GetCollections(s.DB)
	if err != nil {
		return nil, fmt.Errorf("getting collections: %w", err)
	}
	if len(cols) == 0 {
		_, err := store.CreateCollectionWithProfile(s.DB, "Knowledge Base", s.defaultCollectionEmbeddingModel(), s.defaultCollectionEmbeddingDims(), s.defaultCollectionVectorBackend())
		if err != nil {
			return nil, fmt.Errorf("creating default collection: %w", err)
		}
		return store.GetCollections(s.DB)
	}
	return cols, nil
}

//  Chat session management

func (s *ChatService) CreateChat(title string, collectionID int64) (int64, error) {
	if s.DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	id, err := store.CreateChatSession(s.DB, title, collectionID)
	if err != nil {
		return 0, fmt.Errorf("creating chat: %w", err)
	}
	s.recordEvent("chat:create", "Chat created", fmt.Sprintf("%s (collection %d)", title, collectionID), "info", "chat", collectionID, id, 0, "")
	return id, nil
}

func (s *ChatService) defaultCollectionEmbeddingModel() string {
	if s.Engine != nil {
		if model := strings.TrimSpace(s.Engine.EmbeddingModelName()); model != "" {
			return model
		}
	}
	if env := strings.TrimSpace(os.Getenv("EMBED_MODEL_PATH")); env != "" {
		return filepath.Base(env)
	}
	return "local-embedding"
}

func (s *ChatService) defaultCollectionEmbeddingDims() int {
	if s.Engine != nil {
		if dims := s.Engine.EmbeddingDimensions(); dims > 0 {
			return dims
		}
	}
	return 0
}

func (s *ChatService) defaultCollectionVectorBackend() string {
	if s.Engine != nil {
		if backend := strings.TrimSpace(s.Engine.EmbeddingBackend()); backend != "" {
			return backend
		}
	}
	return "sqlite-vec"
}
func (s *ChatService) defaultCollectionProfile() (string, int, string) {
	return s.defaultCollectionEmbeddingModel(), s.defaultCollectionEmbeddingDims(), s.defaultCollectionVectorBackend()
}

func (s *ChatService) GetChats() ([]store.ChatSession, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return store.GetChatSessions(s.DB)
}

func (s *ChatService) GetChatMessages(sessionID int64) ([]store.ChatMessage, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return store.GetChatMessages(s.DB, sessionID)
}

func (s *ChatService) DeleteChat(sessionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := store.DeleteChatSession(s.DB, sessionID); err != nil {
		return err
	}
	s.recordEvent("chat:delete", "Chat deleted", fmt.Sprintf("Chat %d removed", sessionID), "warn", "chat", 0, sessionID, 0, "")
	return nil
}

func (s *ChatService) UpdateChatTitle(sessionID int64, title string) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := store.UpdateChatSessionTitle(s.DB, sessionID, title); err != nil {
		return err
	}
	s.recordEvent("chat:rename", "Chat renamed", fmt.Sprintf("Chat %d → %s", sessionID, title), "info", "chat", 0, sessionID, 0, "")
	return nil
}

func (s *ChatService) ArchiveChat(sessionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := store.ArchiveChatSession(s.DB, sessionID); err != nil {
		return err
	}
	s.recordEvent("chat:archive", "Chat archived", fmt.Sprintf("Chat %d archived", sessionID), "info", "chat", 0, sessionID, 0, "")
	return nil
}

func (s *ChatService) UnarchiveChat(sessionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := store.UnarchiveChatSession(s.DB, sessionID); err != nil {
		return err
	}
	s.recordEvent("chat:unarchive", "Chat restored", fmt.Sprintf("Chat %d restored", sessionID), "info", "chat", 0, sessionID, 0, "")
	return nil
}

func (s *ChatService) PinChat(sessionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := store.PinChatSession(s.DB, sessionID); err != nil {
		return err
	}
	s.recordEvent("chat:pin", "Chat pinned", fmt.Sprintf("Chat %d pinned", sessionID), "info", "chat", 0, sessionID, 0, "")
	return nil
}

func (s *ChatService) UnpinChat(sessionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := store.UnpinChatSession(s.DB, sessionID); err != nil {
		return err
	}
	s.recordEvent("chat:unpin", "Chat unpinned", fmt.Sprintf("Chat %d unpinned", sessionID), "info", "chat", 0, sessionID, 0, "")
	return nil
}

// CancelGeneration cancels an in-progress streaming generation for a session.
func (s *ChatService) CancelGeneration(sessionID int64) error {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	if cancel, ok := s.cancelFuncs[sessionID]; ok {
		cancel()
		// Don't delete from cancelFuncs or emit here — let the goroutine's
		// deferred cleanup and persistCancelledMessage handle persistence + emit.
	}
	return nil
}

//  Messaging

type AgentResponseMeta struct {
	Cancelled      bool     `json:"cancelled"`
	UsedRetrieval  bool     `json:"usedRetrieval"`
	UsedMemory     bool     `json:"usedMemory"`
	UsedDirect     bool     `json:"usedDirect"`
	SourceCount    int      `json:"sourceCount"`
	EvidenceCount  int      `json:"evidenceCount,omitempty"`
	Confidence     float64  `json:"confidence,omitempty"`
	Verified       bool     `json:"verified,omitempty"`
	Verification   string   `json:"verification,omitempty"`
	EvidenceGaps   []string `json:"evidenceGaps,omitempty"`
	Reason         string   `json:"reason,omitempty"`
	RetrievalQuery string   `json:"retrievalQuery,omitempty"`
	TopK           int      `json:"topK,omitempty"`
}

const branchPromptPrefix = "[[branch-parent:"

func encodeBranchPrompt(parentMessageID int64, prompt string) string {
	return fmt.Sprintf("[[branch-parent:%d]]\n%s", parentMessageID, prompt)
}

func extractBranchPrompt(prompt string) (string, int64) {
	trimmed := strings.TrimSpace(prompt)
	if !strings.HasPrefix(trimmed, branchPromptPrefix) {
		return prompt, 0
	}
	end := strings.Index(trimmed, "]]\n")
	if end == -1 {
		return prompt, 0
	}
	idStr := strings.TrimPrefix(trimmed[:end], branchPromptPrefix)
	parentID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return prompt, 0
	}
	return strings.TrimSpace(trimmed[end+3:]), parentID
}
func (m AgentResponseMeta) asMap(sessionID, msgID int64) map[string]any {
	return map[string]any{
		"sessionId":      sessionID,
		"msgId":          msgID,
		"cancelled":      m.Cancelled,
		"usedRetrieval":  m.UsedRetrieval,
		"usedMemory":     m.UsedMemory,
		"usedDirect":     m.UsedDirect,
		"sourceCount":    m.SourceCount,
		"evidenceCount":  m.EvidenceCount,
		"confidence":     m.Confidence,
		"verified":       m.Verified,
		"verification":   m.Verification,
		"evidenceGaps":   m.EvidenceGaps,
		"reason":         m.Reason,
		"retrievalQuery": m.RetrievalQuery,
		"topK":           m.TopK,
	}
}

func (m AgentResponseMeta) json() string {
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *ChatService) startConversationTurn(sessionID int64, collectionID int64, prompt string, parentMessageID int64) (int64, error) {
	if s.app == nil {
		s.app = application.Get()
	}
	if strings.TrimSpace(prompt) == "" {
		return 0, fmt.Errorf("prompt cannot be empty")
	}

	var userMsgID int64
	if s.DB != nil {
		var err error
		userMsgID, err = store.AddChatMessage(s.DB, sessionID, "user", prompt, parentMessageID, "")
		if err != nil {
			return 0, fmt.Errorf("persisting user message: %w", err)
		}
		s.emit("chat:message_saved", map[string]any{
			"sessionId":       sessionID,
			"msgId":           userMsgID,
			"role":            "user",
			"parentMessageId": parentMessageID,
		})
	}

	if s.Engine == nil {
		s.emit("chat:thinking", map[string]any{"sessionId": sessionID})
		s.emit("chat:token", map[string]any{
			"sessionId": sessionID,
			"token":     "Engine not initialized.",
		})
		s.emit("chat:done", map[string]any{"sessionId": sessionID, "msgId": int64(0), "cancelled": false, "usedRetrieval": false, "usedMemory": false, "usedDirect": true, "sourceCount": 0})
		return userMsgID, nil
	}

	go s.runConversationTurn(sessionID, collectionID, prompt, parentMessageID, userMsgID)
	return userMsgID, nil
}

func (s *ChatService) SendMessage(sessionID int64, collectionID int64, prompt string) error {
	cleanPrompt, parentMessageID := extractBranchPrompt(prompt)
	_, err := s.startConversationTurn(sessionID, collectionID, cleanPrompt, parentMessageID)
	return err
}

func (s *ChatService) emitAgentPlan(sessionID int64, plan agent.Plan) {
	intent := "general"
	switch {
	case plan.UseRetrieval:
		intent = "retrieval"
	case plan.UseMemory:
		intent = "memory"
	case !plan.UseDirect:
		intent = "conversation"
	}
	s.emit("chat:plan", map[string]any{
		"sessionId":      sessionID,
		"intent":         intent,
		"useRetrieval":   plan.UseRetrieval,
		"useMemory":      plan.UseMemory,
		"useDirect":      plan.UseDirect,
		"topK":           plan.TopK,
		"retrievalQuery": plan.RetrievalQuery,
		"reason":         plan.Reason,
	})
}

func (s *ChatService) persistCancelledMessage(sessionID int64, parentMessageID int64, meta AgentResponseMeta) {
	meta.Cancelled = true
	s.cancelMu.Lock()
	var cancelledMsgID int64 = -1
	if _, exists := s.cancelFuncs[sessionID]; exists {
		if s.DB != nil {
			if parentMessageID == 0 {
				if msgs, err := store.GetChatMessages(s.DB, sessionID); err == nil && len(msgs) > 0 {
					parentMessageID = msgs[len(msgs)-1].ID
				}
			}
			if id, err := store.AddCancelledChatMessage(s.DB, sessionID, "assistant", ".", parentMessageID, meta.json()); err == nil {
				cancelledMsgID = id
			}
		}
		delete(s.cancelFuncs, sessionID)
	}
	s.cancelMu.Unlock()
	s.emit("chat:done", meta.asMap(sessionID, cancelledMsgID))
}

func (s *ChatService) runConversationTurn(sessionID int64, collectionID int64, prompt string, parentMessageID int64, userMessageID int64) {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelMu.Lock()
	s.cancelFuncs[sessionID] = cancel
	s.cancelMu.Unlock()

	var msgID int64
	var doneEmitted bool
	meta := AgentResponseMeta{UsedDirect: true}
	defer func() {
		cancel()
		if doneEmitted {
			s.cancelMu.Lock()
			delete(s.cancelFuncs, sessionID)
			s.cancelMu.Unlock()
			return
		}
		if ctx.Err() != nil {
			meta.Cancelled = true
			s.persistCancelledMessage(sessionID, userMessageID, meta)
			return
		}
		s.cancelMu.Lock()
		delete(s.cancelFuncs, sessionID)
		s.cancelMu.Unlock()
	}()

	s.emit("chat:thinking", map[string]any{"sessionId": sessionID})
	s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "thinking", "label": "Thinking..."})

	if ctx.Err() != nil {
		meta.Cancelled = true
		s.persistCancelledMessage(sessionID, userMessageID, meta)
		return
	}
	time.Sleep(200 * time.Millisecond)
	if ctx.Err() != nil {
		meta.Cancelled = true
		s.persistCancelledMessage(sessionID, userMessageID, meta)
		return
	}

	var history []llama.ChatMessage
	if s.DB != nil {
		history = s.loadConversationHistory(sessionID)
	}

	ag := s.ensureAgent()
	memoryContext := s.loadConversationMemory(sessionID)
	plan := ag.Decide(agent.Request{
		Prompt:          prompt,
		History:         history,
		CollectionID:    collectionID,
		CollectionName:  s.getCollectionName(collectionID),
		HasDocuments:    s.DB != nil,
		WorkspaceMemory: memoryContext,
	})

	meta.UsedRetrieval = plan.UseRetrieval
	meta.UsedMemory = plan.UseMemory
	meta.UsedDirect = !plan.UseRetrieval && !plan.UseMemory
	meta.RetrievalQuery = plan.RetrievalQuery
	meta.TopK = plan.TopK
	meta.Reason = plan.Reason

	s.emitAgentPlan(sessionID, plan)
	s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "planning", "label": "Planning..."})
	time.Sleep(50 * time.Millisecond)
	if ctx.Err() != nil {
		meta.Cancelled = true
		s.persistCancelledMessage(sessionID, userMessageID, meta)
		return
	}

	var chunks []store.ScoredChunk
	var evidence EvidenceBundle
	if plan.UseRetrieval && s.DB != nil {
		query := strings.TrimSpace(plan.RetrievalQuery)
		if query == "" {
			query = prompt
		}
		meta.RetrievalQuery = query
		s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "searching", "label": "Searching documents..."})
		queryEmb, err := s.Engine.Embed(query)
		if err == nil {
			chunks, err = store.HybridSearch(s.DB, collectionID, query, queryEmb, plan.TopK)
			if err != nil {
				chunks = nil
			}
		}
		evidence = buildEvidenceBundle(s.DB, collectionID, s.getCollectionName(collectionID), query, prompt, chunks, history)
	}
	meta.SourceCount = evidence.SourceCount
	meta.EvidenceCount = evidence.SourceCount
	meta.Confidence = evidence.Confidence
	meta.Verified = evidence.Verified
	meta.Verification = evidence.Verification
	meta.EvidenceGaps = append([]string(nil), evidence.Gaps...)
	s.emit("chat:evidence", map[string]any{
		"sessionId":     sessionID,
		"query":         meta.RetrievalQuery,
		"sourceCount":   meta.SourceCount,
		"evidenceCount": meta.EvidenceCount,
		"confidence":    meta.Confidence,
		"verified":      meta.Verified,
		"verification":  meta.Verification,
		"evidenceGaps":  meta.EvidenceGaps,
	})
	if ctx.Err() != nil {
		meta.Cancelled = true
		s.persistCancelledMessage(sessionID, userMessageID, meta)
		return
	}

	if len(chunks) > 0 {
		s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "found", "label": fmt.Sprintf("Found %d relevant sections ✓", len(chunks))})
		time.Sleep(150 * time.Millisecond)
		if ctx.Err() != nil {
			meta.Cancelled = true
			s.persistCancelledMessage(sessionID, userMessageID, meta)
			return
		}
		s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "summarizing", "label": "Summarizing..."})
	} else {
		s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "thinking", "label": "Thinking..."})
	}
	time.Sleep(100 * time.Millisecond)
	if ctx.Err() != nil {
		meta.Cancelled = true
		s.persistCancelledMessage(sessionID, userMessageID, meta)
		return
	}

	s.emit("chat:thinking:done", map[string]any{"sessionId": sessionID})

	ctxSize := getOptimalContextSize()
	messages, sourceRefs := buildMessagesWithBudget(ag, plan, evidence, prompt, history, memoryContext, ctxSize, s.getCollectionName(collectionID))

	chatCtx, err := s.Engine.ChatModel.NewContext(llama.WithContext(ctxSize))
	if err != nil {
		s.emit("chat:token", map[string]any{"sessionId": sessionID, "token": fmt.Sprintf("\n[Error: %v]\n", err)})
		s.emit("chat:done", meta.asMap(sessionID, int64(0)))
		doneEmitted = true
		return
	}
	defer chatCtx.Close()

	maxTokens := maxResponseTokens
	deltas, errs := chatCtx.ChatStream(ctx, messages, llama.ChatOptions{MaxTokens: &maxTokens})

	var fullResponse strings.Builder
	for {
		select {
		case delta, ok := <-deltas:
			if !ok {
				if s.DB != nil && fullResponse.Len() > 0 {
					assistantMetaJSON := meta.json()
					msgID, err = store.AddChatMessage(s.DB, sessionID, "assistant", fullResponse.String(), userMessageID, assistantMetaJSON)
					if err == nil {
						if len(sourceRefs) > 0 {
							colName := s.getCollectionName(collectionID)
							for _, sr := range sourceRefs {
								filename := s.getChunkFilename(sr.chunk.ChunkID)
								score := normalizeMatchScore(sr.chunk.Score)
								store.AddMessageSource(s.DB, msgID, sessionID, sr.chunk.ChunkID, filename, collectionID, colName, score, sr.chunk.Content, sr.refNum)
							}
							sourceMap := make([]map[string]any, 0, len(sourceRefs))
							for _, sr := range sourceRefs {
								fn := s.getChunkFilename(sr.chunk.ChunkID)
								score := normalizeMatchScore(sr.chunk.Score)
								sourceMap = append(sourceMap, map[string]any{
									"refNumber":      sr.refNum,
									"chunkId":        sr.chunk.ChunkID,
									"content":        sr.chunk.Content,
									"filename":       fn,
									"collectionId":   collectionID,
									"collectionName": colName,
									"similarity":     score,
								})
							}
							s.emit("chat:sources", map[string]any{"sessionId": sessionID, "msgId": msgID, "sources": sourceMap})
						}
						s.persistConversationMemory(sessionID, collectionID, userMessageID, msgID, prompt, fullResponse.String(), evidence, plan, history)
					}
				}
				meta.Cancelled = false
				s.emit("chat:done", meta.asMap(sessionID, msgID))
				doneEmitted = true
				return
			}
			fullResponse.WriteString(delta.Content)
			s.emit("chat:token", map[string]any{"sessionId": sessionID, "token": delta.Content})
		case err := <-errs:
			if err != nil {
				s.emit("chat:token", map[string]any{"sessionId": sessionID, "token": fmt.Sprintf("\n[Error: %v]\n", err)})
				s.emit("chat:done", meta.asMap(sessionID, int64(0)))
				doneEmitted = true
				return
			}
		}
	}
}

//  File Upload & Parsing

type FileUploadResult struct {
	Filename        string `json:"filename"`
	Status          string `json:"status"` // "success", "duplicate", "error", "replaced"
	Message         string `json:"message"`
	DocID           int64  `json:"docId,omitempty"`
	CollectionName  string `json:"collectionName,omitempty"`
	WordCount       int    `json:"wordCount,omitempty"`
	ChunkCount      int    `json:"chunkCount,omitempty"`
	ExistingDocID   int64  `json:"existingDocId,omitempty"`
	ExistingCreated int64  `json:"existingCreated,omitempty"`
}

// UploadFile stages one file then embeds it via the durable two-phase pipeline.
// Prefer StartIngestBatch for multi-file uploads (extract-all-then-embed).
func (s *ChatService) UploadFile(filename string, base64Data string, collectionID int64, replace bool) (*FileUploadResult, error) {
	batch, err := s.StartIngestBatch(collectionID, []IngestFilePayload{{
		Filename:   filename,
		Base64Data: base64Data,
		Replace:    replace,
	}})
	if err != nil {
		return &FileUploadResult{Filename: filename, Status: "error", Message: err.Error()}, nil
	}
	if len(batch.Items) == 0 {
		return &FileUploadResult{Filename: filename, Status: "error", Message: "No result"}, nil
	}
	item := batch.Items[0]
	colName := s.getCollectionName(collectionID)
	switch item.Status {
	case "duplicate":
		return &FileUploadResult{
			Filename: filename, Status: "duplicate", Message: item.Message,
			DocID: item.DocID, ExistingDocID: item.DocID, CollectionName: colName,
		}, nil
	case "error":
		return &FileUploadResult{Filename: filename, Status: "error", Message: item.Message}, nil
	case "replaced":
		status := "replaced"
		if batch.Completed > 0 {
			return &FileUploadResult{
				Filename: filename, Status: status, Message: "File replaced and re-ingested successfully",
				DocID: item.DocID, CollectionName: colName, ChunkCount: 0,
			}, nil
		}
		if batch.Cancelled {
			return &FileUploadResult{
				Filename: filename, Status: "error",
				Message: "Interrupted during embedding — resume from incomplete jobs",
				DocID:   item.DocID, CollectionName: colName,
			}, nil
		}
		if batch.Failed > 0 {
			return &FileUploadResult{Filename: filename, Status: "error", Message: "Embedding failed", DocID: item.DocID}, nil
		}
		return &FileUploadResult{Filename: filename, Status: status, Message: item.Message, DocID: item.DocID, CollectionName: colName}, nil
	case "staged":
		if batch.Completed > 0 {
			return &FileUploadResult{
				Filename: filename, Status: "success", Message: "File ingested successfully",
				DocID: item.DocID, CollectionName: colName,
			}, nil
		}
		if batch.Cancelled {
			return &FileUploadResult{
				Filename: filename, Status: "error",
				Message: "Interrupted during embedding — resume from incomplete jobs",
				DocID:   item.DocID, CollectionName: colName,
			}, nil
		}
		if batch.Failed > 0 {
			return &FileUploadResult{Filename: filename, Status: "error", Message: "Embedding failed", DocID: item.DocID}, nil
		}
		return &FileUploadResult{Filename: filename, Status: "success", Message: item.Message, DocID: item.DocID, CollectionName: colName}, nil
	default:
		return &FileUploadResult{Filename: filename, Status: "error", Message: item.Message}, nil
	}
}

// GetDocumentContent returns the full extracted text content of a document.
func (s *ChatService) GetDocumentContent(docID int64) (string, error) {
	if s.DB == nil {
		return "", fmt.Errorf("database not initialized")
	}
	doc, err := store.GetDocumentByID(s.DB, docID)
	if err != nil {
		return "", fmt.Errorf("getting document: %w", err)
	}
	if doc == nil {
		return "", fmt.Errorf("document not found")
	}
	return doc.Content, nil
}

// GetDocumentChunks returns all stored chunks for a document ordered by ord.
func (s *ChatService) GetDocumentChunks(docID int64) ([]store.ChunkRecord, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	doc, err := store.GetDocumentByID(s.DB, docID)
	if err != nil {
		return nil, fmt.Errorf("getting document: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}
	return store.GetChunksByDocument(s.DB, docID)
}

// CheckFileHash checks if a file hash already exists in a collection.
func (s *ChatService) CheckFileHash(hash string, collectionID int64) (*FileUploadResult, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	existing, err := store.GetDocumentByHash(s.DB, hash, collectionID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}
	createdTime := time.Unix(existing.CreatedAt, 0).Format("Jan 2, 2006 at 15:04")
	return &FileUploadResult{
		Filename: existing.Filename, Status: "duplicate",
		Message:       fmt.Sprintf("Already uploaded %s", createdTime),
		ExistingDocID: existing.ID, ExistingCreated: existing.CreatedAt,
	}, nil
}

// IngestFile stages pasted text then embeds via the durable two-phase pipeline.
func (s *ChatService) IngestFile(collectionID int64, filename string, fileContent string) error {
	if s.Engine == nil || s.DB == nil {
		return fmt.Errorf("engine or database not initialized")
	}

	cols, _ := store.GetCollections(s.DB)
	colExists := false
	for _, c := range cols {
		if c.ID == collectionID {
			colExists = true
			break
		}
	}
	if !colExists {
		id, err := store.CreateCollection(s.DB, "Knowledge Base")
		if err != nil {
			return fmt.Errorf("creating default collection: %w", err)
		}
		collectionID = id
	}

	batch, err := s.StartIngestBatch(collectionID, []IngestFilePayload{{
		Filename:    filename,
		TextContent: fileContent,
	}})
	if err != nil {
		return err
	}
	if batch.Cancelled {
		return fmt.Errorf("ingest interrupted — resume from incomplete jobs")
	}
	if len(batch.Items) > 0 && batch.Items[0].Status == "error" {
		return fmt.Errorf("%s", batch.Items[0].Message)
	}
	if len(batch.Items) > 0 && batch.Items[0].Status == "duplicate" {
		return fmt.Errorf("%s", batch.Items[0].Message)
	}
	if batch.Failed > 0 && batch.Completed == 0 {
		return fmt.Errorf("embedding failed")
	}
	return nil
}

//  Universal Search

type SearchResult struct {
	Content        string  `json:"content"`
	Score          float64 `json:"score"`
	RRFScore       float64 `json:"-"`
	SearchType     string  `json:"searchType"`
	CollectionID   int64   `json:"collectionId"`
	CollectionName string  `json:"collectionName"`
	Filename       string  `json:"filename"`
	ChunkID        int64   `json:"chunkId"`
}

// GetChunkContext returns the target chunk plus its parent and neighbor context.
func (s *ChatService) GetChunkContext(chunkID int64, radius int) ([]store.ChunkRecord, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if radius < 0 {
		radius = 0
	}
	return store.GetChunkNeighborhood(s.DB, chunkID, radius)
}

// SearchMetadata performs a document-level search over filenames, titles,
// summaries and collection names.
func (s *ChatService) SearchMetadata(query string, collectionID int64, topK int) ([]SearchResult, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	hits, err := store.MetadataSearch(s.DB, collectionID, query, clampSearchLimit(topK))
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(hits))
	for _, hit := range hits {
		content := hit.Snippet
		if content == "" {
			content = strings.TrimSpace(hit.Title)
		}
		if content == "" {
			content = hit.Filename
		}
		out = append(out, SearchResult{
			Content:        content,
			Score:          hit.Score,
			SearchType:     "metadata",
			CollectionID:   hit.CollectionID,
			CollectionName: hit.CollectionName,
			Filename:       hit.Filename,
			ChunkID:        hit.ChunkID,
		})
	}
	return out, nil
}

// SearchWorkspace expands retrieval to include the current session's cited source
// chunks so the UI can search a working context, not just documents.
func (s *ChatService) SearchWorkspace(query string, collectionID, sessionID int64, topK int) ([]SearchResult, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	base, err := s.Search(query, collectionID, topK)
	if err != nil {
		return nil, err
	}
	workspace := append([]SearchResult(nil), base...)
	if sessionID > 0 {
		more, err := s.searchSessionSources(query, collectionID, sessionID, topK)
		if err != nil {
			return nil, err
		}
		workspace = mergeSearchResultsByRRF(workspace, more)
	}
	limit := clampSearchLimit(topK)
	if len(workspace) > limit {
		workspace = workspace[:limit]
	}
	return workspace, nil
}

func (s *ChatService) Search(query string, collectionID int64, topK int) ([]SearchResult, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	var queryEmb []float32
	if s.Engine != nil {
		var err error
		queryEmb, err = s.Engine.Embed(query)
		if err != nil {
			queryEmb = nil
		}
	}

	var results []SearchResult

	limit := clampSearchLimit(topK)
	if collectionID > 0 {
		colResults, err := s.searchCollection(query, queryEmb, collectionID, limit)
		if err != nil {
			return nil, err
		}
		return colResults, nil
	}

	cols, err := store.GetCollections(s.DB)
	if err != nil {
		return nil, fmt.Errorf("getting collections: %w", err)
	}

	for _, col := range cols {
		colResults, err := s.searchCollection(query, queryEmb, col.ID, limit)
		if err != nil {
			continue
		}
		results = append(results, colResults...)
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (s *ChatService) searchCollection(query string, queryEmb []float32, collectionID int64, topK int) ([]SearchResult, error) {
	kwResults, err := store.KeywordSearch(s.DB, collectionID, query, topK)
	if err != nil {
		kwResults, err = store.FallbackSearch(s.DB, collectionID, query, topK)
		if err != nil {
			return nil, err
		}
	}

	// Also try vector search if we have an embedding
	var vecResults []store.ScoredChunk
	if queryEmb != nil {
		vecResults, err = store.VectorSearch(s.DB, collectionID, queryEmb, topK)
		if err != nil {
			vecResults = nil
		}
	}

	// Merge via RRF
	colName := s.getCollectionName(collectionID)
	const k = 60
	type scored struct {
		sr  SearchResult
		rrf float64
	}
	merged := make(map[int64]*scored)

	for rank, r := range kwResults {
		fn := s.getChunkFilename(r.ChunkID)
		ns := normalizeBM25Score(r.Score)
		merged[r.ChunkID] = &scored{
			sr: SearchResult{
				Content: r.Content, Score: ns, SearchType: "keyword",
				CollectionID:   collectionID,
				CollectionName: colName, Filename: fn, ChunkID: r.ChunkID,
			},
			rrf: 1.0 / float64(k+rank+1),
		}
	}
	for rank, r := range vecResults {
		fn := s.getChunkFilename(r.ChunkID)
		vecNorm := normalizeDistanceScore(r.Score)
		if existing, ok := merged[r.ChunkID]; ok {
			existing.rrf += 1.0 / float64(k+rank+1)
			existing.sr.SearchType = "hybrid"
			if vecNorm > existing.sr.Score {
				existing.sr.Score = vecNorm
			}
		} else {
			merged[r.ChunkID] = &scored{
				sr: SearchResult{
					Content: r.Content, Score: vecNorm, SearchType: "vector",
					CollectionID:   collectionID,
					CollectionName: colName, Filename: fn, ChunkID: r.ChunkID,
				},
				rrf: 1.0 / float64(k+rank+1),
			}
		}
	}

	out := make([]SearchResult, 0, len(merged))
	for _, sc := range merged {
		out = append(out, sc.sr)
	}
	sort.Slice(out, func(i, j int) bool { return merged[out[i].ChunkID].rrf > merged[out[j].ChunkID].rrf })
	if len(out) > topK {
		out = out[:topK]
	}
	return out, nil
}

func (s *ChatService) searchSessionSources(query string, collectionID, sessionID int64, topK int) ([]SearchResult, error) {
	if s.DB == nil || sessionID <= 0 {
		return nil, nil
	}
	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil, nil
	}
	all, err := store.GetSessionSources(s.DB, sessionID)
	if err != nil {
		return nil, err
	}
	type scoredSource struct {
		result SearchResult
		score  float64
	}
	var scored []scoredSource
	for _, refs := range all {
		for _, ref := range refs {
			if collectionID > 0 && ref.CollectionID != collectionID {
				continue
			}
			score := workspaceSourceScore(query, terms, ref.Filename, ref.CollectionName, ref.Content, ref.Similarity)
			if score <= 0 {
				continue
			}
			scored = append(scored, scoredSource{result: SearchResult{
				Content:        ref.Content,
				Score:          score,
				SearchType:     "workspace",
				CollectionID:   ref.CollectionID,
				CollectionName: ref.CollectionName,
				Filename:       ref.Filename,
				ChunkID:        ref.ChunkID,
			}, score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].result.ChunkID < scored[j].result.ChunkID
		}
		return scored[i].score > scored[j].score
	})
	limit := clampSearchLimit(topK)
	if len(scored) > limit {
		scored = scored[:limit]
	}
	out := make([]SearchResult, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.result)
	}
	return out, nil
}

func clampSearchLimit(topK int) int {
	if topK <= 0 {
		return 20
	}
	if topK > 50 {
		return 50
	}
	return topK
}

func mergeSearchResultsByRRF(base, extra []SearchResult) []SearchResult {
	if len(extra) == 0 {
		return base
	}
	const k = 60
	type scored struct {
		result SearchResult
		rrf    float64
	}
	merged := make(map[string]*scored)
	key := func(r SearchResult) string {
		return fmt.Sprintf("%d:%d", r.CollectionID, r.ChunkID)
	}
	for rank, r := range base {
		merged[key(r)] = &scored{result: r, rrf: 1.0 / float64(k+rank+1)}
	}
	for rank, r := range extra {
		k2 := key(r)
		if existing, ok := merged[k2]; ok {
			existing.rrf += 1.0 / float64(k+rank+1)
			if r.Score > existing.result.Score {
				existing.result.Score = r.Score
			}
			if existing.result.SearchType == "" || existing.result.SearchType == r.SearchType {
				existing.result.SearchType = r.SearchType
			} else if r.SearchType == "workspace" || existing.result.SearchType == "workspace" {
				existing.result.SearchType = "workspace"
			} else if r.SearchType == "metadata" || existing.result.SearchType == "metadata" {
				existing.result.SearchType = "metadata"
			} else {
				existing.result.SearchType = "hybrid"
			}
			existing.result.Content = mergeSearchContent(existing.result.Content, r.Content)
		} else {
			merged[k2] = &scored{result: r, rrf: 1.0 / float64(k+rank+1)}
		}
	}
	out := make([]SearchResult, 0, len(merged))
	for _, item := range merged {
		out = append(out, item.result)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if merged[key(out[i])].rrf == merged[key(out[j])].rrf {
			return out[i].ChunkID < out[j].ChunkID
		}
		return merged[key(out[i])].rrf > merged[key(out[j])].rrf
	})
	return out
}

func mergeSearchContent(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" || strings.Contains(a, b) {
		return a
	}
	if strings.Contains(b, a) {
		return b
	}
	return a + "\n" + b
}

func searchTerms(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	terms := strings.FieldsFunc(query, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		default:
			return true
		}
	})
	if len(terms) == 0 {
		return nil
	}
	uniq := make([]string, 0, len(terms))
	seen := map[string]struct{}{}
	for _, t := range terms {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		uniq = append(uniq, t)
	}
	return uniq
}

func workspaceSourceScore(query string, terms []string, filename, collectionName, content string, similarity float64) float64 {
	if len(terms) == 0 {
		return 0
	}
	f := strings.ToLower(filename)
	c := strings.ToLower(collectionName)
	ct := strings.ToLower(content)
	var score float64
	for _, term := range terms {
		if strings.Contains(f, term) {
			score += 0.35
		}
		if strings.Contains(c, term) {
			score += 0.15
		}
		if strings.Contains(ct, term) {
			score += 0.25
		}
	}
	score = score/float64(len(terms)) + normalizeMatchScore(similarity)*0.15
	if strings.Contains(f, strings.ToLower(query)) {
		score += 0.2
	}
	if score > 1 {
		score = 1
	}
	return score
}

func (s *ChatService) ensureAgent() *agent.Agent {
	if s.agent == nil {
		s.agent = agent.New(
			agent.WithPersona(agent.DefaultPersona()),
			agent.WithTools(agent.DefaultTools()),
			agent.WithPlanner(agent.NewHeuristicPlanner()),
		)
	}
	return s.agent
}

func normalizeDistanceScore(score float64) float64 {
	if score < 0 {
		score = math.Abs(score)
	}
	if score <= 0 {
		return 1
	}
	return math.Max(0, math.Min(1, 1.0/(1.0+score)))
}

func normalizeBM25Score(score float64) float64 {
	if score >= 0 {
		return 0
	}
	abs := math.Abs(score)
	return abs / (1.0 + abs)
}

func normalizeMatchScore(score float64) float64 {
	if score < 0 {
		score = math.Abs(score)
	}
	const bestSingleRankRRF = 1.0 / 61.0
	if score <= 0 {
		return 0
	}
	return math.Max(0, math.Min(1, score/bestSingleRankRRF))
}
func (s *ChatService) getChunkFilename(chunkID int64) string {
	if s.DB == nil {
		return "unknown"
	}
	var filename string
	err := s.DB.QueryRow(`
		SELECT COALESCE(d.filename, 'unknown') FROM documents d
		JOIN chunks c ON c.document_id = d.id
		WHERE c.id = ?
	`, chunkID).Scan(&filename)
	if err != nil {
		return "unknown"
	}
	return filename
}

func (s *ChatService) getCollectionName(collectionID int64) string {
	if s.DB == nil {
		return "Unknown"
	}
	var name string
	err := s.DB.QueryRow("SELECT name FROM collections WHERE id = ?", collectionID).Scan(&name)
	if err != nil {
		return fmt.Sprintf("Collection #%d", collectionID)
	}
	return name
}

// GetMessageSources returns source chunks for a given message.
func (s *ChatService) GetMessageSources(msgID int64) ([]store.SourceChunkRef, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return store.GetMessageSources(s.DB, msgID)
}

// GetSessionSources returns source chunks all messages in a session.
func (s *ChatService) GetSessionSources(sessionID int64) ([]store.SourceChunkRef, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	all, err := store.GetSessionSources(s.DB, sessionID)
	if err != nil {
		return nil, err
	}
	// Flatten the map into a single slice
	var result []store.SourceChunkRef
	for _, sources := range all {
		result = append(result, sources...)
	}
	return result, nil
}

// GetSessionMemory returns persisted rolling conversation memory rows for a session.
func (s *ChatService) GetSessionMemory(sessionID int64) ([]store.ChatSessionMemory, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return store.ListChatSessionMemory(s.DB, sessionID)
}

//  Helpers

func (s *ChatService) emit(name string, data ...any) {
	if s.app != nil {
		s.app.Event.Emit(name, data...)
	}
}

func (s *ChatService) loadConversationHistory(sessionID int64) []llama.ChatMessage {
	if s.DB == nil {
		return nil
	}
	msgs, err := store.GetChatMessages(s.DB, sessionID)
	if err != nil || len(msgs) == 0 {
		return nil
	}

	// Build chat history, limiting to last ~10 messages (5 exchanges)
	// to stay within the 4096-token context window
	start := 0
	if len(msgs) > 10 {
		start = len(msgs) - 10
	}

	var history []llama.ChatMessage
	totalChars := 0

	for i := start; i < len(msgs); i++ {
		m := msgs[i]
		role := "user"
		if m.Role == "assistant" {
			role = "assistant"
		}
		// Skip the last user message (it's the current prompt, added by buildMessages)
		if i == len(msgs)-1 {
			continue
		}
		if totalChars+len(m.Content) > maxHistoryChars {
			break
		}
		history = append(history, llama.ChatMessage{Role: role, Content: m.Content})
		totalChars += len(m.Content)
	}
	return history
}

func (s *ChatService) loadConversationMemory(sessionID int64) string {
	if s.DB == nil || sessionID <= 0 {
		return ""
	}
	mem, err := store.GetLatestChatSessionMemory(s.DB, sessionID, "rolling_summary")
	if err != nil || mem == nil {
		return ""
	}
	return strings.TrimSpace(mem.Summary)
}

func (s *ChatService) persistConversationMemory(sessionID, collectionID, sourceMessageID, messageID int64, prompt, response string, evidence EvidenceBundle, plan agent.Plan, history []llama.ChatMessage) {
	if s.DB == nil || sessionID <= 0 {
		return
	}
	previous := s.loadConversationMemory(sessionID)
	summary := buildConversationMemorySummary(previous, prompt, response, evidence, plan, history)
	if strings.TrimSpace(summary) == "" {
		return
	}
	if err := store.UpsertChatSessionMemory(s.DB, sessionID, collectionID, "rolling_summary", summary, sourceMessageID); err != nil {
		return
	}
	if messageID > 0 {
		_ = store.UpsertChatSessionMemory(s.DB, sessionID, collectionID, "last_turn", summarizeConversationTurn(prompt, response, evidence, plan, history), messageID)
	}
}

func buildConversationMemorySummary(previous, prompt, response string, evidence EvidenceBundle, plan agent.Plan, history []llama.ChatMessage) string {
	lines := uniqueMemoryLines(splitMemoryLines(previous))
	turn := summarizeConversationTurn(prompt, response, evidence, plan, history)
	if turn != "" {
		lines = append(lines, turn)
	}
	lines = dedupeRecentMemoryLines(lines)
	return joinMemoryLines(lines, 1200)
}

func summarizeConversationTurn(prompt, response string, evidence EvidenceBundle, plan agent.Plan, history []llama.ChatMessage) string {
	parts := make([]string, 0, 4)
	if plan.UseRetrieval {
		parts = append(parts, "Retrieved answer")
	} else if plan.UseMemory {
		parts = append(parts, "Memory-based answer")
	} else {
		parts = append(parts, "Direct answer")
	}
	if topic := trimSummaryText(prompt, 120); topic != "" {
		parts = append(parts, "User: "+topic)
	}
	if outcome := trimSummaryText(response, 160); outcome != "" {
		parts = append(parts, "Assistant: "+outcome)
	}
	if sources := memorySourceNames(evidence); len(sources) > 0 {
		parts = append(parts, "Sources: "+strings.Join(sources, ", "))
	}
	if len(history) > 0 && len(parts) < 4 {
		for i := len(history) - 1; i >= 0 && len(parts) < 4; i-- {
			if snippet := trimSummaryText(history[i].Content, 80); snippet != "" {
				parts = append(parts, snippet)
			}
		}
	}
	return strings.Join(parts, " | ")
}

func splitMemoryLines(summary string) []string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return nil
	}
	parts := strings.Split(summary, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func uniqueMemoryLines(lines []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out
}

func dedupeRecentMemoryLines(lines []string) []string {
	if len(lines) <= 8 {
		return lines
	}
	return lines[len(lines)-8:]
}

func joinMemoryLines(lines []string, maxChars int) string {
	if len(lines) == 0 {
		return ""
	}
	var out strings.Builder
	for _, line := range lines {
		if line == "" {
			continue
		}
		candidate := line
		if out.Len() > 0 {
			candidate = "\n" + line
		}
		if maxChars > 0 && out.Len()+len(candidate) > maxChars {
			break
		}
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(line)
	}
	return strings.TrimSpace(out.String())
}

func trimSummaryText(text string, max int) string {
	text = strings.TrimSpace(text)
	if text == "" || max <= 0 {
		return text
	}
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func memorySourceNames(evidence EvidenceBundle) []string {
	if len(evidence.Nodes) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var names []string
	for _, node := range evidence.Nodes {
		name := strings.TrimSpace(node.Filename)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
		if len(names) >= 3 {
			break
		}
	}
	return names
}

type chunkRef struct {
	refNum int
	chunk  store.ScoredChunk
}

func buildMessagesWithBudget(ag *agent.Agent, plan agent.Plan, bundle EvidenceBundle, prompt string, history []llama.ChatMessage, memoryContext string, ctxSize int, collectionName string) ([]llama.ChatMessage, []chunkRef) {
	// Scale character budgets proportionally to context window size
	ratio := float64(ctxSize) / float64(defaultContextSize)
	scaledMaxTotal := int(float64(maxTotalChars) * ratio)
	scaledMaxContext := int(float64(maxContextChars) * ratio)

	// Budget: leave 50% of context for the response itself
	responseBudget := ctxSize * 2 // ~2 chars per token
	promptBudget := scaledMaxTotal
	if promptBudget > responseBudget {
		promptBudget = responseBudget
	}

	contextString := ""
	if plan.UseRetrieval {
		contextString = bundle.ContextString(scaledMaxContext)
		if contextString == "" {
			contextString = "No retrieved evidence was available."
		}
	}
	if scaledMaxContext > 0 && len(contextString) > scaledMaxContext {
		contextString = contextString[:scaledMaxContext]
	}
	var contextBuilder strings.Builder
	contextBuilder.WriteString(contextString)
	sourceRefs := bundle.sourceRefs()

	systemPrompt := "You are a helpful AI assistant."
	if ag != nil {
		systemPrompt = ag.RenderSystemPrompt(plan, collectionName, memoryContext, contextBuilder.String())
	}
	if systemPrompt == "" {
		systemPrompt = "You are a helpful AI assistant."
	}

	messages := []llama.ChatMessage{
		{Role: "system", Content: systemPrompt},
	}

	// Track remaining budget
	remaining := promptBudget - len(systemPrompt) - len(prompt) - 50

	// Include conversation history, respecting budget
	if history != nil {
		for _, h := range history {
			if remaining <= 0 {
				break
			}
			if len(h.Content) > remaining {
				messages = append(messages, llama.ChatMessage{Role: h.Role, Content: h.Content[:remaining] + "..."})
				remaining = 0
			} else {
				messages = append(messages, h)
				remaining -= len(h.Content)
			}
		}
	}

	// Add current user prompt
	messages = append(messages, llama.ChatMessage{Role: "user", Content: prompt})

	return messages, sourceRefs
}
