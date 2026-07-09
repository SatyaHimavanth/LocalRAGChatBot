export interface Message { id: string; sender: "user" | "ai" | "system"; text: string; }
export interface Chat { id: number; title: string; messages: Message[]; createdAt: number; archived: boolean; pinned: boolean; messageSources?: Record<number, SourceRef[]>; }
export interface SourceRef { id?: number; refNumber: number; chunkId: number; content: string; filename: string; collectionId: number; collectionName: string; similarity: number; }
export interface Collection { id: number; name: string; docCount: number; }
export interface DocRecord { id: number; collectionId: number; filename: string; hash: string; content: string; createdAt: number; chunkCount: number; }
export interface SearchResult { content: string; score: number; searchType: string; collectionId: number; collectionName: string; filename: string; chunkId: number; }
export interface IngestProgress { step: string; label: string; pct: number; detail: string; }
export interface ToastMsg { id: string; type: "success"|"error"|"info"; message: string; }
export interface FileUploadItem { id: string; file: File; status: "pending"|"processing"|"duplicate"|"replaced"|"success"|"error"; message?: string; docId?: number; progressMsg?: string; progressPct?: number; }

export type Theme = "dark" | "light";
export interface ThemeVars {
  bg: string; bg2: string; text: string; text2: string; text3: string;
  border: string; inputBg: string; bubbleUser: string; bubbleAI: string;
}

export const themeVars: Record<Theme, ThemeVars> = {
  dark: { bg: "#06070f", bg2: "rgba(255,255,255,0.03)", text: "rgba(255,255,255,0.85)", text2: "rgba(255,255,255,0.75)", text3: "rgba(255,255,255,0.4)", border: "rgba(255,255,255,0.06)", inputBg: "rgba(255,255,255,0.04)", bubbleUser: "rgba(99,102,241,0.2)", bubbleAI: "rgba(255,255,255,0.06)" },
  light: { bg: "#f5f5f7", bg2: "rgba(0,0,0,0.02)", text: "rgba(0,0,0,0.85)", text2: "rgba(0,0,0,0.7)", text3: "rgba(0,0,0,0.4)", border: "rgba(0,0,0,0.08)", inputBg: "rgba(255,255,255,0.8)", bubbleUser: "rgba(99,102,241,0.12)", bubbleAI: "rgba(0,0,0,0.04)" },
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
