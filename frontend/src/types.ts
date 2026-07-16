export interface Message { id: string; sender: "user" | "ai" | "system"; text: string; cancelled?: boolean; parentMessageId?: number; metadata?: AgentResult; }
export interface Chat { id: number; title: string; messages: Message[]; createdAt: number; archived: boolean; pinned: boolean; currentLeafMessageId?: number; messageSources?: Record<number, SourceRef[]>; branchOptions?: Record<number, number[]>; agentPlan?: AgentPlan; lastAgentResult?: AgentResult; }
export interface SourceRef { id?: number; refNumber: number; chunkId: number; content: string; filename: string; collectionId: number; collectionName: string; similarity: number; }
export interface Collection { id: number; name: string; docCount: number; embeddingModel?: string; embeddingDims?: number; vectorBackend?: string; createdAt?: number; updatedAt?: number; }
export interface DocRecord {
  id: number;
  collectionId: number;
  filename: string;
  hash: string;
  content: string;
  summary?: string;
  sourceType?: string;
  sourceSizeBytes?: number;
  wordCount?: number;
  lineCount?: number;
  characterCount?: number;
  paragraphCount?: number;
  title?: string;
  createdAt: number;
  chunkCount: number;
  status?: string;
  expectedChunks?: number;
  batchId?: string;
  errorMessage?: string;
  updatedAt?: number;
}
export type SearchScope = "collection" | "all" | "workspace" | "metadata";
export interface SearchResult { content: string; score: number; searchType: string; collectionId: number; collectionName: string; filename: string; chunkId: number; }
export interface ChunkRecord {
  id: number;
  documentId: number;
  collectionId: number;
  content: string;
  summary: string;
  ord: number;
  level: number;
  role: string;
  parentOrd: number;
  prevOrd: number;
  nextOrd: number;
  chunkHash: string;
  embeddingHash: string;
  headingPath: string;
  updatedAt: number;
}
export interface ExtensionHook {
  id: number;
  hookKey: string;
  name: string;
  hookType: string;
  surface: string;
  description: string;
  state: string;
  enabled: boolean;
  configJson: string;
  lastRunAt: number;
  createdAt: number;
  updatedAt: number;
}

export interface EventLogEntry {
  id: number;
  eventKey: string;
  title: string;
  detail: string;
  severity: string;
  scope: string;
  collectionId: number;
  chatId: number;
  docId: number;
  batchId: string;
  createdAt: number;
}

export interface IngestProgress { step: string; label: string; pct: number; detail: string; phase?: string; }
export interface ToastMsg { id: string; type: "success"|"error"|"info"; message: string; }

export type AgentIntent = "greeting" | "general" | "conversation" | "retrieval" | "follow_up" | "comparison" | "summarization" | "tool_call" | "unknown";
export type AgentPhase = "thinking" | "planning" | "memory" | "retrieval" | "tool" | "generation" | "done";

export interface AgentPlan {
  intent: AgentIntent;
  useRetrieval: boolean;
  useMemory: boolean;
  useDirect: boolean;
  topK: number;
  retrievalQuery?: string;
  reason?: string;
}

export interface AgentResult {
  cancelled: boolean;
  usedRetrieval: boolean;
  usedMemory: boolean;
  usedDirect: boolean;
  sourceCount: number;
  evidenceCount?: number;
  confidence?: number;
  verified?: boolean;
  verification?: string;
  evidenceGaps?: string[];
  reason?: string;
  retrievalQuery?: string;
  topK?: number;
}

export interface AgentStatus {
  phase: AgentPhase;
  label: string;
  detail?: string;
}
export interface FileUploadItem {
  id: string;
  file?: File;
  filename: string;
  status: "pending"|"processing"|"duplicate"|"replaced"|"success"|"error"|"queued"|"embedding"|"failed"|"staged";
  message?: string;
  docId?: number;
  progressMsg?: string;
  progressPct?: number;
  replace?: boolean;
  duplicateStatus?: "checking" | "clear" | "duplicate";
  duplicateMessage?: string;
  existingDocId?: number;
}
export interface IncompleteJob {
  docId: number;
  collectionId: number;
  filename: string;
  status: string;
  chunkCount: number;
  expectedChunks: number;
  batchId: string;
  errorMessage: string;
  progressPct: number;
  updatedAt: number;
  createdAt: number;
}

export interface IngestLogEntry {
  id: string;
  timestamp: number;
  level: "info" | "warn" | "error";
  stage: string;
  message: string;
  filename?: string;
  collectionId?: number;
  batchId?: string;
}

export type Theme = "dark" | "light";
export interface ThemeVars {
  bg: string; bg2: string; text: string; text2: string; text3: string;
  border: string; inputBg: string; bubbleUser: string; bubbleAI: string;
}

export const themeVars: Record<Theme, ThemeVars> = {
  dark: { bg: "#06070f", bg2: "#1a1b2e", text: "rgba(255,255,255,0.85)", text2: "rgba(255,255,255,0.75)", text3: "rgba(255,255,255,0.4)", border: "rgba(255,255,255,0.08)", inputBg: "rgba(255,255,255,0.06)", bubbleUser: "rgba(99,102,241,0.2)", bubbleAI: "rgba(255,255,255,0.06)" },
  light: { bg: "#f5f5f7", bg2: "#ffffff", text: "rgba(0,0,0,0.85)", text2: "rgba(0,0,0,0.7)", text3: "rgba(0,0,0,0.4)", border: "rgba(0,0,0,0.08)", inputBg: "#ffffff", bubbleUser: "rgba(99,102,241,0.12)", bubbleAI: "rgba(0,0,0,0.04)" },
};

export const getErrMsg = (e: any): string => {
  if (!e) return "Unknown error";
  if (typeof e === "string") return e;
  if (e?.message) {
    const msg = e.message;
    if (typeof msg === "string") {
      try { const parsed = JSON.parse(msg); if (parsed?.message) return parsed.message; } catch {}
      return msg;
    }
    return String(msg);
  }
  return String(e);
};
