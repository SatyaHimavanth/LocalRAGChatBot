-- Per-tool MCP selection. All discovered tools start enabled, then users can
-- selectively remove high-cost tools from the agent capability prompt.
CREATE TABLE IF NOT EXISTS mcp_tool_settings (
    server_name TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (server_name, tool_name),
    FOREIGN KEY (server_name) REFERENCES mcp_servers(name) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_mcp_tool_settings_enabled ON mcp_tool_settings(server_name, enabled);
