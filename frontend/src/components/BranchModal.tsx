import { useMemo } from "react";
import type { CSSProperties } from "react";
import type { Theme, ThemeVars } from "../types";
import { themeVars } from "../types";
import { Modal } from "./Modal";
import type { ChatMessage } from "../../bindings/changeme/internal/store/models";

interface BranchRow {
  leafId: number;
  path: ChatMessage[];
  leaf: ChatMessage;
  pathLabel: string;
}

interface BranchModalProps {
  open: boolean;
  theme: Theme;
  messages: ChatMessage[];
  currentLeafMessageId?: number;
  loading?: boolean;
  onClose: () => void;
  onSelectBranch: (leafMessageId: number) => void;
}

export function BranchModal({
  open,
  theme,
  messages,
  currentLeafMessageId,
  loading,
  onClose,
  onSelectBranch,
}: BranchModalProps) {
  const T = themeVars[theme];
  const branches = useMemo(() => buildBranchRows(messages), [messages]);

  return (
    <Modal open={open} onClose={onClose} title="Conversation Branches" theme={theme} wide>
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        <div style={{ fontSize: 12, color: T.text3, lineHeight: 1.5 }}>
          Choose a leaf to switch the active chat branch. Current leaf: <strong style={{ color: T.text }}>{currentLeafMessageId || "none"}</strong>
        </div>

        {loading ? (
          <div style={{ padding: "24px 12px", textAlign: "center", color: T.text3, fontSize: 13 }}>Loading branches…</div>
        ) : branches.length === 0 ? (
          <div style={{ padding: "24px 12px", textAlign: "center", color: T.text3, fontSize: 13 }}>
            No branch history is available yet.
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {branches.map((branch) => {
              const isCurrent = branch.leafId === currentLeafMessageId;
              return (
                <button
                  key={branch.leafId}
                  onClick={() => onSelectBranch(branch.leafId)}
                  style={{
                    ...cardStyle(T, isCurrent),
                    textAlign: "left",
                    width: "100%",
                  }}
                >
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 12, alignItems: "flex-start" }}>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontSize: 13, fontWeight: 600, color: T.text, marginBottom: 4 }}>
                        {branch.leaf.role === "assistant" ? "Assistant branch" : branch.leaf.role === "user" ? "User branch" : "Conversation branch"}
                        {isCurrent && <span style={pillStyle(T, "rgba(99,102,241,0.18)", "rgba(99,102,241,0.95)")}>Current</span>}
                      </div>
                      <div style={{ fontSize: 12, color: T.text2, lineHeight: 1.55, wordBreak: "break-word" }}>{branch.pathLabel}</div>
                    </div>
                    <div style={{ flexShrink: 0, fontSize: 11, color: T.text3, textAlign: "right" }}>
                      <div>Leaf #{branch.leafId}</div>
                      <div>{formatTime(branch.leaf.createdAt)}</div>
                    </div>
                  </div>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </Modal>
  );
}

function buildBranchRows(messages: ChatMessage[]): BranchRow[] {
  if (!messages.length) return [];

  const byId = new Map<number, ChatMessage>();
  const children = new Map<number, number[]>();
  for (const msg of messages) {
    byId.set(msg.id, msg);
    const parent = Number(msg.parentMessageId || 0);
    if (parent > 0) {
      const list = children.get(parent) || [];
      list.push(msg.id);
      children.set(parent, list);
    }
  }

  const leaves = messages.filter((msg) => !children.has(msg.id) || (children.get(msg.id)?.length || 0) === 0);
  const rows: BranchRow[] = [];

  for (const leaf of leaves) {
    const path = buildPath(leaf, byId);
    if (!path.length) continue;
    const pathLabel = path
      .slice(-6)
      .map((msg) => `${msg.role === "user" ? "You" : msg.role === "assistant" ? "AI" : msg.role}: ${summarize(msg.content)}`)
      .join(" → ");
    rows.push({ leafId: leaf.id, path, leaf, pathLabel });
  }

  rows.sort((a, b) => b.leaf.createdAt - a.leaf.createdAt || b.leafId - a.leafId);
  return rows;
}

function buildPath(leaf: ChatMessage, byId: Map<number, ChatMessage>): ChatMessage[] {
  const chain: ChatMessage[] = [];
  const seen = new Set<number>();
  let current: ChatMessage | undefined = leaf;
  while (current && !seen.has(current.id)) {
    seen.add(current.id);
    chain.push(current);
    const parentId = Number(current.parentMessageId || 0);
    if (!parentId) break;
    current = byId.get(parentId);
  }
  return chain.reverse();
}

function summarize(text: string): string {
  const cleaned = (text || "").replace(/\s+/g, " ").trim();
  if (!cleaned) return "(empty)";
  return cleaned.length > 72 ? cleaned.slice(0, 72).trimEnd() + "…" : cleaned;
}

function formatTime(unixSeconds: number): string {
  try {
    return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(unixSeconds * 1000));
  } catch {
    return String(unixSeconds);
  }
}

function cardStyle(T: ThemeVars, active: boolean): CSSProperties {
  return {
    border: "1px solid " + (active ? "rgba(99,102,241,0.45)" : T.border),
    background: active ? "rgba(99,102,241,0.08)" : T.inputBg,
    borderRadius: 12,
    padding: 14,
    cursor: "pointer",
    boxShadow: active ? "0 0 0 1px rgba(99,102,241,0.15)" : "none",
    transition: "background 0.2s, border 0.2s, box-shadow 0.2s",
  };
}

function pillStyle(T: ThemeVars, bg: string, color: string): CSSProperties {
  return {
    display: "inline-flex",
    alignItems: "center",
    gap: 4,
    marginLeft: 8,
    padding: "2px 8px",
    borderRadius: 999,
    background: bg,
    color,
    fontSize: 11,
    fontWeight: 500,
    border: "1px solid " + T.border,
  };
}
