package app

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"changeme/internal/store"

	llama "github.com/tcpipuk/llama-go"
)

type EvidenceNode struct {
	RefNum         int     `json:"refNum"`
	ChunkID        int64   `json:"chunkId"`
	DocumentID     int64   `json:"documentId"`
	CollectionID   int64   `json:"collectionId"`
	CollectionName string  `json:"collectionName"`
	Filename       string  `json:"filename"`
	Role           string  `json:"role"`
	Kind           string  `json:"kind"`
	Relation       string  `json:"relation"`
	Ord            int     `json:"ord"`
	Level          int     `json:"level"`
	Score          float64 `json:"score"`
	Support        float64 `json:"support"`
	Content        string  `json:"content"`
	Summary        string  `json:"summary"`
	HeadingPath    string  `json:"headingPath"`
}

type EvidenceEdge struct {
	FromRef  int     `json:"fromRef"`
	ToRef    int     `json:"toRef"`
	Relation string  `json:"relation"`
	Weight   float64 `json:"weight"`
}

type EvidenceBundle struct {
	Query          string         `json:"query"`
	Prompt         string         `json:"prompt"`
	CollectionID   int64          `json:"collectionId"`
	CollectionName string         `json:"collectionName"`
	Nodes          []EvidenceNode `json:"nodes"`
	Edges          []EvidenceEdge `json:"edges"`
	Coverage       float64        `json:"coverage"`
	Confidence     float64        `json:"confidence"`
	Verified       bool           `json:"verified"`
	Verification   string         `json:"verification"`
	Gaps           []string       `json:"gaps,omitempty"`
	PrimaryCount   int            `json:"primaryCount"`
	ContextCount   int            `json:"contextCount"`
	SourceCount    int            `json:"sourceCount"`

	refs []chunkRef
}

