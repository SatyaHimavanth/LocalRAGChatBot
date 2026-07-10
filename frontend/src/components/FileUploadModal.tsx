import { useState, useRef, useEffect, ChangeEvent } from "react";
import { Events } from "@wailsio/runtime";
import { FileUploadItem, IncompleteJob, Theme, themeVars } from "../types";
import { Modal } from "./Modal";
import { I } from "./Icons";

interface FileUploadModalProps {
  open: boolean;
  onClose: () => void;
  collectionId: number;
  collectionName: string;
  /** Stage+embed a batch of browser files (two-phase on backend). */
  onStartBatch: (files: { file: File; replace: boolean }[]) => Promise<any>;
  onIngestPaste?: (filename: string, content: string) => Promise<string>;
  incompleteJobs: IncompleteJob[];
  onResumeJobs: () => Promise<void>;
  onDiscardJob: (docId: number) => Promise<void>;
  onDiscardAllJobs: () => Promise<void>;
  isIngesting: boolean;
  theme: Theme;
}

export function FileUploadModal({
  open, onClose, collectionId, collectionName,
  onStartBatch, onIngestPaste,
  incompleteJobs, onResumeJobs, onDiscardJob, onDiscardAllJobs,
  isIngesting, theme,
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
  const [phaseLabel, setPhaseLabel] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const T = themeVars[theme];

  const jobsForCollection = incompleteJobs.filter(j => j.collectionId === collectionId);

  useEffect(() => {
    if (!uploading && !pasting && !isIngesting) return;
    const off = Events.On("ingest:progress", (e: any) => {
      if (!e.data) return;
      const phase = e.data.phase || "";
      const step = e.data.step;
      const pct = e.data.pct || 0;
      const label = e.data.label || "";
      const filename = e.data.filename || "";

      if (phase === "staging" || step === "staging") {
        setPhaseLabel(label || `Extracting text…`);
        if (filename) {
          setFiles(prev => prev.map(f =>
            f.filename === filename || f.file?.name === filename
              ? { ...f, status: "processing", progressMsg: label, progressPct: pct }
              : f
          ));
        }
      } else if (phase === "staging_done") {
        setPhaseLabel(label || "Staging complete");
        setFiles(prev => prev.map(f =>
          f.status === "processing" || f.status === "pending"
            ? f
            : f
        ));
      } else if (phase === "embedding" || step === "embedding" || step === "chunked") {
        setPhaseLabel(label || "Embedding…");
        if (filename || e.data.docId) {
          setFiles(prev => prev.map(f => {
            const match = (filename && (f.filename === filename || f.file?.name === filename))
              || (e.data.docId && f.docId === e.data.docId);
            if (!match && filename) {
              // update the processing row by name if docId not yet known
              return f;
            }
            if (filename && (f.filename === filename || f.file?.name === filename)) {
              return { ...f, status: "processing", progressMsg: label, progressPct: pct, docId: e.data.docId || f.docId };
            }
            if (e.data.docId && f.docId === e.data.docId) {
              return { ...f, status: "processing", progressMsg: label, progressPct: pct };
            }
            return f;
          }));
        }
      } else if (step === "doc_ready") {
        if (filename || e.data.docId) {
          setFiles(prev => prev.map(f => {
            if ((filename && (f.filename === filename || f.file?.name === filename)) || (e.data.docId && f.docId === e.data.docId)) {
              return { ...f, status: "success", progressMsg: "✓ Done", progressPct: 100, docId: e.data.docId || f.docId };
            }
            return f;
          }));
        }
      } else if (step === "doc_failed") {
        if (filename || e.data.docId) {
          setFiles(prev => prev.map(f => {
            if ((filename && (f.filename === filename || f.file?.name === filename)) || (e.data.docId && f.docId === e.data.docId)) {
              return { ...f, status: "error", message: e.data.error || "Failed", progressMsg: e.data.error || "Failed", docId: e.data.docId || f.docId };
            }
            return f;
          }));
        }
      } else if (phase === "batch_done" || step === "complete") {
        setPhaseLabel(label || "Done");
      }

      if (mode === "paste" && pasting) {
        if (phase === "staging" || step === "staging") { setPasteLabel("Extracting text…"); setPastePct(5); }
        else if (phase === "embedding" || step === "embedding") { setPasteLabel(label || "Embedding…"); setPastePct(pct); }
        else if (phase === "batch_done" || step === "complete") { setPasteLabel("✓ Done!"); setPastePct(100); }
      }
    });
    return () => off();
  }, [uploading, pasting, isIngesting, mode]);

  useEffect(() => {
    if (open && !uploading && !pasting) {
      if (files.length === 0) {
        setPasteFilename(""); setPasteContent(""); setPasteStatus("");
        setPastePct(0); setPasteLabel(""); setPhaseLabel("");
      }
    }
  }, [open]);

  const handleSelect = (e: ChangeEvent<HTMLInputElement>) => {
    const selected = Array.from(e.target.files || []);
    setFiles(p => [
      ...p,
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
    setPhaseLabel("Extracting text from all files…");
    setFiles(p => p.map(f => f.status === "pending" ? { ...f, status: "processing", progressMsg: "Waiting to stage…", progressPct: 0 } : f));
    try {
      const result: any = await onStartBatch(pending.map(f => ({ file: f.file!, replace: false })));
      const items: any[] = result?.items || result?.Items || [];
      const cancelled = !!(result?.cancelled ?? result?.Cancelled);
      setFiles(p => p.map(f => {
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
        // staged successfully and embed finished (or failed — events may have set error already)
        if (f.status === "error" || f.status === "failed") return f;
        return { ...f, status: "success" as const, docId, progressMsg: "✓ Done", progressPct: 100 };
      }));
    } catch (e: any) {
      const errMsg = e?.message || "Batch ingest failed";
      setFiles(p => p.map(f =>
        f.status === "processing"
          ? { ...f, status: "error", message: errMsg, progressMsg: errMsg }
          : f
      ));
    }
    setUploading(false);
    setPhaseLabel("");
  };

  const removeFile = (id: string) => setFiles(p => p.filter(f => f.id !== id));
  const pendingCount = files.filter(f => f.status === "pending").length;
  const errorCount = files.filter(f => f.status === "error").length;
  const successCount = files.filter(f => f.status === "success" || f.status === "replaced").length;
  const hasCompleted = files.length > 0 && pendingCount === 0 && !uploading && !isIngesting;

  const handlePasteSubmit = async () => {
    if (!pasteContent.trim() || !onIngestPaste || pasting) return;
    const fn = pasteFilename.trim() || "pasted.txt";
    if (fn.length < 3) { setPasteStatus("Filename must be at least 3 characters"); return; }
    setPasting(true); setPasteStatus("Processing…"); setPastePct(0); setPasteLabel("Extracting text…");
    try {
      const result = await onIngestPaste(fn, pasteContent);
      if (result === "success") {
        setPasteStatus("✓ Ingested!"); setPasteLabel("✓ Done!"); setPastePct(100);
        setPasteFilename(""); setPasteContent("");
      } else {
        setPasteStatus(result || "Failed"); setPasteLabel(""); setPastePct(0);
      }
    } catch (e: any) {
      setPasteStatus(e?.message || "Failed"); setPasteLabel(""); setPastePct(0);
    }
    setPasting(false);
  };

  const handleResume = async () => {
    setUploading(true);
    setPhaseLabel("Resuming embedding…");
    try {
      await onResumeJobs();
    } catch (e: any) {
      setPhaseLabel(e?.message || "Resume failed");
    }
    setUploading(false);
    setPhaseLabel("");
  };

  const B = { background: T.bg2, border: "1px solid " + T.border, color: T.text, fontSize: 13, outline: "none" as const, width: "100%", padding: "10px 14px", borderRadius: 8 };
  const busy = uploading || pasting || isIngesting;

  return (
    <Modal open={open} onClose={onClose} title="Add Document" wide theme={theme}>
      <div style={{ fontSize: 12, color: T.text3, marginBottom: 12 }}>
        Target: <strong style={{ color: T.text }}>{collectionName}</strong>
      </div>

      {/* Incomplete jobs from previous sessions */}
      {jobsForCollection.length > 0 && (
        <div style={{ marginBottom: 14, padding: 12, borderRadius: 8, background: "rgba(234,179,8,0.08)", border: "1px solid rgba(234,179,8,0.25)" }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: "rgba(234,179,8,0.95)", marginBottom: 6 }}>
            Incomplete ingest jobs ({jobsForCollection.length})
          </div>
          <div style={{ fontSize: 11, color: T.text3, marginBottom: 8 }}>
            Text was saved before exit. Resume embedding or discard.
          </div>
          <div style={{ maxHeight: 120, overflowY: "auto", marginBottom: 8 }}>
            {jobsForCollection.map(j => (
              <div key={j.docId} style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 11, padding: "4px 0", borderBottom: "1px solid " + T.border }}>
                <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: T.text }}>{j.filename}</span>
                <span style={{ color: T.text3, flexShrink: 0 }}>{j.status} · {j.chunkCount}/{j.expectedChunks || "?"}</span>
                <button
                  onClick={() => onDiscardJob(j.docId)}
                  disabled={busy}
                  style={{ background: "none", border: "none", cursor: busy ? "default" : "pointer", color: "rgba(239,68,68,0.7)", fontSize: 10, padding: 2 }}
                >Discard</button>
              </div>
            ))}
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <button onClick={handleResume} disabled={busy} style={{ ...btnStyle, flex: 1, opacity: busy ? 0.5 : 1, minHeight: 32, padding: "6px" }}>
              Resume embedding
            </button>
            <button onClick={onDiscardAllJobs} disabled={busy} style={{ ...btnStyle, flex: 1, minHeight: 32, padding: "6px", background: "rgba(239,68,68,0.7)", opacity: busy ? 0.5 : 1 }}>
              Discard all
            </button>
          </div>
        </div>
      )}

      <div style={{ display: "flex", gap: 4, marginBottom: 12, background: T.bg2, borderRadius: 8, padding: 3 }}>
        {(["upload", "paste"] as const).map(m => (
          <button key={m} onClick={() => !busy && setMode(m)} style={{ flex: 1, padding: "6px", borderRadius: 6, border: "none", cursor: busy ? "default" : "pointer", fontSize: 12, fontWeight: 500, color: mode === m ? "#fff" : T.text3, background: mode === m ? "rgba(99,102,241,0.6)" : "transparent" }}>
            {m === "upload" ? <><I.Paperclip /> Upload Files</> : <>Paste Text</>}
          </button>
        ))}
      </div>

      {mode === "upload" && (
        <>
          <div style={{ fontSize: 11, color: T.text3, marginBottom: 8 }}>PDF, DOCX, TXT supported · All files are extracted before embedding</div>
          <input ref={fileRef} type="file" multiple accept=".pdf,.docx,.txt" onChange={handleSelect} style={{ display: "none" }} disabled={busy} />
          <div style={{ minHeight: 73, marginBottom: 12 }}>
            {!busy && files.length === 0 && (
              <div style={{ padding: "24px", borderRadius: 8, border: "2px dashed " + T.border, textAlign: "center", color: T.text3, fontSize: 13, cursor: "pointer" }}
                onClick={() => fileRef.current?.click()}
                onDragOver={e => e.preventDefault()}
                onDrop={e => { e.preventDefault(); const dt = e.dataTransfer?.files; if (dt) handleSelect({ target: { files: dt } } as any); }}
              >
                <I.Paperclip /><br />Drop files here or click to browse
              </div>
            )}
          </div>

          {files.length > 0 && (
            <div style={{ minHeight: 200, height: 240, overflowY: "auto", marginBottom: 8 }}>
              {files.map(f => (
                <div key={f.id} style={{ height: 56, display: "flex", flexDirection: "column", justifyContent: "center", marginBottom: 4, borderBottom: "1px solid " + T.border }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12, lineHeight: "20px" }}>
                    <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: T.text }}>{f.filename}</span>
                    <div style={{ display: "flex", alignItems: "center", gap: 4, flexShrink: 0, width: 90, justifyContent: "flex-end" }}>
                      {f.status === "pending" && <span style={{ color: T.text3 }}>Pending</span>}
                      {(f.status === "processing" || f.status === "queued" || f.status === "embedding" || f.status === "staged") && <I.Spinner />}
                      {f.status === "success" && <span style={{ color: "rgba(34,197,94,0.8)" }}>✓ Done</span>}
                      {f.status === "replaced" && <span style={{ color: "rgba(34,197,94,0.8)" }}>↺ Replaced</span>}
                      {f.status === "duplicate" && <span style={{ color: "rgba(234,179,8,0.8)", fontSize: 10 }}>Duplicate</span>}
                      {(f.status === "error" || f.status === "failed") && <span style={{ color: "rgba(239,68,68,0.8)", fontSize: 10 }} title={f.message}>Error</span>}
                      {f.status === "pending" && <button onClick={() => removeFile(f.id)} style={{ background: "none", border: "none", cursor: "pointer", color: T.text3, padding: 2, display: "flex" }}><I.X /></button>}
                    </div>
                  </div>
                  <div style={{ height: 14, marginTop: 4, display: "flex", alignItems: "center", gap: 8, fontSize: 10, color: T.text3 }}>
                    {(f.status === "processing" || f.status === "embedding" || f.status === "queued") ? (
                      <>
                        <div style={{ flex: 1, height: 4, borderRadius: 2, background: T.border, overflow: "hidden" }}>
                          <div style={{ width: Math.max(2, Math.min(100, f.progressPct || 0)) + "%", height: "100%", borderRadius: 2, background: "rgba(99,102,241,0.7)", transition: "width 0.3s ease" }} />
                        </div>
                        <span style={{ flexShrink: 0, width: 140, textAlign: "right", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{f.progressMsg || ""}</span>
                      </>
                    ) : f.status === "success" || f.status === "replaced" ? (
                      <span style={{ color: "rgba(34,197,94,0.8)" }}>{f.progressMsg || "✓ Ingested successfully"}</span>
                    ) : f.status === "error" || f.status === "failed" ? (
                      <span style={{ color: "rgba(239,68,68,0.8)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{f.message || "Upload failed"}</span>
                    ) : (
                      <span>Ready to stage & embed</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}

          <div style={{ minHeight: 50, display: "flex", flexDirection: "column", justifyContent: "center", gap: 6 }}>
            {pendingCount > 0 && !busy && (
              <button onClick={uploadAll} style={btnStyle}>Stage & Embed {pendingCount} File{pendingCount > 1 ? "s" : ""}</button>
            )}
            {busy && mode === "upload" && (
              <div style={{ ...btnStyle, opacity: 0.7, cursor: "default" }}>
                <I.Spinner /> {phaseLabel || "Processing…"}
              </div>
            )}
            {hasCompleted && (
              <>
                {successCount > 0 && <div style={{ fontSize: 12, color: "rgba(34,197,94,0.8)", textAlign: "center" }}>✓ {successCount} succeeded{errorCount > 0 ? `, ${errorCount} failed` : ""}</div>}
                <button onClick={onClose} style={{ ...btnStyle, background: errorCount === 0 ? "rgba(34,197,94,0.8)" : "rgba(99,102,241,0.8)" }}>
                  {errorCount === 0 ? "✓ Done" : "Close"}
                </button>
              </>
            )}
          </div>
        </>
      )}

      {mode === "paste" && onIngestPaste && (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          <input value={pasteFilename} onChange={e => { if (pasteStatus.startsWith("✓")) setPasteStatus(""); setPasteFilename(e.target.value); }} placeholder="Filename (min 3 chars)" style={{ ...B, border: "1px solid " + T.border, background: T.bg2, color: T.text, opacity: pasting ? 0.5 : 1 }} disabled={pasting || isIngesting} />
          {pasteFilename.trim().length > 0 && pasteFilename.trim().length < 3 && !pasting && (
            <div style={{ fontSize: 10, color: "rgba(239,68,68,0.7)", marginTop: -6 }}>Filename must be at least 3 characters</div>
          )}
          <textarea value={pasteContent} onChange={e => { if (pasteStatus.startsWith("✓")) setPasteStatus(""); setPasteContent(e.target.value); }} placeholder="Paste document content..." style={{ ...B, minHeight: 100, resize: "vertical", fontFamily: "monospace", fontSize: 12 }} disabled={pasting || isIngesting} />

          {pasting && pasteLabel && (
            <div style={{ padding: "8px 12px", borderRadius: 6, background: "rgba(99,102,241,0.08)", display: "flex", alignItems: "center", gap: 8 }}>
              <div style={{ flex: 1, height: 4, borderRadius: 2, background: T.border, overflow: "hidden" }}>
                <div style={{ width: pastePct + "%", height: "100%", borderRadius: 2, background: "rgba(99,102,241,0.7)", transition: "width 0.3s ease" }} />
              </div>
              <span style={{ fontSize: 10, color: T.text3, whiteSpace: "nowrap" }}>{pasteLabel}</span>
            </div>
          )}

          {!pasting && !pasteStatus.startsWith("✓") && (
            <button onClick={handlePasteSubmit} disabled={!pasteContent.trim() || pasteFilename.trim().length < 3 || isIngesting} style={{ ...btnStyle, opacity: (!pasteContent.trim() || pasteFilename.trim().length < 3 || isIngesting) ? 0.5 : 1 }}>
              Ingest Text
            </button>
          )}
          {pasteStatus && !pasting && (
            <div style={{ fontSize: 12, padding: "8px", borderRadius: 6, background: pasteStatus.startsWith("✓") ? "rgba(34,197,94,0.1)" : "rgba(239,68,68,0.1)", color: pasteStatus.startsWith("✓") ? "rgba(34,197,94,0.9)" : "rgba(239,68,68,0.9)", textAlign: "center" }}>
              {pasteStatus}
              {pasteStatus.startsWith("✓") && <button onClick={onClose} style={{ marginLeft: 8, padding: "2px 10px", borderRadius: 4, border: "none", cursor: "pointer", fontSize: 11, color: "#fff", background: "rgba(34,197,94,0.8)" }}>Close</button>}
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}

const btnStyle: React.CSSProperties = { width: "100%", minHeight: 38, padding: "10px", borderRadius: 8, border: "none", cursor: "pointer", fontSize: 13, fontWeight: 500, color: "#fff", background: "rgba(99,102,241,0.8)", display: "flex", alignItems: "center", justifyContent: "center", gap: 8 };
