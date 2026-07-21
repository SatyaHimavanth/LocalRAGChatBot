import { useState, useEffect, useRef, useCallback } from "react";
import type { ReactNode } from "react";
import { Events } from "@wailsio/runtime";
import { SendMessage, IngestFile, CreateCollection, UpdateCollectionProfile, GetCollections, CreateChat, GetChats, GetChatMessages, GetChatBranchOptions, SelectChatBranch, UpdateChatTitle, DeleteChat, DeleteCollection, DeleteDocument, GetDocumentsByCollection, GetEventLogs, ArchiveChat, UnarchiveChat, PinChat, UnpinChat, Search, SearchMetadata, SearchWorkspace, GetDocumentContent, GetDocumentChunks, GetSessionSources, GetChunkContext, CancelGeneration, CancelIngest, StartIngestBatch, GetIncompleteJobs, ResumeIngest, DiscardAllIncomplete, GetMCPConfiguration, VerifyMCPConfiguration, SaveMCPConfiguration, SetMCPEnabled, SetMCPServerEnabled, SetMCPToolEnabled } from "../bindings/changeme/internal/app/chatservice";
import { Message, Chat, Collection, DocRecord, SearchResult, SearchScope, ToastMsg, Theme, themeVars, getErrMsg, IncompleteJob, IngestLogEntry, EventLogEntry, AgentPlan, AgentResult, ChunkRecord, MCPConfiguration, MCPServer } from "./types";
import { I } from "./components/Icons";
import { Sidebar } from "./components/Sidebar";
import { ChatPanel } from "./components/ChatPanel";
import { SearchPanel } from "./components/SearchPanel";
import { CollectionsPanel } from "./components/CollectionsPanel";
import { DiagnosticsPanel } from "./components/DiagnosticsModal";
import { ExtensionsPanel } from "./components/ExtensionsPanel";
import { FileUploadModal } from "./components/FileUploadModal";
import { Modal, ConfirmModal } from "./components/Modal";
import { Toast } from "./components/Toast";

const globalCSS = (t: Theme) => `*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:${themeVars[t].bg};color:${themeVars[t].text};overflow:hidden;transition:background 0.3s,color 0.3s}
::-webkit-scrollbar{width:6px}::-webkit-scrollbar-track{background:transparent}
::-webkit-scrollbar-thumb{background:rgba(128,128,128,0.2);border-radius:3px}
input,textarea,button,select{font-family:inherit}
@keyframes spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
@keyframes slideIn{from{transform:translateX(120%);opacity:0}to{transform:translateX(0);opacity:1}}
@keyframes progressPulse{0%,100%{opacity:0.6}50%{opacity:1}}
`;


const buildAgentPlan = (data: any): AgentPlan => ({
  intent: typeof data?.intent === "string" && data.intent.trim() ? data.intent : "unknown",
  useRetrieval: data?.useRetrieval === true,
  useMemory: data?.useMemory === true,
  useDirect: data?.useDirect === true,
  topK: Number(data?.topK ?? 0),
  retrievalQuery: typeof data?.retrievalQuery === "string" ? data.retrievalQuery : "",
  reason: typeof data?.reason === "string" ? data.reason : "",
});

const buildAgentResult = (data: any): AgentResult => ({
  cancelled: data?.cancelled === true,
  usedRetrieval: data?.usedRetrieval === true,
  usedMemory: data?.usedMemory === true,
  usedDirect: data?.usedDirect === true,
  sourceCount: Number(data?.sourceCount ?? 0),
  evidenceCount: Number(data?.evidenceCount ?? 0),
  confidence: typeof data?.confidence === "number" ? data.confidence : Number(data?.confidence ?? 0),
  verified: data?.verified === true,
  verification: typeof data?.verification === "string" ? data.verification : "",
  evidenceGaps: Array.isArray(data?.evidenceGaps) ? data.evidenceGaps.filter((x: any) => typeof x === "string") : [],
  reason: typeof data?.reason === "string" ? data.reason : "",
  retrievalQuery: typeof data?.retrievalQuery === "string" ? data.retrievalQuery : "",
  topK: Number(data?.topK ?? 0),
});

const summarizePlan = (plan: AgentPlan) => {
  if (plan.useRetrieval) {
    return plan.retrievalQuery ? `Planning retrieval for "${plan.retrievalQuery}"` : "Planning retrieval...";
  }
  if (plan.useMemory) return "Planning from conversation memory...";
  if (plan.useDirect) return "Planning direct answer...";
  return "Planning...";
};

const parseStoredAgentResult = (raw: any): AgentResult | undefined => {
  if (!raw) return undefined;
  if (typeof raw === "object") return buildAgentResult(raw);
  if (typeof raw !== "string") return undefined;
  try {
    return buildAgentResult(JSON.parse(raw));
  } catch {
    return undefined;
  }
};

const encodeBranchPrompt = (parentMessageId: number, prompt: string) => `[[branch-parent:${parentMessageId}]]\n${prompt}`;

const initialTheme = (): Theme =>
  typeof window !== "undefined" && typeof window.matchMedia === "function" && window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";

const mapMCPServer = (server: any): MCPServer => ({
  name: String(server?.name ?? server?.Name ?? ""),
  configJson: String(server?.configJson ?? server?.ConfigJSON ?? "{}"),
  enabled: server?.enabled === true || server?.Enabled === true,
  verified: server?.verified === true || server?.Verified === true,
  toolCount: Number(server?.toolCount ?? server?.ToolCount ?? 0),
  toolsJson: String(server?.toolsJson ?? server?.ToolsJSON ?? "[]"),
  lastError: String(server?.lastError ?? server?.LastError ?? ""),
  lastVerifiedAt: Number(server?.lastVerifiedAt ?? server?.LastVerifiedAt ?? 0),
  enabledTools: Array.isArray(server?.enabledTools ?? server?.EnabledTools) ? (server.enabledTools ?? server.EnabledTools).map(String) : [],
});

const mapMCPConfiguration = (value: any): MCPConfiguration => ({
  enabled: value?.enabled === true || value?.Enabled === true,
  servers: Array.isArray(value?.servers ?? value?.Servers) ? (value.servers ?? value.Servers).map(mapMCPServer) : [],
});


