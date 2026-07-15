package app

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"changeme/internal/agent"
	"changeme/internal/store"

	llama "github.com/tcpipuk/llama-go"
)

type EvidenceBundleStats struct {
	Effort          string  `json:"effort"`
	CandidateCount  int     `json:"candidateCount"`
	ExpandedCount   int     `json:"expandedCount"`
	CompressedCount int     `json:"compressedCount"`
	FinalCount      int     `json:"finalCount"`
	Passes          int     `json:"passes"`
	Coverage        float64 `json:"coverage"`
	EstimatedTokens int     `json:"estimatedTokens"`
	TokenBudget     int     `json:"tokenBudget"`
	TimeBudgetMS    int64   `json:"timeBudgetMs"`
}

type EvidenceBundleReport struct {
	Query   string
	Chunks  []store.ScoredChunk
	Summary string
	Stats   EvidenceBundleStats
}

type chunkLocation struct {
	DocumentID int64
	Ord        int64
	Content    string
}

func extractTurnControls(prompt string) (string, int64, agent.EvidenceEffort, string) {
	trimmed := strings.TrimSpace(prompt)
	parentID := int64(0)
	effort := agent.EvidenceEffortMedium
	scope := agent.RetrievalScopeCurrent.String()

	lines := strings.Split(trimmed, "\n")
	bodyStart := 0
	for bodyStart < len(lines) {
		line := strings.TrimSpace(lines[bodyStart])
		if line == "" {
			bodyStart++
			continue
		}
		if strings.HasPrefix(line, "[[branch-parent:") {
			if end := strings.Index(line, "]]"); end > 0 {
				idStr := strings.TrimPrefix(line[:end], "[[branch-parent:")
				if id, err := parseInt64(idStr); err == nil {
					parentID = id
				}
				bodyStart++
				continue
			}
		}
		if strings.HasPrefix(line, "[[effort:") {
			if end := strings.Index(line, "]]"); end > 0 {
				raw := strings.TrimPrefix(line[:end], "[[effort:")
				effort = agent.NormalizeEvidenceEffort(raw)
				bodyStart++
				continue
			}
		}
		if strings.HasPrefix(line, "[[scope:") {
			if end := strings.Index(line, "]]"); end > 0 {
				raw := strings.TrimPrefix(line[:end], "[[scope:")
				scope = agent.NormalizeRetrievalScope(raw).String()
				bodyStart++
				continue
			}
		}
		break
	}

	body := strings.Join(lines[bodyStart:], "\n")
	return strings.TrimSpace(body), parentID, effort, scope
}

func parseInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty integer")
	}
	var v int64
	_, err := fmt.Sscanf(raw, "%d", &v)
	return v, err
}

func (s *ChatService) buildEvidenceBundle(ctx context.Context, sessionID int64, collectionID int64, query string, history []llama.ChatMessage, effort agent.EvidenceEffort, plan agent.Plan) EvidenceBundleReport {
	report := EvidenceBundleReport{Query: strings.TrimSpace(query)}
	stats := EvidenceBundleStats{
		Effort:       effort.String(),
		Passes:       effort.EvidencePasses(),
		TokenBudget:  effort.MaxEvidenceTokens(),
		TimeBudgetMS: effort.MaxEvidenceLatency().Milliseconds(),
	}

	if s.DB == nil {
		report.Stats = stats
		return report
	}

	start := time.Now()
	candidates := make([]store.ScoredChunk, 0)
	if s.Engine != nil {
		if emb, err := s.Engine.Embed(report.Query); err == nil {
			if hits, err := store.HybridSearch(s.DB, collectionID, report.Query, emb, effort.CandidatePoolSize()); err == nil {
				candidates = hits
			}
		}
	}
	if len(candidates) == 0 {
		if hits, err := store.FallbackSearch(s.DB, collectionID, report.Query, effort.CandidatePoolSize()); err == nil {
			candidates = hits
		}
	}
	stats.CandidateCount = len(candidates)

	expanded := s.expandEvidenceChunks(collectionID, candidates, effort.NeighborDepth())
	stats.ExpandedCount = len(expanded)

	compressed := s.trimEvidenceToBudget(expanded, effort.MaxEvidenceTokens())
	stats.CompressedCount = len(compressed)
	stats.FinalCount = len(compressed)
	stats.EstimatedTokens = s.estimateEvidenceTokens(compressed)
	stats.Coverage = s.evidenceCoverage(compressed, effort)
	stats.TimeBudgetMS = effort.MaxEvidenceLatency().Milliseconds()
	if len(compressed) > effort.EvidenceLimit() {
		compressed = compressed[:effort.EvidenceLimit()]
		stats.FinalCount = len(compressed)
		stats.EstimatedTokens = s.estimateEvidenceTokens(compressed)
	}
	if ctx != nil && ctx.Err() != nil {
		report.Stats = stats
		return report
	}
	report.Chunks = compressed
	report.Summary = s.summarizeEvidenceBundle(report.Query, compressed, stats)
	report.Stats = stats
	_ = history
	_ = plan
	_ = start
	return report
}

