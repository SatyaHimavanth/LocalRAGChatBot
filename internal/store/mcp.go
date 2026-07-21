package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type MCPServer struct {
	Name           string   `json:"name"`
	ConfigJSON     string   `json:"configJson"`
	Enabled        bool     `json:"enabled"`
	Verified       bool     `json:"verified"`
	ToolCount      int      `json:"toolCount"`
	ToolsJSON      string   `json:"toolsJson"`
	LastError      string   `json:"lastError"`
	LastVerifiedAt int64    `json:"lastVerifiedAt"`
	EnabledTools   []string `json:"enabledTools"`
}

type MCPConfiguration struct {
	Enabled bool        `json:"enabled"`
	Servers []MCPServer `json:"servers"`
}

func GetMCPConfiguration(db *sql.DB) (MCPConfiguration, error) {
	if db == nil {
		return MCPConfiguration{}, fmt.Errorf("database not initialized")
	}
	config := MCPConfiguration{}
	if err := db.QueryRow(`SELECT enabled FROM mcp_settings WHERE id = 1`).Scan(&config.Enabled); err != nil && err != sql.ErrNoRows {
		return config, err
	}
	rows, err := db.Query(`SELECT name, config_json, enabled, verified, tool_count, tools_json, last_error, last_verified_at FROM mcp_servers ORDER BY name`)
	if err != nil {
		return config, err
	}
	defer rows.Close()
	for rows.Next() {
		var server MCPServer
		if err := rows.Scan(&server.Name, &server.ConfigJSON, &server.Enabled, &server.Verified, &server.ToolCount, &server.ToolsJSON, &server.LastError, &server.LastVerifiedAt); err != nil {
			return config, err
		}
		var configuredToolCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM mcp_tool_settings WHERE server_name = ?`, server.Name).Scan(&configuredToolCount); err != nil {
			return config, err
		}
		toolRows, err := db.Query(`SELECT tool_name FROM mcp_tool_settings WHERE server_name = ? AND enabled = 1 ORDER BY tool_name`, server.Name)
		if err != nil {
			return config, err
		}
		for toolRows.Next() {
			var name string
			if err := toolRows.Scan(&name); err != nil {
				toolRows.Close()
				return config, err
			}
			server.EnabledTools = append(server.EnabledTools, name)
		}
		if err := toolRows.Close(); err != nil {
			return config, err
		}
		// Existing configurations predate per-tool settings. Until the user
		// changes one, preserve the original behaviour of exposing all tools.
		if configuredToolCount == 0 && server.Verified {
			var tools []struct {
				Name string `json:"name"`
			}
			if json.Unmarshal([]byte(server.ToolsJSON), &tools) == nil {
				for _, tool := range tools {
					if tool.Name != "" {
						server.EnabledTools = append(server.EnabledTools, tool.Name)
					}
				}
			}
		}
		config.Servers = append(config.Servers, server)
	}
	return config, rows.Err()
}

func ReplaceMCPServers(db *sql.DB, servers []MCPServer) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM mcp_servers`); err != nil {
		return err
	}
	now := time.Now().Unix()
	for _, server := range servers {
		if _, err = tx.Exec(`INSERT INTO mcp_servers (name, config_json, enabled, verified, tool_count, tools_json, last_error, last_verified_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, server.Name, server.ConfigJSON, server.Enabled, server.Verified, server.ToolCount, server.ToolsJSON, server.LastError, server.LastVerifiedAt, now, now); err != nil {
			return err
		}
		var tools []struct {
			Name string `json:"name"`
		}
		if json.Unmarshal([]byte(server.ToolsJSON), &tools) == nil {
			for _, tool := range tools {
				if tool.Name == "" {
					continue
				}
				if _, err = tx.Exec(`INSERT OR IGNORE INTO mcp_tool_settings (server_name, tool_name, enabled, updated_at) VALUES (?, ?, 1, ?)`, server.Name, tool.Name, now); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

func SetMCPToolEnabled(db *sql.DB, serverName, toolName string, enabled bool) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := db.Exec(`INSERT INTO mcp_tool_settings (server_name, tool_name, enabled, updated_at) VALUES (?, ?, ?, ?) ON CONFLICT(server_name, tool_name) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`, serverName, toolName, enabled, time.Now().Unix())
	return err
}

// HasMCPToolSettings reports whether a server has already received its initial
// per-tool selection rows.
func HasMCPToolSettings(db *sql.DB, serverName string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database not initialized")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mcp_tool_settings WHERE server_name = ?`, serverName).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func SetMCPEnabled(db *sql.DB, enabled bool) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := db.Exec(`INSERT INTO mcp_settings (id, enabled, updated_at) VALUES (1, ?, ?) ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`, enabled, time.Now().Unix())
	return err
}

func SetMCPServerEnabled(db *sql.DB, name string, enabled bool) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	result, err := db.Exec(`UPDATE mcp_servers SET enabled = ?, updated_at = ? WHERE name = ?`, enabled, time.Now().Unix(), name)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return fmt.Errorf("MCP server %q was not found", name)
	}
	return nil
}
