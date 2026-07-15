import { useMemo } from "react";
import type { CSSProperties } from "react";
import { Chat, Collection, DocRecord, EventLogEntry, IngestLogEntry, IncompleteJob, SearchScope, ThemeVars } from "../types";
import { I } from "./Icons";

interface DiagnosticsPanelProps {
  chats: Chat[];
  activeChat: Chat | undefined;
  cols: Collection[];
  activeCollection: Collection | undefined;
  idocs: DocRecord[];
  activeChatId: number;
  activeCollectionId: number;
  searchScope: SearchScope;
  ingestLogs: IngestLogEntry[];
  eventLogs: EventLogEntry[];
  incompleteJobs: IncompleteJob[];
  isIngesting: boolean;
  T: ThemeVars;
  onOpenChat: () => void;
  onOpenSearch: () => void;
  onOpenCollections: () => void;
  onOpenUpload: () => void;
  onRefresh: () => void;
  onResumeQueue: () => void;
  onDiscardQueue: () => void;
}

const pillStyle = (bg: string, fg: string, border: string): CSSProperties => ({
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

const cardStyle = (T: ThemeVars): CSSProperties => ({
  padding: 14,
  borderRadius: 12,
  border: "1px solid " + T.border,
  background: T.bg2,
  boxShadow: "0 8px 24px rgba(0,0,0,0.08)",
});

const metricStyle = (T: ThemeVars): CSSProperties => ({
  padding: "14px 16px",
  borderRadius: 12,
  background: T.inputBg,
  border: "1px solid " + T.border,
});

const formatPct = (value: number) => `${Math.max(0, Math.min(100, Math.round(value)))}%`;

export function DiagnosticsPanel(props: DiagnosticsPanelProps) {
  const T = props.T;

  const totals = useMemo(() => {
    const totalDocs = props.idocs.length;
    const totalChunks = props.idocs.reduce((sum, doc) => sum + (doc.chunkCount || 0), 0);
    const readyDocs = props.idocs.filter((doc) => (doc.status || "ready") === "ready").length;
    const incompleteDocs = props.idocs.filter((doc) => (doc.status || "ready") !== "ready").length;
    const queueRunning = props.incompleteJobs.filter((job) => ["running", "queued", "retrying", "staging", "embedding", "paused"].includes(String(job.status || "").toLowerCase())).length;
    const avgQueueProgress = props.incompleteJobs.length
      ? props.incompleteJobs.reduce((sum, job) => sum + (job.progressPct || 0), 0) / props.incompleteJobs.length
      : 0;

    let confidenceSum = 0;
    let confidenceCount = 0;
    let verifiedCount = 0;
    let retrievalCount = 0;
    let memoryCount = 0;
    let directCount = 0;
    let aiMessages = 0;
    let userMessages = 0;

    for (const chat of props.chats) {
      for (const msg of chat.messages || []) {
        if (msg.sender === "user") userMessages += 1;
        if (msg.sender !== "ai") continue;
        aiMessages += 1;
        const meta = msg.metadata;
        if (meta?.usedRetrieval) retrievalCount += 1;
        if (meta?.usedMemory) memoryCount += 1;
        if (meta?.usedDirect) directCount += 1;
        if (typeof meta?.confidence === "number" && meta.confidence > 0) {
          confidenceSum += meta.confidence;
          confidenceCount += 1;
          if (meta.verified) verifiedCount += 1;
        }
      }
    }

    const avgConfidence = confidenceCount > 0 ? confidenceSum / confidenceCount : 0;
    const verifiedRate = confidenceCount > 0 ? verifiedCount / confidenceCount : 0;

    const statusCounts = props.incompleteJobs.reduce<Record<string, number>>((acc, job) => {
      const key = String(job.status || "unknown").toLowerCase();
      acc[key] = (acc[key] || 0) + 1;
      return acc;
    }, {});

    return {
      totalDocs,
      totalChunks,
      readyDocs,
      incompleteDocs,
      queueRunning,
      avgQueueProgress,
      avgConfidence,
      verifiedRate,
      confidenceCount,
      aiMessages,
      userMessages,
      retrievalCount,
      memoryCount,
      directCount,
      statusCounts,
    };
  }, [props.chats, props.idocs, props.incompleteJobs]);

  const recentLogs = props.ingestLogs.slice(0, 6);
  const recentEvents = props.eventLogs.slice(0, 6);
  const recentJobs = props.incompleteJobs.slice(0, 5);
  const lastLog = props.ingestLogs[0];
  const collectionName = props.activeCollection?.name || "No collection";
  const collectionProfile = props.activeCollection
    ? `${props.activeCollection.embeddingModel || "unset"} · ${props.activeCollection.embeddingDims || 0} dims · ${props.activeCollection.vectorBackend || "sqlite-vec"}`
    : "No active collection profile";

  return (
    <div style={{ flex: 1, display: "flex", flexDirection: "column", padding: 20, overflow: "hidden", minWidth: 0 }}>
      <div style={{ display: "flex", justifyContent: "space-between", gap: 12, alignItems: "center", marginBottom: 16, flexWrap: "wrap" }}>
        <div>
          <div style={{ fontSize: 18, fontWeight: 700, display: "flex", alignItems: "center", gap: 8 }}>
            <I.Warning /> Diagnostics
          </div>
          <div style={{ fontSize: 12, color: T.text3, marginTop: 4 }}>Live workspace snapshot, queue health, and answer-quality signals.</div>
        </div>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
          <button onClick={props.onOpenChat} style={actionBtn(T, false)}>Chat</button>
          <button onClick={props.onOpenSearch} style={actionBtn(T, false)}>Search</button>
          <button onClick={props.onOpenCollections} style={actionBtn(T, false)}>Collections</button>
          <button onClick={props.onOpenUpload} style={actionBtn(T, true)}>Upload</button>
          <button onClick={props.onRefresh} style={actionBtn(T, false)}><I.Refresh /> Refresh</button>
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(210px, 1fr))", gap: 12, marginBottom: 14 }}>
        <Metric title="Collections" value={String(props.cols.length)} detail={props.cols.length > 0 ? `${props.cols.reduce((sum, c) => sum + (c.docCount || 0), 0)} docs total` : "No collections yet"} T={T} />
        <Metric title="Active collection" value={collectionName} detail={collectionProfile} T={T} />
        <Metric title="Documents / chunks" value={`${totals.totalDocs} / ${totals.totalChunks}`} detail={`${totals.readyDocs} ready · ${totals.incompleteDocs} incomplete`} T={T} />
        <Metric title="Ingest queue" value={`${props.incompleteJobs.length}`} detail={`${totals.queueRunning} active · ${formatPct(totals.avgQueueProgress)} avg progress`} T={T} />
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1.35fr 1fr", gap: 14, minHeight: 0, flex: 1 }}>
        <div style={{ display: "flex", flexDirection: "column", gap: 14, minHeight: 0 }}>
          <section style={cardStyle(T)}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, marginBottom: 12 }}>
              <div style={{ fontSize: 14, fontWeight: 600 }}>Quality signals</div>
              <span style={pillStyle("rgba(99,102,241,0.14)", T.text2, T.border)}>Scope: {props.searchScope}</span>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))", gap: 10 }}>
              <Metric title="Assistant turns" value={String(totals.aiMessages)} detail={`${totals.userMessages} user turns`} T={T} />
              <Metric title="Retrieval turns" value={String(totals.retrievalCount)} detail={`${totals.memoryCount} memory · ${totals.directCount} direct`} T={T} />
              <Metric title="Evidence confidence" value={totals.confidenceCount > 0 ? formatPct(totals.avgConfidence * 100) : "n/a"} detail={`${formatPct(totals.verifiedRate * 100)} verified`} T={T} />
            </div>
          </section>

          <section style={cardStyle(T)}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, marginBottom: 10 }}>
              <div style={{ fontSize: 14, fontWeight: 600 }}>Queue progress</div>
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                <span style={pillStyle("rgba(99,102,241,0.14)", T.text2, T.border)}>{props.isIngesting ? "Active ingest" : "Idle"}</span>
                <span style={pillStyle("rgba(34,197,94,0.12)", T.text2, T.border)}>Ready docs: {totals.readyDocs}</span>
                <span style={pillStyle("rgba(245,158,11,0.12)", T.text2, T.border)}>Pending: {props.incompleteJobs.length}</span>
              </div>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 10, maxHeight: 280, overflowY: "auto" }}>
              {recentJobs.length === 0 ? (
                <div style={{ fontSize: 12, color: T.text3 }}>No incomplete jobs are currently waiting.</div>
              ) : recentJobs.map((job) => (
                <div key={`${job.docId}-${job.batchId}`} style={{ padding: 12, borderRadius: 10, border: "1px solid " + T.border, background: T.inputBg }}>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 8, alignItems: "center", marginBottom: 6, flexWrap: "wrap" }}>
                    <div style={{ fontSize: 13, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{job.filename}</div>
                    <span style={pillStyle(statusBg(job.status), statusFg(job.status), T.border)}>{job.status}</span>
                  </div>
                  <div style={{ fontSize: 11, color: T.text3, marginBottom: 8 }}>{job.chunkCount}/{job.expectedChunks} chunks · batch {job.batchId || "n/a"}</div>
                  <div style={{ height: 6, borderRadius: 999, background: "rgba(128,128,128,0.12)", overflow: "hidden" }}>
                    <div style={{ width: `${Math.max(0, Math.min(100, job.progressPct || 0))}%`, height: "100%", borderRadius: 999, background: "rgba(99,102,241,0.7)" }} />
                  </div>
                  <div style={{ marginTop: 6, display: "flex", justifyContent: "space-between", gap: 8, fontSize: 10, color: T.text3, flexWrap: "wrap" }}>
                    <span>updated {new Date((job.updatedAt || 0) * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</span>
                    <span>{formatPct(job.progressPct || 0)}</span>
                  </div>
                </div>
              ))}
            </div>
            <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 10, flexWrap: "wrap" }}>
              <button onClick={props.onResumeQueue} style={actionBtn(T, false)}>Resume queue</button>
              <button onClick={props.onDiscardQueue} style={actionBtn(T, false)}>Discard incomplete</button>
            </div>
          </section>
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: 14, minHeight: 0 }}>
          <section style={cardStyle(T)}>
            <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 10 }}>Recent ingest logs</div>
            <div style={{ display: "flex", flexDirection: "column", gap: 8, maxHeight: 290, overflowY: "auto" }}>
              {recentLogs.length === 0 ? (
                <div style={{ fontSize: 12, color: T.text3 }}>No ingest logs captured yet.</div>
              ) : recentLogs.map((log) => (
                <div key={log.id} style={{ padding: 10, borderRadius: 10, border: "1px solid " + T.border, background: T.inputBg }}>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 8, marginBottom: 4, flexWrap: "wrap" }}>
                    <span style={pillStyle(logBg(log.level), logFg(log.level), T.border)}>{log.level}</span>
                    <span style={{ fontSize: 10, color: T.text3 }}>{new Date(log.timestamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</span>
                  </div>
                  <div style={{ fontSize: 12, color: T.text, lineHeight: 1.5 }}>{log.message}</div>
                  <div style={{ fontSize: 10, color: T.text3, marginTop: 4 }}>{log.stage}{log.filename ? ` · ${log.filename}` : ""}{log.batchId ? ` · ${log.batchId.slice(0, 8)}` : ""}</div>
                </div>
              ))}
            </div>
          </section>

          <section style={cardStyle(T)}>
            <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 10 }}>Recent workspace events</div>
            <div style={{ display: "flex", flexDirection: "column", gap: 8, maxHeight: 260, overflowY: "auto" }}>
              {recentEvents.length === 0 ? (
                <div style={{ fontSize: 12, color: T.text3 }}>No workspace events captured yet.</div>
              ) : recentEvents.map((ev) => (
                <div key={ev.id} style={{ padding: 10, borderRadius: 10, border: "1px solid " + T.border, background: T.inputBg }}>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 8, marginBottom: 4, flexWrap: "wrap" }}>
                    <span style={pillStyle(logBg(ev.severity), logFg(ev.severity), T.border)}>{ev.severity}</span>
                    <span style={{ fontSize: 10, color: T.text3 }}>{new Date(ev.createdAt * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</span>
                  </div>
                  <div style={{ fontSize: 12, color: T.text, lineHeight: 1.5 }}>{ev.title}</div>
                  <div style={{ fontSize: 10, color: T.text3, marginTop: 4 }}>{ev.eventKey}{ev.scope ? ` · ${ev.scope}` : ""}{ev.detail ? ` · ${ev.detail}` : ""}</div>
                </div>
              ))}
            </div>
          </section>

          <section style={cardStyle(T)}>
            <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 10 }}>Workspace snapshot</div>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 12 }}>
              <span style={pillStyle("rgba(99,102,241,0.14)", T.text2, T.border)}>Tab-ready</span>
              <span style={pillStyle("rgba(168,85,247,0.14)", T.text2, T.border)}>Active chat #{props.activeChatId || 0}</span>
              <span style={pillStyle("rgba(34,197,94,0.12)", T.text2, T.border)}>{props.isIngesting ? "Ingest running" : "No active ingest"}</span>
            </div>
            <div style={{ fontSize: 12, color: T.text3, lineHeight: 1.6 }}>
              <div>Last event: {lastLog ? `${lastLog.stage} · ${lastLog.message}` : "none"}</div>
              <div style={{ marginTop: 6 }}>Status counts: {Object.keys(totals.statusCounts).length > 0 ? Object.entries(totals.statusCounts).map(([k,v]) => `${k}=${v}`).join(" · ") : "none"}</div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

