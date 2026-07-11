import { ChangeEvent, memo, useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { FileUploadItem, Theme, themeVars, IncompleteJob } from "../types";
import { Modal } from "./Modal";
import { I } from "./Icons";

interface FileUploadModalProps {
  open: boolean;
  onClose: () => void;
  collectionId: number;
  collectionName: string;
  onStartBatch: (files: { file: File; replace: boolean }[]) => Promise<any>;
  onIngestPaste?: (filename: string, content: string) => Promise<string>;
  isIngesting: boolean;
  incompleteJobs: IncompleteJob[];
  theme: Theme;
}

const ACTIVE_STATUSES: FileUploadItem["status"][] = ["processing", "queued", "embedding", "staged"];

const mapIncompleteJobToFile = (job: IncompleteJob): FileUploadItem => {
  const status = (job.status || "queued").toLowerCase();
  const validStatus = ["pending", "processing", "duplicate", "replaced", "success", "error", "queued", "embedding", "failed", "staged"].includes(status)
    ? status as FileUploadItem["status"]
    : "processing";
  const progressMsg = validStatus === "error"
    ? job.errorMessage || "Failed"
    : validStatus === "success"
      ? "✓ Done"
      : validStatus === "queued"
        ? "Queued"
        : validStatus === "embedding"
          ? "Embedding…"
          : validStatus === "staged"
            ? "Staged"
            : "Processing…";

  return {
    id: `job-${job.docId}`,
    filename: job.filename,
    status: validStatus,
    docId: job.docId,
    progressPct: job.progressPct ?? 0,
    progressMsg,
    message: job.errorMessage || undefined,
  };
};

const getFileStatusLabel = (item: FileUploadItem) => {
  if (item.status === "pending") return "Pending";
  if (item.status === "success") return "✓ Done";
  if (item.status === "replaced") return "↺ Replaced";
  if (item.status === "duplicate") return "Duplicate";
  if (item.status === "error" || item.status === "failed") return "Error";
  if (ACTIVE_STATUSES.includes(item.status)) return "Processing";
  return "Ready";
};

const UploadFileRow = memo(function UploadFileRow({
  item,
  theme,
  onRemove,
}: {
  item: FileUploadItem;
  theme: Theme;
  onRemove: (id: string) => void;
}) {
  const T = themeVars[theme];
  const statusLabel = getFileStatusLabel(item);
  const isActive = ACTIVE_STATUSES.includes(item.status);

  return (
    <div style={{
      padding: "8px 0 10px",
      display: "flex",
      flexDirection: "column",
      gap: 6,
      borderBottom: "1px solid " + T.border,
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12, lineHeight: "20px" }}>
        <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: T.text }}>{item.filename}</span>
        <div style={{ display: "flex", alignItems: "center", gap: 4, flexShrink: 0, width: 96, justifyContent: "flex-end", minHeight: 20 }}>
          {item.status === "pending" && <span style={{ color: T.text3 }}>{statusLabel}</span>}
          {isActive && <I.Spinner />}
          {(item.status === "success" || item.status === "replaced") && (<span style={{color: "rgba(34,197,94,0.9)",display: "flex",alignItems: "center",gap: 4,fontWeight: 500,}}><I.Check />{statusLabel}</span>)}
          {item.status === "duplicate" && <span style={{ color: "rgba(234,179,8,0.8)", fontSize: 10 }}>{statusLabel}</span>}
          {(item.status === "error" || item.status === "failed") && <span style={{ color: "rgba(239,68,68,0.8)", fontSize: 10 }} title={item.message}>{statusLabel}</span>}
          {item.status === "pending" && (
            <button
              onClick={() => onRemove(item.id)}
              style={{ background: "none", border: "none", cursor: "pointer", color: T.text3, padding: 2, display: "flex" }}
              aria-label={`Remove ${item.filename}`}
              title={`Remove ${item.filename}`}
            >
              <I.X />
            </button>
          )}
        </div>
      </div>

      <div style={{ height: 14, display: "flex", alignItems: "center", gap: 8, fontSize: 10, color: T.text3 }}>
        {isActive ? (
          <>
            <div style={{ flex: 1, height: 4, borderRadius: 2, background: T.border, overflow: "hidden" }}>
              <div style={{ width: Math.max(2, Math.min(100, item.progressPct || 0)) + "%", height: "100%", borderRadius: 2, background: "rgba(99,102,241,0.7)", transition: "width 0.2s ease" }} />
            </div>
            <span style={{ flexShrink: 0, width: 96, textAlign: "right", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.progressMsg || ""}</span>
          </>
        ) : item.status === "success" || item.status === "replaced" ? (
          <span style={{ color: "rgba(34,197,94,0.8)" }}>{item.progressMsg || "✓ Ingested successfully"}</span>
        ) : item.status === "error" || item.status === "failed" ? (
          <span style={{ color: "rgba(239,68,68,0.8)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.message || "Upload failed"}</span>
        ) : item.status === "duplicate" ? (
          <span style={{ color: "rgba(234,179,8,0.8)" }}>{item.progressMsg || "Duplicate"}</span>
        ) : (
          <span>Ready to stage & embed</span>
        )}
      </div>
    </div>
  );
}, (prev, next) => prev.item === next.item && prev.theme === next.theme && prev.onRemove === next.onRemove);

export function FileUploadModal({
  open, onClose, collectionId, collectionName,
  onStartBatch, onIngestPaste,
  isIngesting, incompleteJobs, theme,
}: FileUploadModalProps) {
  const [mode, setMode] = useState<"upload" | "paste">("upload");
  const [files, setFiles] = useState<FileUploadItem[]>([]);
  const [uploading, setUploading] = useState(false);
  const [pasting, setPasting] = useState(false);
  const [pasteFilename, setPasteFilename] = useState("");
  const [pasteContent, setPasteContent] = useState("");
  const [pasteStatus, setPasteStatus] = useState("");
  const [pastePct, setPastePct] = useState(0);
  const [pasteLabel, setPasteLabel] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const T = themeVars[theme];

  const modeRef = useRef(mode);
  const pastingRef = useRef(pasting);

  useEffect(() => { modeRef.current = mode; }, [mode]);
  useEffect(() => { pastingRef.current = pasting; }, [pasting]);

  const isActiveFileStatus = (status: FileUploadItem["status"]): status is "processing" | "queued" | "embedding" | "staged" =>
    status === "processing" || status === "queued" || status === "embedding" || status === "staged";

  useEffect(() => {
    if (!open) return;

    const off = Events.On("ingest:progress", (e: any) => {
      if (!e.data) return;
      const phase = e.data.phase || "";
      const step = e.data.step;
      const pct = e.data.pct || 0;
      const label = e.data.label || "";
      const filename = e.data.filename || "";
      const docId = e.data.docId;

      if (phase === "staging" || step === "staging") {
        if (filename) {
          setFiles(prev => prev.map(f =>
            f.filename === filename || f.file?.name === filename
              ? { ...f, status: "processing", progressMsg: "Extracting...", progressPct: 0 }
              : f
          ));
        }
      } else if (phase === "staging_done") {
        // no-op; final UI state is handled after batch completion
      } else if (phase === "embedding" || step === "embedding") {
        const detail = e.data.detail || "";
        const progressTxt = detail ? `${detail}` : (label || "Embedding...");
        if (filename || docId) {
          setFiles(prev => prev.map(f => {
            if (filename && (f.filename === filename || f.file?.name === filename)) {
              return { ...f, status: "processing", progressMsg: progressTxt, progressPct: pct, docId: docId || f.docId };
            }
            if (docId && f.docId === docId) {
              return { ...f, status: "processing", progressMsg: progressTxt, progressPct: pct };
            }
            return f;
          }));
        }
      } else if (step === "chunked") {
        if (filename || docId) {
          setFiles(prev => prev.map(f => {
            if (filename && (f.filename === filename || f.file?.name === filename)) {
              return { ...f, status: "processing", docId: docId || f.docId };
            }
            if (docId && f.docId === docId) {
              return { ...f, status: "processing" };
            }
            return f;
          }));
        }
      } else if (step === "doc_ready") {
        if (filename || docId) {
          setFiles(prev => prev.map(f => {
            if ((filename && (f.filename === filename || f.file?.name === filename)) || (docId && f.docId === docId)) {
              return { ...f, status: "success", progressMsg: "✓ Done", progressPct: 100, docId: docId || f.docId };
            }
            return f;
          }));
        }
      } else if (step === "doc_failed") {
        if (filename || docId) {
          setFiles(prev => prev.map(f => {
            if ((filename && (f.filename === filename || f.file?.name === filename)) || (docId && f.docId === docId)) {
              return { ...f, status: "error", message: e.data.error || "Failed", progressMsg: e.data.error || "Failed", docId: docId || f.docId };
            }
            return f;
          }));
        }
      }

      if (modeRef.current === "paste" && pastingRef.current) {
        if (phase === "staging" || step === "staging") {
          setPasteLabel("Extracting text…");
          setPastePct(5);
        } else if (phase === "embedding" || step === "embedding") {
          setPasteLabel(label || "Embedding…");
          setPastePct(pct);
        } else if (phase === "batch_done" || step === "complete") {
          setPasteLabel("✓ Done!");
          setPastePct(100);
        }
      }
    });

    return () => off();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    setFiles(prev => {
      const existingIds = new Set(prev.map(f => f.docId ? `job-${f.docId}` : f.id));
      const jobs = incompleteJobs.map(mapIncompleteJobToFile).filter(job => !existingIds.has(job.id));
      if (jobs.length === 0) return prev;
      return [...prev, ...jobs];
    });
  }, [open, incompleteJobs]);

  useEffect(() => {
    if (open && !uploading && !pasting) {
      if (files.length === 0) {
        setPasteFilename("");
        setPasteContent("");
        setPasteStatus("");
        setPastePct(0);
        setPasteLabel("");
      }
    }
  }, [open, files, uploading, pasting]);

  const handleSelect = (e: ChangeEvent<HTMLInputElement>) => {
    const selected = Array.from(e.target.files || []) as File[];
    setFiles(prev => [
      ...prev,
      ...selected.map(f => ({
        id: crypto.randomUUID(),
        file: f,
        filename: f.name,
        status: "pending" as const,
        progressPct: 0,
      })),
    ]);
    if (e.target) e.target.value = "";
  };

  const uploadAll = async () => {
    const pending = files.filter(f => f.status === "pending" && f.file);
    if (pending.length === 0 || uploading) return;
    setUploading(true);
    setFiles(prev => prev.map(f => f.status === "pending" ? { ...f, status: "processing", progressMsg: "Waiting to stage…", progressPct: 0 } : f));

    try {
      const result: any = await onStartBatch(pending.map(f => ({ file: f.file!, replace: false })));
      const items: any[] = result?.items || result?.Items || [];
      const cancelled = !!(result?.cancelled ?? result?.Cancelled);

      setFiles(prev => prev.map(f => {
        const item = items.find((i: any) => (i.filename || i.Filename) === f.filename);
        if (!item) {
          if (f.status === "processing") {
            return cancelled
              ? { ...f, status: "error" as const, message: "Interrupted — resume from incomplete jobs", progressMsg: "Interrupted" }
              : { ...f, status: "success" as const, progressMsg: "✓ Done", progressPct: 100 };
          }
          return f;
        }

        const st = item.status || item.Status;
        const msg = item.message || item.Message || "";
        const docId = item.docId ?? item.DocID ?? f.docId;

        if (st === "duplicate") return { ...f, status: "duplicate" as const, message: msg, docId, progressMsg: msg, progressPct: 0 };
        if (st === "error") return { ...f, status: "error" as const, message: msg, docId, progressMsg: msg, progressPct: 0 };
        if (cancelled) return { ...f, status: "error" as const, message: "Interrupted — resume from incomplete jobs", docId, progressMsg: "Interrupted", progressPct: f.progressPct || 0 };
        if (st === "replaced") return { ...f, status: "replaced" as const, docId, progressMsg: "✓ Replaced", progressPct: 100 };

        if (f.status === "error" || f.status === "failed") return f;
        return { ...f, status: "success" as const, docId, progressMsg: "✓ Done", progressPct: 100 };
      }));
    } catch (e: any) {
      const errMsg = e?.message || "Batch ingest failed";
      setFiles(prev => prev.map(f =>
        f.status === "processing"
          ? { ...f, status: "error", message: errMsg, progressMsg: errMsg }
          : f
      ));
    } finally {
      setUploading(false);
    }
  };

  const removeFile = useCallback((id: string) => setFiles(prev => prev.filter(f => f.id !== id)), []);
  const pendingCount = files.filter(f => f.status === "pending").length;
  const errorCount = files.filter(f => f.status === "error").length;
  const successCount = files.filter(f => f.status === "success" || f.status === "replaced").length;
  const hasCompleted = files.length > 0 && pendingCount === 0 && !uploading && !isIngesting;
  const isProcessing = uploading || pasting || isIngesting || files.some(f => isActiveFileStatus(f.status));
  const busy = uploading || pasting || isIngesting;

  const handlePasteSubmit = async () => {
    if (!pasteContent.trim() || !onIngestPaste || pasting) return;
    const fn = pasteFilename.trim() || "pasted.txt";
    if (fn.length < 3) {
      setPasteStatus("Filename must be at least 3 characters");
      return;
    }
    setPasting(true);
    setPasteStatus("Processing…");
    setPastePct(0);
    setPasteLabel("Extracting text…");
    try {
      const result = await onIngestPaste(fn, pasteContent);
      if (result === "success") {
        setPasteStatus("✓ Ingested!");
        setPasteLabel("✓ Done!");
        setPastePct(100);
        setPasteFilename("");
        setPasteContent("");
      } else {
        setPasteStatus(result || "Failed");
        setPasteLabel("");
        setPastePct(0);
      }
    } catch (e: any) {
      setPasteStatus(e?.message || "Failed");
      setPasteLabel("");
      setPastePct(0);
    } finally {
      setPasting(false);
    }
  };

  const B = { background: T.bg2, border: "1px solid " + T.border, color: T.text, fontSize: 13, outline: "none" as const, width: "100%", padding: "10px 14px", borderRadius: 8 };

  const handleClose = () => {
    if (isProcessing) {
      onClose();
      return;
    }
    setFiles([]);
    setUploading(false);
    setPasting(false);
    setPasteFilename("");
    setPasteContent("");
    setPasteStatus("");
    setPastePct(0);
    setPasteLabel("");
    onClose();
  };

  const footerButtonStyle = isProcessing && mode === "upload"
    ? { ...btnStyle, opacity: 0.7, cursor: "default" as const }
    : btnStyle;

  return (
    <Modal open={open} onClose={handleClose} title="Add Document" wide theme={theme}>
      <div style={{ display: "flex", flexDirection: "column", gap: 12, minHeight: 0 }}>
        <div style={{ fontSize: 12, color: T.text3 }}>
          Target: <strong style={{ color: T.text }}>{collectionName}</strong>
        </div>

        <div style={{ display: "flex", gap: 4, background: T.bg2, borderRadius: 8, padding: 3, flexShrink: 0 }}>
          {(["upload", "paste"] as const).map(m => (
            <button
              key={m}
              onClick={() => !busy && setMode(m)}
              style={{
                flex: 1,
                padding: "6px",
                borderRadius: 6,
                border: "none",
                cursor: busy ? "default" : "pointer",
                fontSize: 12,
                fontWeight: 500,
                color: mode === m ? "#fff" : T.text3,
                background: mode === m ? "rgba(99,102,241,0.6)" : "transparent",
              }}
            >
              {m === "upload" ? <><I.Paperclip /> Upload Files</> : <>Paste Text</>}
            </button>
          ))}
        </div>

        {mode === "upload" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 12, minHeight: 0 }}>
            <div style={{ fontSize: 11, color: T.text3 }}>PDF, DOCX, TXT supported · All files are extracted before embedding</div>
            <input ref={fileRef} type="file" multiple accept=".pdf,.docx,.txt" onChange={handleSelect} style={{ display: "none" }} disabled={busy} />

            <div style={{ minHeight: 72, flexShrink: 0 }}>
              {!busy && files.length === 0 && (
                <div
                  style={{
                    padding: "24px",
                    borderRadius: 8,
                    border: "2px dashed " + T.border,
                    textAlign: "center",
                    color: T.text3,
                    fontSize: 13,
                    cursor: "pointer",
                  }}
                  onClick={() => fileRef.current?.click()}
                  onDragOver={e => e.preventDefault()}
                  onDrop={e => {
                    e.preventDefault();
                    const dt = e.dataTransfer?.files;
                    if (dt) handleSelect({ target: { files: dt } } as any);
                  }}
                >
                  <I.Paperclip /><br />Drop files here or click to browse
                </div>
              )}
            </div>

            <div style={{ flex: 1, minHeight: 260, overflowY: "auto" , overflowX: "hidden", paddingRight: 2 }}>
              {files.length > 0 ? (
                files.map(f => (
                  <UploadFileRow key={f.id} item={f} theme={theme} onRemove={removeFile} />
                ))
              ) : (
                <div style={{ minHeight: 1 }} />
              )}
            </div>

            <div style={{ minHeight: 58, display: "flex", alignItems: "center" }}>
              {pendingCount > 0 && !busy && (
                <button onClick={uploadAll} style={btnStyle}>
                  Stage & Embed {pendingCount} File{pendingCount > 1 ? "s" : ""}
                </button>
              )}

              {busy && mode === "upload" && (
                <button disabled style={footerButtonStyle}>
                  <I.Spinner /> Processing
                </button>
              )}

              {hasCompleted && (
                <div style={{ width: "100%", display: "flex", flexDirection: "column", gap: 6 }}>
                  {successCount > 0 && (
                    <div style={{ fontSize: 12, color: "rgba(34,197,94,0.8)", textAlign: "center" }}>
                      ✓ {successCount} succeeded{errorCount > 0 ? `, ${errorCount} failed` : ""}
                    </div>
                  )}
                  <button onClick={handleClose} style={{ ...btnStyle, background: errorCount === 0 ? "rgba(34,197,94,0.8)" : "rgba(99,102,241,0.8)" }}>
                    {errorCount === 0 ? "✓ Done" : "Close"}
                  </button>
                </div>
              )}

              {!busy && pendingCount === 0 && !hasCompleted && (
                <div style={{ width: "100%", minHeight: 58 }} />
              )}
            </div>
          </div>
        )}

        {mode === "paste" && onIngestPaste && (
          <div style={{ display: "flex", flexDirection: "column", gap: 8, minHeight: 0 }}>
            <input
              value={pasteFilename}
              onChange={e => {
                if (pasteStatus.startsWith("✓")) setPasteStatus("");
                setPasteFilename(e.target.value);
              }}
              placeholder="Filename (min 3 chars)"
              style={{ ...B, border: "1px solid " + T.border, background: T.bg2, color: T.text, opacity: pasting ? 0.5 : 1 }}
              disabled={pasting || isIngesting}
            />
            {pasteFilename.trim().length > 0 && pasteFilename.trim().length < 3 && !pasting && (
              <div style={{ fontSize: 10, color: "rgba(239,68,68,0.7)", marginTop: -6 }}>Filename must be at least 3 characters</div>
            )}
            <textarea
              value={pasteContent}
              onChange={e => {
                if (pasteStatus.startsWith("✓")) setPasteStatus("");
                setPasteContent(e.target.value);
              }}
              placeholder="Paste document content..."
              style={{ ...B, minHeight: 100, resize: "vertical", fontFamily: "monospace", fontSize: 12 }}
              disabled={pasting || isIngesting}
            />

            {pasting && pasteLabel && (
              <div style={{ padding: "8px 12px", borderRadius: 6, background: "rgba(99,102,241,0.08)", display: "flex", alignItems: "center", gap: 8 }}>
                <div style={{ flex: 1, height: 4, borderRadius: 2, background: T.border, overflow: "hidden" }}>
                  <div style={{ width: pastePct + "%", height: "100%", borderRadius: 2, background: "rgba(99,102,241,0.7)", transition: "width 0.2s ease" }} />
                </div>
                <span style={{ fontSize: 10, color: T.text3, whiteSpace: "nowrap" }}>{pasteLabel}</span>
              </div>
            )}

            {!pasting && !pasteStatus.startsWith("✓") && (
              <button
                onClick={handlePasteSubmit}
                disabled={!pasteContent.trim() || pasteFilename.trim().length < 3 || isIngesting}
                style={{ ...btnStyle, opacity: (!pasteContent.trim() || pasteFilename.trim().length < 3 || isIngesting) ? 0.5 : 1 }}
              >
                Ingest Text
              </button>
            )}

            {pasteStatus && !pasting && (
              <div style={{ fontSize: 12, padding: "8px", borderRadius: 6, background: pasteStatus.startsWith("✓") ? "rgba(34,197,94,0.1)" : "rgba(239,68,68,0.1)", color: pasteStatus.startsWith("✓") ? "rgba(34,197,94,0.9)" : "rgba(239,68,68,0.9)", textAlign: "center" }}>
                {pasteStatus}
                {pasteStatus.startsWith("✓") && <button onClick={handleClose} style={{ marginLeft: 8, padding: "2px 10px", borderRadius: 4, border: "none", cursor: "pointer", fontSize: 11, color: "#fff", background: "rgba(34,197,94,0.8)" }}>Close</button>}
              </div>
            )}
          </div>
        )}
      </div>
    </Modal>
  );
}

const btnStyle: React.CSSProperties = {
  width: "100%",
  minHeight: 38,
  padding: "10px",
  borderRadius: 8,
  border: "none",
  cursor: "pointer",
  fontSize: 13,
  fontWeight: 500,
  color: "#fff",
  background: "rgba(99,102,241,0.8)",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  gap: 8,
};
