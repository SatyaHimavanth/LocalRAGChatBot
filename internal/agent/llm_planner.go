package agent

import (
	"context"
	"strings"
)

// Completer is the minimal surface LLMPlanner needs from the inference
// engine. It is intentionally narrow (one string in, one string out) so the
// planner package does not need to depend on llama.ChatOptions/ChatContext
// directly, and so it can be faked in tests.
type Completer interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error)
}

// LLMPlanner asks the loaded model to classify the turn instead of guessing
// from keywords. It always falls back to a HeuristicPlanner so a slow,
// malformed, or failed router call degrades gracefully instead of blocking
// the turn.
type LLMPlanner struct {
	Completer Completer
	Fallback  *HeuristicPlanner
	TopK      int
}

// NewLLMPlanner builds an LLMPlanner with a sensible fallback.
func NewLLMPlanner(c Completer) *LLMPlanner {
	return &LLMPlanner{Completer: c, Fallback: NewHeuristicPlanner(), TopK: 5}
}

const routerSystemPrompt = `You are a routing classifier for a local RAG chat app. Given the user's latest message and a little context, output exactly one line in this format:

ACTION: <RETRIEVE|MEMORY|DIRECT>
QUERY: <search phrase, only if ACTION is RETRIEVE, else empty>

RETRIEVE - the user is asking about their uploaded documents/collection, or a follow-up that depends on them, AND documents are available.
MEMORY - the user is referring back to something earlier in this conversation, and no document lookup is needed.
DIRECT - a fresh question, greeting, or general-knowledge request that needs neither documents nor prior turns.

If no documents are available, never output RETRIEVE. Output nothing else.`

// Decide implements Planner using a live model call, with a heuristic
// fallback on any failure.
func (lp *LLMPlanner) Decide(ctx context.Context, req Request, persona Persona, tools []ToolSpec) (Plan, error) {
	topK := lp.TopK
	if topK == 0 {
		topK = 5
	}
	fallback := lp.Fallback
	if fallback == nil {
		fallback = NewHeuristicPlanner()
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" || lp.Completer == nil {
		return fallback.Decide(ctx, req, persona, tools)
	}

	var userPrompt strings.Builder
	userPrompt.WriteString("Documents available: ")
	if req.HasDocuments {
		userPrompt.WriteString("yes")
	} else {
		userPrompt.WriteString("no")
	}
	userPrompt.WriteString("\nRecent turns:\n")
	start := 0
	if len(req.History) > 4 {
		start = len(req.History) - 4
	}
	for _, h := range req.History[start:] {
		userPrompt.WriteString(h.Role)
		userPrompt.WriteString(": ")
		userPrompt.WriteString(truncate(h.Content, 200))
		userPrompt.WriteByte('\n')
	}
	userPrompt.WriteString("Latest message: ")
	userPrompt.WriteString(prompt)

	raw, err := lp.Completer.Complete(ctx, routerSystemPrompt, userPrompt.String(), 40)
	if err != nil {
		return fallback.Decide(ctx, req, persona, tools)
	}

	action, query := parseRouterOutput(raw)
	plan := Plan{TopK: topK}
	switch action {
	case "RETRIEVE":
		if !req.HasDocuments {
			// Model ignored the instruction; don't trust it over reality.
			return fallback.Decide(ctx, req, persona, tools)
		}
		plan.UseRetrieval = true
		if query == "" {
			query = prompt
		}
		plan.RetrievalQuery = rewriteQuery(query, req.History)
		plan.Reason = "llm planner: retrieve"
	case "MEMORY":
		plan.UseMemory = true
		plan.Reason = "llm planner: memory"
	case "DIRECT":
		plan.UseDirect = true
		plan.Reason = "llm planner: direct"
	default:
		// Model didn't follow the format; don't guess, fall back.
		return fallback.Decide(ctx, req, persona, tools)
	}
	return plan, nil
}

func parseRouterOutput(raw string) (action, query string) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "ACTION:"):
			v := strings.ToUpper(strings.TrimSpace(line[len("ACTION:"):]))
			switch {
			case strings.Contains(v, "RETRIEVE"):
				action = "RETRIEVE"
			case strings.Contains(v, "MEMORY"):
				action = "MEMORY"
			case strings.Contains(v, "DIRECT"):
				action = "DIRECT"
			}
		case strings.HasPrefix(upper, "QUERY:"):
			query = strings.TrimSpace(line[len("QUERY:"):])
		}
	}
	return action, query
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
