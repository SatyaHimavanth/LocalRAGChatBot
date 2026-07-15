package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"changeme/internal/ingest"
	"changeme/internal/store"
)

// IngestFilePayload is one file or paste item in a batch.
type IngestFilePayload struct {
	Filename    string `json:"filename"`
	Base64Data  string `json:"base64Data"`  // file bytes (base64); empty when using TextContent
	TextContent string `json:"textContent"` // paste path
	Replace     bool   `json:"replace"`
}

// StageResult is the outcome of staging a single file/text.
type StageResult struct {
	Filename string `json:"filename"`
	Status   string `json:"status"` // staged | duplicate | error | replaced
	Message  string `json:"message"`
	DocID    int64  `json:"docId,omitempty"`
}

// IngestBatchResult summarizes stage + embed for a batch.
type IngestBatchResult struct {
	BatchID   string        `json:"batchId"`
	Items     []StageResult `json:"items"`
	Staged    int           `json:"staged"`
	Completed int           `json:"completed"`
	Failed    int           `json:"failed"`
	Cancelled bool          `json:"cancelled"`
}

// IngestJob is a durable incomplete/ready job exposed to the UI.
type IngestJob struct {
	DocID          int64  `json:"docId"`
	CollectionID   int64  `json:"collectionId"`
	Filename       string `json:"filename"`
	Status         string `json:"status"`
	ChunkCount     int    `json:"chunkCount"`
	ExpectedChunks int    `json:"expectedChunks"`
	BatchID        string `json:"batchId"`
	ErrorMessage   string `json:"errorMessage"`
	ProgressPct    int    `json:"progressPct"`
	UpdatedAt      int64  `json:"updatedAt"`
	CreatedAt      int64  `json:"createdAt"`
}

const (
	chunkSize    = 500
	chunkOverlap = 100
)

// ingest runtime state on ChatService
type ingestRuntime struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	active bool
	wg     sync.WaitGroup
}

func (s *ChatService) ensureIngestRuntime() *ingestRuntime {
	s.ingestOnce.Do(func() {
		s.ingestRT = &ingestRuntime{}
	})
	return s.ingestRT
}

// NewBatchID returns a new random batch identifier.
func (s *ChatService) NewBatchID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// IsIngesting reports whether staging/embedding is currently running.
func (s *ChatService) IsIngesting() bool {
	rt := s.ensureIngestRuntime()
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.active
}

