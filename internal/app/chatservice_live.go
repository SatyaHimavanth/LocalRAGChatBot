package app

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"changeme/internal/engine"
	"changeme/internal/ingest"
	"changeme/internal/store"

	llama "github.com/tcpipuk/llama-go"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ─── Constants ───
const (
	// ─── Context window safety limits ───
	maxTotalChars  = 12000
	maxHistoryChars = 3000
	maxContextChars = 4000

	// Default context size if we can't detect RAM
	defaultContextSize = 4096

	// Max response tokens — generous enough for complete answers,
	// safe enough to prevent runaway generation / OOM
)

var maxResponseTokens int = 1024

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
					return kb / 1024 / 1024 // kB → GB
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
}

func (s *ChatService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.app = application.Get()
	return nil
}

// ─── Greeting patterns for non-RAG responses ───
var greetingPatterns = []string{
	"hi", "hello", "hey", "greetings", "howdy", "good morning", "good afternoon",
	"good evening", "good night", "what's up", "sup", "yo", "how are you",
	"how are you doing", "how do you do", "nice to meet you", "pleased to meet you",
	"thanks", "thank you", "thank you so much", "thanks a lot", "appreciate it",
	"bye", "goodbye", "see you", "see you later", "talk to you later", "cya",
	"have a good day", "have a nice day", "take care",
	"what can you do", "who are you", "what are you", "tell me about yourself",
}

func isGreeting(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	// Remove trailing punctuation for comparison
	cleaned := strings.TrimRight(lower, ",.!? \t")
	for _, p := range greetingPatterns {
		if cleaned == p {
			return true
		}
	}
	return false
}

var greetingResponses = map[string]string{
	"hi":             "Hi there! How can I help you today?",
	"hello":          "Hello! Feel free to ask me anything or upload a document to get started.",
	"hey":            "Hey! What can I do for you?",
	"good morning":   "Good morning! Hope you're having a great day. How can I help?",
	"good afternoon": "Good afternoon! What can I assist you with?",
	"good evening":   "Good evening! How can I help you today?",
	"how are you":    "I'm doing well, thank you for asking! How can I assist you?",
	"thanks":         "You're welcome! Let me know if you need anything else.",
	"thank you":      "You're welcome! Happy to help.",
	"bye":            "Goodbye! Feel free to come back anytime.",
	"goodbye":        "Goodbye! Take care.",
	"what can you do": "I'm a local RAG (Retrieval-Augmented Generation) assistant. You can upload documents to my collections and then ask me questions about them. I'll search through the content to find relevant answers!",
	"who are you":    "I'm your local RAG assistant, running entirely on your machine with no external API calls. I can answer questions based on documents you upload to the collections.",
}

func getGreetingResponse(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	// Exact match first
	if resp, ok := greetingResponses[lower]; ok {
		return resp
	}
	// Prefix match
	for pattern, resp := range greetingResponses {
		if strings.HasPrefix(lower, pattern+",") || strings.HasPrefix(lower, pattern+"!") || strings.HasPrefix(lower, pattern+".") {
			return resp
		}
	}
	return "Hello! How can I assist you today?"
}

// ════════════════════════════════════════════════
//  Collection management
// ════════════════════════════════════════════════

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

// ════════════════════════════════════════════════
//  Chat session management
// ════════════════════════════════════════════════

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

// ════════════════════════════════════════════════
//  Messaging
// ════════════════════════════════════════════════