func buildEvidenceBundle(db *sql.DB, collectionID int64, collectionName, query, prompt string, chunks []store.ScoredChunk, history []llama.ChatMessage) EvidenceBundle {
	bundle := EvidenceBundle{
		Query:          strings.TrimSpace(query),
		Prompt:         strings.TrimSpace(prompt),
		CollectionID:   collectionID,
		CollectionName: strings.TrimSpace(collectionName),
	}
	if db == nil || len(chunks) == 0 {
		bundle.Verification = "No retrieved evidence"
		bundle.Gaps = []string{"No retrieved chunks matched the query"}
		bundle.Confidence = 0.08
		return bundle
	}

	terms := evidenceTerms(query, prompt, history)
	if len(terms) == 0 {
		terms = evidenceTerms(query, prompt, nil)
	}

	type edgeSeed struct {
		fromChunkID int64
		toChunkID   int64
		relation    string
		weight      float64
	}

	nodeByChunkID := map[int64]*EvidenceNode{}
	edgeSeeds := make([]edgeSeed, 0, len(chunks)*3)

	for _, hit := range chunks {
		if hit.ChunkID == 0 {
			continue
		}
		center, err := store.GetChunkByID(db, hit.ChunkID)
		if err != nil || center == nil {
			continue
		}
		doc, _ := store.GetDocumentByID(db, center.DocumentID)
		filename := ""
		if doc != nil {
			filename = strings.TrimSpace(doc.Filename)
		}
		neighborhood, err := store.GetChunkNeighborhood(db, hit.ChunkID, 1)
		if err != nil {
			neighborhood = []store.ChunkRecord{*center}
		}

		primaryKey := center.ID
		for _, rec := range neighborhood {
			kind, relation := classifyEvidenceRelation(center, rec)
			if rec.ID == center.ID {
				kind = "primary"
				relation = "retrieved"
			}
			support := evidenceSupport(rec, hit.Score, terms, kind == "primary")
			node := newEvidenceNode(rec, hit, kind, relation, support, filename)
			if existing, ok := nodeByChunkID[rec.ID]; ok {
				if node.Support > existing.Support || node.Score > existing.Score {
					*existing = mergeEvidenceNode(*existing, node)
				}
			} else {
				copyNode := node
				nodeByChunkID[rec.ID] = &copyNode
			}
			if rec.ID != center.ID {
				edgeSeeds = append(edgeSeeds, edgeSeed{fromChunkID: primaryKey, toChunkID: rec.ID, relation: relation, weight: support})
			}
		}
	}

	if len(nodeByChunkID) == 0 {
		bundle.Verification = "Retrieved chunks could not be expanded into evidence"
		bundle.Gaps = []string{"Evidence expansion returned no usable chunks"}
		bundle.Confidence = 0.12
		return bundle
	}

	nodes := make([]EvidenceNode, 0, len(nodeByChunkID))
	for _, node := range nodeByChunkID {
		nodes = append(nodes, *node)
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		pi := evidenceKindPriority(nodes[i].Kind)
		pj := evidenceKindPriority(nodes[j].Kind)
		if pi != pj {
			return pi < pj
		}
		if nodes[i].Support == nodes[j].Support {
			if nodes[i].Score == nodes[j].Score {
				return nodes[i].ChunkID < nodes[j].ChunkID
			}
			return nodes[i].Score > nodes[j].Score
		}
		return nodes[i].Support > nodes[j].Support
	})
	for i := range nodes {
		nodes[i].RefNum = i + 1
	}

	refByChunkID := make(map[int64]int, len(nodes))
	for _, n := range nodes {
		refByChunkID[n.ChunkID] = n.RefNum
	}
	bundle.Edges = make([]EvidenceEdge, 0, len(edgeSeeds))
	for _, edge := range edgeSeeds {
		fromRef, okFrom := refByChunkID[edge.fromChunkID]
		toRef, okTo := refByChunkID[edge.toChunkID]
		if !okFrom || !okTo || fromRef == toRef {
			continue
		}
		bundle.Edges = append(bundle.Edges, EvidenceEdge{FromRef: fromRef, ToRef: toRef, Relation: edge.relation, Weight: edge.weight})
	}
	sort.SliceStable(bundle.Edges, func(i, j int) bool {
		if bundle.Edges[i].Weight == bundle.Edges[j].Weight {
			if bundle.Edges[i].FromRef == bundle.Edges[j].FromRef {
				return bundle.Edges[i].ToRef < bundle.Edges[j].ToRef
			}
			return bundle.Edges[i].FromRef < bundle.Edges[j].FromRef
		}
		return bundle.Edges[i].Weight > bundle.Edges[j].Weight
	})

	bundle.Nodes = nodes
	bundle.refs = make([]chunkRef, 0, len(nodes))
	for _, node := range nodes {
		content := strings.TrimSpace(node.Content)
		if content == "" {
			content = strings.TrimSpace(node.Summary)
		}
		bundle.refs = append(bundle.refs, chunkRef{refNum: node.RefNum, chunk: store.ScoredChunk{ChunkID: node.ChunkID, Content: content, Score: node.Score}})
	}
	bundle.PrimaryCount = countEvidenceKind(nodes, "primary")
	bundle.ContextCount = len(nodes) - bundle.PrimaryCount
	bundle.SourceCount = len(nodes)
	bundle.Coverage = evidenceCoverage(nodes, terms)
	bundle.Confidence, bundle.Verified, bundle.Verification, bundle.Gaps = evaluateEvidenceBundle(nodes, bundle.Edges, bundle.Coverage, bundle.PrimaryCount, bundle.ContextCount, terms)
	return bundle
}

func (b EvidenceBundle) sourceRefs() []chunkRef {
	if len(b.refs) == 0 {
		return nil
	}
	out := make([]chunkRef, len(b.refs))
	copy(out, b.refs)
	return out
}

func (b EvidenceBundle) ContextString(maxChars int) string {
	if len(b.Nodes) == 0 {
		return ""
	}
	var out strings.Builder
	if b.Verification != "" {
		out.WriteString("Evidence verdict: ")
		out.WriteString(b.Verification)
		out.WriteByte('\n')
	}
	if b.Confidence > 0 {
		out.WriteString(fmt.Sprintf("Evidence confidence: %.0f%%\n", math.Round(b.Confidence*100)))
	}
	if b.Coverage > 0 {
		out.WriteString(fmt.Sprintf("Coverage: %.0f%%\n", math.Round(b.Coverage*100)))
	}
	for _, node := range b.Nodes {
		block := formatEvidenceNode(node)
		if block == "" {
			continue
		}
		if maxChars > 0 && out.Len()+len(block)+2 > maxChars {
			if out.Len() > 0 {
				out.WriteString("\n")
			}
			truncated := block
			remaining := maxChars - out.Len()
			if remaining > 24 && len(truncated) > remaining-3 {
				truncated = truncated[:remaining-3] + "..."
			}
			out.WriteString(truncated)
			break
		}
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(block)
	}
	if len(b.Edges) > 0 {
		edgeBlock := formatEvidenceEdges(b.Edges)
		if edgeBlock != "" {
			if maxChars > 0 && out.Len()+len(edgeBlock)+2 > maxChars {
				return strings.TrimSpace(out.String())
			}
			out.WriteString("\n\n")
			out.WriteString(edgeBlock)
		}
	}
	if len(b.Gaps) > 0 {
		gapBlock := "Open gaps: " + strings.Join(b.Gaps, "; ")
		if maxChars <= 0 || out.Len()+len(gapBlock)+1 <= maxChars {
			if out.Len() > 0 {
				out.WriteString("\n\n")
			}
			out.WriteString(gapBlock)
		}
	}
	return strings.TrimSpace(out.String())
}

