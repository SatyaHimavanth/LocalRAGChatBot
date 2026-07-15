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

func toStoreIngestMetadata(meta ingest.DocumentMetadata) store.DocumentIngestMetadata {
	return store.DocumentIngestMetadata{
		SourceType:      meta.Loader,
		SourceSizeBytes: meta.SizeBytes,
		WordCount:       meta.WordCount,
		LineCount:       meta.LineCount,
		CharacterCount:  meta.CharacterCount,
		ParagraphCount:  meta.ParagraphCount,
		Title:           meta.Title,
		Summary:         meta.Summary,
	}
}

func (s *ChatService) recordIngestLog(jobID, docID, collectionID int64, batchID, level, stage, message string, start time.Time, throughput float64) {
	if s.DB == nil || jobID == 0 {
		return
	}
	duration := time.Since(start)
	_ = store.AddIngestionLog(s.DB, jobID, docID, collectionID, batchID, level, stage, message, duration.Milliseconds(), 0, throughput)
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
		s.recordEvent("ingest:cancel", "Ingest cancelled", "Active ingest worker cancellation requested", "warn", "ingest", 0, 0, 0, "")
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

// PauseIngest requests a graceful pause of the active ingest worker.
func (s *ChatService) PauseIngest() error {
	rt := s.ensureIngestRuntime()
	rt.mu.Lock()
	cancel := rt.cancel
	rt.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// GetIngestionQueue returns persisted queue jobs for a collection.
func (s *ChatService) GetIngestionQueue(collectionID int64) ([]store.IngestionJob, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return store.GetIngestionJobs(s.DB, collectionID)
}

// GetIngestionLogs returns the job logs for a particular ingestion job.
func (s *ChatService) GetIngestionLogs(jobID int64, limit int) ([]store.IngestionLog, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return store.GetIngestionLogs(s.DB, jobID, limit)
}

// RetryFailedIngest marks a job as retrying so the UI can trigger a resume.
func (s *ChatService) RetryFailedIngest(jobID int64) error {
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := store.RetryIngestionJob(s.DB, jobID); err != nil {
		return err
	}
	s.recordEvent("ingest:retry", "Retry queued", fmt.Sprintf("Job %d marked retrying", jobID), "info", "ingest", 0, 0, 0, "")
	return nil
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
	if err := store.DeleteDocument(s.DB, docID); err != nil {
		return err
	}
	s.recordEvent("ingest:discard_one", "Incomplete job discarded", fmt.Sprintf("Document %d removed from queue", docID), "warn", "ingest", 0, 0, docID, "")
	return nil
}

// DiscardAllIncomplete deletes all non-ready documents.
func (s *ChatService) DiscardAllIncomplete() (int64, error) {
	if s.DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	count, err := store.DiscardIncompleteDocuments(s.DB)
	if err != nil {
		return 0, err
	}
	s.recordEvent("ingest:discard_all", "Incomplete jobs discarded", fmt.Sprintf("%d incomplete document(s) removed", count), "warn", "ingest", 0, 0, 0, "")
	return count, nil
}

// FindDuplicateDocuments performs duplicate classification for a document before staging.
func (s *ChatService) FindDuplicateDocuments(collectionID int64, filename, content string) ([]store.DuplicateMatch, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}
	chunks := ingest.SplitText(content, chunkSize, chunkOverlap)
	chunkHashes := make([]string, 0, len(chunks))
	for _, c := range chunks {
		chunkHashes = append(chunkHashes, ingest.HashChunkContent(c.Content))
	}
	return store.FindPotentialDuplicates(s.DB, collectionID, filename, ingest.HashNormalizedText(content), chunkHashes, 10)
}

// ResumeIngest continues embedding for all resumable incomplete documents.
func (s *ChatService) ResumeIngest() (*IngestBatchResult, error) {
	s.recordEvent("ingest:resume_request", "Ingest resume requested", "Queued documents will be resumed", "info", "ingest", 0, 0, 0, "resume")
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
	var jobID int64
	if id, err := store.CreateIngestionJob(s.DB, batchID, collectionID, len(files)); err == nil {
		jobID = id
		_ = store.UpdateIngestionJob(s.DB, jobID, "running", "staging", 0, 0, 0, "starting ingest batch")
		s.recordEvent("ingest:batch_start", "Ingest batch started", fmt.Sprintf("%d item(s) staged for collection %d", len(files), collectionID), "info", "ingest", collectionID, 0, 0, batchID)
	}

	// ── Phase A: stage all files ──────────────────────────────────────────
	total := len(files)
	for i, f := range files {
		if ctx.Err() != nil {
			result.Cancelled = true
			s.emit("ingest:progress", map[string]any{
				"phase": "cancelled", "step": "cancelled", "label": "Ingest cancelled",
				"pct": 0, "batchId": batchID,
			})
			s.recordEvent("ingest:batch_cancelled", "Ingest cancelled", "Batch cancelled during staging", "warn", "ingest", collectionID, 0, 0, batchID)
			return result, nil
		}
		stageStart := time.Now()
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

		item := s.stageOne(f, collectionID, batchID, jobID)
		s.recordIngestLog(jobID, item.DocID, collectionID, batchID, "info", "staging", item.Message, stageStart, 0)
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
	s.recordEvent("ingest:staged", "Documents staged", fmt.Sprintf("%d document(s) staged", result.Staged), "info", "ingest", collectionID, 0, 0, batchID)
	if jobID > 0 {
		_ = store.UpdateIngestionJob(s.DB, jobID, "running", "embedding", 50, result.Staged, 0, "staging complete")
		_ = store.AddIngestionLog(s.DB, jobID, 0, collectionID, batchID, "info", "staging", fmt.Sprintf("staged %d document(s)", result.Staged), 0, 0, 0)
	}

	if result.Staged == 0 {
		s.emit("ingest:progress", map[string]any{
			"phase": "batch_done", "step": "complete",
			"label": "Nothing to embed", "pct": 100, "batchId": batchID,
		})
		s.recordEvent("ingest:batch_empty", "Nothing to embed", "Stage phase completed without any staged documents", "info", "ingest", collectionID, 0, 0, batchID)
		return result, nil
	}

	// ── Phase B: embed all queued docs in this batch ──────────────────────
	completed, failed, cancelled := s.embedResumable(ctx, batchID, jobID)
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
	if jobID > 0 {
		status := "completed"
		if cancelled {
			status = "paused"
		} else if failed > 0 {
			status = "failed"
		}
		_ = store.UpdateIngestionJob(s.DB, jobID, status, "completed", 100, completed, failed, label)
	}
	s.recordEvent("ingest:batch_complete", "Ingest batch complete", fmt.Sprintf("%d ready, %d failed", completed, failed), "info", "ingest", collectionID, 0, 0, batchID)
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
	var jobID int64
	if job, err := store.GetIngestionJobByBatchID(s.DB, batchID); err == nil && job != nil {
		jobID = job.ID
	}
	completed, failed, cancelled := s.embedResumable(ctx, batchID, jobID)
	result := &IngestBatchResult{
		BatchID:   batchID,
		Completed: completed,
		Failed:    failed,
		Cancelled: cancelled,
	}
	s.recordEvent("ingest:batch_complete", "Resume complete", fmt.Sprintf("%d ready, %d failed", completed, failed), "info", "ingest", 0, 0, 0, batchID)
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

func (s *ChatService) stageOne(f IngestFilePayload, collectionID int64, batchID string, jobID int64) StageResult {
	filename := strings.TrimSpace(f.Filename)
	if filename == "" {
		filename = "document.txt"
	}

	loaded, err := s.extractDocument(f, filename)
	if err != nil {
		return StageResult{Filename: filename, Status: "error", Message: err.Error()}
	}
	text := strings.TrimSpace(loaded.Text)
	if text == "" {
		return StageResult{Filename: filename, Status: "error", Message: "No extractable text found"}
	}

	meta := toStoreIngestMetadata(loaded.Metadata)
	if meta.Title == "" {
		meta.Title = filename
	}
	hash := ingest.HashNormalizedText(text)
	plan := ingest.BuildChunkPlan(text, meta.Title, chunkSize, chunkOverlap)
	if len(plan) == 0 {
		return StageResult{Filename: filename, Status: "error", Message: "No content to ingest"}
	}
	expected := len(plan)
	chunkHashes := make([]string, 0, len(plan))
	for _, c := range plan {
		if c.Role == "summary" {
			continue
		}
		chunkHashes = append(chunkHashes, ingest.HashChunkContent(c.Content))
	}
	if matches, err := store.FindPotentialDuplicates(s.DB, collectionID, filename, hash, chunkHashes, 5); err == nil && len(matches) > 0 && !f.Replace {
		best := matches[0]
		return StageResult{
			Filename: filename,
			Status:   "duplicate",
			Message:  fmt.Sprintf("Possible %s duplicate of %s (%s)", best.Kind, best.Filename, best.Reason),
			DocID:    best.DocumentID,
		}
	}

	existing, err := store.GetDocumentByHash(s.DB, hash, collectionID)
	if err != nil {
		return StageResult{Filename: filename, Status: "error", Message: "Database error checking duplicates"}
	}
	globalExisting, _ := store.GetDocumentByHashAny(s.DB, hash)
	if globalExisting != nil && globalExisting.CollectionID != collectionID && !f.Replace {
		return StageResult{Filename: filename, Status: "duplicate", Message: fmt.Sprintf("Already exists in another collection: %s", globalExisting.Filename), DocID: globalExisting.ID}
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
		// Replace or re-queue incomplete same-hash doc without wiping preserved chunks.
		if err := store.UpdateDocumentContent(s.DB, existing.ID, text, hash); err != nil {
			return StageResult{Filename: filename, Status: "error", Message: "Failed to update document content"}
		}
		if err := store.UpdateDocumentMetadata(s.DB, existing.ID, meta); err != nil {
			return StageResult{Filename: filename, Status: "error", Message: "Failed to update document metadata"}
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

	docID, err := store.AddDocumentWithMetadata(s.DB, collectionID, filename, hash, text, store.DocStatusQueued, batchID, expected, meta)
	if err != nil {
		return StageResult{Filename: filename, Status: "error", Message: fmt.Sprintf("Registering document: %v", err)}
	}
	return StageResult{Filename: filename, Status: "staged", Message: "Queued for embedding", DocID: docID}
}

func (s *ChatService) extractDocument(f IngestFilePayload, filename string) (ingest.LoadedDocument, error) {
	if strings.TrimSpace(f.TextContent) != "" {
		return ingest.LoadDocumentBytes([]byte(f.TextContent), filename)
	}
	if strings.TrimSpace(f.Base64Data) == "" {
		return ingest.LoadedDocument{}, fmt.Errorf("no file data or text content provided")
	}
	data, err := base64.StdEncoding.DecodeString(f.Base64Data)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(f.Base64Data)
		if err != nil {
			return ingest.LoadedDocument{}, fmt.Errorf("failed to decode file data")
		}
	}
	return ingest.LoadDocumentBytes(data, filename)
}

// embedResumable embeds all resumable documents. batchID "" or "resume" = all.
func (s *ChatService) embedResumable(ctx context.Context, batchID string, jobID int64) (completed, failed int, cancelled bool) {
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

		if err := s.embedOneDocument(ctx, &docs[di], di+1, len(docs), jobID); err != nil {
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
		s.recordEvent("ingest:doc_ready", "Document ready", fmt.Sprintf("%s ready with %d chunks", doc.Filename, doc.ExpectedChunks), "info", "ingest", doc.CollectionID, 0, doc.ID, doc.BatchID)
	}
	return completed, failed, false
}

func (s *ChatService) embedOneDocument(ctx context.Context, doc *store.Document, docIndex, docTotal int, jobID int64) error {
	_ = store.UpdateDocumentStatus(s.DB, doc.ID, store.DocStatusEmbedding, "")

	chunks := ingest.BuildChunkPlan(doc.Content, doc.Title, chunkSize, chunkOverlap)
	total := len(chunks)
	if total == 0 {
		return fmt.Errorf("no content to embed")
	}
	if doc.ExpectedChunks != total {
		_ = store.UpdateDocumentIngest(s.DB, doc.ID, store.DocStatusEmbedding, total, doc.BatchID, "")
		doc.ExpectedChunks = total
	}

	docStart := time.Now()
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

	existingChunks, _ := store.GetChunksByDocument(s.DB, doc.ID)
	existingByOrd := make(map[int]store.ChunkRecord, len(existingChunks))
	for _, c := range existingChunks {
		existingByOrd[c.Ord] = c
	}

	for i, chunk := range chunks {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		chunkHash := ingest.HashChunkContent(chunk.Content)
		isSummary := chunk.Role == "summary"
		if existing, ok := existingByOrd[chunk.Ord]; ok {
			if existing.ChunkHash == chunkHash {
				if isSummary || existing.EmbeddingHash != "" {
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
			}
			if err := store.DeleteChunk(s.DB, existing.ID); err != nil {
				return fmt.Errorf("deleting stale chunk %d: %w", i+1, err)
			}
		}

		var embedding []float32
		if !isSummary {
			var err error
			embedding, err = s.Engine.Embed(chunk.Content)
			if err != nil {
				return fmt.Errorf("embedding chunk %d: %w", i+1, err)
			}
		}
		if _, err := store.InsertChunkWithHierarchy(s.DB, doc.ID, doc.CollectionID, chunk.Content, chunk.Ord, chunk.Level, chunk.Role, chunk.ParentOrd, chunk.PrevOrd, chunk.NextOrd, chunk.Summary, chunk.HeadingPath, chunkHash, embedding); err != nil {
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
			"role":     chunk.Role,
		})
	}

	if len(existingChunks) > total {
		for _, stale := range existingChunks[total:] {
			_ = store.DeleteChunk(s.DB, stale.ID)
		}
	}

	// Verify completeness
	cnt, err := store.CountChunks(s.DB, doc.ID)
	if err != nil {
		return err
	}
	if cnt < total {
		s.recordEvent("ingest:doc_incomplete", "Document incomplete", fmt.Sprintf("%s wrote %d/%d chunks", doc.Filename, cnt, total), "error", "ingest", doc.CollectionID, 0, doc.ID, doc.BatchID)
		return fmt.Errorf("incomplete: %d/%d chunks written", cnt, total)
	}
	if jobID > 0 {
		s.recordIngestLog(jobID, doc.ID, doc.CollectionID, doc.BatchID, "info", "completed", fmt.Sprintf("%s ready", doc.Filename), docStart, float64(total))
	}
	if err := store.UpdateDocumentStatus(s.DB, doc.ID, store.DocStatusReady, ""); err != nil {
		return err
	}
	s.recordEvent("ingest:doc_ready", "Document ready", fmt.Sprintf("%s ready with %d chunks", doc.Filename, total), "info", "ingest", doc.CollectionID, 0, doc.ID, doc.BatchID)
	return nil
}