function Metric({ title, value, detail, T }: { title: string; value: string; detail: string; T: ThemeVars }) {
  return (
    <div style={metricStyle(T)}>
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
    fontWeight: 600,
    display: "inline-flex",
    alignItems: "center",
    gap: 6,
  };
}

function logBg(level: string) {
  if (level === "error") return "rgba(239,68,68,0.14)";
  if (level === "warn") return "rgba(245,158,11,0.14)";
  return "rgba(99,102,241,0.14)";
}

function logFg(level: string) {
  if (level === "error") return "rgba(239,68,68,0.9)";
  if (level === "warn") return "rgba(245,158,11,0.95)";
  return "rgba(99,102,241,0.9)";
}

function statusBg(status: string) {
  const s = String(status || "").toLowerCase();
  if (s === "running" || s === "embedding") return "rgba(99,102,241,0.14)";
  if (s === "paused") return "rgba(245,158,11,0.14)";
  if (s === "retrying") return "rgba(168,85,247,0.14)";
  if (s === "cancelled") return "rgba(239,68,68,0.14)";
  return "rgba(34,197,94,0.12)";
}

function statusFg(status: string) {
  const s = String(status || "").toLowerCase();
  if (s === "running" || s === "embedding") return "rgba(99,102,241,0.9)";
  if (s === "paused") return "rgba(245,158,11,0.95)";
  if (s === "retrying") return "rgba(168,85,247,0.95)";
  if (s === "cancelled") return "rgba(239,68,68,0.9)";
  return "rgba(34,197,94,0.9)";
}