func evidenceTerms(query, prompt string, history []llama.ChatMessage) []string {
	merged := strings.TrimSpace(query + " " + prompt)
	if len(history) > 0 {
		for i := len(history) - 1; i >= 0 && i >= len(history)-3; i-- {
			merged += " " + history[i].Content
		}
	}
	terms := searchTerms(merged)
	if len(terms) == 0 {
		return nil
	}
	return terms
}

func evidenceKindPriority(kind string) int {
	switch kind {
	case "primary":
		return 0
	case "summary":
		return 1
	case "previous":
		return 2
	case "next":
		return 3
	case "context":
		return 4
	default:
		return 5
	}
}

func classifyEvidenceRelation(center, rec store.ChunkRecord) (kind string, relation string) {
	switch {
	case rec.Role == "summary":
		return "summary", "summarizes"
	case rec.ID == center.ID:
		return "primary", "retrieved"
	case rec.Ord < center.Ord:
		return "previous", "supports"
	case rec.Ord > center.Ord:
		return "next", "supports"
	default:
		return "context", "related"
	}
}

func newEvidenceNode(rec store.ChunkRecord, hit store.ScoredChunk, kind, relation, filename string, support float64) EvidenceNode {
	content := strings.TrimSpace(rec.Content)
	if rec.Role == "summary" && strings.TrimSpace(rec.Summary) != "" {
		content = strings.TrimSpace(rec.Summary)
	}
	return EvidenceNode{
		ChunkID:        rec.ID,
		DocumentID:     rec.DocumentID,
		CollectionID:   rec.CollectionID,
		Kind:           kind,
		Relation:       relation,
		Ord:            rec.Ord,
		Level:          rec.Level,
		Role:           rec.Role,
		Score:          hit.Score,
		Support:        support,
		Content:        content,
		Summary:        strings.TrimSpace(rec.Summary),
		HeadingPath:    strings.TrimSpace(rec.HeadingPath),
		Filename:       strings.TrimSpace(filename),
		CollectionName: "",
	}
}

func mergeEvidenceNode(existing, incoming EvidenceNode) EvidenceNode {
	if incoming.Support > existing.Support {
		existing.Support = incoming.Support
	}
	if incoming.Score > existing.Score {
		existing.Score = incoming.Score
	}
	if existing.Content == "" {
		existing.Content = incoming.Content
	}
	if existing.Summary == "" {
		existing.Summary = incoming.Summary
	}
	if existing.HeadingPath == "" {
		existing.HeadingPath = incoming.HeadingPath
	}
	if incoming.Kind == "primary" {
		existing.Kind = incoming.Kind
		existing.Relation = incoming.Relation
	}
	if existing.Role == "" {
		existing.Role = incoming.Role
	}
	if existing.Filename == "" {
		existing.Filename = incoming.Filename
	}
	return existing
}

func evidenceSupport(rec store.ChunkRecord, hitScore float64, terms []string, primary bool) float64 {
	content := strings.ToLower(strings.TrimSpace(rec.Content + " " + rec.Summary + " " + rec.HeadingPath))
	score := normalizeMatchScore(hitScore) * 0.55
	if primary {
		score += 0.25
	}
	for _, term := range terms {
		if term == "" {
			continue
		}
		if strings.Contains(strings.ToLower(rec.HeadingPath), term) {
			score += 0.12
		}
		if strings.Contains(content, term) {
			score += 0.18
		}
	}
	if rec.Role == "summary" {
		score += 0.08
	}
	if score > 1 {
		score = 1
	}
	return score
}

func evidenceCoverage(nodes []EvidenceNode, terms []string) float64 {
	if len(terms) == 0 || len(nodes) == 0 {
		return 0
	}
	matched := 0
	for _, term := range terms {
		if evidenceTermCovered(term, nodes) {
			matched++
		}
	}
	return float64(matched) / float64(len(terms))
}