func (s *ChatService) expandEvidenceChunks(collectionID int64, base []store.ScoredChunk, depth int) []store.ScoredChunk {
	if s.DB == nil || depth <= 0 || len(base) == 0 {
		return base
	}
	seen := make(map[int64]store.ScoredChunk, len(base))
	for _, chunk := range base {
		seen[chunk.ChunkID] = chunk
	}

	for _, chunk := range base {
		loc, err := s.getChunkLocation(chunk.ChunkID, collectionID)
		if err != nil {
			continue
		}
		startOrd := loc.Ord - int64(depth)
		if startOrd < 1 {
			startOrd = 1
		}
		rows, err := s.DB.Query(`
			SELECT id, content, ord FROM chunks
			WHERE collection_id = ? AND document_id = ? AND ord BETWEEN ? AND ?
			ORDER BY ord ASC
		`, collectionID, loc.DocumentID, startOrd, loc.Ord+int64(depth))
		if err != nil {
			continue
		}
		for rows.Next() {
			var id int64
			var content string
			var ord int64
			if err := rows.Scan(&id, &content, &ord); err != nil {
				continue
			}
			if id == chunk.ChunkID {
				continue
			}
			distance := math.Abs(float64(ord - loc.Ord))
			boost := 1.0 - (distance * 0.12)
			if boost < 0.55 {
				boost = 0.55
			}
			candidate := store.ScoredChunk{ChunkID: id, Content: content, Score: chunk.Score * boost}
			if existing, ok := seen[id]; !ok || candidate.Score > existing.Score {
				seen[id] = candidate
			}
		}
		rows.Close()
	}

	out := make([]store.ScoredChunk, 0, len(seen))
	for _, chunk := range seen {
		out = append(out, chunk)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ChunkID < out[j].ChunkID
		}
		return out[i].Score > out[j].Score
	})
	return out
}

func (s *ChatService) getChunkLocation(chunkID int64, collectionID int64) (chunkLocation, error) {
	var loc chunkLocation
	if s.DB == nil {
		return loc, fmt.Errorf("database not initialized")
	}
	row := s.DB.QueryRow(`
		SELECT document_id, ord, content FROM chunks
		WHERE id = ? AND collection_id = ?
	`, chunkID, collectionID)
	if err := row.Scan(&loc.DocumentID, &loc.Ord, &loc.Content); err != nil {
		return loc, err
	}
	return loc, nil
}