export default function App() {
  const [theme, setTheme] = useState<Theme>(initialTheme);
  const [tab, setTab] = useState<"chat"|"search"|"cols"|"diag"|"ext">("chat");
  const [chats, setChats] = useState<Chat[]>([]);
  const [activeChatId, setActiveChatId] = useState(0);
  const [input, setInput] = useState("");
  const [generatingChatIds, setGeneratingChatIds] = useState<Set<number>>(() => new Set());
  const [statusMsgsByChat, setStatusMsgsByChat] = useState<Record<number, Message[]>>({});
  const activeChat=chats.find(c=>c.id===activeChatId);
  const gen = generatingChatIds.has(activeChatId);
  const statusMsgs = statusMsgsByChat[activeChatId] || [];
  const isArchived=activeChat?.archived===true;
  const createdInitialChat = useRef(false);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [mcpConfiguration, setMcpConfiguration] = useState<MCPConfiguration>({ enabled: false, servers: [] });
  const [mcpLoading, setMcpLoading] = useState(false);

  const [sq, setSq]=useState(""); const [sResults,setSResults]=useState<SearchResult[]>([]);
  const [sBusy,setSBusy]=useState(false); const [sDone,setSDone]=useState(false);
  const [searchFilter,setSearchFilter]=useState("all");
  const [searchScope,setSearchScope]=useState<SearchScope>("collection");
  const [searchLimit,setSearchLimit]=useState(20);
  const [searchMinScore,setSearchMinScore]=useState(0);

  const [cols,setCols]=useState<Collection[]>([]);
  const [activeColId,setActiveColId]=useState(1);
  const [newColName,setNewColName]=useState("");
  const [isIngesting, setIsIngesting] = useState(false);
  const [collectionProfileModal, setCollectionProfileModal] = useState<{ open: boolean; collectionId: number; name: string; embeddingModel: string; embeddingDims: string; vectorBackend: string }>({
    open: false,
    collectionId: 0,
    name: "",
    embeddingModel: "",
    embeddingDims: "0",
    vectorBackend: "sqlite-vec",
  });
  const [idocs,setIdocs]=useState<DocRecord[]>([]);
  const [selectedDocId,setSelectedDocId]=useState<number|null>(null);
  const [selectedDocContent,setSelectedDocContent]=useState("");
  const [selectedDocChunks,setSelectedDocChunks]=useState<ChunkRecord[]>([]);
  const [chunkContextModal, setChunkContextModal] = useState<{ open: boolean; loading: boolean; error: string; title: string; items: ChunkRecord[] }>({
    open: false,
    loading: false,
    error: "",
    title: "",
    items: [],
  });

  const [showUploadModal,setShowUploadModal]=useState(false);
  const [incompleteJobs,setIncompleteJobs]=useState<IncompleteJob[]>([]);
  const [ingestLogs,setIngestLogs]=useState<IngestLogEntry[]>([]);
  const [eventLogs,setEventLogs]=useState<EventLogEntry[]>([]);
  const closeUploadModal = () => { setShowUploadModal(false); loadCols(); loadDocs(activeColId); loadIncompleteJobs(); };
  const [showResumeModal, setShowResumeModal] = useState(false);

  const [toasts,setToasts]=useState<ToastMsg[]>([]);
  const addToast=useCallback((type:"success"|"error"|"info",message:string)=>{const id=crypto.randomUUID();setToasts(p=>[...p,{id,type,message}]);setTimeout(()=>setToasts(p=>p.filter(t=>t.id!==id)),5000)},[]);
  const dismissToast=(id:string)=>setToasts(p=>p.filter(t=>t.id!==id));
  const pushIngestLog = useCallback((entry: IngestLogEntry) => {
    setIngestLogs(prev => [entry, ...prev].slice(0, 80));
  }, []);
  const loadEventLogs = useCallback(async () => {
    try {
      const rows: any = await GetEventLogs(30);
      const mapped = (rows || []).map((x: any) => ({
        id: Number(x.id ?? x.ID ?? 0),
        eventKey: String(x.eventKey ?? x.EventKey ?? ""),
        title: String(x.title ?? x.Title ?? ""),
        detail: String(x.detail ?? x.Detail ?? ""),
        severity: String(x.severity ?? x.Severity ?? "info"),
        scope: String(x.scope ?? x.Scope ?? "workspace"),
        collectionId: Number(x.collectionId ?? x.CollectionID ?? 0),
        chatId: Number(x.chatId ?? x.ChatID ?? 0),
        docId: Number(x.docId ?? x.DocID ?? 0),
        batchId: String(x.batchId ?? x.BatchID ?? ""),
        createdAt: Number(x.createdAt ?? x.CreatedAt ?? 0),
      })) as EventLogEntry[];
      setEventLogs(mapped);
      return mapped;
    } catch (e) {
      console.error("Failed to load event logs:", e);
      setEventLogs([]);
    }
  }, []);

  const loadMCPConfiguration = useCallback(async () => {
    setMcpLoading(true);
    try { setMcpConfiguration(mapMCPConfiguration(await GetMCPConfiguration())); }
    catch (error) { console.error("Failed to load MCP configuration:", error); }
    finally { setMcpLoading(false); }
  }, []);

  const loadIncompleteJobs = useCallback(async () => {
    try {
      const jobs: any = await GetIncompleteJobs();
      const list = (jobs || []).map((j: any) => ({
        docId: j.docId ?? j.DocID ?? 0,
        collectionId: j.collectionId ?? j.CollectionID ?? 0,
        filename: j.filename ?? j.Filename ?? "",
        status: j.status ?? j.Status ?? "",
        chunkCount: j.chunkCount ?? j.ChunkCount ?? 0,
        expectedChunks: j.expectedChunks ?? j.ExpectedChunks ?? 0,
        batchId: j.batchId ?? j.BatchID ?? "",
        errorMessage: j.errorMessage ?? j.ErrorMessage ?? "",
        progressPct: j.progressPct ?? j.ProgressPct ?? 0,
        updatedAt: j.updatedAt ?? j.UpdatedAt ?? 0,
        createdAt: j.createdAt ?? j.CreatedAt ?? 0,
      })) as IncompleteJob[];
      setIncompleteJobs(list);
      if (list.length > 0) setIsIngesting(false);
      return list;
    } catch (e) {
      console.error("Failed to load incomplete jobs:", e);
    }
  }, []);

  const [confirm,setConfirm]=useState<{open:boolean;title:string;message:string;detail:string;confirmLabel:string;onConfirm:()=>void}>({open:false,title:"",message:"",detail:"",confirmLabel:"Delete",onConfirm:()=>{}});
  const [renameModal,setRenameModal]=useState<{open:boolean;chatId:number;value:string}>({open:false,chatId:0,value:""});
  const [ctxMenuChatId,setCtxMenuChatId]=useState<number|null>(null);
  const [ctxMenuPos,setCtxMenuPos]=useState({x:0,y:0});
  const ctxRef=useRef<HTMLDivElement>(null);
  const [colDropdownOpen,setColDropdownOpen]=useState(false);
  const [colSearch,setColSearch]=useState("");
  const colDropdownRef=useRef<HTMLDivElement>(null);
  const selectedDoc = selectedDocId ? idocs.find(doc => doc.id === selectedDocId) : undefined;

  useEffect(()=>{const h=(e:MouseEvent)=>{if(ctxRef.current&&!ctxRef.current.contains(e.target as Node))setCtxMenuChatId(null)};document.addEventListener("mousedown",h);return()=>document.removeEventListener("mousedown",h)},[]);
  useEffect(()=>{const h=(e:MouseEvent)=>{if(colDropdownRef.current&&!colDropdownRef.current.contains(e.target as Node))setColDropdownOpen(false)};document.addEventListener("mousedown",h);return()=>document.removeEventListener("mousedown",h)},[]);
  useEffect(()=>{(async ()=>{ await loadCols(); await loadChats(); await loadEventLogs(); const list = await loadIncompleteJobs(); if (list && list.length > 0) setShowResumeModal(true); })();},[]);
  useEffect(() => { if (tab === "ext") void loadMCPConfiguration(); }, [tab, loadMCPConfiguration]);


useEffect(()=>{
  const offT = Events.On("chat:token", (e:any) => {
    const sid = Number(e?.data?.sessionId ?? 0);
    if (!sid) return;
    setStatusMsgsByChat(p => ({ ...p, [sid]: [] }));
    setChats(p => p.map(c => {
      if (c.id !== sid) return c;
      const ms = [...c.messages];
      const last = ms[ms.length - 1];
      if (last && last.sender === "ai") {
        ms[ms.length - 1] = { ...last, text: last.text + e.data.token };
      } else {
        ms.push({ id: crypto.randomUUID(), sender: "ai", text: e.data.token });
      }
      return { ...c, messages: ms };
    }));
  });

  const offMessageSaved = Events.On("chat:message_saved", (e:any) => {
    const sid = e?.data?.sessionId;
    const backendMsgId = Number(e?.data?.msgId ?? 0);
    if (!sid || !backendMsgId) return;
    setChats(p => p.map(c => {
      if (c.id !== sid) return c;
      const ms = [...c.messages];
      for (let i = ms.length - 1; i >= 0; i--) {
        if (ms[i].sender === "user") {
          ms[i] = { ...ms[i], id: backendMsgId.toString() };
          break;
        }
      }
      return { ...c, messages: ms, currentLeafMessageId: backendMsgId };
    }));
  });

  const offPlan = Events.On("chat:plan", (e:any) => {
    const sid = Number(e?.data?.sessionId ?? 0);
    if (!sid) return;
    const plan = buildAgentPlan(e.data);
    setStatusMsgsByChat(p => ({ ...p, [sid]: [{ id: crypto.randomUUID(), sender: "system", text: summarizePlan(plan) }] }));
    setChats(p => p.map(c => c.id === sid ? { ...c, agentPlan: plan } : c));
  });

  const offD = Events.On("chat:done", (e:any) => {
    const sid = Number(e?.data?.sessionId ?? 0);
    if (!sid) return;
    setGeneratingChatIds(p => {
      const next = new Set(p);
      next.delete(sid);
      return next;
    });
    setStatusMsgsByChat(p => ({ ...p, [sid]: [] }));
    const backendMsgId = e?.data?.msgId;
    const wasCancelled = e?.data?.cancelled === true;
    const agentResult = buildAgentResult(e?.data);

    // Cancelled with msgId == -1 means no message was saved yet — add a placeholder
    if (wasCancelled && backendMsgId === -1) {
      setChats(p => p.map(c => {
        if (c.id !== sid) return c;
        const ms = [...c.messages];
        ms.push({ id: crypto.randomUUID(), sender: "ai" as const, text: ".", cancelled: true, metadata: agentResult });
        return { ...c, messages: ms, lastAgentResult: agentResult };
      }));
      return;
    }

    // Cancelled mid-stream — mark the existing message (identified by msgId) as cancelled
    if (wasCancelled && backendMsgId > 0) {
      setChats(p => p.map(c => {
        if (c.id !== sid) return c;
        const ms = [...c.messages];
        let found = false;
        for (let i = ms.length - 1; i >= 0; i--) {
          if (ms[i].sender === "ai") {
            ms[i] = { ...ms[i], cancelled: true, id: backendMsgId.toString(), metadata: agentResult };
            found = true;
            break;
          }
        }
        if (!found) {
          ms.push({ id: backendMsgId.toString(), sender: "ai" as const, text: ".", cancelled: true, metadata: agentResult });
        }
        return { ...c, messages: ms, lastAgentResult: agentResult, currentLeafMessageId: backendMsgId };
      }));
      return;
    }

    // Normal completion — update the last AI message's id and attach metadata
    if (backendMsgId && sid) {
      setChats(p => p.map(c => {
        if (c.id !== sid) return c;
        const ms = [...c.messages];
        let found = false;
        for (let i = ms.length - 1; i >= 0; i--) {
          if (ms[i].sender === "ai") {
            ms[i] = { ...ms[i], id: backendMsgId.toString(), metadata: agentResult };
            found = true;
            break;
          }
        }
        if (!found) {
          ms.push({ id: backendMsgId.toString(), sender: "ai" as const, text: "", metadata: agentResult });
        }
        return { ...c, messages: ms, lastAgentResult: agentResult, currentLeafMessageId: backendMsgId };
      }));
      // Reload the persisted leaf and its sibling list so the branch switcher
      // appears immediately after an edited prompt completes.
      void loadChats();
    }
  });

  const offStatus = Events.On("chat:status", (e:any) => {
    const sid = Number(e?.data?.sessionId ?? 0);
    if (!sid) return;
    setStatusMsgsByChat(p => ({ ...p, [sid]: [{ id: crypto.randomUUID(), sender: "system", text: String(e?.data?.label || "Thinking...") }] }));
  });
  const offSources = Events.On("chat:sources", (e:any) => {
    const sid = e.data.sessionId;
    const msgId = e.data.msgId;
    const sources = e.data.sources;
    if (sources && msgId) {
      setChats(p => p.map(c => {
        if (c.id !== sid) return c;
        return { ...c, messageSources: { ...c.messageSources, [msgId]: sources } };
      }));
    }
  });

  return () => { offT(); offMessageSaved(); offPlan(); offD(); offStatus(); offSources(); };
},[]);


  // Track durable ingest lifecycle via backend events
  const ingestCountRef = useRef({ success: 0, error: 0, total: 0 });
  useEffect(()=>{
    const off = Events.On("ingest:progress", (e: any) => {
      if (!e.data) return;
      const phase = e.data.phase || "";
      const step = e.data.step || "";
      const label = e.data.label || e.data.message || step || phase || "ingest";
      if (phase === "staging" || phase === "embedding" || step === "staging" || step === "embedding" || step === "chunked") {
        setIsIngesting(true);
      }
      pushIngestLog({
        id: crypto.randomUUID(),
        timestamp: Date.now(),
        level: step === "doc_failed" || phase === "failed" ? "error" : "info",
        stage: phase || step || "ingest",
        message: label,
        filename: e.data.filename || undefined,
        collectionId: e.data.collectionId ?? activeColId,
        batchId: e.data.batchId || undefined,
      });
      if (step === "doc_ready") {
        ingestCountRef.current.success += 1;
        const fn = e.data.filename || "";
        if (fn) addToast("success", `✓ ${fn} ready`);
      }
      if (step === "doc_failed") {
        ingestCountRef.current.error += 1;
        const fn = e.data.filename || "";
        if (fn) addToast("error", `✗ ${fn} failed`);
      }
      if (phase === "staging_done" && typeof e.data.staged === "number") {
        ingestCountRef.current.total = e.data.staged;
      }
      if (phase === "batch_done" || step === "complete") {
        setIsIngesting(false);
        loadIncompleteJobs();
      }
    });
    return () => off();
  },[loadIncompleteJobs, activeColId, pushIngestLog, addToast]);

  // Auto-refresh collections and show toast when processing finishes
  const prevIngesting = useRef(false);
  useEffect(() => {
    if (prevIngesting.current && !isIngesting) {
      const { success, error, total } = ingestCountRef.current;
      if (total > 0 || success > 0 || error > 0) {
        if (error > 0) {
          addToast("info", `Ingestion complete: ${success} ready${error > 0 ? `, ${error} failed` : ""}`);
        } else if (success > 0) {
          addToast("success", `Ingestion complete: ${success} document${success > 1 ? "s" : ""} ready`);
        }
      }
      ingestCountRef.current = { success: 0, error: 0, total: 0 };
      loadCols();
      loadDocs(activeColId);
      loadIncompleteJobs();
    }
    prevIngesting.current = isIngesting;
  }, [isIngesting]);

  

  const loadCols=async()=>{try{const c:any=await GetCollections();if(c?.length){const mapped=(c||[]).map((x:any)=>({id:Number(x.id),name:String(x.name||""),docCount:Number(x.docCount||0),embeddingModel:String(x.embeddingModel ?? x.EmbeddingModel ?? ""),embeddingDims:Number(x.embeddingDims ?? x.EmbeddingDims ?? 0),vectorBackend:String(x.vectorBackend ?? x.VectorBackend ?? "sqlite-vec"),createdAt:Number(x.createdAt ?? x.CreatedAt ?? 0),updatedAt:Number(x.updatedAt ?? x.UpdatedAt ?? 0)}));setCols(mapped);setActiveColId(prev=>mapped.some((x:any)=>x.id===prev)?prev:mapped[0].id)}}catch(e){console.error(e)}};
  const loadDocs=async(colId:number)=>{try{const d:any=await GetDocumentsByCollection(colId);setIdocs((d||[]).map((x:any)=>({
    id:Number(x.id),collectionId:Number(x.collectionId),filename:String(x.filename||""),hash:String(x.hash||""),content:String(x.content||""),summary:String(x.summary||""),
    sourceType:String(x.sourceType||""),sourceSizeBytes:Number(x.sourceSizeBytes||0),wordCount:Number(x.wordCount||0),lineCount:Number(x.lineCount||0),
    characterCount:Number(x.characterCount||0),paragraphCount:Number(x.paragraphCount||0),title:String(x.title||""),createdAt:Number(x.createdAt||0),
    chunkCount:Number(x.chunkCount||0),status:String(x.status||"ready"),expectedChunks:Number(x.expectedChunks||0),batchId:String(x.batchId||""),
    errorMessage:String(x.errorMessage||""),updatedAt:Number(x.updatedAt||0),
  })))}catch(e){setIdocs([])}};
  useEffect(()=>{if(tab==="cols"){loadDocs(activeColId);setSelectedDocId(null);setSelectedDocContent("");setSelectedDocChunks([])}},[activeColId,tab]);

  const loadChats = async () => {
    try {
      const s: any = await GetChats();
      if (s?.length) {
        const loaded: Chat[] = [];
        for (const sess of s) {
          const ms: any = await GetChatMessages(sess.id);
          let msgSources: Record<number, any[]> = {};
          try {
            const sources: any = await GetSessionSources(sess.id);
            if (sources?.length > 0) {
              for (const src of sources) {
                if (!msgSources[src.messageId]) msgSources[src.messageId] = [];
                msgSources[src.messageId].push({
                  refNumber: src.refNumber,
                  chunkId: src.chunkId,
                  content: src.content,
                  filename: src.filename,
                  collectionId: src.collectionId,
                  collectionName: src.collectionName,
                  similarity: src.similarity,
                });
              }
            }
          } catch (e) {
            /* ignore source loading errors */
          }

          const messages = (ms || []).map((m: any) => ({
            id: m.id.toString(),
            sender: m.role === "user" ? "user" : "ai",
            text: m.content,
            cancelled: m.cancelled === true,
            parentMessageId: Number(m.parentMessageId ?? m.ParentMessageID ?? 0) || undefined,
            metadata: parseStoredAgentResult(m.agentMetadataJson ?? m.agentMetadataJSON ?? m.agentMetadata ?? ""),
          })) as Message[];
          const branchOptions: Record<number, number[]> = {};
          await Promise.all(messages.filter((m) => m.sender === "user").map(async (m) => {
            const messageId = Number(m.id);
            if (!messageId) return;
            try {
              const options: any[] = await GetChatBranchOptions(sess.id, messageId) || [];
              if (options.length > 1) branchOptions[messageId] = options.map((option) => Number(option.id));
            } catch { /* Branch controls are optional for legacy databases. */ }
          }));

          loaded.push({
            id: sess.id,
            title: sess.title || "New Chat",
            messages,
            createdAt: sess.createdAt * 1000,
            archived: sess.archived === true,
            pinned: sess.pinned === true,
            currentLeafMessageId: Number(sess.currentLeafMessageId ?? sess.currentLeafMessageID ?? 0) || undefined,
            messageSources: msgSources,
            branchOptions,
          });
        }
        setChats(loaded);
        setActiveChatId(currentId => loaded.some(chat => chat.id === currentId) ? currentId : loaded[0].id);
      } else if (!createdInitialChat.current) {
        createdInitialChat.current = true;
        newChat();
      }
    } catch (e) {
      console.error(e);
    }
  };

  const newChat = async () => {const empty=chats.find(c=>c.messages.length===0&&!c.archived);if(empty){setActiveChatId(empty.id);setTab("chat");return}try{const id=await CreateChat("New Chat",activeColId);setChats(p=>[{id,title:"New Chat",messages:[],createdAt:Date.now(),archived:false,pinned:false},...p]);setActiveChatId(id);setTab("chat")}catch(e){console.error(e)}};

  const submitPrompt = async (
    prompt: string,
    options?: { parentMessageId?: number; replaceFromMessageId?: number; renameChat?: boolean }
  ) => {
    if (!prompt.trim() || gen || isArchived) return;

    let tid = activeChatId;
    if (!tid) {
      try {
        tid = await CreateChat("New Chat", activeColId);
        setChats((p) => [{ id: tid, title: "New Chat", messages: [], createdAt: Date.now(), archived: false, pinned: false }, ...p]);
        setActiveChatId(tid);
      } catch (e) {
        console.error(e);
        return;
      }
    }

    const msg = prompt;
    const tempId = crypto.randomUUID();
    setInput("");
    setGeneratingChatIds(p => new Set(p).add(tid));
    setStatusMsgsByChat(p => ({ ...p, [tid]: [{ id: crypto.randomUUID(), sender: "system", text: "Thinking..." }] }));

    const oldChat = chats.find((c) => c.id === tid);
    const shouldRename = options?.renameChat !== false && !options?.parentMessageId;
    const isNew = oldChat?.title === "New Chat";
    const newTitle = shouldRename && isNew ? (msg.length > 25 ? msg.slice(0, 25) + "..." : msg) : oldChat?.title || "New Chat";
    if (shouldRename && isNew && tid) {
      try {
        await UpdateChatTitle(tid, newTitle);
      } catch (e) {
        /* ignore */
      }
    }

    const promptForBackend = options?.parentMessageId ? encodeBranchPrompt(options.parentMessageId, msg) : msg;
    setChats((p) =>
      p.map((c) =>
        c.id === tid
          ? {
              ...c,
              title: newTitle,
              messages: [
                ...(options?.replaceFromMessageId
                  ? c.messages.slice(0, Math.max(0, c.messages.findIndex((message) => Number(message.id) === options.replaceFromMessageId)))
                  : c.messages),
                { id: tempId, sender: "user", text: msg, parentMessageId: options?.parentMessageId },
              ],
            }
          : c
      )
    );

    try {
      await SendMessage(tid, activeColId, promptForBackend);
    } catch (e) {
      console.error(e);
      setGeneratingChatIds(p => {
        const next = new Set(p);
        next.delete(tid);
        return next;
      });
      setStatusMsgsByChat(p => ({ ...p, [tid]: [] }));
      setChats((p) => p.map((c) => {
        if (c.id !== tid) return c;
        const ms = [...c.messages];
        for (let i = ms.length - 1; i >= 0; i--) {
          if (ms[i].sender === "user" && ms[i].text === msg) {
            ms.splice(i, 1);
            break;
          }
        }
        return { ...c, messages: ms };
      }));
    }
  };

  const send = async () => {
    await submitPrompt(input, { renameChat: true });
  };

  const handleBranchFromMessage = async (messageId: number, prompt: string) => {
    const edited = activeChat?.messages.find((message) => Number(message.id) === messageId);
    await submitPrompt(prompt, {
      parentMessageId: edited?.parentMessageId,
      replaceFromMessageId: messageId,
      renameChat: false,
    });
  };

  const handleSelectBranch = async (messageId: number) => {
    if (!activeChatId || gen) return;
    try {
      await SelectChatBranch(activeChatId, messageId);
      await loadChats();
    } catch (e) {
      addToast("error", getErrMsg(e));
    }
  };

  // Two-phase batch: extract all files first, then embed (durable on backend).
  const handleStartBatch = useCallback(async (items: { file: File; replace: boolean }[]) => {
    setIsIngesting(true);
    try {
      const payloads: { filename: string; base64Data: string; textContent: string; replace: boolean }[] = [];
      for (const item of items) {
        const reader = new FileReader();
        const dataUrl = await new Promise<string>((resolve, reject) => {
          reader.onload = () => { const r = reader.result as string; resolve(r.split(",")[1] || ""); };
          reader.onerror = reject;
          reader.readAsDataURL(item.file);
        });
        payloads.push({
          filename: item.file.name,
          base64Data: dataUrl,
          textContent: "",
          replace: item.replace,
        });
      }
      return await StartIngestBatch(activeColId, payloads as any);
    } finally {
      setIsIngesting(false);
      loadCols();
      loadDocs(activeColId);
      loadIncompleteJobs();
    }
  }, [activeColId, loadIncompleteJobs]);

  // Paste text via modal (uses durable pipeline)
  const handleIngestPaste = useCallback(async (filename: string, content: string): Promise<string> => {
    try {
      let fn = filename.trim();
      if (fn.length < 3) return "Filename must be at least 3 characters";
      fn = fn.replace(/[^a-zA-Z0-9.-]/g, "_");
      if (!fn.includes(".")) fn += ".txt";
      setIsIngesting(true);
      await IngestFile(activeColId, fn, content);
      return "success";
    } catch (e: any) {
      return getErrMsg(e);
    } finally {
      setIsIngesting(false);
      loadIncompleteJobs();
    }
  }, [activeColId, loadIncompleteJobs]);

  const handleResumeJobs = useCallback(async () => {
    setShowUploadModal(true);
    setIsIngesting(true);
    pushIngestLog({ id: crypto.randomUUID(), timestamp: Date.now(), level: "info", stage: "queue", message: "Resuming incomplete ingestion jobs", collectionId: activeColId });
    try {
      await ResumeIngest();
    } finally {
      setIsIngesting(false);
      loadCols();
      loadDocs(activeColId);
      loadIncompleteJobs();
    }
  }, [activeColId, loadIncompleteJobs, pushIngestLog]);

  const handleDiscardAllJobs = useCallback(async () => {
    try {
      pushIngestLog({ id: crypto.randomUUID(), timestamp: Date.now(), level: "info", stage: "queue", message: "Discarding incomplete ingestion jobs", collectionId: activeColId });
      const n: any = await DiscardAllIncomplete();
      await loadIncompleteJobs();
      loadDocs(activeColId);
      loadCols();
      addToast("info", `Discarded ${n ?? 0} incomplete document(s)`);
    } catch (e: any) {
      addToast("error", getErrMsg(e));
    }
  }, [activeColId, loadIncompleteJobs, addToast, pushIngestLog]);

  const handleStopGeneration = useCallback(async () => {
    try { await CancelGeneration(activeChatId); } catch (e) { /* ignore */ }
    // Don't manipulate state here — the chat:done event handler
    // (fired by CancelGeneration's emit) handles adding the cancelled flag
  }, [activeChatId]);

  const handleCancelIngest = useCallback(async () => {
    try {
      pushIngestLog({ id: crypto.randomUUID(), timestamp: Date.now(), level: "warn", stage: "queue", message: "Cancelling active ingestion", collectionId: activeColId });
      await CancelIngest();
    } catch (e) {
      console.error(e);
    }
  }, [activeColId, pushIngestLog]);

  const viewDocumentContent = async (docId: number) => {
    setSelectedDocId(docId);
    setSelectedDocContent("Loading...");
    setSelectedDocChunks([]);
    try {
      const [content, chunks]: any = await Promise.all([
        GetDocumentContent(docId),
        GetDocumentChunks(docId),
      ]);
      setSelectedDocContent(typeof content === "string" ? content : "");
      setSelectedDocChunks((chunks || []).map((x: any) => ({
        id: Number(x.id ?? x.ID ?? 0),
        documentId: Number(x.documentId ?? x.DocumentID ?? 0),
        collectionId: Number(x.collectionId ?? x.CollectionID ?? 0),
        content: String(x.content ?? x.Content ?? ""),
        summary: String(x.summary ?? x.Summary ?? ""),
        ord: Number(x.ord ?? x.Ord ?? 0),
        level: Number(x.level ?? x.Level ?? 0),
        role: String(x.role ?? x.Role ?? ""),
        parentOrd: Number(x.parentOrd ?? x.ParentOrd ?? -1),
        prevOrd: Number(x.prevOrd ?? x.PrevOrd ?? -1),
        nextOrd: Number(x.nextOrd ?? x.NextOrd ?? -1),
        chunkHash: String(x.chunkHash ?? x.ChunkHash ?? ""),
        embeddingHash: String(x.embeddingHash ?? x.EmbeddingHash ?? ""),
        headingPath: String(x.headingPath ?? x.HeadingPath ?? ""),
        updatedAt: Number(x.updatedAt ?? x.UpdatedAt ?? 0),
      })) as ChunkRecord[]);
    }
    catch (e) {
      setSelectedDocContent("(Could not load content)");
      setSelectedDocChunks([]);
    }
  };

  const decodeHeadingPath = (raw?: string) => {
    if (!raw) return "";
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        return parsed.filter(Boolean).join(" > " );
      }
    } catch {}
    return raw;
  };

  const openChunkContext = useCallback(async (chunkId: number, filename: string) => {
    setChunkContextModal({ open: true, loading: true, error: "", title: `Chunk context — ${filename}`, items: [] });
    try {
      const rows: any = await GetChunkContext(chunkId, 1);
      const items = (rows || []).map((row: any) => ({
        id: row.id ?? row.ID ?? 0,
        documentId: row.documentId ?? row.DocumentID ?? 0,
        collectionId: row.collectionId ?? row.CollectionID ?? 0,
        content: row.content ?? row.Content ?? "",
        summary: row.summary ?? row.Summary ?? "",
        ord: row.ord ?? row.Ord ?? 0,
        level: row.level ?? row.Level ?? 0,
        role: row.role ?? row.Role ?? "",
        parentOrd: row.parentOrd ?? row.ParentOrd ?? -1,
        prevOrd: row.prevOrd ?? row.PrevOrd ?? -1,
        nextOrd: row.nextOrd ?? row.NextOrd ?? -1,
        chunkHash: row.chunkHash ?? row.ChunkHash ?? "",
        embeddingHash: row.embeddingHash ?? row.EmbeddingHash ?? "",
        headingPath: row.headingPath ?? row.HeadingPath ?? "",
        updatedAt: row.updatedAt ?? row.UpdatedAt ?? 0,
      })) as ChunkRecord[];
      setChunkContextModal({ open: true, loading: false, error: "", title: `Chunk context — ${filename}`, items });
    } catch (e) {
      setChunkContextModal(prev => ({ ...prev, open: true, loading: false, error: getErrMsg(e), title: `Chunk context — ${filename}`, items: [] }));
    }
  }, []);

  const handleDeleteChatAction = async (chatId: number) => { setConfirm({open:false,title:"",message:"",detail:"",confirmLabel:"",onConfirm:()=>{}});setCtxMenuChatId(null);try{await DeleteChat(chatId);setChats(p=>p.filter(c=>c.id!==chatId));if(activeChatId===chatId)setActiveChatId(0)}catch(e){console.error(e)}};
  const handleArchiveChat = async (chatId: number) => { setCtxMenuChatId(null); try { await ArchiveChat(chatId); setChats(p => p.map(c => c.id === chatId ? { ...c, archived: true, title: "[Archived] " + c.title.replace(/^\[Archived\]\s*/, "") } : c)); } catch (e) { console.error(e); } };
  const handleUnarchiveChat = async (chatId: number) => { setCtxMenuChatId(null); try { await UnarchiveChat(chatId); setChats(p => p.map(c => c.id === chatId ? { ...c, archived: false, title: c.title.replace(/^\[Archived\]\s*/, "") } : c)); } catch (e) { console.error(e); } };
  const handlePinChat = async (chatId: number) => { setCtxMenuChatId(null); const chat = chats.find(c => c.id === chatId); if (!chat) return; if (chat.pinned) { try { await UnpinChat(chatId); setChats(p => p.map(c => c.id === chatId ? { ...c, pinned: false } : c)); } catch (e) { console.error(e); } } else { try { await PinChat(chatId); setChats(p => p.map(c => c.id === chatId ? { ...c, pinned: true } : c)); } catch (e) { console.error(e); } } };
  const confirmDeleteChat = (chatId: number) => { const chat = chats.find(c => c.id === chatId); setConfirm({ open: true, title: "Delete Chat", message: "Delete this chat?", detail: `"${chat?.title || 'New Chat'}" will be permanently deleted.`, confirmLabel: "Delete Chat", onConfirm: () => { handleDeleteChatAction(chatId); } }); };

  const createCol = async () => { if (!newColName.trim()) return; try { const id = await CreateCollection(newColName); setCols(p => [...p, { id, name: newColName, docCount: 0, embeddingModel: "", embeddingDims: 0, vectorBackend: "sqlite-vec", createdAt: Math.floor(Date.now()/1000), updatedAt: Math.floor(Date.now()/1000) }]); setNewColName(""); setActiveColId(id); addToast("success", `Collection "${newColName}" created`); } catch (e: any) { addToast("error", getErrMsg(e)); } };
  const confirmDeleteCollection = (colId: number) => { const col = cols.find(c => c.id === colId); setConfirm({ open: true, title: "Delete Collection", message: `Delete "${col?.name || 'Unknown'}"?`, detail: `Permanently delete collection and ${col?.docCount || 0} documents.`, confirmLabel: "Delete Collection", onConfirm: async () => { setConfirm({ open: false, title: "", message: "", detail: "", confirmLabel: "", onConfirm: () => {} }); try { await DeleteCollection(colId); setCols(p => p.filter(c => c.id !== colId)); if (activeColId === colId) setActiveColId(cols.filter(c => c.id !== colId)[0]?.id || 0); setIdocs([]); addToast("success", "Deleted"); } catch (e) { addToast("error", getErrMsg(e)); } } }); };
  const confirmDeleteDocument = (docId: number) => { const doc = idocs.find(d => d.id === docId); setConfirm({ open: true, title: "Delete Document", message: `Delete "${doc?.filename || 'Unknown'}"?`, detail: "Permanently delete document and all chunks.", confirmLabel: "Delete Document", onConfirm: async () => { setConfirm({ open: false, title: "", message: "", detail: "", confirmLabel: "", onConfirm: () => {} }); try { await DeleteDocument(docId); setIdocs(p => p.filter(d => d.id !== docId)); setCols(p => p.map(c => c.id === activeColId ? { ...c, docCount: Math.max(0, c.docCount - 1) } : c)); addToast("success", "Deleted"); } catch (e) { addToast("error", getErrMsg(e)); } } }); };
  const openCollectionProfile = useCallback(() => {
    const col = cols.find(c => c.id === activeColId);
    if (!col) return;
    setCollectionProfileModal({
      open: true,
      collectionId: col.id,
      name: col.name,
      embeddingModel: col.embeddingModel || "",
      embeddingDims: String(col.embeddingDims ?? 0),
      vectorBackend: col.vectorBackend || "sqlite-vec",
    });
  }, [cols, activeColId]);

  const saveCollectionProfile = async () => {
    const colId = collectionProfileModal.collectionId;
    if (!colId) return;
    try {
      await UpdateCollectionProfile(colId, collectionProfileModal.embeddingModel.trim(), Number(collectionProfileModal.embeddingDims || 0), collectionProfileModal.vectorBackend.trim() || "sqlite-vec");
      setCols(prev => prev.map(c => c.id === colId ? { ...c, embeddingModel: collectionProfileModal.embeddingModel.trim(), embeddingDims: Number(collectionProfileModal.embeddingDims || 0), vectorBackend: collectionProfileModal.vectorBackend.trim() || "sqlite-vec", updatedAt: Math.floor(Date.now()/1000) } : c));
      setCollectionProfileModal({ open: false, collectionId: 0, name: "", embeddingModel: "", embeddingDims: "0", vectorBackend: "sqlite-vec" });
      addToast("success", "Collection profile updated");
    } catch (e) {
      addToast("error", getErrMsg(e));
    }
  };

  const doSearch = async () => { if (!sq.trim()) return; setSBusy(true); setSDone(true); try { let r: any = []; const limit = Math.max(1, Math.min(searchLimit || 20, 50)); if (searchScope === "metadata") { r = await SearchMetadata(sq, activeColId > 0 ? activeColId : 0, limit); } else if (searchScope === "workspace") { r = await SearchWorkspace(sq, activeColId > 0 ? activeColId : 0, activeChatId || 0, limit); } else if (searchScope === "all") { r = await Search(sq, 0, limit); } else { r = await Search(sq, activeColId > 0 ? activeColId : 0, limit); } setSResults(r || []); } catch (e) { console.error(e); setSResults([]); } setSBusy(false); };
  const clearSearch = () => { setSq(""); setSResults([]); setSDone(false); setSearchFilter("all"); setSearchScope("collection"); setSearchLimit(20); setSearchMinScore(0); };
  const displayScore = (score: number) => Math.max(0, Math.min(score * 100, 100)).toFixed(1);
  const filteredResults = sDone ? sResults.filter(r => { if (searchMinScore > 0 && (r.score * 100) < searchMinScore) return false; if (searchFilter === "all") return true; if (searchFilter === "keyword") return r.searchType === "keyword"; if (searchFilter === "vector") return r.searchType === "vector"; if (searchFilter === "hybrid") return r.searchType === "hybrid"; if (searchFilter === "metadata") return r.searchType === "metadata"; if (searchFilter === "workspace") return r.searchType === "workspace"; return true; }) : [];

  const filteredCols = cols.filter(c => c.name.toLowerCase().includes(colSearch.toLowerCase()));
  const activeCol = cols.find(c => c.id === activeColId);
  const collSelector = (
    <div ref={colDropdownRef} style={{ position: "relative", display: "inline-block" }}>
      <div onClick={() => setColDropdownOpen(!colDropdownOpen)} style={{ display: "flex", alignItems: "center", gap: 4, padding: "3px 8px", borderRadius: 6, border: "1px solid rgba(128,128,128,0.2)", cursor: "pointer", fontSize: 12, color: themeVars[theme].text3, background: themeVars[theme].inputBg, userSelect: "none" }}>
        <span style={{ color: themeVars[theme].text, fontWeight: 500 }}>{activeCol?.name || "Select"}</span><I.Down/>
      </div>
      {colDropdownOpen && <div style={{ position: "absolute", top: "100%", left: 0, marginTop: 4, zIndex: 100, width: 220, background: theme === "dark" ? "#1a1b2e" : "#fff", border: "1px solid rgba(128,128,128,0.15)", borderRadius: 8, overflow: "hidden", boxShadow: "0 8px 24px rgba(0,0,0,0.15)" }}>
        <input value={colSearch} onChange={e => setColSearch(e.target.value)} placeholder="Search..." autoFocus style={{ width: "100%", padding: "8px 10px", border: "none", borderBottom: "1px solid rgba(128,128,128,0.1)", background: "transparent", color: themeVars[theme].text, fontSize: 12, outline: "none" }} />
        <div style={{ maxHeight: 3 * 44, overflowY: "auto" }}>
          {filteredCols.length === 0 ? <div style={{ padding: 10, fontSize: 12, color: themeVars[theme].text3, textAlign: "center" }}>No collections</div>
            : filteredCols.map(c => <div key={c.id} onClick={() => { setActiveColId(c.id); setColDropdownOpen(false); setColSearch(""); }} style={{ padding: "10px 12px", cursor: "pointer", fontSize: 12, background: c.id === activeColId ? "rgba(99,102,241,0.1)" : "transparent", color: c.id === activeColId ? themeVars[theme].text : themeVars[theme].text2, borderLeft: c.id === activeColId ? "3px solid rgba(99,102,241,0.8)" : "3px solid transparent" }}>
              <div style={{ fontWeight: 500, marginBottom: 2 }}>{c.name}</div>
              <div style={{ fontSize: 10, color: themeVars[theme].text3 }}>{c.docCount} docs</div>
            </div>)}
        </div>
      </div>}
    </div>
  );

  const openRename = (chatId: number) => { const chat = chats.find(c => c.id === chatId); if (!chat) return; setRenameModal({ open: true, chatId, value: chat.title }); setCtxMenuChatId(null); };
  const submitRename = async () => { if (!renameModal.value.trim()) { setRenameModal({ open: false, chatId: 0, value: "" }); return; } try { await UpdateChatTitle(renameModal.chatId, renameModal.value.trim()); setChats(p => p.map(c => c.id === renameModal.chatId ? { ...c, title: renameModal.value.trim() } : c)); addToast("success", "Renamed"); setRenameModal({ open: false, chatId: 0, value: "" }); } catch (e) { addToast("error", "Rename failed"); } };

  const T = themeVars[theme];
  return (<>
    <style>{globalCSS(theme)}</style>
    <div style={{ display: "flex", height: "100vh", width: "100vw", background: T.bg, color: T.text, transition: "background 0.3s,color 0.3s" }}>
      <Sidebar chats={chats} activeChatId={activeChatId} tab={tab} sidebarOpen={sidebarOpen} isIngesting={isIngesting} generatingChatIds={generatingChatIds} theme={theme}
        onNewChat={newChat} onSelectChat={(id) => { setActiveChatId(id); setTab("chat"); }}
        onTabChange={setTab} onToggleSidebar={() => setSidebarOpen(!sidebarOpen)}
        onCtxMenu={(id, x, y) => { setCtxMenuChatId(id); setCtxMenuPos({ x, y }); }} />

      {tab === "chat" && <ChatPanel activeChat={activeChat} isArchived={isArchived} input={input} gen={gen} statusMsgs={statusMsgs} T={T} theme={theme} collSelector={collSelector}
        onInputChange={setInput} onSend={send} onThemeToggle={() => setTheme(theme === "dark" ? "light" : "dark")} onOpenUploadModal={() => setShowUploadModal(true)}
        onStopGeneration={gen ? handleStopGeneration : undefined} onRerunFromMessage={handleBranchFromMessage} onSelectBranch={handleSelectBranch} />}

      {tab === "search" && <SearchPanel sq={sq} sResults={sResults} sBusy={sBusy} sDone={sDone} searchFilter={searchFilter} filteredResults={filteredResults} searchScope={searchScope} searchLimit={searchLimit} searchMinScore={searchMinScore} T={T} displayScore={displayScore}
        onSearch={doSearch} onClear={clearSearch} onSqChange={setSq} onFilterChange={setSearchFilter} onScopeChange={setSearchScope} onLimitChange={setSearchLimit} onMinScoreChange={setSearchMinScore} onInspectChunk={openChunkContext} />}

      {tab === "cols" && <CollectionsPanel cols={cols} activeColId={activeColId} activeCollection={activeCol} idocs={idocs} selectedDocId={selectedDocId} selectedDocName={selectedDoc?.filename || selectedDoc?.title || "Document"} selectedDocContent={selectedDocContent} selectedDocChunks={selectedDocChunks} T={T}
        incompleteJobs={incompleteJobs} ingestLogs={ingestLogs} isIngesting={isIngesting}
        onSelectCol={setActiveColId} onDeleteCol={confirmDeleteCollection} onDeleteDoc={confirmDeleteDocument} onViewDoc={viewDocumentContent} onInspectChunk={openChunkContext} onRefresh={() => loadDocs(activeColId)}
        newColName={newColName} onNewColNameChange={setNewColName} onCreateCol={createCol} onOpenUploadModal={() => setShowUploadModal(true)} onEditCollectionProfile={openCollectionProfile}
        onResumeQueue={handleResumeJobs} onDiscardQueue={handleDiscardAllJobs} onCancelIngest={handleCancelIngest} />}

      {tab === "diag" && <DiagnosticsPanel
        chats={chats}
        activeChat={activeChat}
        cols={cols}
        activeCollection={activeCol}
        idocs={idocs}
        activeChatId={activeChatId}
        activeCollectionId={activeColId}
        searchScope={searchScope}
        ingestLogs={ingestLogs}
        eventLogs={eventLogs}
        incompleteJobs={incompleteJobs}
        isIngesting={isIngesting}
        T={T}
        onOpenChat={() => setTab("chat")}
        onOpenSearch={() => setTab("search")}
        onOpenCollections={() => setTab("cols")}
        onOpenUpload={() => setShowUploadModal(true)}
        onRefresh={() => {
          void loadCols();
          void loadDocs(activeColId);
          void loadChats();
          void loadIncompleteJobs();
          void loadEventLogs();
        }}
        onResumeQueue={handleResumeJobs}
        onDiscardQueue={handleDiscardAllJobs}
      />}

      {tab === "ext" && <ExtensionsPanel T={T} configuration={mcpConfiguration} loading={mcpLoading}
        onRefresh={loadMCPConfiguration}
        onVerify={async (configJSON) => ((await VerifyMCPConfiguration(configJSON)) || []).map(mapMCPServer)}
        onSave={async (configJSON) => { const servers = ((await SaveMCPConfiguration(configJSON)) || []).map(mapMCPServer); await loadMCPConfiguration(); return servers; }}
        onSetEnabled={async (enabled) => { await SetMCPEnabled(enabled); await loadMCPConfiguration(); }}
        onSetServerEnabled={async (name, enabled) => { await SetMCPServerEnabled(name, enabled); await loadMCPConfiguration(); }}
        onSetToolEnabled={async (serverName, toolName, enabled) => { await SetMCPToolEnabled(serverName, toolName, enabled); await loadMCPConfiguration(); }} />}

      {/* Context Menu */}
      {ctxMenuChatId !== null && (() => { const chat = chats.find(c => c.id === ctxMenuChatId); if (!chat) return null; const a = chat.archived; return (
        <div ref={ctxRef} style={{ position: "fixed", zIndex: 999, left: ctxMenuPos.x, top: ctxMenuPos.y, background: T.bg2, border: "1px solid "+T.border, borderRadius: 10, boxShadow: "0 12px 40px rgba(0,0,0,0.2)", padding: "4px", minWidth: 170 }}>
          {!a && ctxMenuItem("Rename", <I.Rename />, () => openRename(ctxMenuChatId!), T)}
          {ctxMenuItem("Delete", <I.Trash />, () => { confirmDeleteChat(ctxMenuChatId!); }, T, true)}
          {a ? ctxMenuItem("Unarchive", <I.Unarchive />, () => handleUnarchiveChat(ctxMenuChatId!), T) : ctxMenuItem("Archive", <I.Archive />, () => handleArchiveChat(ctxMenuChatId!), T)}
          {!a && ctxMenuItem(chat.pinned ? "Unpin" : "Pin", <I.Pin />, () => handlePinChat(ctxMenuChatId!), T)}
        </div>); })()}

      {/* Upload Modal */}
      <FileUploadModal
        open={showUploadModal}
        onClose={closeUploadModal}
        collectionId={activeColId}
        collectionName={activeCol?.name || "Unknown"}
        onStartBatch={handleStartBatch}
        onIngestPaste={handleIngestPaste}
        isIngesting={isIngesting}
        incompleteJobs={incompleteJobs}
        theme={theme}
      />

      {/* Collection Profile Modal */}
      <Modal open={collectionProfileModal.open} onClose={() => setCollectionProfileModal({ open: false, collectionId: 0, name: "", embeddingModel: "", embeddingDims: "0", vectorBackend: "sqlite-vec" })} title={`Collection Profile — ${collectionProfileModal.name || "Collection"}`} theme={theme}>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <label style={{ fontSize: 12, color: T.text3 }}>Embedding model</label>
          <input value={collectionProfileModal.embeddingModel} onChange={e => setCollectionProfileModal(p => ({ ...p, embeddingModel: e.target.value }))} placeholder="nomic-embed-text-v1.5" style={{ width: "100%", padding: "10px 14px", borderRadius: 8, border: "1px solid "+T.border, background: T.inputBg, color: T.text, fontSize: 13, outline: "none" }} />
          <label style={{ fontSize: 12, color: T.text3 }}>Embedding dimensions</label>
          <input type="number" min={0} value={collectionProfileModal.embeddingDims} onChange={e => setCollectionProfileModal(p => ({ ...p, embeddingDims: e.target.value }))} style={{ width: "100%", padding: "10px 14px", borderRadius: 8, border: "1px solid "+T.border, background: T.inputBg, color: T.text, fontSize: 13, outline: "none" }} />
          <label style={{ fontSize: 12, color: T.text3 }}>Vector backend</label>
          <input value={collectionProfileModal.vectorBackend} onChange={e => setCollectionProfileModal(p => ({ ...p, vectorBackend: e.target.value }))} placeholder="sqlite-vec" style={{ width: "100%", padding: "10px 14px", borderRadius: 8, border: "1px solid "+T.border, background: T.inputBg, color: T.text, fontSize: 13, outline: "none" }} />
          <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 4 }}>
            <button onClick={() => setCollectionProfileModal({ open: false, collectionId: 0, name: "", embeddingModel: "", embeddingDims: "0", vectorBackend: "sqlite-vec" })} style={{ padding: "8px 16px", borderRadius: 8, border: "1px solid "+T.border, cursor: "pointer", fontSize: 13, color: T.text2, background: "transparent" }}>Cancel</button>
            <button onClick={saveCollectionProfile} style={{ padding: "8px 16px", borderRadius: 8, border: "none", cursor: "pointer", fontSize: 13, fontWeight: 500, color: "#fff", background: "rgba(99,102,241,0.8)" }}>Save Profile</button>
          </div>
        </div>
      </Modal>

      {/* Rename Modal */}
      <Modal open={renameModal.open} onClose={() => setRenameModal({ open: false, chatId: 0, value: "" })} title="Rename Chat" theme={theme}>
        <input value={renameModal.value} onChange={e => setRenameModal(p => ({ ...p, value: e.target.value }))} onKeyDown={e => e.key === "Enter" && submitRename()} autoFocus style={{ width: "100%", padding: "10px 14px", borderRadius: 8, border: "1px solid "+T.border, background: T.inputBg, color: T.text, fontSize: 13, outline: "none", marginBottom: 12 }} />
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <button onClick={() => setRenameModal({ open: false, chatId: 0, value: "" })} style={{ padding: "8px 16px", borderRadius: 8, border: "1px solid "+T.border, cursor: "pointer", fontSize: 13, color: T.text2, background: "transparent" }}>Cancel</button>
          <button onClick={submitRename} style={{ padding: "8px 16px", borderRadius: 8, border: "none", cursor: "pointer", fontSize: 13, fontWeight: 500, color: "#fff", background: "rgba(99,102,241,0.8)" }}>Rename</button>
        </div>
      </Modal>

      {/* Chunk context modal */}
      <Modal open={chunkContextModal.open} onClose={() => setChunkContextModal({ open: false, loading: false, error: "", title: "", items: [] })} title={chunkContextModal.title || "Chunk context"} theme={theme} wide>
        {chunkContextModal.loading ? (
          <div style={{fontSize:13,color:T.text3}}>Loading chunk context...</div>
        ) : chunkContextModal.error ? (
          <div style={{fontSize:13,color:"rgba(239,68,68,0.9)"}}>{chunkContextModal.error}</div>
        ) : chunkContextModal.items.length === 0 ? (
          <div style={{fontSize:13,color:T.text3}}>No context rows available for this chunk.</div>
        ) : (
          <div style={{display:"flex",flexDirection:"column",gap:10}}>
            {chunkContextModal.items.map(item => (
              <div key={item.id} style={{padding:12,borderRadius:8,border:"1px solid "+T.border,background:T.inputBg}}>
                <div style={{display:"flex",justifyContent:"space-between",gap:8,flexWrap:"wrap",fontSize:11,color:T.text3,marginBottom:6}}>
                  <span style={{textTransform:"uppercase",fontWeight:600,color:item.role === "summary" ? "rgba(34,197,94,0.85)" : T.text3}}>{item.role || "chunk"}</span>
                  <span>ord {item.ord}</span>
                  <span>level {item.level}</span>
                  <span>chunk #{item.id}</span>
                </div>
                <div style={{fontSize:12,color:T.text2,marginBottom:6}}>
                  {item.headingPath ? <strong>Heading path:</strong> : null}
                  {item.headingPath ? ` ${decodeHeadingPath(item.headingPath)}` : ""}
                </div>
                {item.summary && (
                  <div style={{fontSize:12,color:T.text2,marginBottom:8}}>
                    <strong>Summary:</strong> {item.summary}
                  </div>
                )}
                <div style={{fontSize:12,lineHeight:1.6,whiteSpace:"pre-wrap",wordBreak:"break-word",fontFamily:"monospace",color:T.text}}>
                  {item.content}
                </div>
                <div style={{display:"flex",gap:10,flexWrap:"wrap",marginTop:8,fontSize:10,color:T.text3}}>
                  <span>parent {item.parentOrd >= 0 ? item.parentOrd : "—"}</span>
                  <span>prev {item.prevOrd >= 0 ? item.prevOrd : "—"}</span>
                  <span>next {item.nextOrd >= 0 ? item.nextOrd : "—"}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </Modal>

      {/* Resume Modal — force user to choose Resume or Discard */}
      <ConfirmModal
        open={showResumeModal}
        title="Incomplete Documents"
        message={`${incompleteJobs.length} document(s) were left incomplete when the app was last closed. Text was saved. Resume embedding or discard them.`}
        detail={"Choose an action to continue."}
        theme={theme}
        leftLabel={isIngesting ? "Resuming..." : "Resume embedding"}
        leftAction={async () => { setShowResumeModal(false); await handleResumeJobs(); }}
        confirmLabel={"Discard all"}
        onConfirm={async () => { await handleDiscardAllJobs(); setShowResumeModal(false); }}
        onCancel={() => { /* fallback: do nothing */ }}
        disableBackdropClose={true}
      />

      <ConfirmModal open={confirm.open} title={confirm.title} message={confirm.message} detail={confirm.detail} confirmLabel={confirm.confirmLabel} onConfirm={confirm.onConfirm} onCancel={() => setConfirm({ open: false, title: "", message: "", detail: "", confirmLabel: "", onConfirm: () => {} })} theme={theme} />
      <Toast toasts={toasts} onDismiss={dismissToast} theme={theme} />
    </div>
  </>);
}

// (btnStyleSmall removed because it's unused.)

function ctxMenuItem(label: string, icon: ReactNode, onClick: () => void, T: any, isDanger?: boolean) {
  return (
    <div onClick={onClick} style={{ display: "flex", alignItems: "center", gap: 10, padding: "8px 12px", borderRadius: 6, cursor: "pointer", fontSize: 13, color: isDanger ? "rgba(239,68,68,0.85)" : T.text2 }}
      onMouseEnter={e => (e.target as HTMLElement).style.background = "rgba(128,128,128,0.06)"}
      onMouseLeave={e => (e.target as HTMLElement).style.background = "transparent"}>
      {icon} {label}
    </div>
  );
}
