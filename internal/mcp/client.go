// Package mcp provides the small, dependency-free MCP discovery client used by
// the desktop extension. It supports the stdio configurations used by VS Code,
// Claude Desktop, and Codex, plus JSON-RPC HTTP endpoints.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const protocolVersion = "2024-11-05"

type Configuration struct {
	MCPServers map[string]ServerConfig    `json:"mcpServers"`
	RawServers map[string]json.RawMessage `json:"-"`
}

type ServerConfig struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Type    string            `json:"type,omitempty"`
}

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ParseConfiguration accepts the conventional {"mcpServers": {...}} format.
func ParseConfiguration(raw string) (Configuration, error) {
	var envelope struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	if err := decoder.Decode(&envelope); err != nil {
		return Configuration{}, fmt.Errorf("invalid MCP JSON: %w", err)
	}
	config := Configuration{MCPServers: make(map[string]ServerConfig, len(envelope.MCPServers)), RawServers: envelope.MCPServers}
	for name, rawServer := range envelope.MCPServers {
		var server ServerConfig
		if err := json.Unmarshal(rawServer, &server); err != nil {
			return Configuration{}, fmt.Errorf("invalid MCP server %q: %w", name, err)
		}
		config.MCPServers[name] = server
	}
	if len(config.MCPServers) == 0 {
		return Configuration{}, fmt.Errorf("MCP JSON must contain at least one entry in mcpServers")
	}
	for name, server := range config.MCPServers {
		name = strings.TrimSpace(name)
		if name == "" {
			return Configuration{}, fmt.Errorf("MCP server names cannot be empty")
		}
		if strings.TrimSpace(server.Command) == "" && strings.TrimSpace(server.URL) == "" {
			return Configuration{}, fmt.Errorf("MCP server %q needs either command or url", name)
		}
		if server.Command != "" && server.URL != "" {
			return Configuration{}, fmt.Errorf("MCP server %q cannot define both command and url", name)
		}
	}
	return config, nil
}

// ListTools establishes a short-lived MCP connection and returns the tools
// exposed by a server. No commands are passed through a shell.
func ListTools(ctx context.Context, server ServerConfig) ([]Tool, error) {
	if strings.TrimSpace(server.Command) != "" {
		return listToolsStdio(ctx, server)
	}
	if strings.EqualFold(strings.TrimSpace(server.Type), "sse") {
		return nil, fmt.Errorf("SSE MCP endpoints are not supported yet; use stdio or streamable HTTP")
	}
	return listToolsHTTP(ctx, server)
}

func listToolsStdio(ctx context.Context, server ServerConfig) ([]Tool, error) {
	cmd := exec.CommandContext(ctx, server.Command, server.Args...)
	cmd.Dir = strings.TrimSpace(server.Cwd)
	cmd.Env = append([]string(nil), os.Environ()...)
	for key, value := range server.Env {
		if strings.TrimSpace(key) != "" {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open MCP stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open MCP stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start MCP server: %w", err)
	}
	defer func() { _ = stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	client := &stdioClient{in: stdin, out: scanner}
	if err := client.initialize(ctx); err != nil {
		return nil, err
	}
	return client.listTools(ctx)
}

type stdioClient struct {
	in  io.Writer
	out *bufio.Scanner
}

func (c *stdioClient) send(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.in, "%s\n", data)
	return err
}

func (c *stdioClient) request(ctx context.Context, id int, method string, params any) (json.RawMessage, error) {
	if err := c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	type response struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	result := make(chan struct {
		raw json.RawMessage
		err error
	}, 1)
	go func() {
		for c.out.Scan() {
			var reply response
			if err := json.Unmarshal(c.out.Bytes(), &reply); err != nil {
				continue
			}
			if reply.ID != id {
				continue
			}
			if reply.Error != nil {
				result <- struct {
					raw json.RawMessage
					err error
				}{nil, fmt.Errorf("MCP %s: %s", method, reply.Error.Message)}
				return
			}
			result <- struct {
				raw json.RawMessage
				err error
			}{reply.Result, nil}
			return
		}
		if err := c.out.Err(); err != nil {
			result <- struct {
				raw json.RawMessage
				err error
			}{nil, err}
			return
		}
		result <- struct {
			raw json.RawMessage
			err error
		}{nil, fmt.Errorf("MCP server closed its output")}
	}()
	select {
	case reply := <-result:
		return reply.raw, reply.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *stdioClient) initialize(ctx context.Context) error {
	if _, err := c.request(ctx, 1, "initialize", map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "LocalRAG", "version": "1.0"}}); err != nil {
		return fmt.Errorf("initialize MCP server: %w", err)
	}
	return c.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
}

func (c *stdioClient) listTools(ctx context.Context) ([]Tool, error) {
	var tools []Tool
	var cursor string
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.request(ctx, 2, "tools/list", params)
		if err != nil {
			return nil, fmt.Errorf("list MCP tools: %w", err)
		}
		var page struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("read MCP tools: %w", err)
		}
		tools = append(tools, page.Tools...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

func listToolsHTTP(ctx context.Context, server ServerConfig) ([]Tool, error) {
	url := strings.TrimSpace(server.URL)
	if url == "" {
		return nil, fmt.Errorf("MCP HTTP URL is empty")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	headers := cloneHeaders(server.Headers)
	var sessionID string
	call := func(id int, method string, params any) (json.RawMessage, error) {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if got := resp.Header.Get("Mcp-Session-Id"); got != "" {
			sessionID = got
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("MCP HTTP %s: %s", resp.Status, strings.TrimSpace(string(data)))
		}
		var reply struct {
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &reply); err != nil {
			return nil, fmt.Errorf("decode MCP HTTP response: %w", err)
		}
		if reply.Error != nil {
			return nil, fmt.Errorf("MCP %s: %s", method, reply.Error.Message)
		}
		return reply.Result, nil
	}
	if _, err := call(1, "initialize", map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "LocalRAG", "version": "1.0"}}); err != nil {
		return nil, fmt.Errorf("initialize MCP server: %w", err)
	}
	// MCP requires this notification after a successful initialize request.
	notifyBody, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	notifyReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(notifyBody))
	if err != nil {
		return nil, err
	}
	notifyReq.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		notifyReq.Header.Set(key, value)
	}
	if sessionID != "" {
		notifyReq.Header.Set("Mcp-Session-Id", sessionID)
	}
	notifyResp, err := client.Do(notifyReq)
	if err != nil {
		return nil, fmt.Errorf("notify MCP server initialized: %w", err)
	}
	_ = notifyResp.Body.Close()
	if notifyResp.StatusCode < 200 || notifyResp.StatusCode >= 300 {
		return nil, fmt.Errorf("MCP initialized notification: %s", notifyResp.Status)
	}
	raw, err := call(2, "tools/list", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("list MCP tools: %w", err)
	}
	var page struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("read MCP tools: %w", err)
	}
	sort.Slice(page.Tools, func(i, j int) bool { return page.Tools[i].Name < page.Tools[j].Name })
	return page.Tools, nil
}

func cloneHeaders(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
