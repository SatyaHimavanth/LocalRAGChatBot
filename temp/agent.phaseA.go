package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// EvidenceEffort controls how aggressively the agent should gather evidence.
// It intentionally refers to the retrieval/evidence pipeline, not model reasoning.
type EvidenceEffort string

const (
	EvidenceEffortLow    EvidenceEffort = "low"
	EvidenceEffortMedium EvidenceEffort = "medium"
	EvidenceEffortHigh   EvidenceEffort = "high"
)

// NormalizeEvidenceEffort converts an arbitrary string into a supported effort level.
func NormalizeEvidenceEffort(raw string) EvidenceEffort {
	switch EvidenceEffort(strings.ToLower(strings.TrimSpace(raw))) {
	case EvidenceEffortLow:
		return EvidenceEffortLow
	case EvidenceEffortHigh:
		return EvidenceEffortHigh
	default:
		return EvidenceEffortMedium
	}
}

// String returns the stable wire value.
func (e EvidenceEffort) String() string {
	switch e {
	case EvidenceEffortLow:
		return "low"
	case EvidenceEffortHigh:
		return "high"
	default:
		return "medium"
	}
}

// Label returns a human-friendly label for the UI.
func (e EvidenceEffort) Label() string {
	switch e {
	case EvidenceEffortLow:
		return "Low"
	case EvidenceEffortHigh:
		return "High"
	default:
		return "Medium"
	}
}

// TopK controls the initial candidate pool size for evidence gathering.
func (e EvidenceEffort) TopK() int {
	switch e {
	case EvidenceEffortLow:
		return 4
	case EvidenceEffortHigh:
		return 12
	default:
		return 8
	}
}

// CandidatePoolSize controls the broader evidence pool before compression.
func (e EvidenceEffort) CandidatePoolSize() int {
	switch e {
	case EvidenceEffortLow:
		return 12
	case EvidenceEffortHigh:
		return 36
	default:
		return 24
	}
}

// NeighborDepth controls how many adjacent chunks to include around each hit.
func (e EvidenceEffort) NeighborDepth() int {
	switch e {
	case EvidenceEffortLow:
		return 0
	case EvidenceEffortHigh:
		return 2
	default:
		return 1
	}
}

// ContextMultiplier slightly expands the prompt budget as evidence effort rises.
func (e EvidenceEffort) ContextMultiplier() float64 {
	switch e {
	case EvidenceEffortLow:
		return 0.85
	case EvidenceEffortHigh:
		return 1.20
	default:
		return 1.00
	}
}

// EvidencePasses returns the maximum number of retrieval refinement passes.
func (e EvidenceEffort) EvidencePasses() int {
	switch e {
	case EvidenceEffortLow:
		return 1
	case EvidenceEffortHigh:
		return 3
	default:
		return 2
	}
}

// EvidenceLimit returns the maximum number of chunks kept after compression.
func (e EvidenceEffort) EvidenceLimit() int {
	switch e {
	case EvidenceEffortLow:
		return 6
	case EvidenceEffortHigh:
		return 16
	default:
		return 10
	}
}

// MaxEvidenceTokens returns the approximate token budget for retrieved evidence.
func (e EvidenceEffort) MaxEvidenceTokens() int {
	switch e {
	case EvidenceEffortLow:
		return 6000
	case EvidenceEffortHigh:
		return 20000
	default:
		return 12000
	}
}

// MaxEvidenceLatency returns the soft time budget for evidence construction.
func (e EvidenceEffort) MaxEvidenceLatency() time.Duration {
	switch e {
	case EvidenceEffortLow:
		return 650 * time.Millisecond
	case EvidenceEffortHigh:
		return 4 * time.Second
	default:
		return 1500 * time.Millisecond
	}
}

// CoverageThreshold is the minimum acceptable coverage before we stop
// expanding the evidence bundle.
func (e EvidenceEffort) CoverageThreshold() float64 {
	switch e {
	case EvidenceEffortLow:
		return 0.55
	case EvidenceEffortHigh:
		return 0.85
	default:
		return 0.72
	}
}

