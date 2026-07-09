import React, { useState, useEffect, useRef, useCallback } from "react";
import { Events } from "@wailsio/runtime";
import { SendMessage, IngestFile, CreateCollection, GetCollections, CreateChat, GetChats, GetChatMessages, UpdateChatTitle, DeleteChat, DeleteCollection, DeleteDocument, GetDocumentsByCollection, ArchiveChat, UnarchiveChat, PinChat, UnpinChat, Search, UploadFile, GetDocumentContent, GetSessionSources } from "../bindings/changeme/internal/app/chatservice";
import { Message, Chat, Collection, DocRecord, SearchResult, ToastMsg, Theme, themeVars, getErrMsg } from "./types";
import { I } from "./components/Icons";
import { Sidebar } from "./components/Sidebar";
import { ChatPanel } from "./components/ChatPanel";
import { SearchPanel } from "./components/SearchPanel";
import { CollectionsPanel } from "./components/CollectionsPanel";
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

export default function App() {
  const [theme, setTheme] = useState<Theme>("dark");
  const [tab, setTab] = useState<"chat"|"search"|"cols">("chat");
  const [chats, setChats] = useState<Chat[]>([]);
  const [activeChatId, setActiveChatId] = useState(0);
  const [input, setInput] = useState("");
  const [gen, setGen] = useState(false);
  const [statusMsgs, setStatusMsgs] = useState<Message[]>([]);
  const activeChat=chats.find(c=>c.id===activeChatId);
  const isArchived=activeChat?.archived===true;
  const createdInitialChat = useRef(false);
  const [sidebarOpen, setSidebarOpen] = useState(true);

  const [sq, setSq]=useState(""); const [sResults,setSResults]=useState<SearchResult[]>([]);
  const [sBusy,setSBusy]=useState(false); const [sDone,setSDone]=useState(false);
  const [searchFilter,setSearchFilter]=useState("all");

  const [cols,setCols]=useState<Collection[]>([]);
  const [activeColId,setActiveColId]=useState(1);
  const [newColName,setNewColName]=useState("");
  const [isIngesting]=useState(false);
  const [idocs,setIdocs]=useState<DocRecord[]>([]);
  const [selectedDocId,setSelectedDocId]=useState<number|null>(null);
  const [selectedDocContent,setSelectedDocContent]=useState("");

  const [showUploadModal,setShowUploadModal]=useState(false);

  const [toasts,setToasts]=useState<ToastMsg[]>([]);
  const addToast=useCallback((type:"success"|"error"|"info",message:string)=>{const id=crypto.randomUUID();setToasts(p=>[...p,{id,type,message}]);setTimeout(()=>setToasts(p=>p.filter(t=>t.id!==id)),5000)},[]);
  const dismissToast=(id:string)=>setToasts(p=>p.filter(t=>t.id!==id));

  const [confirm,setConfirm]=useState<{open:boolean;title:string;message:string;detail:string;confirmLabel:string;onConfirm:()=>void}>({open:false,title:"",message:"",detail:"",confirmLabel:"Delete",onConfirm:()=>{}});
  const [renameModal,setRenameModal]=useState<{open:boolean;chatId:number;value:string}>({open:false,chatId:0,value:""});
  const [ctxMenuChatId,setCtxMenuChatId]=useState<number|null>(null);
  const [ctxMenuPos,setCtxMenuPos]=useState({x:0,y:0});
  const ctxRef=useRef<HTMLDivElement>(null);
  const [colDropdownOpen,setColDropdownOpen]=useState(false);
  const [colSearch,setColSearch]=useState("");
  const colDropdownRef=useRef<HTMLDivElement>(null);

  useEffect(()=>{const h=(e:MouseEvent)=>{if(ctxRef.current&&!ctxRef.current.contains(e.target as Node))setCtxMenuChatId(null)};document.addEventListener("mousedown",h);return()=>document.removeEventListener("mousedown",h)},[]);
  useEffect(()=>{const h=(e:MouseEvent)=>{if(colDropdownRef.current&&!colDropdownRef.current.contains(e.target as Node))setColDropdownOpen(false)};document.addEventListener("mousedown",h);return()=>document.removeEventListener("mousedown",h)},[]);
  useEffect(()=>{loadCols();loadChats()},[]);

  useEffect(()=>{
    const offT=Events.On("chat:token",(e:any)=>{setStatusMsgs([]);const sid=e.data.sessionId;setChats(p=>p.map(c=>{if(c.id!==sid)return c;const ms=[...c.messages];const last=ms[ms.length-1];if(last&&last.sender==="ai"){ms[ms.length-1]={...last,text:last.text+e.data.token}}else{ms.push({id:crypto.randomUUID(),sender:"ai",text:e.data.token})}return{...c,messages:ms}}))});
    const offD=Events.On("chat:done",(e:any)=>{
      setGen(false);setStatusMsgs([]);
      // chat:done now carries msgId from backend â€” update the last AI message's id to match
      if (e?.data?.msgId) {
        const sid = e.data.sessionId;
        const backendMsgId = e.data.msgId;
        setChats(p=>p.map(c=>{
          if(c.id!==sid)return c;
          const ms = [...c.messages];
          // Find the last AI message (which had a UUID id) and replace its id with the backend msgId
          for(let i=ms.length-1;i>=0;i--){
            if(ms[i].sender==="ai"){
              ms[i] = {...ms[i], id: backendMsgId.toString()};
              break;
            }
          }
          return {...c, messages: ms};
        }));
      }
    });
    const offStatus=Events.On("chat:status",(e:any)=>{setStatusMsgs([{id:crypto.randomUUID(),sender:"system",text:e.data.label}])});
    const offSources=Events.On("chat:sources",(e:any)=>{
      const sid=e.data.sessionId;
      const msgId=e.data.msgId;
      const sources=e.data.sources;
      if (sources && msgId) {
        setChats(p=>p.map(c=>{
          if(c.id!==sid)return c;
          return {...c, messageSources:{...c.messageSources, [msgId]:sources}};
        }));
      }
    });
    return()=>{offT();offD();offStatus();offSources()};
  },[]);

  const loadCols=async()=>{try{const c:any=await GetCollections();if(c?.length){setCols(c);setActiveColId(c[0].id)}}catch(e){console.error(e)}};
  const loadDocs=async(colId:number)=>{try{const d:any=await GetDocumentsByCollection(colId);setIdocs(d?.length?d:[])}catch(e){setIdocs([])}};
  useEffect(()=>{if(tab==="cols"){loadDocs(activeColId);setSelectedDocId(null);setSelectedDocContent("")}},[activeColId,tab]);

  const loadChats=async()=>{
    try{const s:any=await GetChats();if(s?.length){const loaded:Chat[]=[];for(const sess of s){const ms:any=await GetChatMessages(sess.id);
      // Load source references for this session
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
      } catch(e) { /* ignore source loading errors */ }
      loaded.push({id:sess.id,title:sess.title||"New Chat",messages:(ms||[]).map((m:any)=>({id:m.id.toString(),sender:m.role==="user"?"user":"ai",text:m.content})),createdAt:sess.createdAt*1000,archived:sess.archived===true,pinned:sess.pinned===true,messageSources:msgSources})}setChats(loaded);if(loaded.length&&!activeChatId)setActiveChatId(loaded[0].id)}else if(!createdInitialChat.current){createdInitialChat.current=true;newChat()}}catch(e){console.error(e)}
  };

  const newChat=async()=>{const empty=chats.find(c=>c.messages.length===0&&!c.archived);if(empty){setActiveChatId(empty.id);setTab("chat");return}try{const id=await CreateChat("New Chat",activeColId);setChats(p=>[{id,title:"New Chat",messages:[],createdAt:Date.now(),archived:false,pinned:false},...p]);setActiveChatId(id);setTab("chat")}catch(e){console.error(e)}};

  const send=async()=>{
    if(!input.trim()||gen||isArchived)return;
    let tid=activeChatId;if(!tid){try{tid=await CreateChat("New Chat",activeColId);setChats(p=>[{id:tid,title:"New Chat",messages:[],createdAt:Date.now(),archived:false,pinned:false},...p]);setActiveChatId(tid)}catch(e){console.error(e);return}}
    const msg=input;setInput("");setGen(true);setStatusMsgs([{id:crypto.randomUUID(),sender:"system",text:"Thinking..."}]);
    const oldChat=chats.find(c=>c.id===tid);const isNew=oldChat?.title==="New Chat";
    const newTitle=isNew?(msg.length>25?msg.slice(0,25)+"...":msg):oldChat?.title||"New Chat";
    if(isNew&&tid){try{await UpdateChatTitle(tid,newTitle)}catch(e){}}
    setChats(p=>p.map(c=>c.id===tid?{...c,title:newTitle,messages:[...c.messages,{id:crypto.randomUUID(),sender:"user",text:msg}]}:c));
    try{await SendMessage(tid,activeColId,msg)}catch(e){console.error(e);setGen(false);setStatusMsgs([])}
  };

  // File upload via modal â€” does NOT update App state during processing.
  // Only returns result strings. Collections refresh on modal close.
  const processFile = useCallback(async (file: File, replace: boolean): Promise<string> => {
    try {
      const reader = new FileReader();
      const dataUrl = await new Promise<string>((resolve, reject) => {
        reader.onload = () => { const r = reader.result as string; resolve(r.split(",")[1]); };
        reader.onerror = reject; reader.readAsDataURL(file);
      });
      const r: any = await UploadFile(file.name, dataUrl, activeColId, replace);
      const status = r?.status ?? r?.Status;
      const message = r?.message ?? r?.Message;
      if (status === "duplicate" && !replace) {
        return "duplicate";
      } else if (status === "success" || status === "replaced") {
        return status;
      } else {
        return message || `Failed to process "${file.name}"`;
      }
    } catch (e: any) {
      return getErrMsg(e);
    }
  }, [activeColId]);

  // Paste text via modal
  const handleIngestPaste = useCallback(async (filename: string, content: string): Promise<string> => {
    try {
      let fn = filename.trim();
      if (fn.length < 3) return "Filename must be at least 3 characters";
      fn = fn.replace(/[^a-zA-Z0-9.-]/g, "_");
      if (!fn.includes(".")) fn += ".txt";
      await IngestFile(activeColId, fn, content);
      return "success";
    } catch (e: any) {
      return getErrMsg(e);
    }
  }, [activeColId]);

  const viewDocumentContent = async (docId: number) => {
    setSelectedDocId(docId);
    try { const content: string = await GetDocumentContent(docId); setSelectedDocContent(content); }
    catch (e) { setSelectedDocContent("(Could not load content)"); }
  };

  const handleDeleteChatAction = async (chatId: number) => { setConfirm({open:false,title:"",message:"",detail:"",confirmLabel:"",onConfirm:()=>{}});setCtxMenuChatId(null);try{await DeleteChat(chatId);setChats(p=>p.filter(c=>c.id!==chatId));if(activeChatId===chatId)setActiveChatId(0)}catch(e){console.error(e)}};
  const handleArchiveChat = async (chatId: number) => { setCtxMenuChatId(null); try { await ArchiveChat(chatId); setChats(p => p.map(c => c.id === chatId ? { ...c, archived: true, title: "[Archived] " + c.title.replace(/^\[Archived\]\s*/, "") } : c)); } catch (e) { console.error(e); } };
  const handleUnarchiveChat = async (chatId: number) => { setCtxMenuChatId(null); try { await UnarchiveChat(chatId); setChats(p => p.map(c => c.id === chatId ? { ...c, archived: false, title: c.title.replace(/^\[Archived\]\s*/, "") } : c)); } catch (e) { console.error(e); } };
  const handlePinChat = async (chatId: number) => { setCtxMenuChatId(null); const chat = chats.find(c => c.id === chatId); if (!chat) return; if (chat.pinned) { try { await UnpinChat(chatId); setChats(p => p.map(c => c.id === chatId ? { ...c, pinned: false } : c)); } catch (e) { console.error(e); } } else { try { await PinChat(chatId); setChats(p => p.map(c => c.id === chatId ? { ...c, pinned: true } : c)); } catch (e) { console.error(e); } } };
  const confirmDeleteChat = (chatId: number) => { const chat = chats.find(c => c.id === chatId); setConfirm({ open: true, title: "Delete Chat", message: "Delete this chat?", detail: `"${chat?.title || 'New Chat'}" will be permanently deleted.`, confirmLabel: "Delete Chat", onConfirm: () => { handleDeleteChatAction(chatId); } }); };

  const createCol = async () => { if (!newColName.trim()) return; try { const id = await CreateCollection(newColName); setCols(p => [...p, { id, name: newColName, docCount: 0 }]); setNewColName(""); setActiveColId(id); addToast("success", `Collection "${newColName}" created`); } catch (e: any) { addToast("error", getErrMsg(e)); } };
  const confirmDeleteCollection = (colId: number) => { const col = cols.find(c => c.id === colId); setConfirm({ open: true, title: "Delete Collection", message: `Delete "${col?.name || 'Unknown'}"?`, detail: `Permanently delete collection and ${col?.docCount || 0} documents.`, confirmLabel: "Delete Collection", onConfirm: async () => { setConfirm({ open: false, title: "", message: "", detail: "", confirmLabel: "", onConfirm: () => {} }); try { await DeleteCollection(colId); setCols(p => p.filter(c => c.id !== colId)); if (activeColId === colId) setActiveColId(cols.filter(c => c.id !== colId)[0]?.id || 0); setIdocs([]); addToast("success", "Deleted"); } catch (e) { addToast("error", getErrMsg(e)); } } }); };
  const confirmDeleteDocument = (docId: number) => { const doc = idocs.find(d => d.id === docId); setConfirm({ open: true, title: "Delete Document", message: `Delete "${doc?.filename || 'Unknown'}"?`, detail: "Permanently delete document and all chunks.", confirmLabel: "Delete Document", onConfirm: async () => { setConfirm({ open: false, title: "", message: "", detail: "", confirmLabel: "", onConfirm: () => {} }); try { await DeleteDocument(docId); setIdocs(p => p.filter(d => d.id !== docId)); setCols(p => p.map(c => c.id === activeColId ? { ...c, docCount: Math.max(0, c.docCount - 1) } : c)); addToast("success", "Deleted"); } catch (e) { addToast("error", getErrMsg(e)); } } }); };

  const doSearch = async () => { if (!sq.trim()) return; setSBusy(true); setSDone(true); try { const r: any = await Search(sq, 0); setSResults(r || []); } catch (e) { console.error(e); setSResults([]); } setSBusy(false); };
  const clearSearch = () => { setSq(""); setSResults([]); setSDone(false); setSearchFilter("all"); };
  const displayScore = (score: number) => Math.max(0, Math.min(score * 100, 100)).toFixed(1);
  const filteredResults = sDone ? sResults.filter(r => { if (searchFilter === "all") return true; if (searchFilter === "keyword") return r.searchType === "keyword"; if (searchFilter === "vector") return r.searchType === "vector"; if (searchFilter === "hybrid") return r.searchType === "hybrid"; return true; }) : [];

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
      <Sidebar chats={chats} activeChatId={activeChatId} tab={tab} sidebarOpen={sidebarOpen} isIngesting={isIngesting} theme={theme}
        onNewChat={newChat} onSelectChat={(id) => { setActiveChatId(id); setTab("chat"); }}
        onTabChange={setTab} onToggleSidebar={() => setSidebarOpen(!sidebarOpen)}
        onCtxMenu={(id, x, y) => { setCtxMenuChatId(id); setCtxMenuPos({ x, y }); }} />

      {tab === "chat" && <ChatPanel activeChat={activeChat} isArchived={isArchived} input={input} gen={gen} statusMsgs={statusMsgs} T={T} theme={theme} collSelector={collSelector}
        onInputChange={setInput} onSend={send} onThemeToggle={() => setTheme(theme === "dark" ? "light" : "dark")} onOpenUploadModal={() => setShowUploadModal(true)} />}

      {tab === "search" && <SearchPanel sq={sq} sResults={sResults} sBusy={sBusy} sDone={sDone} searchFilter={searchFilter} filteredResults={filteredResults} T={T} displayScore={displayScore}
        onSearch={doSearch} onClear={clearSearch} onSqChange={setSq} onFilterChange={setSearchFilter} />}

      {tab === "cols" && <CollectionsPanel cols={cols} activeColId={activeColId} idocs={idocs} selectedDocId={selectedDocId} selectedDocContent={selectedDocContent} T={T}
        onSelectCol={setActiveColId} onDeleteCol={confirmDeleteCollection} onDeleteDoc={confirmDeleteDocument} onViewDoc={viewDocumentContent} onRefresh={() => loadDocs(activeColId)}
        newColName={newColName} onNewColNameChange={setNewColName} onCreateCol={createCol} onOpenUploadModal={() => setShowUploadModal(true)} />}

      {/* Context Menu */}
      {ctxMenuChatId !== null && (() => { const chat = chats.find(c => c.id === ctxMenuChatId); if (!chat) return null; const a = chat.archived; return (
        <div ref={ctxRef} style={{ position: "fixed", zIndex: 999, left: ctxMenuPos.x, top: ctxMenuPos.y, background: T.bg2, border: "1px solid "+T.border, borderRadius: 10, boxShadow: "0 12px 40px rgba(0,0,0,0.2)", padding: "4px", minWidth: 170 }}>
          {!a && ctxMenuItem("Rename", <I.Rename />, () => openRename(ctxMenuChatId!), T)}
          {ctxMenuItem("Delete", <I.Trash />, () => { confirmDeleteChat(ctxMenuChatId!); }, T, true)}
          {a ? ctxMenuItem("Unarchive", <I.Unarchive />, () => handleUnarchiveChat(ctxMenuChatId!), T) : ctxMenuItem("Archive", <I.Archive />, () => handleArchiveChat(ctxMenuChatId!), T)}
          {!a && ctxMenuItem(chat.pinned ? "Unpin" : "Pin", <I.Pin />, () => handlePinChat(ctxMenuChatId!), T)}
        </div>); })()}

      {/* Upload Modal */}
      <FileUploadModal open={showUploadModal} onClose={() => { setShowUploadModal(false); loadCols(); loadDocs(activeColId); }} collectionId={activeColId} collectionName={activeCol?.name || "Unknown"} onUpload={processFile} onIngestPaste={handleIngestPaste} theme={theme} />

      {/* Rename Modal */}
      <Modal open={renameModal.open} onClose={() => setRenameModal({ open: false, chatId: 0, value: "" })} title="Rename Chat" theme={theme}>
        <input value={renameModal.value} onChange={e => setRenameModal(p => ({ ...p, value: e.target.value }))} onKeyDown={e => e.key === "Enter" && submitRename()} autoFocus style={{ width: "100%", padding: "10px 14px", borderRadius: 8, border: "1px solid "+T.border, background: T.inputBg, color: T.text, fontSize: 13, outline: "none", marginBottom: 12 }} />
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <button onClick={() => setRenameModal({ open: false, chatId: 0, value: "" })} style={{ padding: "8px 16px", borderRadius: 8, border: "1px solid "+T.border, cursor: "pointer", fontSize: 13, color: T.text2, background: "transparent" }}>Cancel</button>
          <button onClick={submitRename} style={{ padding: "8px 16px", borderRadius: 8, border: "none", cursor: "pointer", fontSize: 13, fontWeight: 500, color: "#fff", background: "rgba(99,102,241,0.8)" }}>Rename</button>
        </div>
      </Modal>

      <ConfirmModal open={confirm.open} title={confirm.title} message={confirm.message} detail={confirm.detail} confirmLabel={confirm.confirmLabel} onConfirm={confirm.onConfirm} onCancel={() => setConfirm({ open: false, title: "", message: "", detail: "", confirmLabel: "", onConfirm: () => {} })} theme={theme} />
      <Toast toasts={toasts} onDismiss={dismissToast} theme={theme} />
    </div>
  </>);
}

function ctxMenuItem(label: string, icon: React.ReactNode, onClick: () => void, T: any, isDanger?: boolean) {
  return (
    <div onClick={onClick} style={{ display: "flex", alignItems: "center", gap: 10, padding: "8px 12px", borderRadius: 6, cursor: "pointer", fontSize: 13, color: isDanger ? "rgba(239,68,68,0.85)" : T.text2 }}
      onMouseEnter={e => (e.target as HTMLElement).style.background = "rgba(128,128,128,0.06)"}
      onMouseLeave={e => (e.target as HTMLElement).style.background = "transparent"}>
      {icon} {label}
    </div>
  );
}

