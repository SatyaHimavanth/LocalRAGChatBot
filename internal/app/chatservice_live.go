package app

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
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
	return nil
}

//  Collection management

func (s *ChatService) CreateCollection(name string) (int64, error) {
	if s.DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	if strings.TrimSpace(name) == "" {
		return 0, fmt.Errorf("collection name cannot be empty")
	}
	// Check for duplicate collection names
	cols, _ := store.GetCollections(s.DB)
	for _, c := range cols {
		if strings.EqualFold(c.Name, strings.TrimSpace(name)) {
			return 0, fmt.Errorf("a collection named '%s' already exists", strings.TrimSpace(name))
		}
	}
	id, err := store.CreateCollection(s.DB, strings.TrimSpace(name))
	if err != nil {
		return 0, fmt.Errorf("creating collection: %w", err)
	}
	return id, nil
}

func (s *ChatService) DeleteCollection(collectionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return store.DeleteCollection(s.DB, collectionID)
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
		_, err := store.CreateCollection(s.DB, "Knowledge Base")
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
	return id, nil
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

func (s *ChatService) GetChatMessagesFlat(sessionID int64) ([]store.ChatMessage, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return store.GetChatMessagesFlat(s.DB, sessionID)
}

func (s *ChatService) SetChatCurrentLeafMessage(sessionID, leafMessageID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if leafMessageID < 0 {
		return fmt.Errorf("invalid leaf message id")
	}
	if leafMessageID > 0 {
		msg, err := store.GetChatMessageByID(s.DB, sessionID, leafMessageID)
		if err != nil {
			return fmt.Errorf("validating branch leaf: %w", err)
		}
		if msg == nil {
			return fmt.Errorf("message %d not found in session", leafMessageID)
		}
	}
	return store.SetSessionLeafMessageID(s.DB, sessionID, leafMessageID)
}

func (s *ChatService) DeleteChat(sessionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return store.DeleteChatSession(s.DB, sessionID)
}

func (s *ChatService) UpdateChatTitle(sessionID int64, title string) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return store.UpdateChatSessionTitle(s.DB, sessionID, title)
}

func (s *ChatService) ArchiveChat(sessionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return store.ArchiveChatSession(s.DB, sessionID)
}

func (s *ChatService) UnarchiveChat(sessionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return store.UnarchiveChatSession(s.DB, sessionID)
}

func (s *ChatService) PinChat(sessionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return store.PinChatSession(s.DB, sessionID)
}

func (s *ChatService) UnpinChat(sessionID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return store.UnpinChatSession(s.DB, sessionID)
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
	Cancelled            bool     `json:"cancelled"`
	UsedRetrieval        bool     `json:"usedRetrieval"`
	UsedMemory           bool     `json:"usedMemory"`
	UsedWorkspaceMemory  bool     `json:"usedWorkspaceMemory"`
	UsedDirect           bool     `json:"usedDirect"`
	SourceCount          int      `json:"sourceCount"`
	EvidenceEffort       string   `json:"evidenceEffort,omitempty"`
	EvidenceCoverage     float64  `json:"evidenceCoverage,omitempty"`
	EvidencePasses       int      `json:"evidencePasses,omitempty"`
	EvidenceCandidates   int      `json:"evidenceCandidates,omitempty"`
	EvidenceExpanded     int      `json:"evidenceExpanded,omitempty"`
	EvidenceCompressed   int      `json:"evidenceCompressed,omitempty"`
	EvidenceTokens       int      `json:"evidenceTokens,omitempty"`
	EvidenceBudgetTokens int      `json:"evidenceBudgetTokens,omitempty"`
	EvidenceTimeBudgetMS int64    `json:"evidenceTimeBudgetMs,omitempty"`
	EvidenceSummary      string   `json:"evidenceSummary,omitempty"`
	VerificationScore    float64  `json:"verificationScore,omitempty"`
	VerificationVerdict  string   `json:"verificationVerdict,omitempty"`
	VerificationSummary  string   `json:"verificationSummary,omitempty"`
	VerificationIssues   []string `json:"verificationIssues,omitempty"`
	Reason               string   `json:"reason,omitempty"`
	RetrievalQuery       string   `json:"retrievalQuery,omitempty"`
	RetrievalScope       string   `json:"retrievalScope,omitempty"`
	TopK                 int      `json:"topK,omitempty"`
}

