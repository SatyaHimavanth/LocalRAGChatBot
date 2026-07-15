export interface Message { id: string; sender: "user" | "ai" | "system"; text: string; cancelled?: boolean; parentMessageId?: number; metadata?: AgentResult; }
export interface Chat { id: number; title: string; messages: Message[]; createdAt: number; archived: boolean; pinned: boolean; currentLeafMessageId?: number; messageSources?: Record<number, SourceRef[]>; agentPlan?: AgentPlan; lastAgentResult?: AgentResult; }
export interface SourceRef { id?: number; refNumber: number; chunkId: number; content: string; filename: string; collectionId: number; collectionName: string; similarity: number; }
export interface Collection { id: number; name: string; docCount: number; }
export interface CollectionInsight {
  id: number;
  name: string;
  createdAt: number;
  totalDocumentCount: number;
  readyDocumentCount: number;
  incompleteDocumentCount: number;
  chunkCount: number;
  chatCount: number;
  latestDocumentUpdatedAt: number;
}
export interface DocRecord {
  id: number;
  collectionId: number;
  filename: string;
  hash: string;
  content: string;
  createdAt: number;
  chunkCount: number;
  status?: string;
  expectedChunks?: number;
  batchId?: string;
  errorMessage?: string;
  updatedAt?: number;
}
export interface SearchResult { content: string; score: number; searchType: string; collectionId: number; collectionName: string; filename: string; title?: string; sectionPath?: string; chunkSummary?: string; chunkId: number; }
export interface IngestProgress { step: string; label: string; pct: number; detail: string; phase?: string; }
export interface ToastMsg { id: string; type: "success"|"error"|"info"; message: string; }

export type AgentIntent = "greeting" | "general" | "conversation" | "retrieval" | "follow_up" | "comparison" | "summarization" | "tool_call" | "unknown";
export type AgentPhase = "thinking" | "planning" | "memory" | "retrieval" | "tool" | "generation" | "done";
export type EvidenceEffort = "low" | "medium" | "high";
export type RetrievalScope = "current" | "all";

export interface AgentPlan {
  intent: AgentIntent;
  useRetrieval: boolean;
  useMemory: boolean;
  useWorkspaceMemory: boolean;
  useDirect: boolean;
  topK: number;
  evidenceEffort: EvidenceEffort;
  evidenceCandidateLimit?: number;
  evidencePasses?: number;
  evidenceTokenBudget?: number;
  evidenceCoverageTarget?: number;
  evidenceTimeBudgetMs?: number;
  retrievalQuery?: string;
  reason?: string;
}

export interface AgentResult {
  cancelled: boolean;
  usedRetrieval: boolean;
  usedMemory: boolean;
  usedWorkspaceMemory: boolean;
  usedDirect: boolean;
  sourceCount: number;
  evidenceEffort: EvidenceEffort;
  evidenceCoverage?: number;
  evidencePasses?: number;
  evidenceCandidates?: number;
  evidenceExpanded?: number;
  evidenceCompressed?: number;
  evidenceTokens?: number;
  evidenceBudgetTokens?: number;
  evidenceTimeBudgetMs?: number;
  evidenceSummary?: string;
  verificationScore?: number;
  verificationVerdict?: string;
  verificationSummary?: string;
  verificationIssues?: string[];
  reason?: string;
  retrievalQuery?: string;
  topK?: number;
}

export interface WorkspaceMemorySnapshot {
  sessionId: number;
  collectionId: number;
  collectionName: string;
  summary: string;
  notes: string;
  lastMessageId: number;
  updatedAt: number;
  recentQuestions: string[];
  recentDocuments: string[];
  latestAssistant?: string;
  latestSignal?: string;
  hasSummary?: boolean;
  hasNotes?: boolean;
}

export interface DiagnosticsSnapshot {
  dbReady: boolean;
  goVersion: string;
  goos: string;
  goarch: string;
  numCpu: number;
  totalRamGb: number;
  recommendedContextSize: number;
  memoryAllocMb: number;
  memorySysMb: number;
  activeGenerations: number;
  ingestActive: boolean;
  collections: number;
  chats: number;
  messages: number;
  documents: number;
  readyDocuments: number;
  incompleteDocuments: number;
  chunks: number;
  messageSources: number;
  dbPageSize: number;
  dbPageCount: number;
  dbApproxBytes: number;
  collectedAtUnix: number;
}

export interface AgentStatus {
  phase: AgentPhase;
  label: string;
  detail?: string;
  effort?: EvidenceEffort;
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
