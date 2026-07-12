package agent

import (
	"strings"

	llama "github.com/tcpipuk/llama-go"
)

// Request is the minimum input required for planning.
type Request struct {
	Prompt         string
	History        []llama.ChatMessage
	CollectionID   int64
	CollectionName string
	HasDocuments   bool
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
	Decide(req Request, persona Persona, tools []ToolSpec) Plan
}

// HeuristicPlanner provides a model-agnostic baseline planner.
type HeuristicPlanner struct {
	TopK int
}

// NewHeuristicPlanner returns a sensible default planner.
func NewHeuristicPlanner() *HeuristicPlanner {
	return &HeuristicPlanner{TopK: 5}
}

func (p *HeuristicPlanner) Decide(req Request, _ Persona, _ []ToolSpec) Plan {
	prompt := normalize(req.Prompt)
	plan := Plan{
		UseMemory: len(req.History) > 0,
		UseDirect: true,
		TopK:      p.TopK,
	}

	if prompt == "" {
		plan.Reason = "empty prompt"
		return plan
	}

	if isGreeting(prompt) || isSmallTalk(prompt) {
		plan.Reason = "small talk"
		return plan
	}

	if looksLikeDocumentQuery(prompt) || (req.HasDocuments && looksLikeFollowUp(prompt)) {
		plan.UseRetrieval = true
		plan.UseDirect = false
		plan.RetrievalQuery = rewriteQuery(prompt, req.History)
		plan.Reason = "document or follow-up query"
		return plan
	}

	if looksLikeGeneralFollowUp(prompt, req.History) {
		plan.Reason = "history answerable follow-up"
		return plan
	}

	if req.HasDocuments && maybeDocumentRelated(prompt) {
		plan.UseRetrieval = true
		plan.UseDirect = false
		plan.RetrievalQuery = rewriteQuery(prompt, req.History)
		plan.Reason = "possibly document related"
		return plan
	}

	plan.Reason = "general conversation"
	return plan
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func isGreeting(text string) bool {
	patterns := []string{
		"hi", "hello", "hey", "greetings", "howdy", "good morning", "good afternoon",
		"good evening", "good night", "what's up", "sup", "yo", "how are you",
		"how are you doing", "nice to meet you", "pleased to meet you",
	}
	cleaned := strings.TrimRight(text, ",.!? \t")
	for _, p := range patterns {
		if cleaned == p {
			return true
		}
	}
	return false
}

func isSmallTalk(text string) bool {
	patterns := []string{
		"thanks", "thank you", "bye", "goodbye", "take care", "see you", "see you later",
		"who are you", "what are you", "tell me about yourself", "what can you do",
	}
	for _, p := range patterns {
		if text == p || strings.Contains(text, p) {
			return true
		}
	}
	return false
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
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

func looksLikeFollowUp(text string) bool {
	patterns := []string{"that", "it", "this", "they", "those", "previous", "earlier", "above", "same", "what about", "how about", "and the", "and what", "also", "elaborate", "more detail", "continue"}
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return len(text) < 40
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
		if strings.Contains(text, p) {
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
