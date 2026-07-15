package agent

// ToolSpec describes an available capability that the agent can reason about.
// The current integration uses these as prompt-time capabilities; the runtime
// can later be extended to execute them directly.
type ToolSpec struct {
	Name        string
	Description string
	Usage       string
}

// DefaultTools returns the first capabilities the agent should be aware of.
func DefaultTools() []ToolSpec {
	return []ToolSpec{
		{
			Name:        "retrieve_documents",
			Description: "Search the local collections using hybrid retrieval and return the most relevant chunks.",
			Usage:       "Use for questions that depend on uploaded files, summaries, citations, or collection contents.",
		},
		{
			Name:        "conversation_memory",
			Description: "Use the conversation history to resolve follow-up questions and references to earlier turns.",
			Usage:       "Use for pronouns, short follow-ups, and questions that refer to 'that', 'it', or earlier context.",
		},
		{
			Name:        "workspace_memory",
			Description: "Review workspace memory such as recent uploads, active collection context, and prior app activity.",
			Usage:       "Use for questions about current work, prior uploads, or earlier app state that is not in the current turn history.",
		},
	}
}
