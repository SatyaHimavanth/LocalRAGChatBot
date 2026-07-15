package extension

// HookDescriptor defines one future-extension integration surface.
type HookDescriptor struct {
	Key            string
	Name           string
	HookType       string
	Surface        string
	Description    string
	DefaultState   string
	DefaultEnabled bool
	DefaultConfig  string
}

// DefaultHookDescriptors returns the canonical phase-8 extension surfaces.
func DefaultHookDescriptors() []HookDescriptor {
	return []HookDescriptor{
		{
			Key:            "graphrag",
			Name:           "GraphRAG hooks",
			HookType:       "graph_rag",
			Surface:        "retrieval",
			Description:    "Graph expansion, entity linking, community summaries, and multi-hop evidence assembly.",
			DefaultState:   "planned",
			DefaultEnabled: false,
			DefaultConfig:  `{"graph":"disabled","community_summary":"disabled","hop_limit":2}`,
		},
		{
			Key:            "sql_agent",
			Name:           "SQL agent hooks",
			HookType:       "sql_agent",
			Surface:        "analytics",
			Description:    "Read-only analytical queries, table mapping, and structured result synthesis.",
			DefaultState:   "planned",
			DefaultEnabled: false,
			DefaultConfig:  `{"mode":"read_only","max_rows":200,"dialect":"sqlite"}`,
		},
		{
			Key:            "mcp",
			Name:           "MCP integration hooks",
			HookType:       "mcp",
			Surface:        "tooling",
			Description:    "External tool registry, tool-level auth, and selective capability exposure.",
			DefaultState:   "planned",
			DefaultEnabled: false,
			DefaultConfig:  `{"transport":"stdio","auth":"selective","tool_groups":["open","protected"]}`,
		},
		{
			Key:            "ocr",
			Name:           "OCR pipeline hooks",
			HookType:       "ocr",
			Surface:        "ingestion",
			Description:    "Layout-aware OCR for scanned PDFs, screenshots, and mixed-image documents.",
			DefaultState:   "planned",
			DefaultEnabled: false,
			DefaultConfig:  `{"layout":"preserve","tables":"retain","images":"detect"}`,
		},
	}
}
