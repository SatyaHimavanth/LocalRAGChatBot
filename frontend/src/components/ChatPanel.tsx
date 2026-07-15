import { useEffect, useRef, useState, useCallback } from "react";
import type { ReactNode, MouseEvent, CSSProperties } from "react";
import { Message, ThemeVars, Theme, SourceRef } from "../types";
import { I } from "./Icons";
import { Markdown } from "./Markdown";
import { Modal } from "./Modal";

interface ChatPanelProps {
  activeChat: { id: number; title: string; messages: Message[]; pinned?: boolean; archived?: boolean; messageSources?: Record<number, SourceRef[]> } | undefined;
  isArchived: boolean;
  input: string;
  gen: boolean;
  statusMsgs: Message[];
  T: ThemeVars;
  theme: Theme;
  collSelector: ReactNode;
  onInputChange: (v: string) => void;
  onSend: () => void;
  onThemeToggle: () => void;
  onOpenUploadModal: () => void;
  onStopGeneration?: () => void;
  onRerunFromMessage?: (messageId: number, prompt: string) => void;
}

export function ChatPanel({
  activeChat,
  isArchived,
  input,
  gen,
  statusMsgs,
  T,
  theme,
  collSelector,
  onInputChange,
  onSend,
  onThemeToggle,
  onOpenUploadModal,
  onStopGeneration,
  onRerunFromMessage,
}: ChatPanelProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const lastUserMsgRef = useRef<HTMLDivElement>(null);
  const prevMsgCount = useRef(activeChat?.messages.length || 0);
  const [showScrollDown, setShowScrollDown] = useState(false);
  const [sourceModal, setSourceModal] = useState<{ sources: SourceRef[]; refNum: number } | null>(null);
  const messagesRef = useRef<HTMLDivElement>(null);
  const [copiedMessageId, setCopiedMessageId] = useState<number | null>(null);
  const [editModal, setEditModal] = useState<{ open: boolean; messageId: number; value: string } | null>(null);

  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setShowScrollDown(el.scrollHeight - el.scrollTop - el.clientHeight > 200);
  }, []);

  useEffect(() => {
    const el = scrollRef.current;
    const msgs = activeChat?.messages || [];
    const msgCount = msgs.length;
    if (msgCount <= prevMsgCount.current) {
      prevMsgCount.current = msgCount;
      return;
    }
    prevMsgCount.current = msgCount;
    requestAnimationFrame(() => {
      if (!lastUserMsgRef.current || !el) return;
      el.scrollTo({ top: Math.max(0, lastUserMsgRef.current.offsetTop - el.clientHeight * 0.5), behavior: "smooth" });
    });
  }, [activeChat?.messages]);

  // Handle clicks on source reference badges within markdown content
  const handleMessagesClick = useCallback(
    (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      const refBadge = target.closest("[data-ref]");
      if (!refBadge) return;
      const refNum = parseInt(refBadge.getAttribute("data-ref") || "0");
      if (!refNum) return;
      const msgDiv = target.closest("[data-msg-id]");
      if (!msgDiv) return;
      const msgId = parseInt(msgDiv.getAttribute("data-msg-id") || "0");
      if (!msgId || !activeChat?.messageSources) return;
      const sources = activeChat.messageSources[msgId];
      if (sources) setSourceModal({ sources, refNum });
    },
    [activeChat?.messageSources]
  );

  const copyToClipboard = useCallback(async (msgId: number, text: string) => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      } else {
        const ta = document.createElement("textarea");
        ta.value = text;
        ta.style.position = "fixed";
        ta.style.opacity = "0";
        document.body.appendChild(ta);
        ta.focus();
        ta.select();
        document.execCommand("copy");
        document.body.removeChild(ta);
      }
      setCopiedMessageId(msgId);
      window.setTimeout(() => setCopiedMessageId((current) => (current === msgId ? null : current)), 1300);
    } catch {
      setCopiedMessageId(null);
    }
  }, []);

  const openEditModal = useCallback((messageId: number, currentText: string) => {
    setEditModal({ open: true, messageId, value: currentText });
  }, []);

  const submitEdit = useCallback(() => {
    if (!editModal || !onRerunFromMessage) return;
    const nextPrompt = editModal.value.trim();
    if (!nextPrompt) return;
    onRerunFromMessage(editModal.messageId, nextPrompt);
    setEditModal(null);
  }, [editModal, onRerunFromMessage]);

  const renderFlags = (m: Message) => {
    const badges: ReactNode[] = [];
    const meta = m.metadata;
    const cancelled = m.cancelled || meta?.cancelled;

    if (meta?.usedRetrieval) {
      badges.push(
        <span key="retrieval" style={flagStyle("rgba(99,102,241,0.14)", "rgba(99,102,241,0.9)", T.border)}>
          Retrieval
        </span>
      );
    }
    if (meta?.usedMemory) {
      badges.push(
        <span key="memory" style={flagStyle("rgba(168,85,247,0.14)", "rgba(168,85,247,0.95)", T.border)}>
          Memory
        </span>
      );
    }
    if (meta?.usedDirect) {
      badges.push(
        <span key="direct" style={flagStyle("rgba(34,197,94,0.12)", "rgba(34,197,94,0.9)", T.border)}>
          Direct
        </span>
      );
    }
    if ((meta?.sourceCount || 0) > 0) {
      badges.push(
        <span key="sources" style={flagStyle("rgba(148,163,184,0.12)", T.text2, T.border)}>
          Sources {meta?.sourceCount}
        </span>
      );
    }
    if ((meta?.evidenceCount || 0) > 0) {
      badges.push(
        <span key="evidence" style={flagStyle("rgba(59,130,246,0.12)", "rgba(59,130,246,0.9)", T.border)}>
          Evidence {meta?.evidenceCount}
        </span>
      );
    }
    if (typeof meta?.confidence === "number" && meta.confidence > 0) {
      badges.push(
        <span key="confidence" style={flagStyle("rgba(16,185,129,0.12)", "rgba(16,185,129,0.9)", T.border)}>
          Confidence {Math.round(meta.confidence * 100)}%
        </span>
      );
    }
    if (((meta?.evidenceCount || 0) > 0 || (meta?.confidence || 0) > 0) && typeof meta?.verified === "boolean") {
      badges.push(
        <span key="verified" style={flagStyle(meta.verified ? "rgba(34,197,94,0.12)" : "rgba(245,158,11,0.14)", meta.verified ? "rgba(34,197,94,0.95)" : "rgba(245,158,11,0.95)", T.border)}>
          {meta.verified ? "Verified" : "Review"}
        </span>
      );
    }
    if (cancelled) {
      badges.push(
        <span key="cancelled" style={flagStyle("rgba(239,68,68,0.12)", "rgba(239,68,68,0.85)", T.border)}>
          Cancelled
        </span>
      );
    }

    if (badges.length === 0) return null;

    return (
      <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", gap: 6 }}>
        {badges}
      </div>
    );
  };

  return (
    <div style={{ flex: 1, display: "flex", flexDirection: "column", height: "100%", minWidth: 0 }}>
      <div style={{ padding: "12px 20px", borderBottom: "1px solid " + T.border, display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div style={{ minWidth: 0 }}>
          <div style={{ fontSize: 15, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {activeChat?.title || "New Chat"}
            {activeChat?.pinned && " 📌"}
            {isArchived && (
              <span style={{ marginLeft: 8, fontSize: 11, color: T.text3, background: "rgba(128,128,128,0.1)", padding: "2px 8px", borderRadius: 4 }}>
                Archived
              </span>
            )}
          </div>
          <div style={{ fontSize: 11, color: T.text3, marginTop: 2 }}>Collection: {collSelector}</div>
        </div>
        <button onClick={onThemeToggle} style={{ background: "none", border: "none", cursor: "pointer", color: T.text3, padding: 6 }} title="Toggle theme">
          {theme === "dark" ? <I.Sun /> : <I.Moon />}
        </button>
      </div>

      <div ref={scrollRef} onScroll={handleScroll} style={{ flex: 1, overflowY: "auto", padding: "16px 20px", display: "flex", flexDirection: "column", position: "relative" }}>
        {(!activeChat || activeChat.messages.length === 0) && statusMsgs.length === 0 ? (
          isArchived ? (
            <div style={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", flex: 1, color: T.text3 }}>
              <h2 style={{ fontSize: 20, fontWeight: 300, marginBottom: 8 }}>📦 Archived</h2>
              <p style={{ fontSize: 13, textAlign: "center", maxWidth: 300 }}>Use ⋯ to unarchive.</p>
            </div>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", flex: 1, color: T.text3 }}>
              <h2 style={{ fontSize: 24, fontWeight: 300, marginBottom: 8 }}>Ask Knowledge Base</h2>
              <p style={{ fontSize: 13 }}>Type a query or upload a document.</p>
            </div>
          )
        ) : (
          <div ref={messagesRef} onClick={handleMessagesClick}>
            {activeChat?.messages.map((m, idx) => {
              const isLastUser = m.sender === "user" && idx === activeChat.messages.length - 1;
              const msgId = Number(m.id);
              const numericMsgId = Number.isFinite(msgId) ? msgId : 0;
              const msgSources = activeChat.messageSources?.[numericMsgId];
              const hasSources = !!msgSources && msgSources.length > 0;
              const processed = hasSources ? insertSourceBadges(m.text, msgSources) : m.text;
              const canEdit = m.sender === "user" && !isArchived && numericMsgId > 0 && !!onRerunFromMessage;
              const canRerun = m.sender === "user" && !isArchived && numericMsgId > 0 && !!onRerunFromMessage;
              const canCopy = m.sender === "ai" && numericMsgId > 0;
              return (
                <div key={m.id} ref={isLastUser ? lastUserMsgRef : undefined} data-msg-id={numericMsgId} style={{ marginBottom: 16, display: "flex", flexDirection: "column", alignItems: m.sender === "user" ? "flex-end" : "flex-start" }}>
                  <div style={{ fontSize: 11, color: T.text3, marginBottom: 4 }}>{m.sender === "user" ? "You" : "LocalRAG AI"}</div>
                  <div style={{ maxWidth: "80%", padding: "10px 14px", borderRadius: 12, background: m.sender === "user" ? T.bubbleUser : T.bubbleAI, fontSize: 13, lineHeight: 1.5, wordBreak: "break-word", overflow: "hidden" }}>
                    <Markdown text={processed} hasPreformattedHtml={hasSources} />
                  </div>

                  {m.sender === "user" && canEdit && canRerun && (
                    <div style={{ marginTop: 6, display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
                      <button onClick={() => openEditModal(numericMsgId, m.text)} style={messageActionBtnStyle(T)} title="Edit and rerun">
                        <I.Rename />
                        <span>Edit</span>
                      </button>
                      <button onClick={() => onRerunFromMessage?.(numericMsgId, m.text)} style={messageActionBtnStyle(T)} title="Rerun this question">
                        <I.Refresh />
                        <span>Rerun</span>
                      </button>
                    </div>
                  )}

                  {m.sender === "ai" && (
                    <>
                      {renderFlags(m)}
                      {hasSources && (
                        <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", gap: 6, alignItems: "center" }}>
                          <span style={{ ...flagStyle("rgba(148,163,184,0.12)", T.text2, T.border), fontWeight: 600 }}>Citations</span>
                          {Array.from(new Map(msgSources.map((s) => [s.refNumber, s])).values()).map((src) => (
                            <button
                              key={src.refNumber}
                              onClick={() => setSourceModal({ sources: msgSources, refNum: src.refNumber })}
                              style={messageActionBtnStyle(T)}
                              title={`Open citation [${src.refNumber}]`}
                            >
                              <span>[{src.refNumber}]</span>
                              <span>{src.filename}</span>
                            </button>
                          ))}
                        </div>
                      )}
                      {canCopy && (
                        <div style={{ marginTop: 6, display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
                          <button onClick={() => copyToClipboard(numericMsgId, m.text)} style={messageActionBtnStyle(T)} title="Copy response">
                            <I.Copy />
                            <span>{copiedMessageId === numericMsgId ? "Copied" : "Copy"}</span>
                          </button>
                        </div>
                      )}
                    </>
                  )}
                </div>
              );
            })}
          </div>
        )}

        {statusMsgs.map((sm) => (
          <div key={sm.id} style={{ marginBottom: 12, display: "flex", flexDirection: "column", alignItems: "flex-start" }}>
            <div style={{ fontSize: 11, color: T.text3, marginBottom: 4 }}>LocalRAG AI</div>
            <StatusBubble label={sm.text} T={T} />
          </div>
        ))}
        <div style={{ flex: 1, minHeight: 0 }} />
        {showScrollDown && (
          <button
            onClick={() => scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" })}
            style={{ position: "sticky", bottom: 8, alignSelf: "center", zIndex: 10, padding: "6px 14px", borderRadius: 20, border: "none", cursor: "pointer", fontSize: 11, fontWeight: 500, color: "#fff", background: "rgba(99,102,241,0.8)", boxShadow: "0 4px 12px rgba(0,0,0,0.3)", display: "flex", alignItems: "center", gap: 4 }}
          >
            <I.Down /> Scroll to bottom
          </button>
        )}
      </div>

      {/* Source Reference Modal */}
      {sourceModal && (
        <div onClick={() => setSourceModal(null)} style={{ position: "fixed", top: 0, left: 0, right: 0, bottom: 0, zIndex: 9999, display: "flex", alignItems: "center", justifyContent: "center", background: "rgba(0,0,0,0.6)", backdropFilter: "blur(4px)" }}>
          <div onClick={(e) => e.stopPropagation()} style={{ background: T.bg2, border: "1px solid " + T.border, borderRadius: 14, boxShadow: "0 20px 60px rgba(0,0,0,0.5)", padding: 24, width: "90%", maxWidth: 600, maxHeight: "80vh", display: "flex", flexDirection: "column", transition: "background 0.3s, border 0.3s" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16, flexShrink: 0 }}>
              <div style={{ fontSize: 16, fontWeight: 600, color: T.text }}>Source [{sourceModal.refNum}]</div>
              <button onClick={() => setSourceModal(null)} style={{ background: "none", border: "none", cursor: "pointer", color: T.text3, padding: 2 }}><I.X /></button>
            </div>
            {(() => {
              const src = sourceModal.sources.find((s) => s.refNumber === sourceModal.refNum);
              if (!src) return <div style={{ color: T.text3, fontSize: 13 }}>Source not found.</div>;
              return (
                <div style={{ overflowY: "auto", flex: 1 }}>
                  <div style={{ display: "flex", gap: 8, marginBottom: 12, flexWrap: "wrap" }}>
                    <span style={{ fontSize: 11, padding: "3px 8px", borderRadius: 4, background: "rgba(99,102,241,0.15)", color: "rgba(99,102,241,0.8)" }}>{src.filename}</span>
                    <span style={{ fontSize: 11, padding: "3px 8px", borderRadius: 4, background: "rgba(34,197,94,0.12)", color: "rgba(34,197,94,0.8)" }}>{src.collectionName}</span>
                    <span style={{ fontSize: 11, padding: "3px 8px", borderRadius: 4, background: T.bg2, border: "1px solid " + T.border, color: T.text3 }}>{(src.similarity * 100).toFixed(1)}% match</span>
                  </div>
                  <div style={{ fontSize: 12, lineHeight: 1.7, color: T.text2, whiteSpace: "pre-wrap", fontFamily: "monospace", padding: "12px", borderRadius: 8, border: "1px solid " + T.border, background: T.inputBg, wordBreak: "break-word", overflowX: "auto", transition: "background 0.3s, border 0.3s" }}>
                    {src.content}
                  </div>
                  {sourceModal.sources.length > 1 && (
                    <div style={{ display: "flex", gap: 6, marginTop: 12, flexWrap: "wrap" }}>
                      {sourceModal.sources.map((s) => (
                        <span
                          key={s.refNumber}
                          onClick={() => setSourceModal({ sources: sourceModal.sources, refNum: s.refNumber })}
                          style={{ fontSize: 11, padding: "3px 8px", borderRadius: 4, cursor: "pointer", background: s.refNumber === sourceModal.refNum ? "rgba(99,102,241,0.25)" : "transparent", color: s.refNumber === sourceModal.refNum ? "rgba(99,102,241,0.9)" : T.text3, border: s.refNumber === sourceModal.refNum ? "1px solid rgba(99,102,241,0.4)" : "1px solid transparent" }}
                        >
                          [{s.refNumber}] {s.filename}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              );
            })()}
          </div>
        </div>
      )}

      <div style={{ padding: "12px 16px", borderTop: "1px solid " + T.border }}>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <button onClick={onOpenUploadModal} style={{ background: "none", border: "none", cursor: "pointer", color: T.text3, padding: "6px", display: "flex", flexShrink: 0 }} title="Upload documents"><I.Paperclip /></button>
          <input value={input} onChange={(e) => onInputChange(e.target.value)} onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && onSend()} placeholder={isArchived ? "Archived..." : "Ask a question..."} style={{ flex: 1, padding: "10px 14px", borderRadius: 8, border: "1px solid " + T.border, background: T.inputBg, color: T.text, fontSize: 13, outline: "none", opacity: isArchived ? 0.4 : 1, transition: "background 0.3s" }} disabled={isArchived} />
          <button onClick={onSend} disabled={gen || isArchived} style={{ padding: "8px 14px", borderRadius: 8, border: "none", cursor: "pointer", fontSize: 13, fontWeight: 500, color: "#fff", background: "rgba(99,102,241,0.8)", opacity: (gen || isArchived) ? 0.5 : 1, display: "flex", alignItems: "center", justifyContent: "center", minWidth: 36 }}>{gen ? <I.Spinner /> : <I.Send />}</button>
          {gen && onStopGeneration && (
            <button onClick={onStopGeneration} style={{ padding: "8px 14px", borderRadius: 8, border: "none", cursor: "pointer", fontSize: 13, fontWeight: 500, color: "#fff", background: "rgba(239,68,68,0.8)", display: "flex", alignItems: "center", justifyContent: "center", gap: 4, minWidth: 36 }} title="Stop generation">
              ■
            </button>
          )}
        </div>
      </div>

      {editModal && (
        <Modal open={editModal.open} onClose={() => setEditModal(null)} title="Edit and Rerun" theme={theme}>
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            <textarea
              value={editModal.value}
              onChange={(e) => setEditModal((p) => (p ? { ...p, value: e.target.value } : p))}
              rows={6}
              autoFocus
              style={{ width: "100%", padding: "10px 14px", borderRadius: 8, border: "1px solid " + T.border, background: T.inputBg, color: T.text, fontSize: 13, outline: "none", resize: "vertical", minHeight: 120 }}
            />
            <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
              <button onClick={() => setEditModal(null)} style={secondaryBtnStyle(T)}>Cancel</button>
              <button onClick={submitEdit} style={primaryBtnStyle}>Rerun</button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}

function StatusBubble({ label, T }: { label: string; T?: ThemeVars }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 18px", marginBottom: 12, borderRadius: 16, borderBottomLeftRadius: 4, background: "rgba(99,102,241,0.1)", border: "1px solid rgba(99,102,241,0.15)", alignSelf: "flex-start", maxWidth: "fit-content" }}>
      <I.Spinner /><span style={{ fontSize: 12, color: T ? T.text3 : "rgba(255,255,255,0.6)" }}>{label}</span>
    </div>
  );
}

function insertSourceBadges(text: string, sources: SourceRef[]): string {
  return text.replace(/\[(\d+)\]/g, (_m, num) => {
    const refNum = parseInt(num);
    const src = sources.find((s) => s.refNumber === refNum);
    return `<sup><span data-ref="${refNum}" style="cursor:pointer;color:rgba(99,102,241,0.9);font-weight:600;font-size:11px;background:rgba(99,102,241,0.12);padding:1px 5px;border-radius:3px;margin:0 1px;display:inline-block" title="${src ? `Source: ${src.filename}` : `Reference ${refNum}`}">[${refNum}]</span></sup>`;
  });
}

function flagStyle(bg: string, color: string, border: string): CSSProperties {
  return {
    fontSize: 10,
    padding: "3px 8px",
    borderRadius: 999,
    background: bg,
    color,
    border: "1px solid " + border,
    lineHeight: 1.2,
    display: "inline-flex",
    alignItems: "center",
    gap: 4,
  };
}

function messageActionBtnStyle(T: ThemeVars): CSSProperties {
  return {
    display: "inline-flex",
    alignItems: "center",
    gap: 6,
    padding: "4px 10px",
    borderRadius: 999,
    border: "1px solid " + T.border,
    background: T.inputBg,
    color: T.text2,
    fontSize: 11,
    cursor: "pointer",
    lineHeight: 1,
  };
}

function secondaryBtnStyle(T: ThemeVars): CSSProperties {
  return {
    padding: "8px 16px",
    borderRadius: 8,
    border: "1px solid " + T.border,
    cursor: "pointer",
    fontSize: 13,
    color: T.text2,
    background: "transparent",
  };
}

const primaryBtnStyle: CSSProperties = {
  padding: "8px 16px",
  borderRadius: 8,
  border: "none",
  cursor: "pointer",
  fontSize: 13,
  fontWeight: 500,
  color: "#fff",
  background: "rgba(99,102,241,0.8)",
};