func (s *ChatService) SendMessage(sessionID int64, collectionID int64, prompt string) error {
	if s.app == nil {
		s.app = application.Get()
	}
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("prompt cannot be empty")
	}

	// Persist user message
	if s.DB != nil {
		if _, err := store.AddChatMessage(s.DB, sessionID, "user", prompt); err != nil {
			return fmt.Errorf("persisting user message: %w", err)
		}
	}

	// ── Check for greeting / small talk ──
	if isGreeting(prompt) {
		response := getGreetingResponse(prompt)
		go func() {
			s.emit("chat:thinking", map[string]any{"sessionId": sessionID})
			s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "responding", "label": "Responding..."})
			time.Sleep(100 * time.Millisecond)
			s.emit("chat:token", map[string]any{
				"sessionId": sessionID,
				"token":     response,
			})
			if s.DB != nil {
				store.AddChatMessage(s.DB, sessionID, "assistant", response)
			}
			s.emit("chat:done", sessionID)
		}()
		return nil
	}

	// ── Engine check ──
	if s.Engine == nil {
		s.emit("chat:thinking", map[string]any{"sessionId": sessionID})
		s.emit("chat:token", map[string]any{
			"sessionId": sessionID,
			"token":     "Engine not initialized.",
		})
		s.emit("chat:done", sessionID)
		return nil
	}

	// Run the entire process in a background goroutine
	go func() {
		s.emit("chat:thinking", map[string]any{"sessionId": sessionID})
		s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "thinking", "label": "Thinking..."})
		time.Sleep(200 * time.Millisecond)

		// Load conversation history
		var history []llama.ChatMessage
		if s.DB != nil {
			history = s.loadConversationHistory(sessionID)
		}

		// Search knowledge base
		s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "searching", "label": "Searching documents..."})

		var chunks []store.ScoredChunk
		if s.DB != nil {
			queryEmb, err := s.Engine.Embed(prompt)
			if err == nil {
				chunks, err = store.HybridSearch(s.DB, collectionID, prompt, queryEmb, 5)
				if err != nil {
					chunks = nil
				}
			}
		}

		if len(chunks) > 0 {
			s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "found", "label": fmt.Sprintf("Found %d relevant sections ✓", len(chunks))})
			time.Sleep(150 * time.Millisecond)
			s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "summarizing", "label": "Summarizing..."})
		} else {
			s.emit("chat:status", map[string]any{"sessionId": sessionID, "status": "thinking", "label": "Thinking..."})
		}
		time.Sleep(100 * time.Millisecond)

		s.emit("chat:thinking:done", map[string]any{"sessionId": sessionID})

		// Get optimal context size
		ctxSize := getOptimalContextSize()

		// Build messages with both context AND conversation history
		messages := buildMessagesWithBudget(chunks, prompt, history, ctxSize)

		chatCtx, err := s.Engine.ChatModel.NewContext(llama.WithContext(ctxSize))
		if err != nil {
			s.emit("chat:token", map[string]any{"sessionId": sessionID, "token": fmt.Sprintf("\n[Error: %v]\n", err)})
			s.emit("chat:done", sessionID)
			return
		}
		defer chatCtx.Close()

		deltas, errs := chatCtx.ChatStream(context.Background(), messages, llama.ChatOptions{
			MaxTokens: &maxResponseTokens,
		})

		var fullResponse strings.Builder
		for {
			select {
			case delta, ok := <-deltas:
				if !ok {
					if s.DB != nil && fullResponse.Len() > 0 {
						store.AddChatMessage(s.DB, sessionID, "assistant", fullResponse.String())
					}
					s.emit("chat:done", sessionID)
					return
				}
				fullResponse.WriteString(delta.Content)
				s.emit("chat:token", map[string]any{
					"sessionId": sessionID,
					"token":     delta.Content,
				})
			case err := <-errs:
				if err != nil {
					s.emit("chat:token", map[string]any{
						"sessionId": sessionID,
						"token":     fmt.Sprintf("\n[Error: %v]\n", err),
					})
					s.emit("chat:done", sessionID)
					return
				}
			}
		}
	}()

	return nil
}

// ════════════════════════════════════════════════
//  File Upload & Parsing
// ════════════════════════════════════════════════

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