func (s *ChatService) buildWorkspaceMemory(sessionID int64, collectionID int64) string {
	if s.DB == nil {
		return ""
	}
	snap, err := s.buildWorkspaceSnapshot(sessionID)
	if err != nil || snap == nil {
		return ""
	}
	var lines []string
	if summary := strings.TrimSpace(snap.Summary); summary != "" {
		lines = append(lines, "Workspace summary:")
		lines = append(lines, summary)
	}
	if notes := strings.TrimSpace(snap.Notes); notes != "" {
		lines = append(lines, "User workspace notes:")
		lines = append(lines, notes)
	}
	if len(snap.RecentQuestions) > 0 {
		lines = append(lines, "Recent questions in this conversation:")
		for i, q := range snap.RecentQuestions {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, q))
		}
	}
	if collectionID > 0 && len(snap.RecentDocuments) > 0 {
		lines = append(lines, "Recent documents in the active collection:")
		for _, d := range snap.RecentDocuments {
			lines = append(lines, "- "+d)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (s *ChatService) estimateChunkTokens(content string) int {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0
	}
	tokens := len(content) / 4
	if tokens <= 0 {
		tokens = 1
	}
	return tokens
}

func (s *ChatService) estimateEvidenceTokens(chunks []store.ScoredChunk) int {
	total := 0
	for _, chunk := range chunks {
		total += s.estimateChunkTokens(chunk.Content)
	}
	return total
}

func (s *ChatService) evidenceCoverage(chunks []store.ScoredChunk, effort agent.EvidenceEffort) float64 {
	budget := effort.MaxEvidenceTokens()
	if budget <= 0 {
		budget = effort.EvidenceLimit() * 120
	}
	if budget <= 0 {
		budget = 1
	}
	coverage := float64(s.estimateEvidenceTokens(chunks)) / float64(budget)
	if coverage > 1 {
		coverage = 1
	}
	return coverage
}

func (s *ChatService) trimEvidenceToBudget(chunks []store.ScoredChunk, tokenBudget int) []store.ScoredChunk {
	if tokenBudget <= 0 || len(chunks) == 0 {
		return chunks
	}
	sorted := append([]store.ScoredChunk(nil), chunks...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Score == sorted[j].Score {
			return sorted[i].ChunkID < sorted[j].ChunkID
		}
		return sorted[i].Score > sorted[j].Score
	})
	kept := make([]store.ScoredChunk, 0, len(sorted))
	total := 0
	for _, chunk := range sorted {
		tokens := s.estimateChunkTokens(chunk.Content)
		if len(kept) > 0 && total+tokens > tokenBudget {
			continue
		}
		kept = append(kept, chunk)
		total += tokens
		if total >= tokenBudget {
			break
		}
	}
	if len(kept) == 0 {
		if len(sorted) > 0 {
			return []store.ScoredChunk{sorted[0]}
		}
		return nil
	}
	return kept
}

func (s *ChatService) summarizeEvidenceBundle(query string, chunks []store.ScoredChunk, stats EvidenceBundleStats) string {
	if len(chunks) == 0 {
		return fmt.Sprintf("Evidence bundle empty for query %q.", strings.TrimSpace(query))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Query: %s\n", strings.TrimSpace(query))
	fmt.Fprintf(&b, "Passes: %d | Coverage: %.0f%% | Evidence budget: %d tokens | Kept: %d chunks\n", stats.Passes, stats.Coverage*100, stats.TokenBudget, stats.FinalCount)
	for i, chunk := range chunks {
		if i >= 3 {
			break
		}
		snippet := strings.TrimSpace(chunk.Content)
		if len(snippet) > 140 {
			snippet = snippet[:140] + "..."
		}
		fmt.Fprintf(&b, "[%d] %s\n", i+1, snippet)
	}
	return strings.TrimSpace(b.String())
}

func (s *ChatService) evidencePreview(chunks []store.ScoredChunk, collectionID int64) []map[string]any {
	if len(chunks) == 0 {
		return nil
	}
	preview := make([]map[string]any, 0, min(3, len(chunks)))
	for i, chunk := range chunks {
		if i >= 3 {
			break
		}
		label, section, summary := s.getChunkPreviewMeta(chunk.ChunkID)
		entry := map[string]any{
			"chunkId":      chunk.ChunkID,
			"filename":     s.getChunkFilename(chunk.ChunkID),
			"score":        normalizeMatchScore(chunk.Score),
			"content":      truncateSnippet(chunk.Content, 120),
			"collectionId": collectionID,
		}
		if label != "" {
			entry["title"] = label
		}
		if section != "" {
			entry["sectionPath"] = section
		}
		if summary != "" {
			entry["summary"] = truncateSnippet(summary, 160)
		}
		preview = append(preview, entry)
	}
	return preview
}

func (s *ChatService) getChunkPreviewMeta(chunkID int64) (title, section, summary string) {
	if s.DB == nil {
		return "", "", ""
	}
	rec, err := store.GetChunkByID(s.DB, chunkID)
	if err != nil || rec == nil {
		return "", "", ""
	}
	return strings.TrimSpace(rec.Title), strings.TrimSpace(rec.SectionPath), strings.TrimSpace(rec.Summary)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncateSnippet(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return text[:limit-3] + "..."
}
