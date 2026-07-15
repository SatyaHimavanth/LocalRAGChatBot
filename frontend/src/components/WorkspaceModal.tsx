import { useEffect } from "react";
import { Modal } from "./Modal";
import { Theme, ThemeVars, WorkspaceMemorySnapshot } from "../types";

interface WorkspaceModalProps {
  open: boolean;
  loading: boolean;
  data: WorkspaceMemorySnapshot | null;
  draftNotes: string;
  onDraftNotesChange: (value: string) => void;
  onSaveNotes: () => void;
  onRefresh: () => void;
  onClose: () => void;
  theme: Theme;
  T: ThemeVars;
}

export function WorkspaceModal({
  open,
  loading,
  data,
  draftNotes,
  onDraftNotesChange,
  onSaveNotes,
  onRefresh,
  onClose,
  theme,
  T,
}: WorkspaceModalProps) {
  useEffect(() => {
    if (!open) return;
  }, [open]);

  return (
    <Modal open={open} onClose={onClose} title="Workspace Memory" wide theme={theme}>
      <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center", justifyContent: "space-between" }}>
          <div style={{ fontSize: 12, color: T.text3 }}>
            {data?.collectionName ? <>Collection: <strong style={{ color: T.text }}>{data.collectionName}</strong></> : "No collection selected"}
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <button onClick={onRefresh} disabled={loading} style={secondaryBtn(T)}>{loading ? "Refreshing..." : "Refresh summary"}</button>
            <button onClick={onSaveNotes} disabled={loading} style={primaryBtn}>Save notes</button>
          </div>
        </div>

        <section style={panelStyle(T)}>
          <div style={sectionHeaderStyle(T)}>Auto-generated summary</div>
          <pre style={preStyle(T)}>{loading ? "Loading workspace..." : (data?.summary?.trim() || "No summary yet. Ask a few questions or refresh after conversation.")}</pre>
        </section>

        <section style={panelStyle(T)}>
          <div style={sectionHeaderStyle(T)}>Your notes</div>
          <textarea
            value={draftNotes}
            onChange={(e) => onDraftNotesChange(e.target.value)}
            rows={5}
            placeholder="Capture the session state, decisions, follow-up tasks, or reminders here."
            style={{ ...textareaStyle(T), minHeight: 120 }}
          />
        </section>

        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))", gap: 12 }}>
          <section style={panelStyle(T)}>
            <div style={sectionHeaderStyle(T)}>Recent questions</div>
            <div style={listStyle(T)}>
              {(data?.recentQuestions?.length ? data.recentQuestions : ["No recent questions yet."]).map((q, idx) => (
                <div key={`${q}-${idx}`} style={listItemStyle(T)}>• {q}</div>
              ))}
            </div>
          </section>
          <section style={panelStyle(T)}>
            <div style={sectionHeaderStyle(T)}>Recent documents</div>
            <div style={listStyle(T)}>
              {(data?.recentDocuments?.length ? data.recentDocuments : ["No documents in the active collection yet."]).map((d, idx) => (
                <div key={`${d}-${idx}`} style={listItemStyle(T)}>• {d}</div>
              ))}
            </div>
          </section>
        </div>

        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
          <div style={{ fontSize: 11, color: T.text3 }}>
            {data?.latestSignal ? <>Latest signal: {data.latestSignal}</> : <>Workspace memory updates automatically after each assistant turn.</>}
          </div>
          <div style={{ fontSize: 11, color: T.text3 }}>
            {data?.updatedAt ? `Updated ${new Date(data.updatedAt * 1000).toLocaleString()}` : "Not saved yet"}
          </div>
        </div>
      </div>
    </Modal>
  );
}

function panelStyle(T: ThemeVars) {
  return {
    border: "1px solid " + T.border,
    background: T.inputBg,
    borderRadius: 12,
    padding: 14,
    display: "flex",
    flexDirection: "column" as const,
    gap: 10,
  };
}

function sectionHeaderStyle(T: ThemeVars) {
  return { fontSize: 12, fontWeight: 600, color: T.text };
}

function listStyle(T: ThemeVars) {
  return {
    display: "flex",
    flexDirection: "column" as const,
    gap: 8,
    maxHeight: 180,
    overflowY: "auto" as const,
    color: T.text2,
    fontSize: 12,
    lineHeight: 1.5,
  };
}

function listItemStyle(T: ThemeVars) {
  return { color: T.text2, fontSize: 12, lineHeight: 1.5 };
}

function preStyle(T: ThemeVars) {
  return {
    margin: 0,
    whiteSpace: "pre-wrap" as const,
    wordBreak: "break-word" as const,
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
    fontSize: 12,
    lineHeight: 1.6,
    color: T.text2,
    background: T.bg2,
    border: "1px solid " + T.border,
    borderRadius: 10,
    padding: 12,
    maxHeight: 220,
    overflowY: "auto" as const,
  };
}

function textareaStyle(T: ThemeVars) {
  return {
    width: "100%",
    padding: "10px 12px",
    borderRadius: 10,
    border: "1px solid " + T.border,
    background: T.bg2,
    color: T.text,
    fontSize: 13,
    outline: "none",
    resize: "vertical" as const,
  };
}

function secondaryBtn(T: ThemeVars) {
  return {
    padding: "8px 12px",
    borderRadius: 8,
    border: "1px solid " + T.border,
    background: "transparent",
    color: T.text2,
    cursor: "pointer",
    fontSize: 12,
  } as const;
}

const primaryBtn = {
  padding: "8px 12px",
  borderRadius: 8,
  border: "none",
  background: "rgba(99,102,241,0.8)",
  color: "#fff",
  cursor: "pointer",
  fontSize: 12,
} as const;
