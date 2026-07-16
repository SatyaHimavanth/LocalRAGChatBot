package agent

import (
	"context"
	"strings"

	llama "github.com/tcpipuk/llama-go"
)

// Request is the minimum input required for planning.
type Request struct {
	Prompt          string
	History         []llama.ChatMessage
	CollectionID    int64
	CollectionName  string
	HasDocuments    bool
	WorkspaceMemory string
}

// Plan describes how the agent should answer the request.
type Plan struct {
	UseRetrieval   bool
	RetrievalQuery string
	UseMemory      bool
	UseDirect      bool
	TopK           int
	Reason         string
}

// Planner decides whether to answer directly or use retrieval.
type Planner interface {
	Decide(ctx context.Context, req Request, persona Persona, tools []ToolSpec) (Plan, error)
}

// HeuristicPlanner provides a model-agnostic baseline planner.
type HeuristicPlanner struct {
	TopK int
}

// NewHeuristicPlanner returns a sensible default planner.
func NewHeuristicPlanner() *HeuristicPlanner {
	return &HeuristicPlanner{TopK: 5}
}

func (p *HeuristicPlanner) Decide(_ context.Context, req Request, _ Persona, _ []ToolSpec) (Plan, error) {
	prompt := normalize(req.Prompt)
	plan := Plan{
		UseDirect: true,
		TopK:      p.TopK,
	}

	if prompt == "" {
		plan.Reason = "empty prompt"
		return plan, nil
	}

	if req.HasDocuments && (looksLikeDocumentQuery(prompt) || looksLikeFollowUp(prompt)) {
		plan.UseRetrieval = true
		plan.UseDirect = false
		plan.RetrievalQuery = rewriteQuery(prompt, req.History)
		plan.Reason = "document or follow-up query"
		return plan, nil
	}

	if !req.HasDocuments && looksLikeDocumentQuery(prompt) {
		plan.Reason = "document query but active collection is empty"
		return plan, nil
	}

	if looksLikeGeneralFollowUp(prompt, req.History) || (strings.TrimSpace(req.WorkspaceMemory) != "" && looksLikeFollowUp(prompt)) {
		plan.UseMemory = true
		plan.UseDirect = false
		plan.Reason = "history or workspace memory answerable follow-up"
		return plan, nil
	}

	if req.HasDocuments && maybeDocumentRelated(prompt) {
		plan.UseRetrieval = true
		plan.UseDirect = false
		plan.RetrievalQuery = rewriteQuery(prompt, req.History)
		plan.Reason = "possibly document related"
		return plan, nil
	}

	plan.Reason = "general conversation"
	return plan, nil
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func looksLikeDocumentQuery(text string) bool {
	patterns := []string{
		"document", "documents", "file", "files", "pdf", "collection", "collections",
		"uploaded", "upload", "source", "sources", "retrieve", "search", "find",
		"cite", "citation", "summarize", "summarise", "summary", "according to",
		"in the file", "in the document", "from the document", "based on the document",
		"what does it say", "what does this say", "what did the document say",
	}
	for _, p := range patterns {
		if containsWord(text, p) {
			return true
		}
	}
	return false
}

func looksLikeFollowUp(text string) bool {
	patterns := []string{"that", "it", "this", "they", "those", "previous", "earlier", "above", "same", "what about", "how about", "and the", "and what", "also", "elaborate", "more detail", "continue"}
	for _, p := range patterns {
		if containsWord(text, p) {
			return true
		}
	}
	return false
}

// containsWord matches p as a whole word/phrase rather than a raw substring,
// so e.g. "this" doesn't match inside "enthusiastic" and "it" doesn't match
// inside "editor".
func containsWord(text, p string) bool {
	idx := 0
	for {
		i := strings.Index(text[idx:], p)
		if i < 0 {
			return false
		}
		start := idx + i
		end := start + len(p)
		beforeOK := start == 0 || !isWordByte(text[start-1])
		afterOK := end == len(text) || !isWordByte(text[end])
		if beforeOK && afterOK {
			return true
		}
		idx = start + 1
	}
}

func isWordByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

func looksLikeGeneralFollowUp(text string, history []llama.ChatMessage) bool {
	if len(history) == 0 {
		return false
	}
	if looksLikeDocumentQuery(text) {
		return false
	}
	return looksLikeFollowUp(text)
}

func maybeDocumentRelated(text string) bool {
	patterns := []string{"explain", "tell me", "show me", "what is", "what are", "how does", "why did", "compare", "difference", "details"}
	for _, p := range patterns {
		if containsWord(text, p) {
			return true
		}
	}
	return false
}

func rewriteQuery(prompt string, history []llama.ChatMessage) string {
	base := strings.TrimSpace(prompt)
	if len(history) == 0 {
		return base
	}

	var parts []string
	parts = append(parts, base)

	// Pull in a small amount of recent context to rewrite ambiguous follow-ups.
	for i := len(history) - 1; i >= 0 && len(parts) < 4; i-- {
		msg := strings.TrimSpace(history[i].Content)
		if msg == "" {
			continue
		}
		if len(msg) > 160 {
			msg = msg[:160]
		}
		parts = append(parts, msg)
	}

	return strings.Join(parts, " | ")
}