// Guidance returns a short instruction line for the system prompt.
func (e EvidenceEffort) Guidance() string {
	switch e {
	case EvidenceEffortLow:
		return "Evidence effort is low. Prefer a faster, narrower evidence bundle and keep the answer concise."
	case EvidenceEffortHigh:
		return "Evidence effort is high. Gather broader neighboring context, favor accuracy over speed, and verify the evidence before answering."
	default:
		return "Evidence effort is medium. Balance speed and coverage with a moderately broad evidence bundle."
	}
}

// Agent ties persona, tools and planner together.
// It intentionally stays lightweight so the app can swap models without
// changing the high-level chat programming model.
type Agent struct {
	Persona Persona
	Tools   []ToolSpec
	Planner Planner

	mu sync.RWMutex
}

// Option configures an Agent.
type Option func(*Agent)

// New builds an agent with sensible defaults.
func New(opts ...Option) *Agent {
	a := &Agent{
		Persona: DefaultPersona(),
		Tools:   DefaultTools(),
		Planner: NewHeuristicPlanner(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(a)
		}
	}
	if a.Planner == nil {
		a.Planner = NewHeuristicPlanner()
	}
	if len(a.Tools) == 0 {
		a.Tools = DefaultTools()
	}
	return a
}

// WithPersona overrides the default persona.
func WithPersona(p Persona) Option {
	return func(a *Agent) { a.Persona = p }
}

// WithTools overrides the default capability list.
func WithTools(tools []ToolSpec) Option {
	return func(a *Agent) { a.Tools = append([]ToolSpec(nil), tools...) }
}

// WithPlanner overrides the default planner.
func WithPlanner(planner Planner) Option {
	return func(a *Agent) { a.Planner = planner }
}

// DefaultTools returns the first capabilities the agent should be aware of.
func DefaultTools() []ToolSpec {
	return []ToolSpec{
		{
			Name:        "retrieve_documents",
			Description: "Search the active collection, expand neighboring chunks, and return grounded evidence for the answer.",
			Schema:      `{"query":"string","collectionId":"number","effort":"low|medium|high","limit":"number"}`,
		},
		{
			Name:        "conversation_history",
			Description: "Inspect recent conversation turns, previous questions, and branch context before answering follow-ups.",
			Schema:      `{"sessionId":"number","branchId":"number","limit":"number"}`,
		},
		{
			Name:        "workspace_memory",
			Description: "Review workspace memory such as recent uploads, open documents, and prior app context.",
			Schema:      `{"sessionId":"number","collectionId":"number"}`,
		},
	}
}

// Decide returns the plan for a request.
func (a *Agent) Decide(req Request) Plan {
	a.mu.RLock()
	planner := a.Planner
	persona := a.Persona
	tools := append([]ToolSpec(nil), a.Tools...)
	a.mu.RUnlock()
	if planner == nil {
		planner = NewHeuristicPlanner()
	}
	return planner.Decide(req, persona, tools)
}

// RenderSystemPrompt composes the system prompt from persona, tools and state.
func (a *Agent) RenderSystemPrompt(plan Plan, collectionName string, docContext string, workspaceMemory string, evidenceSummary string) string {
	a.mu.RLock()
	persona := a.Persona
	tools := append([]ToolSpec(nil), a.Tools...)
	a.mu.RUnlock()
	return persona.RenderSystemPrompt(plan, tools, collectionName, docContext, workspaceMemory, evidenceSummary)
}

// ToolPrompt returns a compact tool overview suitable for the system prompt.
func ToolPrompt(tools []ToolSpec) string {
	if len(tools) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Available capabilities:\n")
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(name)
		if d := strings.TrimSpace(tool.Description); d != "" {
			b.WriteString(": ")
			b.WriteString(d)
		}
		if schema := strings.TrimSpace(tool.Schema); schema != "" {
			b.WriteString("\n  schema: ")
			b.WriteString(schema)
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}