func evidenceTermCovered(term string, nodes []EvidenceNode) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return false
	}
	for _, node := range nodes {
		if strings.Contains(strings.ToLower(node.Content), term) || strings.Contains(strings.ToLower(node.Summary), term) || strings.Contains(strings.ToLower(node.HeadingPath), term) || strings.Contains(strings.ToLower(node.Filename), term) {
			return true
		}
	}
	return false
}

func countEvidenceKind(nodes []EvidenceNode, kind string) int {
	count := 0
	for _, node := range nodes {
		if node.Kind == kind {
			count++
		}
	}
	return count
}

func evaluateEvidenceBundle(nodes []EvidenceNode, edges []EvidenceEdge, coverage float64, primaryCount, contextCount int, terms []string) (float64, bool, string, []string) {
	gaps := make([]string, 0, 4)
	if primaryCount == 0 {
		gaps = append(gaps, "No primary evidence chunks found")
	}
	if coverage < 0.35 && len(terms) > 0 {
		gaps = append(gaps, "Retrieved evidence weakly matches the query terms")
	}
	if primaryCount > 0 && len(nodes) <= 2 {
		gaps = append(gaps, "Evidence set is narrow")
	}
	if len(edges) == 0 && len(nodes) > 1 {
		gaps = append(gaps, "No explicit evidence relationships were discovered")
	}

	if len(nodes) == 0 {
		return 0.08, false, "Evidence unavailable", gaps
	}

	var supportSum float64
	for _, node := range nodes {
		supportSum += node.Support
	}
	supportAvg := supportSum / float64(len(nodes))
	diversity := math.Min(1, float64(distinctDocuments(nodes))/3.0)
	adjacencyBonus := 0.0
	if len(edges) > 0 {
		adjacencyBonus = math.Min(0.15, float64(len(edges))/10.0)
	}

	confidence := 0.12 + (coverage * 0.32) + (supportAvg * 0.34) + (diversity * 0.12) + adjacencyBonus
	if primaryCount > 0 {
		confidence += 0.06
	}
	if contextCount > primaryCount {
		confidence += 0.04
	}
	if confidence > 1 {
		confidence = 1
	}
	verified := confidence >= 0.45 && primaryCount > 0
	verdict := "Evidence is thin"
	switch {
	case verified && confidence >= 0.75:
		verdict = "Evidence strongly supports the answer"
	case verified:
		verdict = "Evidence supports the answer"
	case confidence >= 0.30:
		verdict = "Evidence partially supports the answer"
	}
	if !verified && len(gaps) == 0 {
		gaps = append(gaps, "Evidence confidence is low")
	}
	return confidence, verified, verdict, gaps
}

func distinctDocuments(nodes []EvidenceNode) int {
	seen := map[int64]struct{}{}
	for _, node := range nodes {
		if node.DocumentID == 0 {
			continue
		}
		seen[node.DocumentID] = struct{}{}
	}
	return len(seen)
}

func formatEvidenceNode(node EvidenceNode) string {
	parts := []string{fmt.Sprintf("[%d]", node.RefNum)}
	label := strings.TrimSpace(node.Filename)
	if label == "" {
		label = fmt.Sprintf("Chunk %d", node.ChunkID)
	}
	if node.Kind != "" && node.Kind != "primary" {
		label += " (" + node.Kind + ")"
	}
	if node.HeadingPath != "" {
		label += " — " + node.HeadingPath
	}
	if node.Support > 0 {
		label += fmt.Sprintf(" [support %.2f]", node.Support)
	}
	parts = append(parts, label)
	content := strings.TrimSpace(node.Content)
	if content == "" {
		content = strings.TrimSpace(node.Summary)
	}
	if content != "" {
		parts = append(parts, content)
	}
	if node.Relation != "" && node.Kind != "primary" {
		parts = append(parts, fmt.Sprintf("Relation: %s", node.Relation))
	}
	return strings.Join(parts, "\n")
}

func formatEvidenceEdges(edges []EvidenceEdge) string {
	if len(edges) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Evidence graph:\n")
	for _, edge := range edges {
		fmt.Fprintf(&b, "[%d] -> [%d] %s (%.2f)\n", edge.FromRef, edge.ToRef, edge.Relation, edge.Weight)
	}
	return strings.TrimSpace(b.String())
}
