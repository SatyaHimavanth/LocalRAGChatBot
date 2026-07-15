import { memo, useMemo } from "react";
import { IncompleteJob, Theme, themeVars } from "../types";
import { I } from "./Icons";

interface IngestQueuePanelProps {
  jobs: IncompleteJob[];
  isIngesting: boolean;
  onResumeAll: () => void;
  onDiscardAll: () => void;
  onDiscardJob: (docId: number) => void;
  onRefresh: () => void;
  theme: Theme;
}

const QueueItem = memo(function QueueItem({
  job,
  theme,
  onDiscardJob,
}: {
  job: IncompleteJob;
  theme: Theme;
  onDiscardJob: (docId: number) => void;
}) {
  const T = themeVars[theme];
  const updated = job.updatedAt ? new Date(job.updatedAt * 1000).toLocaleString() : "—";
  const status = (job.status || "queued").toLowerCase();
  const bar = Math.max(0, Math.min(100, job.progressPct || 0));
  const isFailed = status === "failed" || !!job.errorMessage;
  const statusLabel = isFailed
    ? "Failed"
    : status === "embedding"
      ? "Embedding"
      : status === "queued"
        ? "Queued"
        : status === "staged"
          ? "Staged"
          : status === "ready"
            ? "Ready"
            : "Processing";

  return (
    <div style={{
      border: "1px solid " + T.border,
      borderRadius: 10,
      padding: 10,
      background: T.bg2,
      display: "flex",
      flexDirection: "column",
      gap: 8,
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 13, fontWeight: 600, color: T.text, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{job.filename}</div>
          <div style={{ fontSize: 11, color: T.text3, marginTop: 2 }}>
            Doc #{job.docId} · {statusLabel} · {job.chunkCount}/{job.expectedChunks || "?"} chunks
          </div>
        </div>
        <button
          onClick={() => onDiscardJob(job.docId)}
          style={{ background: "none", border: "none", cursor: "pointer", color: "rgba(239,68,68,0.75)", padding: 4, flexShrink: 0 }}
          title="Discard this document"
          aria-label={`Discard ${job.filename}`}
        >
          <I.Trash />
        </button>
      </div>
      <div style={{ height: 6, borderRadius: 999, background: T.border, overflow: "hidden" }}>
        <div style={{ width: `${bar}%`, height: "100%", borderRadius: 999, background: isFailed ? "rgba(239,68,68,0.7)" : "rgba(99,102,241,0.75)", transition: "width 0.2s ease" }} />
      </div>
      <div style={{ display: "flex", justifyContent: "space-between", gap: 8, fontSize: 11, color: T.text3 }}>
        <span>{bar}% complete</span>
        <span>{updated}</span>
      </div>
      {job.errorMessage ? <div style={{ fontSize: 11, color: "rgba(239,68,68,0.85)", lineHeight: 1.5 }}>{job.errorMessage}</div> : null}
    </div>
  );
}, (a, b) => a.job === b.job && a.theme === b.theme && a.onDiscardJob === b.onDiscardJob);

export function IngestQueuePanel({
  jobs,
  isIngesting,
  onResumeAll,
  onDiscardAll,
  onDiscardJob,
  onRefresh,
  theme,
}: IngestQueuePanelProps) {
  const T = themeVars[theme];

  const stats = useMemo(() => {
    const total = jobs.length;
    const failed = jobs.filter(j => (j.status || "").toLowerCase() === "failed" || !!j.errorMessage).length;
    const embedding = jobs.filter(j => (j.status || "").toLowerCase() === "embedding").length;
    const queued = jobs.filter(j => (j.status || "").toLowerCase() === "queued" || (j.status || "").toLowerCase() === "staged").length;
    const avg = total > 0 ? Math.round(jobs.reduce((sum, j) => sum + Math.max(0, Math.min(100, j.progressPct || 0)), 0) / total) : 0;
    return { total, failed, embedding, queued, avg };
  }, [jobs]);

  if (jobs.length === 0) {
    return (
      <div style={{ marginBottom: 18, padding: 14, borderRadius: 12, border: "1px solid " + T.border, background: T.bg2 }}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "center", marginBottom: 8 }}>
          <div>
            <div style={{ fontSize: 13, fontWeight: 700, color: T.text }}>Ingest Queue</div>
            <div style={{ fontSize: 11, color: T.text3, marginTop: 2 }}>No incomplete documents right now.</div>
          </div>
          <button onClick={onRefresh} style={{ background: "none", border: "none", cursor: "pointer", color: T.text3, padding: 4 }} title="Refresh queue"><I.Refresh /></button>
        </div>
        <div style={{ fontSize: 12, color: T.text3 }}>Uploaded documents that are still processing will appear here with resume and discard controls.</div>
      </div>
    );
  }

  return (
    <div style={{ marginBottom: 18, padding: 14, borderRadius: 12, border: "1px solid " + T.border, background: T.bg2 }}>
      <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "flex-start", marginBottom: 10 }}>
        <div>
          <div style={{ fontSize: 13, fontWeight: 700, color: T.text }}>Ingest Queue</div>
          <div style={{ fontSize: 11, color: T.text3, marginTop: 2 }}>
            {stats.total} incomplete document{stats.total === 1 ? "" : "s"} · {stats.embedding} embedding · {stats.queued} queued · {stats.failed} failed · avg {stats.avg}%
          </div>
        </div>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", justifyContent: "flex-end" }}>
          <button onClick={onRefresh} style={{ padding: "7px 10px", borderRadius: 8, border: "1px solid " + T.border, cursor: "pointer", fontSize: 12, color: T.text2, background: T.inputBg, display: "flex", alignItems: "center", gap: 6 }}><I.Refresh />Refresh</button>
          <button onClick={onResumeAll} disabled={isIngesting} style={{ padding: "7px 12px", borderRadius: 8, border: "none", cursor: isIngesting ? "not-allowed" : "pointer", fontSize: 12, fontWeight: 600, color: "#fff", background: isIngesting ? "rgba(99,102,241,0.45)" : "rgba(99,102,241,0.82)", display: "flex", alignItems: "center", gap: 6 }}>
            {isIngesting ? <I.Spinner /> : <I.Check />}
            Resume all
          </button>
          <button onClick={onDiscardAll} style={{ padding: "7px 12px", borderRadius: 8, border: "1px solid rgba(239,68,68,0.35)", cursor: "pointer", fontSize: 12, color: "rgba(239,68,68,0.9)", background: "transparent", display: "flex", alignItems: "center", gap: 6 }}>
            <I.Trash />Discard all
          </button>
        </div>
      </div>
      <div style={{ height: 7, borderRadius: 999, background: T.border, overflow: "hidden", marginBottom: 12 }}>
        <div style={{ width: `${stats.avg}%`, height: "100%", borderRadius: 999, background: "rgba(99,102,241,0.75)" }} />
      </div>
      <div style={{ display: "grid", gap: 10 }}>
        {jobs.map(job => <QueueItem key={job.docId} job={job} theme={theme} onDiscardJob={onDiscardJob} />)}
      </div>
    </div>
  );
}
