import { useEffect, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { ExtensionHook, MCPConfiguration, MCPServer, MCPTool, ThemeVars } from "../types";
import { I } from "./Icons";

interface ExtensionsPanelProps {
  hooks: ExtensionHook[];
  loading: boolean;
  T: ThemeVars;
  onRefresh: () => void;
  onSaveHook: (hookKey: string, enabled: boolean, configJson: string, state: string) => void;
  onResetHooks: () => void;
}

type DraftMap = Record<string, { enabled: boolean; configJson: string; state: string }>;

const cardStyle = (T: ThemeVars): CSSProperties => ({
  padding: 14,
  borderRadius: 14,
  border: "1px solid " + T.border,
  background: T.bg2,
  boxShadow: "0 8px 24px rgba(0,0,0,0.08)",
});

const chipStyle = (bg: string, fg: string, border: string): CSSProperties => ({
  display: "inline-flex",
  alignItems: "center",
  gap: 6,
  padding: "4px 10px",
  borderRadius: 999,
  border: "1px solid " + border,
  background: bg,
  color: fg,
  fontSize: 11,
  whiteSpace: "nowrap",
});

const switchTrack = (enabled: boolean): CSSProperties => ({
  width: 42,
  height: 24,
  borderRadius: 999,
  position: "relative",
  border: "1px solid " + (enabled ? "rgba(99,102,241,0.55)" : "rgba(128,128,128,0.24)"),
  background: enabled ? "rgba(99,102,241,0.2)" : "rgba(128,128,128,0.1)",
  cursor: "pointer",
  flexShrink: 0,
});

const switchKnob = (enabled: boolean): CSSProperties => ({
  width: 18,
  height: 18,
  borderRadius: "50%",
  position: "absolute",
  top: 2,
  left: enabled ? 21 : 2,
  transition: "left 0.15s ease",
  background: enabled ? "rgba(99,102,241,0.95)" : "rgba(255,255,255,0.72)",
});

export function LegacyExtensionsPanel({ hooks, loading, T, onRefresh, onSaveHook, onResetHooks }: ExtensionsPanelProps) {
  const [drafts, setDrafts] = useState<DraftMap>({});

  useEffect(() => {
    const next: DraftMap = {};
    for (const hook of hooks) {
      next[hook.hookKey] = {
        enabled: hook.enabled,
        configJson: hook.configJson || "{}",
        state: hook.state || "planned",
      };
    }
    setDrafts(next);
  }, [hooks]);

  const totals = useMemo(() => {
    const enabled = hooks.filter((h) => h.enabled).length;
    const surfaces = Array.from(new Set(hooks.map((h) => h.surface || "unknown")));
    return { enabled, total: hooks.length, surfaces };
  }, [hooks]);

  const updateDraft = (hookKey: string, patch: Partial<{ enabled: boolean; configJson: string; state: string }>) => {
    setDrafts((prev) => ({
      ...prev,
      [hookKey]: {
        enabled: patch.enabled ?? prev[hookKey]?.enabled ?? false,
        configJson: patch.configJson ?? prev[hookKey]?.configJson ?? "{}",
        state: patch.state ?? prev[hookKey]?.state ?? "planned",
      },
    }));
  };

  const saveHook = (hookKey: string) => {
    const draft = drafts[hookKey];
    if (!draft) return;
    onSaveHook(hookKey, draft.enabled, draft.configJson, draft.state);
  };

  return (
    <div style={{ flex: 1, display: "flex", flexDirection: "column", padding: 20, overflow: "hidden", minWidth: 0 }}>
      <div style={{ display: "flex", justifyContent: "space-between", gap: 12, alignItems: "center", marginBottom: 16, flexWrap: "wrap" }}>
        <div>
          <div style={{ fontSize: 18, fontWeight: 700, display: "flex", alignItems: "center", gap: 8 }}>
            <I.Bolt /> Phase 8 Extensions
          </div>
          <div style={{ fontSize: 12, color: T.text3, marginTop: 4 }}>Reserved hook surfaces for GraphRAG, SQL agents, MCP, and OCR.</div>
        {loading && <div style={{ fontSize: 11, color: T.text3, marginTop: 4, display: "flex", alignItems: "center", gap: 6 }}><I.Spinner /> Loading hooks...</div>}
        </div>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
          <button onClick={onRefresh} style={actionBtn(T, false)}><I.Refresh /> Refresh</button>
          <button onClick={onResetHooks} style={actionBtn(T, false)}>Reset defaults</button>
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 12, marginBottom: 14 }}>
        <Metric title="Hooks" value={`${totals.enabled}/${totals.total}`} detail="Enabled surfaces" T={T} />
        <Metric title="Surfaces" value={totals.surfaces.join(" · ") || "none"} detail="retrieval, analytics, tooling, ingestion" T={T} />
        <Metric title="Status" value="Hook registry" detail="Persisted, editable, and ready for future integrations" T={T} />
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 12, overflowY: "auto", paddingRight: 4 }}>
        {hooks.length === 0 ? (
          <div style={cardStyle(T)}>
            <div style={{ fontSize: 13, color: T.text3 }}>No extension hooks are registered yet.</div>
          </div>
        ) : hooks.map((hook) => {
          const draft = drafts[hook.hookKey] || { enabled: hook.enabled, configJson: hook.configJson || "{}", state: hook.state || "planned" };
          return (
            <section key={hook.hookKey} style={cardStyle(T)}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "start", flexWrap: "wrap" }}>
                <div style={{ minWidth: 0, flex: 1 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <div style={{ fontSize: 15, fontWeight: 650 }}>{hook.name}</div>
                    <span style={chipStyle("rgba(99,102,241,0.12)", T.text2, T.border)}>{hook.surface}</span>
                    <span style={chipStyle("rgba(168,85,247,0.12)", T.text2, T.border)}>{hook.hookType}</span>
                    <span style={chipStyle(statusBg(hook.state), statusFg(hook.state), T.border)}>{hook.state}</span>
                  </div>
                  <div style={{ fontSize: 11, color: T.text3, marginTop: 6, lineHeight: 1.5 }}>{hook.description}</div>
                </div>
                <div style={{ display: "flex", flexDirection: "column", alignItems: "end", gap: 8 }}>
                  <button
                    onClick={() => {
                      const nextEnabled = !draft.enabled;
                      updateDraft(hook.hookKey, { enabled: nextEnabled });
                      onSaveHook(hook.hookKey, nextEnabled, draft.configJson, draft.state);
                    }}
                    style={{ ...switchTrack(draft.enabled), padding: 0 }}
                    title={draft.enabled ? "Disable hook" : "Enable hook"}
                  >
                    <span style={switchKnob(draft.enabled)} />
                  </button>
                  <span style={chipStyle(draft.enabled ? "rgba(34,197,94,0.14)" : "rgba(128,128,128,0.12)", T.text2, T.border)}>{draft.enabled ? "enabled" : "disabled"}</span>
                </div>
              </div>

              <div style={{ display: "grid", gridTemplateColumns: "1fr", gap: 10, marginTop: 12 }}>
                <label style={fieldLabel(T)}>State</label>
                <input
                  value={draft.state}
                  onChange={(e) => updateDraft(hook.hookKey, { state: e.target.value })}
                  placeholder="planned"
                  style={inputStyle(T)}
                />
                <label style={fieldLabel(T)}>Config JSON</label>
                <textarea
                  value={draft.configJson}
                  onChange={(e) => updateDraft(hook.hookKey, { configJson: e.target.value })}
                  rows={4}
                  spellCheck={false}
                  style={{ ...inputStyle(T), minHeight: 96, resize: "vertical", fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}
                />
              </div>

              <div style={{ display: "flex", justifyContent: "space-between", gap: 8, marginTop: 12, flexWrap: "wrap" }}>
                <div style={{ fontSize: 11, color: T.text3, lineHeight: 1.5 }}>
                  <div>Key: {hook.hookKey}</div>
                  <div>Updated: {hook.updatedAt ? new Date(hook.updatedAt * 1000).toLocaleString() : "n/a"}</div>
                </div>
                <button onClick={() => saveHook(hook.hookKey)} style={actionBtn(T, true)}>Save hook</button>
              </div>
            </section>
          );
        })}
      </div>
    </div>
  );
}