func (m AgentResponseMeta) asMap(sessionID, msgID int64) map[string]any {
	return map[string]any{
		"sessionId":            sessionID,
		"msgId":                msgID,
		"cancelled":            m.Cancelled,
		"usedRetrieval":        m.UsedRetrieval,
		"usedMemory":           m.UsedMemory,
		"usedWorkspaceMemory":  m.UsedWorkspaceMemory,
		"usedDirect":           m.UsedDirect,
		"sourceCount":          m.SourceCount,
		"evidenceEffort":       m.EvidenceEffort,
		"evidenceCoverage":     m.EvidenceCoverage,
		"evidencePasses":       m.EvidencePasses,
		"evidenceCandidates":   m.EvidenceCandidates,
		"evidenceExpanded":     m.EvidenceExpanded,
		"evidenceCompressed":   m.EvidenceCompressed,
		"evidenceTokens":       m.EvidenceTokens,
		"evidenceBudgetTokens": m.EvidenceBudgetTokens,
		"evidenceTimeBudgetMs": m.EvidenceTimeBudgetMS,
		"evidenceSummary":      m.EvidenceSummary,
		"verificationScore":    m.VerificationScore,
		"verificationVerdict":  m.VerificationVerdict,
		"verificationSummary":  m.VerificationSummary,
		"verificationIssues":   m.VerificationIssues,
		"reason":               m.Reason,
		"retrievalQuery":       m.RetrievalQuery,
		"retrievalScope":       m.RetrievalScope,
		"topK":                 m.TopK,
	}
}

func (m AgentResponseMeta) json() string {
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *ChatService) startConversationTurn(sessionID int64, collectionID int64, prompt string, parentMessageID int64, effort agent.EvidenceEffort) (int64, error) {
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

	go s.runConversationTurn(sessionID, collectionID, prompt, parentMessageID, userMsgID, effort)
	return userMsgID, nil
}

