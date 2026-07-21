-- MCP server extension configuration and verified tool discovery state.
CREATE TABLE IF NOT EXISTS mcp_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL
);

INSERT OR IGNORE INTO mcp_settings (id, enabled, updated_at) VALUES (1, 0, strftime('%s','now'));

CREATE TABLE IF NOT EXISTS mcp_servers (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    config_json TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 0,
    verified INTEGER NOT NULL DEFAULT 0,
    tool_count INTEGER NOT NULL DEFAULT 0,
    tools_json TEXT NOT NULL DEFAULT '[]',
    last_error TEXT NOT NULL DEFAULT '',
    last_verified_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_mcp_servers_enabled ON mcp_servers(enabled, verified);
