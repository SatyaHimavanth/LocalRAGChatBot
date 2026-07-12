package agent

import (
	"strings"
	"sync"
)

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
func (a *Agent) RenderSystemPrompt(plan Plan, collectionName string, docContext string) string {
	a.mu.RLock()
	persona := a.Persona
	tools := append([]ToolSpec(nil), a.Tools...)
	a.mu.RUnlock()
	return persona.RenderSystemPrompt(plan, tools, collectionName, docContext)
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
		if usage := strings.TrimSpace(tool.Usage); usage != "" {
			b.WriteString(" (use: ")
			b.WriteString(usage)
			b.WriteString(")")
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}