// UploadFile handles file upload: base64 data -> extract text -> hash -> check dupe -> ingest.
// If replace is true and file exists, it replaces the old content.
func (s *ChatService) UploadFile(filename string, base64Data string, collectionID int64, replace bool) (*FileUploadResult, error) {
	if s.Engine == nil || s.DB == nil {
		return &FileUploadResult{Filename: filename, Status: "error", Message: "Engine or database not initialized"}, nil
	}

	// 1. Decode base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return &FileUploadResult{Filename: filename, Status: "error", Message: "Failed to decode file data"}, nil
	}

	// 2. Compute hash
	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	// 3. Extract text using parser
	text, err := ingest.ParseFileBytes(data, filename)
	if err != nil {
		return &FileUploadResult{Filename: filename, Status: "error", Message: err.Error()}, nil
	}
	if strings.TrimSpace(text) == "" {
		return &FileUploadResult{Filename: filename, Status: "error", Message: "No extractable text found in file"}, nil
	}

	// 4. Check for duplicate
	existing, err := store.GetDocumentByHash(s.DB, hash, collectionID)
	if err != nil {
		return &FileUploadResult{Filename: filename, Status: "error", Message: "Database error checking duplicates"}, nil
	}
	if existing != nil {
		createdTime := time.Unix(existing.CreatedAt, 0).Format("Jan 2, 2006 at 15:04")
		if !replace {
			return &FileUploadResult{
				Filename:        filename,
				Status:          "duplicate",
				Message:         fmt.Sprintf("This file is already present in the collection (uploaded %s)", createdTime),
				ExistingDocID:   existing.ID,
				ExistingCreated: existing.CreatedAt,
			}, nil
		}
		// Replace: delete old chunks, update content, re-ingest
		if err := store.DeleteDocumentChunks(s.DB, existing.ID); err != nil {
			return &FileUploadResult{Filename: filename, Status: "error", Message: "Failed to replace file: " + err.Error()}, nil
		}
		if err := store.UpdateDocumentContent(s.DB, existing.ID, text, hash); err != nil {
			return &FileUploadResult{Filename: filename, Status: "error", Message: "Failed to update document"}, nil
		}
		// Re-ingest with new content
		words := len(strings.Fields(text))
		chunks := ingest.SplitText(text, 500, 100)
		for i, chunk := range chunks {
			embedding, err := s.Engine.Embed(chunk.Content)
			if err != nil {
				return &FileUploadResult{Filename: filename, Status: "error", Message: fmt.Sprintf("Embedding error: %v", err)}, nil
			}
			if _, err := store.InsertChunk(s.DB, existing.ID, collectionID, chunk.Content, chunk.Ord, embedding); err != nil {
				return &FileUploadResult{Filename: filename, Status: "error", Message: fmt.Sprintf("Insert error: %v", err)}, nil
			}
			_ = i
		}
		colName := s.getCollectionName(collectionID)
		return &FileUploadResult{
			Filename: filename, Status: "replaced", Message: "File replaced and re-ingested successfully",
			DocID: existing.ID, CollectionName: colName, WordCount: words, ChunkCount: len(chunks),
		}, nil
	}

	// 5. New file: emit progress, ingest
	colName := s.getCollectionName(collectionID)
	words := len(strings.Fields(text))
	chunks := ingest.SplitText(text, 500, 100)
	total := len(chunks)
	if total == 0 {
		return &FileUploadResult{Filename: filename, Status: "error", Message: "No content to ingest"}, nil
	}

	// Register document with hash and content
	docID, err := store.AddDocument(s.DB, collectionID, filename, hash, text)
	if err != nil {
		return &FileUploadResult{Filename: filename, Status: "error", Message: fmt.Sprintf("Registering document: %v", err)}, nil
	}

	embedErr := false
	for i, chunk := range chunks {
		embedding, err := s.Engine.Embed(chunk.Content)
		if err != nil {
			embedErr = true
			break
		}
		if _, err := store.InsertChunk(s.DB, docID, collectionID, chunk.Content, chunk.Ord, embedding); err != nil {
			embedErr = true
			break
		}
		_ = i
	}

	// If embedding failed, clean up the document record so it doesn't appear as a broken entry
	if embedErr {
		store.DeleteDocument(s.DB, docID)
		return &FileUploadResult{Filename: filename, Status: "error", Message: "Embedding failed partway through. Document cleaned up. Try again."}, nil
	}

	return &FileUploadResult{
		Filename: filename, Status: "success", Message: fmt.Sprintf("File ingested: %d chunks", total),
		DocID: docID, CollectionName: colName, WordCount: words, ChunkCount: total,
	}, nil
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
		Message: fmt.Sprintf("Already uploaded %s", createdTime),
		ExistingDocID: existing.ID, ExistingCreated: existing.CreatedAt,
	}, nil
}

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

	s.emit("ingest:progress", map[string]any{
		"step": "chunking", "label": "Chunking text...", "pct": 0, "detail": "",
	})

	chunks := ingest.SplitText(fileContent, 500, 100)
	total := len(chunks)
	if total == 0 {
		return fmt.Errorf("no content to ingest")
	}

	s.emit("ingest:progress", map[string]any{
		"step": "chunked", "label": fmt.Sprintf("Chunked into %d parts ✓", total),
		"pct": 5, "detail": fmt.Sprintf("%d chunks", total),
	})

	docID, err := store.AddDocument(s.DB, collectionID, filename, "", "")
	if err != nil {
		return fmt.Errorf("registering document: %w", err)
	}

	for i, chunk := range chunks {
		pct := 5 + ((i * 90) / total)
		s.emit("ingest:progress", map[string]any{
			"step": "embedding", "label": fmt.Sprintf("Embedding chunk %d/%d...", i+1, total),
			"pct": pct, "detail": fmt.Sprintf("%d/%d", i+1, total),
		})

		embedding, err := s.Engine.Embed(chunk.Content)
		if err != nil {
			return fmt.Errorf("generating chunk embedding: %w", err)
		}
		_, err = store.InsertChunk(s.DB, docID, collectionID, chunk.Content, chunk.Ord, embedding)
		if err != nil {
			return fmt.Errorf("inserting chunk: %w", err)
		}
	}

	s.emit("ingest:progress", map[string]any{
		"step": "complete", "label": fmt.Sprintf("Complete! %d chunks ingested.", total),
		"pct": 100, "detail": "",
	})
	return nil
}