function Metric({ title, value, detail, T }: { title: string; value: string; detail: string; T: ThemeVars }) {
  return (
    <div style={{ padding: 14, borderRadius: 12, border: "1px solid " + T.border, background: T.bg2, boxShadow: "0 8px 24px rgba(0,0,0,0.08)" }}>
      <div style={{ fontSize: 11, color: T.text3, textTransform: "uppercase", letterSpacing: 0.6 }}>{title}</div>
      <div style={{ fontSize: 18, fontWeight: 700, marginTop: 4, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{value}</div>
      <div style={{ fontSize: 11, color: T.text3, marginTop: 4, lineHeight: 1.5 }}>{detail}</div>
    </div>
  );
}

function actionBtn(T: ThemeVars, primary: boolean): CSSProperties {
  return {
    padding: "8px 12px",
    borderRadius: 10,
    border: primary ? "none" : "1px solid " + T.border,
    background: primary ? "rgba(99,102,241,0.82)" : "transparent",
    color: primary ? "#fff" : T.text2,
    cursor: "pointer",
    fontSize: 12,
    fontWeight: 500,
    display: "inline-flex",
    alignItems: "center",
    gap: 6,
  };
}

function inputStyle(T: ThemeVars): CSSProperties {
  return {
    width: "100%",
    padding: "10px 12px",
    borderRadius: 10,
    border: "1px solid " + T.border,
    background: T.inputBg,
    color: T.text,
    fontSize: 13,
    outline: "none",
  };
}

function fieldLabel(T: ThemeVars): CSSProperties {
  return {
    fontSize: 12,
    color: T.text3,
    marginBottom: -2,
  };
}

function statusBg(state: string) {
  const key = String(state || "").toLowerCase();
  if (key === "ready" || key === "enabled") return "rgba(34,197,94,0.12)";
  if (key === "partial" || key === "preview") return "rgba(245,158,11,0.12)";
  return "rgba(148,163,184,0.12)";
}

function statusFg(state: string) {
  const key = String(state || "").toLowerCase();
  if (key === "ready" || key === "enabled") return "rgba(34,197,94,0.95)";
  if (key === "partial" || key === "preview") return "rgba(245,158,11,0.95)";
  return "rgba(148,163,184,0.95)";
}

interface MCPPanelProps {
  T: ThemeVars;
  configuration: MCPConfiguration;
  loading: boolean;
  onRefresh: () => Promise<void>;
  onVerify: (configJSON: string) => Promise<MCPServer[]>;
  onSave: (configJSON: string) => Promise<MCPServer[]>;
  onSetEnabled: (enabled: boolean) => Promise<void>;
  onSetServerEnabled: (name: string, enabled: boolean) => Promise<void>;
  onSetToolEnabled: (serverName: string, toolName: string, enabled: boolean) => Promise<void>;
}

const exampleMCPConfig = `{
  "mcpServers": {
    "example": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "C:/allowed-folder"]
    }
  }
}`;

function configToEditor(servers: MCPServer[]) {
  const mcpServers: Record<string, unknown> = {};
  for (const server of servers) {
    try { mcpServers[server.name] = JSON.parse(server.configJson); } catch { /* Invalid saved entries are represented in their result card. */ }
  }
  return JSON.stringify({ mcpServers }, null, 2);
}

function parseTools(raw: string): MCPTool[] {
  try { const tools = JSON.parse(raw); return Array.isArray(tools) ? tools : []; } catch { return []; }
}

// MCP is a real extension: users provide their own standard mcpServers JSON,
// verify tool discovery, and selectively expose verified servers to the agent.
export function ExtensionsPanel({ T, configuration, loading, onRefresh, onVerify, onSave, onSetEnabled, onSetServerEnabled, onSetToolEnabled }: MCPPanelProps) {
  const [editor, setEditor] = useState(exampleMCPConfig);
  const [results, setResults] = useState<MCPServer[]>([]);
  const [hasUnsavedVerification, setHasUnsavedVerification] = useState(false);
  const [working, setWorking] = useState<"verify" | "save" | "toggle" | "" >("");
  const [notice, setNotice] = useState("");
  const [expandedServers, setExpandedServers] = useState<Set<string>>(() => new Set());

  useEffect(() => {
    if (configuration.servers.length > 0) setEditor(configToEditor(configuration.servers));
  }, [configuration.servers]);

  const runVerification = async () => {
    setWorking("verify"); setNotice("");
    try {
      const verified = await onVerify(editor);
      setResults(verified);
      setHasUnsavedVerification(true);
      const failed = verified.filter((server) => !server.verified).length;
      setNotice(failed ? `${failed} server(s) could not be reached. Review the error below.` : `${verified.length} server(s) verified successfully.`);
    } catch (error) { setNotice(error instanceof Error ? error.message : String(error)); }
    finally { setWorking(""); }
  };

  const saveConfiguration = async () => {
    setWorking("save"); setNotice("");
    try {
      const saved = await onSave(editor);
      // The parent refreshes persisted tool selections during save. Clear the
      // temporary verification result so these chips show the saved state.
      setResults([]);
      setHasUnsavedVerification(false);
      const usable = saved.filter((server) => server.verified).length;
      setNotice(`Saved ${saved.length} server(s). ${usable} verified server(s) can now be enabled.`);
    } catch (error) { setNotice(error instanceof Error ? error.message : String(error)); }
    finally { setWorking(""); }
  };

  const toggleMaster = async () => {
    setWorking("toggle"); setNotice("");
    try { await onSetEnabled(!configuration.enabled); } catch (error) { setNotice(error instanceof Error ? error.message : String(error)); } finally { setWorking(""); }
  };

  const toggleServer = async (server: MCPServer) => {
    setWorking("toggle"); setNotice("");
    try { await onSetServerEnabled(server.name, !server.enabled); } catch (error) { setNotice(error instanceof Error ? error.message : String(error)); } finally { setWorking(""); }
  };

  const toggleTool = async (server: MCPServer, tool: MCPTool) => {
    setWorking("toggle"); setNotice("");
    try {
      await onSetToolEnabled(server.name, tool.name, !server.enabledTools.includes(tool.name));
      setResults([]);
    } catch (error) { setNotice(error instanceof Error ? error.message : String(error)); }
    finally { setWorking(""); }
  };

  const displayedServers = results.length > 0 ? results : configuration.servers;
  return (
    <div style={{ flex: 1, minWidth: 0, padding: 20, display: "flex", flexDirection: "column", overflow: "hidden" }}>
      <div style={{ display: "flex", alignItems: "start", justifyContent: "space-between", gap: 16, flexWrap: "wrap" }}>
        <div>
          <div style={{ fontSize: 18, fontWeight: 700, display: "flex", alignItems: "center", gap: 8 }}><I.Bolt /> MCP extensions</div>
          <div style={{ fontSize: 12, color: T.text3, marginTop: 4 }}>Connect local or HTTP MCP servers using the standard <code>mcpServers</code> JSON format.</div>
        </div>
        <button onClick={toggleMaster} disabled={working !== ""} style={{ ...switchTrack(configuration.enabled), padding: 0, cursor: working ? "wait" : "pointer" }} title={configuration.enabled ? "Disable MCP extension" : "Enable MCP extension"}>
          <span style={switchKnob(configuration.enabled)} />
        </button>
      </div>

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 12, gap: 12, flexWrap: "wrap" }}>
        <span style={chipStyle(configuration.enabled ? "rgba(34,197,94,0.14)" : "rgba(128,128,128,0.12)", configuration.enabled ? "rgba(34,197,94,0.95)" : T.text3, T.border)}>{configuration.enabled ? "MCP extension enabled" : "MCP extension disabled"}</span>
        <button onClick={() => void onRefresh()} disabled={loading || working !== ""} style={actionBtn(T, false)}><I.Refresh /> Refresh</button>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "minmax(280px, 1.15fr) minmax(260px, 0.85fr)", gap: 14, marginTop: 16, overflowY: "auto", paddingRight: 4 }}>
        <section style={cardStyle(T)}>
          <div style={{ fontSize: 14, fontWeight: 650 }}>Server configuration</div>
          <p style={{ margin: "6px 0 10px", color: T.text3, fontSize: 12, lineHeight: 1.55 }}>Add more than one named server under <code>mcpServers</code>. Verification starts configured local commands or contacts the configured HTTP endpoint.</p>
          <textarea value={editor} onChange={(event) => setEditor(event.target.value)} rows={18} spellCheck={false} style={{ ...inputStyle(T), minHeight: 300, resize: "vertical", fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace", lineHeight: 1.5 }} />
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginTop: 10 }}>
            <button onClick={runVerification} disabled={working !== ""} style={actionBtn(T, false)}>{working === "verify" ? <I.Spinner /> : <I.Refresh />} Verify tools</button>
            <button onClick={saveConfiguration} disabled={working !== ""} style={actionBtn(T, true)}>{working === "save" ? <I.Spinner /> : <I.Bolt />} Verify and save</button>
          </div>
          <div style={{ marginTop: 10, fontSize: 11, color: T.text3, lineHeight: 1.5 }}>Sensitive headers and environment values are kept in the local database. They are never written to app logs.</div>
        </section>

        <section style={{ ...cardStyle(T), display: "flex", flexDirection: "column", minHeight: 0 }}>
          <div style={{ fontSize: 14, fontWeight: 650 }}>Verified servers</div>
          <div style={{ fontSize: 12, color: T.text3, marginTop: 5, lineHeight: 1.5 }}>Enable a verified server here, then turn on the master MCP extension to include its tools in the agent capability list.</div>
          {notice && <div style={{ marginTop: 10, padding: "8px 10px", borderRadius: 8, background: notice.includes("could not") || notice.includes("invalid") ? "rgba(239,68,68,0.1)" : "rgba(99,102,241,0.1)", color: notice.includes("could not") || notice.includes("invalid") ? "rgba(220,38,38,0.95)" : T.text2, fontSize: 12, lineHeight: 1.45 }}>{notice}</div>}
          <div style={{ display: "flex", flexDirection: "column", gap: 10, marginTop: 12, overflowY: "auto", paddingRight: 2 }}>
            {displayedServers.length === 0 ? <div style={{ color: T.text3, fontSize: 12, padding: "18px 0" }}>No MCP servers have been saved yet.</div> : displayedServers.map((server) => {
              const tools = parseTools(server.toolsJson);
              return <div key={server.name} style={{ padding: 11, borderRadius: 10, border: "1px solid " + T.border, background: T.inputBg }}>
                <div style={{ display: "flex", gap: 10, justifyContent: "space-between", alignItems: "start" }}>
                  <div style={{ minWidth: 0 }}><div style={{ fontSize: 13, fontWeight: 650, overflow: "hidden", textOverflow: "ellipsis" }}>{server.name}</div><div style={{ marginTop: 3, fontSize: 11, color: server.verified ? "rgba(34,197,94,0.95)" : "rgba(220,38,38,0.95)" }}>{server.verified ? `${server.toolCount} tool${server.toolCount === 1 ? "" : "s"} found · ${server.enabledTools.length} enabled` : "Connection failed"}</div></div>
                  <button onClick={() => toggleServer(server)} disabled={!server.verified || hasUnsavedVerification || working !== ""} style={{ ...switchTrack(server.enabled), padding: 0, cursor: !server.verified || hasUnsavedVerification || working ? "not-allowed" : "pointer", opacity: server.verified ? 1 : 0.5 }} title={hasUnsavedVerification ? "Save this verified configuration before enabling it" : server.enabled ? "Remove tools from agent" : "Add tools to agent"}><span style={switchKnob(server.enabled)} /></button>
                </div>
                {server.lastError && <div style={{ marginTop: 7, fontSize: 11, color: "rgba(220,38,38,0.95)", lineHeight: 1.45, wordBreak: "break-word" }}>{server.lastError}</div>}
                {tools.length > 0 && (() => {
                  const expanded = expandedServers.has(server.name);
                  const visibleTools = expanded ? tools : tools.slice(0, 8);
                  return <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", gap: 5 }}>
                    {visibleTools.map((tool) => {
                      const toolEnabled = server.enabledTools.includes(tool.name);
                      const disabled = !server.verified || hasUnsavedVerification || working !== "";
                      return <button key={tool.name} onClick={() => toggleTool(server, tool)} disabled={disabled} title={disabled ? hasUnsavedVerification ? "Save this verified configuration before selecting tools" : tool.description : `${toolEnabled ? "Disable" : "Enable"} ${tool.name}`} style={{ ...chipStyle(toolEnabled ? "rgba(34,197,94,0.14)" : "rgba(128,128,128,0.12)", toolEnabled ? "rgba(34,197,94,0.95)" : T.text3, T.border), cursor: disabled ? "not-allowed" : "pointer", opacity: disabled ? 0.65 : 1 }}>{tool.name}</button>;
                    })}
                    {tools.length > 8 && <button onClick={() => setExpandedServers((current) => { const next = new Set(current); if (next.has(server.name)) next.delete(server.name); else next.add(server.name); return next; })} style={{ ...chipStyle("rgba(128,128,128,0.1)", T.text3, T.border), cursor: "pointer" }} title={expanded ? "Show fewer tools" : "Show all tools"}>{expanded ? "Show less" : `+${tools.length - 8}`}</button>}
                  </div>;
                })()}
              </div>;
            })}
          </div>
        </section>
      </div>
    </div>
  );
}
