import type { CSSProperties, ReactNode } from "react";
import { Modal } from "./Modal";
import { themeVars, Theme, ThemeVars, DiagnosticsSnapshot } from "../types";

interface DiagnosticsModalProps {
  open: boolean;
  theme: Theme;
  loading?: boolean;
  data?: DiagnosticsSnapshot | null;
  onClose: () => void;
  onRefresh: () => void;
}

export function DiagnosticsModal({ open, theme, loading, data, onClose, onRefresh }: DiagnosticsModalProps) {
  const T = themeVars[theme];
  const snap = data || null;
  return (
    <Modal open={open} onClose={onClose} title="Diagnostics" theme={theme} wide>
      <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, flexWrap: "wrap" }}>
          <div style={{ fontSize: 12, color: T.text3, lineHeight: 1.5 }}>
            Runtime and database health snapshot for the current LocalRAG instance.
          </div>
          <button onClick={onRefresh} style={secondaryBtnStyle(T)}>{loading ? "Refreshing…" : "Refresh"}</button>
        </div>

        {loading && !snap ? (
          <div style={{ padding: "28px 12px", textAlign: "center", color: T.text3, fontSize: 13 }}>Loading diagnostics…</div>
        ) : snap ? (
          <>
            <div style={gridStyle}>
              {metricCard("DB", snap.dbReady ? "Connected" : "Offline", T, snap.dbReady ? "rgba(34,197,94,0.12)" : "rgba(239,68,68,0.12)")}
              {metricCard("Go", snap.goVersion, T)}
              {metricCard("Platform", `${snap.goos}/${snap.goarch}`, T)}
              {metricCard("CPU", `${snap.numCpu}`, T)}
              {metricCard("RAM", `${snap.totalRamGb || 0} GB`, T)}
              {metricCard("Context", `${snap.recommendedContextSize || 0} tokens`, T)}
              {metricCard("Memory", `${snap.memoryAllocMb} MB alloc`, T)}
              {metricCard("Ingest", snap.ingestActive ? "Running" : "Idle", T, snap.ingestActive ? "rgba(245,158,11,0.12)" : "rgba(148,163,184,0.12)")}
            </div>

            <Section title="Content" T={T}>
              <div style={statRowStyle}>
                {statItem("Collections", snap.collections)}
                {statItem("Chats", snap.chats)}
                {statItem("Messages", snap.messages)}
                {statItem("Documents", snap.documents)}
                {statItem("Ready docs", snap.readyDocuments)}
                {statItem("Incomplete", snap.incompleteDocuments)}
                {statItem("Chunks", snap.chunks)}
                {statItem("Sources", snap.messageSources)}
              </div>
            </Section>

            <Section title="SQLite" T={T}>
              <div style={{ fontSize: 12, color: T.text2, lineHeight: 1.65, display: "grid", gap: 6 }}>
                <div>Page count: <strong style={{ color: T.text }}>{formatNumber(snap.dbPageCount)}</strong></div>
                <div>Page size: <strong style={{ color: T.text }}>{formatBytes(snap.dbPageSize)}</strong></div>
                <div>Approx DB size: <strong style={{ color: T.text }}>{formatBytes(snap.dbApproxBytes)}</strong></div>
                <div>Active generations: <strong style={{ color: T.text }}>{snap.activeGenerations}</strong></div>
                <div>Collected at: <strong style={{ color: T.text }}>{formatTime(snap.collectedAtUnix)}</strong></div>
              </div>
            </Section>
          </>
        ) : (
          <div style={{ padding: "28px 12px", textAlign: "center", color: T.text3, fontSize: 13 }}>No diagnostics available.</div>
        )}
      </div>
    </Modal>
  );
}

function Section({ title, children, T }: { title: string; children: ReactNode; T: ThemeVars }) {
  return (
    <div style={{ border: "1px solid " + T.border, borderRadius: 12, padding: 14, background: T.inputBg }}>
      <div style={{ fontSize: 12, fontWeight: 600, color: T.text, marginBottom: 10 }}>{title}</div>
      {children}
    </div>
  );
}

function metricCard(label: string, value: string, T: ThemeVars, bg?: string) {
  return (
    <div style={{ ...cardStyle(T, bg), minHeight: 72 }}>
      <div style={{ fontSize: 11, color: T.text3, marginBottom: 6 }}>{label}</div>
      <div style={{ fontSize: 14, fontWeight: 600, color: T.text, wordBreak: "break-word" }}>{value}</div>
    </div>
  );
}

function statItem(label: string, value: number) {
  return (
    <div style={{ minWidth: 110, flex: "1 1 110px", padding: "8px 10px", borderRadius: 8, border: "1px solid rgba(128,128,128,0.12)" }}>
      <div style={{ fontSize: 11, color: "rgba(128,128,128,0.7)", marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 14, fontWeight: 600 }}>{formatNumber(value)}</div>
    </div>
  );
}

function formatNumber(value: number): string {
  if (!Number.isFinite(value)) return "0";
  return new Intl.NumberFormat().format(Math.max(0, Math.floor(value)));
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let idx = 0;
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024;
    idx++;
  }
  return `${size.toFixed(size >= 10 || idx === 0 ? 0 : 1)} ${units[idx]}`;
}

function formatTime(unixSeconds: number): string {
  if (!unixSeconds) return "—";
  try {
    return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(unixSeconds * 1000));
  } catch {
    return String(unixSeconds);
  }
}

function cardStyle(T: ThemeVars, bg?: string): CSSProperties {
  return {
    border: "1px solid " + T.border,
    background: bg || T.bg2,
    borderRadius: 12,
    padding: 14,
    transition: "background 0.2s, border 0.2s",
  };
}

const gridStyle: CSSProperties = {
  display: "grid",
  gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))",
  gap: 10,
};

const statRowStyle: CSSProperties = {
  display: "flex",
  flexWrap: "wrap",
  gap: 8,
};

function secondaryBtnStyle(T: ThemeVars): CSSProperties {
  return {
    padding: "8px 14px",
    borderRadius: 8,
    border: "1px solid " + T.border,
    cursor: "pointer",
    fontSize: 13,
    color: T.text2,
    background: "transparent",
  };
}
