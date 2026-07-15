-- 0009_extension_hooks.sql
-- Future extension hook registry for GraphRAG, SQL agents, MCP, and OCR.

CREATE TABLE IF NOT EXISTS extension_hooks (
    id INTEGER PRIMARY KEY,
    hook_key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    hook_type TEXT NOT NULL,
    surface TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'planned',
    enabled INTEGER NOT NULL DEFAULT 0,
    config_json TEXT NOT NULL DEFAULT '{}',
    last_run_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_extension_hooks_surface ON extension_hooks(surface);
CREATE INDEX IF NOT EXISTS idx_extension_hooks_enabled ON extension_hooks(enabled);
CREATE INDEX IF NOT EXISTS idx_extension_hooks_state ON extension_hooks(state);

INSERT OR IGNORE INTO extension_hooks (hook_key, name, hook_type, surface, description, state, enabled, config_json, last_run_at, created_at, updated_at) VALUES
    ('graphrag', 'GraphRAG hooks', 'graph_rag', 'retrieval', 'Graph expansion, entity linking, community summaries, and multi-hop evidence assembly.', 'planned', 0, '{"graph":"disabled","community_summary":"disabled","hop_limit":2}', 0, strftime('%s','now'), strftime('%s','now')),
    ('sql_agent', 'SQL agent hooks', 'sql_agent', 'analytics', 'Read-only analytical queries, table mapping, and structured result synthesis.', 'planned', 0, '{"mode":"read_only","max_rows":200,"dialect":"sqlite"}', 0, strftime('%s','now'), strftime('%s','now')),
    ('mcp', 'MCP integration hooks', 'mcp', 'tooling', 'External tool registry, tool-level auth, and selective capability exposure.', 'planned', 0, '{"transport":"stdio","auth":"selective","tool_groups":["open","protected"]}', 0, strftime('%s','now'), strftime('%s','now')),
    ('ocr', 'OCR pipeline hooks', 'ocr', 'ingestion', 'Layout-aware OCR for scanned PDFs, screenshots, and mixed-image documents.', 'planned', 0, '{"layout":"preserve","tables":"retain","images":"detect"}', 0, strftime('%s','now'), strftime('%s','now'));