// ════════════════════════════════════════════════
//  Universal Search
// ════════════════════════════════════════════════

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
		sr    SearchResult
		rrf   float64
	}
	merged := make(map[int64]*scored)

	for rank, r := range kwResults {
		fn := s.getChunkFilename(r.ChunkID)
		ns := 1.0 / (1.0 + math.Abs(r.Score))
		merged[r.ChunkID] = &scored{
			sr: SearchResult{
				Content: r.Content, Score: ns, SearchType: "keyword",
				CollectionID: collectionID,
				CollectionName: colName, Filename: fn, ChunkID: r.ChunkID,
			},
			rrf: 1.0 / float64(k+rank+1),
		}
	}
	for rank, r := range vecResults {
		fn := s.getChunkFilename(r.ChunkID)
		vecNorm := 1.0 / (1.0 + math.Abs(r.Score))
		if existing, ok := merged[r.ChunkID]; ok {
			existing.rrf += 1.0 / float64(k+rank+1)
			existing.sr.SearchType = "hybrid"
		} else {
			merged[r.ChunkID] = &scored{
				sr: SearchResult{
					Content: r.Content, Score: vecNorm, SearchType: "vector",
					CollectionID: collectionID,
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
	if len(out) > 20 {
		out = out[:20]
	}
	return out, nil
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

// ════════════════════════════════════════════════
//  Helpers
// ════════════════════════════════════════════════

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

func buildMessagesWithBudget(chunks []store.ScoredChunk, prompt string, history []llama.ChatMessage, ctxSize int) []llama.ChatMessage {
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

	// Build context string from RAG chunks, capped
	var contextBuilder strings.Builder
	for _, chunk := range chunks {
		if chunk.Content == "" {
			continue
		}
		if contextBuilder.Len()+len(chunk.Content)+3 > scaledMaxContext {
			remaining := scaledMaxContext - contextBuilder.Len()
			if remaining > 10 {
				contextBuilder.WriteString("- ")
				if len(chunk.Content) > remaining-3 {
					contextBuilder.WriteString(chunk.Content[:remaining-10] + "...\n")
				} else {
					contextBuilder.WriteString(chunk.Content)
					contextBuilder.WriteByte('\n')
				}
			}
			break
		}
		contextBuilder.WriteString("- ")
		contextBuilder.WriteString(chunk.Content)
		contextBuilder.WriteByte('\n')
	}

	systemPrompt := "You are a helpful AI assistant."
	if contextBuilder.Len() > 0 {
		systemPrompt += " Use the Document Context below to answer questions. If the context doesn't contain relevant information, answer from your general knowledge."
		systemPrompt += "\n\nDocument Context:\n" + contextBuilder.String()
	} else {
		systemPrompt += " You have the conversation history below. Answer from your general knowledge. If the user asks about specific documents they uploaded, let them know no documents are available."
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

	return messages
}
