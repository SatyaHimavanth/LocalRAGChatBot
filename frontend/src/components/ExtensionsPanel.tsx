import { useEffect, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { ExtensionHook, ThemeVars } from "../types";
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

export function ExtensionsPanel({ hooks, loading, T, onRefresh, onSaveHook, onResetHooks }: ExtensionsPanelProps) {
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
