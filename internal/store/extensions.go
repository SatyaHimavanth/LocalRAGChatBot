package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"changeme/internal/extension"
)

// ExtensionHook describes one future integration surface (GraphRAG, SQL Agent,
// MCP, OCR, etc.). The table is intentionally small and generic so new hook
// points can be added without schema churn.
type ExtensionHook struct {
	ID          int64  `json:"id"`
	HookKey     string `json:"hookKey"`
	Name        string `json:"name"`
	HookType    string `json:"hookType"`
	Surface     string `json:"surface"`
	Description string `json:"description"`
	State       string `json:"state"`
	Enabled     bool   `json:"enabled"`
	ConfigJSON  string `json:"configJson"`
	LastRunAt   int64  `json:"lastRunAt"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// EnsureDefaultExtensionHooks seeds the canonical phase-8 extension records.
func EnsureDefaultExtensionHooks(db *sql.DB) error {
	if db == nil {
		return nil
	}
	now := time.Now().Unix()
	for _, d := range extension.DefaultHookDescriptors() {
		if _, err := db.Exec(`INSERT OR IGNORE INTO extension_hooks (hook_key, name, hook_type, surface, description, state, enabled, config_json, last_run_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, d.Key, d.Name, d.HookType, d.Surface, d.Description, d.DefaultState, boolToInt(d.DefaultEnabled), d.DefaultConfig, 0, now, now); err != nil {
			return err
		}
	}
	return nil
}

// GetExtensionHooks returns all registered future-extension hooks.
func GetExtensionHooks(db *sql.DB) ([]ExtensionHook, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if err := EnsureDefaultExtensionHooks(db); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, hook_key, name, hook_type, surface, description, state, COALESCE(enabled,0), COALESCE(config_json,''), COALESCE(last_run_at,0), created_at, updated_at
		FROM extension_hooks
		ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hooks []ExtensionHook
	for rows.Next() {
		var h ExtensionHook
		var enabledInt int64
		if err := rows.Scan(&h.ID, &h.HookKey, &h.Name, &h.HookType, &h.Surface, &h.Description, &h.State, &enabledInt, &h.ConfigJSON, &h.LastRunAt, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		h.Enabled = enabledInt != 0
		if h.ConfigJSON == "" {
			h.ConfigJSON = "{}"
		}
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

// UpdateExtensionHook updates one hook row by key.
func UpdateExtensionHook(db *sql.DB, hookKey string, enabled bool, configJSON string, state string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	hookKey = strings.TrimSpace(hookKey)
	if hookKey == "" {
		return fmt.Errorf("hook key cannot be empty")
	}
	configJSON = normalizeHookConfig(configJSON)
	state = strings.TrimSpace(state)
	if state == "" {
		state = "planned"
	}
	_, err := db.Exec(`UPDATE extension_hooks SET enabled = ?, config_json = ?, state = ?, updated_at = ? WHERE hook_key = ?`, boolToInt(enabled), configJSON, state, time.Now().Unix(), hookKey)
	return err
}

// ResetExtensionHooks restores the default phase-8 hook descriptors.
func ResetExtensionHooks(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	now := time.Now().Unix()
	for _, d := range extension.DefaultHookDescriptors() {
		if _, err := db.Exec(`UPDATE extension_hooks SET name = ?, hook_type = ?, surface = ?, description = ?, state = ?, enabled = ?, config_json = ?, updated_at = ? WHERE hook_key = ?`,
			d.Name, d.HookType, d.Surface, d.Description, d.DefaultState, boolToInt(d.DefaultEnabled), d.DefaultConfig, now, d.Key); err != nil {
			return err
		}
	}
	return nil
}

func normalizeHookConfig(configJSON string) string {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON == "" {
		return "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(configJSON), &v); err != nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