// CancelIngest requests cancellation of the active ingest worker.
func (s *ChatService) CancelIngest() {
	rt := s.ensureIngestRuntime()
	rt.mu.Lock()
	cancel := rt.cancel
	rt.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// WaitIngestIdle blocks until ingest finishes or timeout.
func (s *ChatService) WaitIngestIdle(timeoutMs int) {
	rt := s.ensureIngestRuntime()
	done := make(chan struct{})
	go func() {
		rt.wg.Wait()
		close(done)
	}()
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	select {
	case <-done:
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
	}
}

// CleanupIncompleteOnStartup removes staging rows and resets interrupted embedding jobs.
func (s *ChatService) CleanupIncompleteOnStartup() {
	if s.DB == nil {
		return
	}
	if _, err := store.DeleteStagingDocuments(s.DB); err != nil {
		// non-fatal
	}
	_ = store.ResetEmbeddingToQueued(s.DB)
}

// GetIncompleteJobs returns non-ready documents for resume/discard UI.
func (s *ChatService) GetIncompleteJobs() ([]IngestJob, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	docs, err := store.GetIncompleteDocuments(s.DB)
	if err != nil {
		return nil, err
	}
	out := make([]IngestJob, 0, len(docs))
	for _, d := range docs {
		pct := 0
		if d.ExpectedChunks > 0 {
			pct = (d.ChunkCount * 100) / d.ExpectedChunks
			if pct > 100 {
				pct = 100
			}
		}
		out = append(out, IngestJob{
			DocID:          d.ID,
			CollectionID:   d.CollectionID,
			Filename:       d.Filename,
			Status:         d.Status,
			ChunkCount:     d.ChunkCount,
			ExpectedChunks: d.ExpectedChunks,
			BatchID:        d.BatchID,
			ErrorMessage:   d.ErrorMessage,
			ProgressPct:    pct,
			UpdatedAt:      d.UpdatedAt,
			CreatedAt:      d.CreatedAt,
		})
	}
	return out, nil
}

// DiscardIngestJob deletes one incomplete document.
func (s *ChatService) DiscardIngestJob(docID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	doc, err := store.GetDocumentByID(s.DB, docID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}
	if doc.Status == store.DocStatusReady {
		return fmt.Errorf("cannot discard a ready document; delete it instead")
	}
	return store.DeleteDocument(s.DB, docID)
}

// DiscardAllIncomplete deletes all non-ready documents.
func (s *ChatService) DiscardAllIncomplete() (int64, error) {
	if s.DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	return store.DiscardIncompleteDocuments(s.DB)
}

// ResumeIngest continues embedding for all resumable incomplete documents.
func (s *ChatService) ResumeIngest() (*IngestBatchResult, error) {
	return s.processQueue("")
}

// StartIngestBatch stages every file first, then embeds all staged jobs in the batch.
func (s *ChatService) StartIngestBatch(collectionID int64, files []IngestFilePayload) (*IngestBatchResult, error) {
	if s.Engine == nil || s.DB == nil {
		return nil, fmt.Errorf("engine or database not initialized")
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files provided")
	}

	rt := s.ensureIngestRuntime()
	rt.mu.Lock()
	if rt.active {
		rt.mu.Unlock()
		return nil, fmt.Errorf("an ingest job is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	rt.cancel = cancel
	rt.active = true
	rt.wg.Add(1)
	rt.mu.Unlock()

	defer func() {
		cancel()
		rt.mu.Lock()
		rt.active = false
		rt.cancel = nil
		rt.mu.Unlock()
		rt.wg.Done()
	}()

	batchID := s.NewBatchID()
	result := &IngestBatchResult{BatchID: batchID, Items: make([]StageResult, 0, len(files))}

	// ── Phase A: stage all files ──────────────────────────────────────────
	total := len(files)
	for i, f := range files {
		if ctx.Err() != nil {
			result.Cancelled = true
			s.emit("ingest:progress", map[string]any{
				"phase": "cancelled", "step": "cancelled", "label": "Ingest cancelled",
				"pct": 0, "batchId": batchID,
			})
			return result, nil
		}
		s.emit("ingest:progress", map[string]any{
			"phase": "staging", "step": "staging",
			"label":    fmt.Sprintf("Extracting text %d/%d...", i+1, total),
			"pct":      ((i) * 100) / total,
			"detail":   fmt.Sprintf("%d/%d", i+1, total),
			"filename": f.Filename,
			"batchId":  batchID,
			"index":    i + 1,
			"total":    total,
		})

		item := s.stageOne(f, collectionID, batchID)
		result.Items = append(result.Items, item)
		if item.Status == "staged" || item.Status == "replaced" {
			result.Staged++
		}
	}

	s.emit("ingest:progress", map[string]any{
		"phase": "staging_done", "step": "staging_done",
		"label":   fmt.Sprintf("Staged %d document(s)", result.Staged),
		"pct":     100,
		"batchId": batchID,
		"staged":  result.Staged,
	})

	if result.Staged == 0 {
		s.emit("ingest:progress", map[string]any{
			"phase": "batch_done", "step": "complete",
			"label": "Nothing to embed", "pct": 100, "batchId": batchID,
		})
		return result, nil
	}

	// ── Phase B: embed all queued docs in this batch ──────────────────────
	completed, failed, cancelled := s.embedResumable(ctx, batchID)
	result.Completed = completed
	result.Failed = failed
	result.Cancelled = cancelled

	label := fmt.Sprintf("Batch complete: %d ready, %d failed", completed, failed)
	if cancelled {
		label = fmt.Sprintf("Batch interrupted: %d ready, %d failed (resume later)", completed, failed)
	}
	s.emit("ingest:progress", map[string]any{
		"phase": "batch_done", "step": "complete",
		"label": label, "pct": 100, "batchId": batchID,
		"completed": completed, "failed": failed, "cancelled": cancelled,
	})
	return result, nil
}

// processQueue embeds all resumable docs (optionally filtered by batchID).
func (s *ChatService) processQueue(batchID string) (*IngestBatchResult, error) {
	if s.Engine == nil || s.DB == nil {
		return nil, fmt.Errorf("engine or database not initialized")
	}

	rt := s.ensureIngestRuntime()
	rt.mu.Lock()
	if rt.active {
		rt.mu.Unlock()
		return nil, fmt.Errorf("an ingest job is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	rt.cancel = cancel
	rt.active = true
	rt.wg.Add(1)
	rt.mu.Unlock()

	defer func() {
		cancel()
		rt.mu.Lock()
		rt.active = false
		rt.cancel = nil
		rt.mu.Unlock()
		rt.wg.Done()
	}()

	if batchID == "" {
		batchID = "resume"
	}
	completed, failed, cancelled := s.embedResumable(ctx, batchID)
	result := &IngestBatchResult{
		BatchID:   batchID,
		Completed: completed,
		Failed:    failed,
		Cancelled: cancelled,
	}
	s.emit("ingest:progress", map[string]any{
		"phase": "batch_done", "step": "complete",
		"label":     fmt.Sprintf("Resume complete: %d ready, %d failed", completed, failed),
		"pct":       100,
		"batchId":   batchID,
		"completed": completed,
		"failed":    failed,
		"cancelled": cancelled,
	})
	return result, nil
}

func (s *ChatService) stageOne(f IngestFilePayload, collectionID int64, batchID string) StageResult {
	filename := strings.TrimSpace(f.Filename)
	if filename == "" {
		filename = "document.txt"
	}

	text, err := s.extractText(f)
	if err != nil {
		return StageResult{Filename: filename, Status: "error", Message: err.Error()}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return StageResult{Filename: filename, Status: "error", Message: "No extractable text found"}
	}

	profile := ingest.BuildDocumentProfile(filename, text)
	text = profile.NormalizedText
	hash := profile.ContentHash
	chunks, _ := ingest.BuildChunkSpecs(filename, text, chunkSize, chunkOverlap)
	if len(chunks) == 0 {
		return StageResult{Filename: filename, Status: "error", Message: "No content to ingest"}
	}
	expected := len(chunks)

	existing, err := store.GetDocumentByHash(s.DB, hash, collectionID)
	if err != nil {
		return StageResult{Filename: filename, Status: "error", Message: "Database error checking duplicates"}
	}
	if existing != nil {
		if existing.Status == store.DocStatusReady && !f.Replace {
			createdTime := time.Unix(existing.CreatedAt, 0).Format("Jan 2, 2006 at 15:04")
			return StageResult{
				Filename: filename,
				Status:   "duplicate",
				Message:  fmt.Sprintf("Already in collection (uploaded %s)", createdTime),
				DocID:    existing.ID,
			}
		}
		// Replace or re-queue incomplete same-hash doc
		if err := store.DeleteDocumentChunks(s.DB, existing.ID); err != nil {
			return StageResult{Filename: filename, Status: "error", Message: "Failed to clear old chunks: " + err.Error()}
		}
		if err := store.UpdateDocumentContent(s.DB, existing.ID, text, hash); err != nil {
			return StageResult{Filename: filename, Status: "error", Message: "Failed to update document content"}
		}
		if err := store.UpdateDocumentSummary(s.DB, existing.ID, profile.Summary); err != nil {
			return StageResult{Filename: filename, Status: "error", Message: "Failed to update document summary"}
		}
		if err := store.UpdateDocumentIngest(s.DB, existing.ID, store.DocStatusQueued, expected, batchID, ""); err != nil {
			return StageResult{Filename: filename, Status: "error", Message: "Failed to queue document"}
		}
		status := "replaced"
		if existing.Status != store.DocStatusReady {
			status = "staged"
		}
		return StageResult{Filename: filename, Status: status, Message: "Queued for embedding", DocID: existing.ID}
	}

	docID, err := store.AddDocumentWithStatus(s.DB, collectionID, filename, hash, text, store.DocStatusQueued, batchID, expected)
	if err != nil {
		return StageResult{Filename: filename, Status: "error", Message: fmt.Sprintf("Registering document: %v", err)}
	}
	if err := store.UpdateDocumentSummary(s.DB, docID, profile.Summary); err != nil {
		return StageResult{Filename: filename, Status: "error", Message: "Failed to save document summary"}
	}
	return StageResult{Filename: filename, Status: "staged", Message: "Queued for embedding", DocID: docID}
}

func (s *ChatService) extractText(f IngestFilePayload) (string, error) {
	if strings.TrimSpace(f.TextContent) != "" {
		return f.TextContent, nil
	}
	if strings.TrimSpace(f.Base64Data) == "" {
		return "", fmt.Errorf("no file data or text content provided")
	}
	data, err := base64.StdEncoding.DecodeString(f.Base64Data)
	if err != nil {
		// try raw URL-safe / without padding variants
		data, err = base64.RawStdEncoding.DecodeString(f.Base64Data)
		if err != nil {
			return "", fmt.Errorf("failed to decode file data")
		}
	}
	return ingest.ParseFileBytes(data, f.Filename)
}

// embedResumable embeds all resumable documents. batchID "" or "resume" = all.
func (s *ChatService) embedResumable(ctx context.Context, batchID string) (completed, failed int, cancelled bool) {
	filter := batchID
	if batchID == "resume" {
		filter = ""
	}
	docs, err := store.GetResumableDocuments(s.DB, filter)
	if err != nil || len(docs) == 0 {
		return 0, 0, false
	}

	for di, doc := range docs {
		if ctx.Err() != nil {
			// leave current/remaining as queued
			_ = store.UpdateDocumentStatus(s.DB, doc.ID, store.DocStatusQueued, "Interrupted — resume to continue")
			return completed, failed, true
		}

		if err := s.embedOneDocument(ctx, &docs[di], di+1, len(docs)); err != nil {
			if ctx.Err() != nil {
				_ = store.UpdateDocumentStatus(s.DB, doc.ID, store.DocStatusQueued, "Interrupted — resume to continue")
				return completed, failed, true
			}
			_ = store.UpdateDocumentStatus(s.DB, doc.ID, store.DocStatusFailed, err.Error())
			failed++
			s.emit("ingest:progress", map[string]any{
				"phase": "embedding", "step": "doc_failed",
				"label": fmt.Sprintf("Failed: %s", doc.Filename),
				"docId": doc.ID, "filename": doc.Filename, "error": err.Error(),
			})
			continue
		}
		completed++
		s.emit("ingest:progress", map[string]any{
			"phase": "embedding", "step": "doc_ready",
			"label": fmt.Sprintf("Ready: %s", doc.Filename),
			"docId": doc.ID, "filename": doc.Filename, "pct": 100,
		})
	}
	return completed, failed, false
}

func (s *ChatService) embedOneDocument(ctx context.Context, doc *store.Document, docIndex, docTotal int) error {
	_ = store.UpdateDocumentStatus(s.DB, doc.ID, store.DocStatusEmbedding, "")

	chunks, profile := ingest.BuildChunkSpecs(doc.Filename, doc.Content, chunkSize, chunkOverlap)
	total := len(chunks)
	if total == 0 {
		return fmt.Errorf("no content to embed")
	}
	if doc.ExpectedChunks != total {
		_ = store.UpdateDocumentIngest(s.DB, doc.ID, store.DocStatusEmbedding, total, doc.BatchID, "")
		doc.ExpectedChunks = total
	}
	_ = store.UpdateDocumentSummary(s.DB, doc.ID, profile.Summary)

	s.emit("ingest:progress", map[string]any{
		"phase": "embedding", "step": "chunked",
		"label":    fmt.Sprintf("Embedding %s (%d/%d docs)", doc.Filename, docIndex, docTotal),
		"pct":      0,
		"detail":   fmt.Sprintf("%d chunks", total),
		"docId":    doc.ID,
		"filename": doc.Filename,
		"docIndex": docIndex,
		"docTotal": docTotal,
	})

	for i, chunk := range chunks {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		exists, err := store.HasChunkOrd(s.DB, doc.ID, chunk.Ord)
		if err != nil {
			return err
		}
		if exists {
			pct := ((i + 1) * 100) / total
			s.emit("ingest:progress", map[string]any{
				"phase": "embedding", "step": "embedding",
				"label":    fmt.Sprintf("Embedding %s — chunk %d/%d", doc.Filename, i+1, total),
				"pct":      pct,
				"detail":   fmt.Sprintf("%d/%d", i+1, total),
				"docId":    doc.ID,
				"filename": doc.Filename,
			})
			continue
		}

		embedding, err := s.Engine.Embed(chunk.Content)
		if err != nil {
			return fmt.Errorf("embedding chunk %d: %w", i+1, err)
		}
		if _, err := store.InsertChunkWithMetadata(s.DB, doc.ID, doc.CollectionID, chunk.Content, chunk.Ord, store.ChunkMetadata{
			Title:        chunk.Title,
			SectionPath:  chunk.SectionPath,
			HeadingLevel: chunk.HeadingLevel,
			Summary:      chunk.Summary,
			ContentHash:  chunk.ContentHash,
			TokenCount:   chunk.TokenCount,
			CharCount:    chunk.CharCount,
		}, embedding); err != nil {
			return fmt.Errorf("inserting chunk %d: %w", i+1, err)
		}

		pct := ((i + 1) * 100) / total
		s.emit("ingest:progress", map[string]any{
			"phase": "embedding", "step": "embedding",
			"label":    fmt.Sprintf("Embedding %s — chunk %d/%d", doc.Filename, i+1, total),
			"pct":      pct,
			"detail":   fmt.Sprintf("%d/%d", i+1, total),
			"docId":    doc.ID,
			"filename": doc.Filename,
		})
	}

	// Verify completeness
	cnt, err := store.CountChunks(s.DB, doc.ID)
	if err != nil {
		return err
	}
	if cnt < total {
		return fmt.Errorf("incomplete: %d/%d chunks written", cnt, total)
	}
	if err := store.UpdateChunkNeighbors(s.DB, doc.ID); err != nil {
		return err
	}
	return store.UpdateDocumentStatus(s.DB, doc.ID, store.DocStatusReady, "")
}