func (s *ChatService) SendMessage(sessionID int64, collectionID int64, prompt string) error {
	cleanPrompt, parentMessageID, effort, scope := extractTurnControls(prompt)
	effectiveCollectionID := collectionID
	if scope == agent.RetrievalScopeAll.String() {
		effectiveCollectionID = 0
	}
	_, err := s.startConversationTurn(sessionID, effectiveCollectionID, cleanPrompt, parentMessageID, effort)
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
		"sessionId":              sessionID,
		"intent":                 intent,
		"useRetrieval":           plan.UseRetrieval,
		"useMemory":              plan.UseMemory,
		"useWorkspaceMemory":     plan.UseWorkspaceMemory,
		"useDirect":              plan.UseDirect,
		"topK":                   plan.TopK,
		"evidenceEffort":         plan.EvidenceEffort.String(),
		"evidenceCandidateLimit": plan.EvidenceCandidateLimit,
		"evidencePasses":         plan.EvidencePasses,
		"evidenceTokenBudget":    plan.EvidenceTokenBudget,
		"evidenceCoverageTarget": plan.EvidenceCoverageTarget,
		"evidenceTimeBudgetMs":   plan.EvidenceTimeBudgetMS,
		"retrievalQuery":         plan.RetrievalQuery,
		"reason":                 plan.Reason,
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

func (s *ChatService) runConversationTurn(sessionID int64, collectionID int64, prompt string, parentMessageID int64, userMessageID int64, effort agent.EvidenceEffort) {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelMu.Lock()
	s.cancelFuncs[sessionID] = cancel
	s.cancelMu.Unlock()

	var msgID int64
	var doneEmitted bool
	meta := AgentResponseMeta{UsedDirect: true, EvidenceEffort: effort.String()}
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
	plan := ag.Decide(agent.Request{
		Prompt:         prompt,
		History:        history,
		CollectionID:   collectionID,
		CollectionName: s.getCollectionName(collectionID),
		HasDocuments:   s.DB != nil,
		Effort:         effort,
	})

	meta.UsedRetrieval = plan.UseRetrieval
	meta.UsedMemory = plan.UseMemory
	meta.UsedWorkspaceMemory = plan.UseWorkspaceMemory
	meta.UsedDirect = !plan.UseRetrieval && !plan.UseMemory && !plan.UseWorkspaceMemory
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
	evidenceSummary := ""
	if plan.UseRetrieval && s.DB != nil {
		query := strings.TrimSpace(plan.RetrievalQuery)
		if query == "" {
			query = prompt
		}
		meta.RetrievalQuery = query
		report := s.buildEvidenceBundle(ctx, sessionID, collectionID, query, history, effort, plan)
		chunks = report.Chunks
		meta.RetrievalQuery = report.Query
		meta.EvidenceCoverage = report.Stats.Coverage
		meta.EvidencePasses = report.Stats.Passes
		meta.EvidenceCandidates = report.Stats.CandidateCount
		meta.EvidenceExpanded = report.Stats.ExpandedCount
		meta.EvidenceCompressed = report.Stats.CompressedCount
		meta.EvidenceTokens = report.Stats.EstimatedTokens
		meta.EvidenceBudgetTokens = report.Stats.TokenBudget
		meta.EvidenceTimeBudgetMS = report.Stats.TimeBudgetMS
		evidenceSummary = report.Summary
		s.emit("chat:evidence", map[string]any{
			"sessionId":       sessionID,
			"query":           report.Query,
			"summary":         report.Summary,
			"coverage":        report.Stats.Coverage,
			"passes":          report.Stats.Passes,
			"candidateCount":  report.Stats.CandidateCount,
			"expandedCount":   report.Stats.ExpandedCount,
			"compressedCount": report.Stats.CompressedCount,
			"finalCount":      report.Stats.FinalCount,
			"tokenBudget":     report.Stats.TokenBudget,
			"estimatedTokens": report.Stats.EstimatedTokens,
			"timeBudgetMs":    report.Stats.TimeBudgetMS,
			"preview":         s.evidencePreview(report.Chunks, collectionID),
		})
	}
	workspaceMemory := ""
	if plan.UseWorkspaceMemory || plan.UseMemory {
		workspaceMemory = s.buildWorkspaceMemory(sessionID, collectionID)
		if workspaceMemory != "" {
			meta.UsedWorkspaceMemory = true
		}
	}
	meta.SourceCount = len(chunks)
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
	messages, sourceRefs := buildMessagesWithBudget(ag, plan, effort, chunks, prompt, history, ctxSize, s.getCollectionName(collectionID), workspaceMemory, evidenceSummary)

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
					responseText := fullResponse.String()
					report := s.verifyAnswer(prompt, responseText, chunks, meta, plan)
					meta.VerificationScore = report.Confidence
					meta.VerificationVerdict = report.Verdict
					meta.VerificationSummary = report.Summary
					meta.VerificationIssues = report.Issues
					assistantMetaJSON := meta.json()
					msgID, err = store.AddChatMessage(s.DB, sessionID, "assistant", responseText, userMessageID, assistantMetaJSON)
					if err == nil {
						s.refreshWorkspaceMemoryForTurn(sessionID)
					}
					if err == nil && len(sourceRefs) > 0 {
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
	Title          string  `json:"title,omitempty"`
	SectionPath    string  `json:"sectionPath,omitempty"`
	ChunkSummary   string  `json:"chunkSummary,omitempty"`
	ChunkID        int64   `json:"chunkId"`
}

func (s *ChatService) Search(query string, collectionID int64) ([]SearchResult, error) {
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

	if collectionID > 0 {
		colResults, err := s.searchCollection(query, queryEmb, collectionID)
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
		colResults, err := s.searchCollection(query, queryEmb, col.ID)
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

	if len(results) > 20 {
		results = results[:20]
	}
	return results, nil
}

func (s *ChatService) searchCollection(query string, queryEmb []float32, collectionID int64) ([]SearchResult, error) {
	kwResults, err := store.KeywordSearch(s.DB, collectionID, query, 20)
	if err != nil {
		kwResults, err = store.FallbackSearch(s.DB, collectionID, query, 20)
		if err != nil {
			return nil, err
		}
	}

	// Also try vector search if we have an embedding
	var vecResults []store.ScoredChunk
	if queryEmb != nil {
		vecResults, err = store.VectorSearch(s.DB, collectionID, queryEmb, 20)
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
		title, section, summary := s.getChunkPreviewMeta(r.ChunkID)
		ns := normalizeBM25Score(r.Score)
		merged[r.ChunkID] = &scored{
			sr: SearchResult{
				Content: r.Content, Score: ns, SearchType: "keyword",
				CollectionID:   collectionID,
				CollectionName: colName, Filename: fn, Title: title, SectionPath: section, ChunkSummary: summary,
				ChunkID: r.ChunkID,
			},
			rrf: 1.0 / float64(k+rank+1),
		}
	}
	for rank, r := range vecResults {
		fn := s.getChunkFilename(r.ChunkID)
		title, section, summary := s.getChunkPreviewMeta(r.ChunkID)
		vecNorm := normalizeDistanceScore(r.Score)
		if existing, ok := merged[r.ChunkID]; ok {
			existing.rrf += 1.0 / float64(k+rank+1)
			existing.sr.SearchType = "hybrid"
			if vecNorm > existing.sr.Score {
				existing.sr.Score = vecNorm
			}
			if existing.sr.Title == "" {
				existing.sr.Title = title
			}
			if existing.sr.SectionPath == "" {
				existing.sr.SectionPath = section
			}
			if existing.sr.ChunkSummary == "" {
				existing.sr.ChunkSummary = summary
			}
		} else {
			merged[r.ChunkID] = &scored{
				sr: SearchResult{
					Content: r.Content, Score: vecNorm, SearchType: "vector",
					CollectionID:   collectionID,
					CollectionName: colName, Filename: fn, Title: title, SectionPath: section, ChunkSummary: summary,
					ChunkID: r.ChunkID,
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
	if len(out) > 20 {
		out = out[:20]
	}
	return out, nil
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

type chunkRef struct {
	refNum int
	chunk  store.ScoredChunk
}

func buildMessagesWithBudget(ag *agent.Agent, plan agent.Plan, effort agent.EvidenceEffort, chunks []store.ScoredChunk, prompt string, history []llama.ChatMessage, ctxSize int, collectionName string, workspaceMemory string, evidenceSummary string) ([]llama.ChatMessage, []chunkRef) {
	// Scale character budgets proportionally to context window size
	ratio := float64(ctxSize) / float64(defaultContextSize)
	effortMultiplier := effort.ContextMultiplier()
	scaledMaxTotal := int(float64(maxTotalChars) * ratio * effortMultiplier)
	scaledMaxContext := int(float64(maxContextChars) * ratio * effortMultiplier)

	// Budget: leave 50% of context for the response itself
	responseBudget := ctxSize * 2 // ~2 chars per token
	promptBudget := scaledMaxTotal
	if promptBudget > responseBudget {
		promptBudget = responseBudget
	}

	// Build context string from RAG chunks with numbered references, capped
	var contextBuilder strings.Builder
	var sourceRefs []chunkRef
	for i, chunk := range chunks {
		if chunk.Content == "" {
			continue
		}
		refNum := i + 1
		if contextBuilder.Len()+len(chunk.Content)+10 > scaledMaxContext {
			if contextBuilder.Len() > 0 {
				break
			}
			trunc := chunk.Content
			if len(trunc) > scaledMaxContext-20 {
				trunc = trunc[:scaledMaxContext-20] + "..."
			}
			contextBuilder.WriteString(fmt.Sprintf("[%d] %s\n", refNum, trunc))
			sourceRefs = append(sourceRefs, chunkRef{refNum: refNum, chunk: chunk})
			break
		}
		contextBuilder.WriteString(fmt.Sprintf("[%d] %s\n", refNum, chunk.Content))
		sourceRefs = append(sourceRefs, chunkRef{refNum: refNum, chunk: chunk})
	}

	systemPrompt := "You are a helpful AI assistant."
	if ag != nil {
		systemPrompt = ag.RenderSystemPrompt(plan, collectionName, contextBuilder.String(), workspaceMemory, evidenceSummary)
	}
	if systemPrompt == "" {
		systemPrompt = "You are a helpful AI assistant."
	}

	messages := []llama.ChatMessage{{Role: "system", Content: systemPrompt}}

	remaining := promptBudget - len(systemPrompt) - len(prompt) - 50
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

	messages = append(messages, llama.ChatMessage{Role: "user", Content: prompt})
	return messages, sourceRefs
}
