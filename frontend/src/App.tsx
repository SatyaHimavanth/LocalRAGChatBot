import React, { useState, useEffect, useRef, useCallback } from "react";
import { Events } from "@wailsio/runtime";
import {
	SendMessage, IngestFile, CreateCollection, GetCollections,
	CreateChat, GetChats, GetChatMessages, UpdateChatTitle,
	DeleteChat, DeleteCollection, DeleteDocument, GetDocumentsByCollection,
	ArchiveChat, UnarchiveChat, PinChat, UnpinChat, Search,
} from "../bindings/changeme/internal/app/chatservice";

// ─── Types ───
interface Message { id: string; sender: "user" | "ai" | "system"; text: string; }
interface Chat { id: number; title: string; messages: Message[]; createdAt: number; archived: boolean; pinned: boolean; }
interface Collection { id: number; name: string; docCount: number; }
interface DocRecord { id: number; collectionId: number; filename: string; createdAt: number; }
interface SearchResult { content: string; score: number; searchType: string; collectionId: number; collectionName: string; filename: string; chunkId: number; }
interface IngestProgress { step: string; label: string; pct: number; detail: string; }
interface ToastMsg { id: string; type: "success"|"error"|"info"; message: string; }

// Helper to extract readable error message from Wails errors
const getErrMsg = (e: any): string => {
	if (!e) return "Unknown error";
	if (typeof e === "string") return e;
	// Wails v3 wraps Go errors as objects with a `message` field
	// that often contains a JSON string like {"message":"...","cause":{},"kind":"RuntimeError"}
	if (e?.message) {
		const msg = e.message;
		if (typeof msg === "string") {
			try {
				const parsed = JSON.parse(msg);
				if (parsed?.message) return parsed.message;
			} catch {}
			return msg;
		}
		return String(msg);
	}
	return String(e);
};

// ─── Styles ───
const S = {
  global: `*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#06070f;color:rgba(255,255,255,0.85);overflow:hidden}
::-webkit-scrollbar{width:6px}::-webkit-scrollbar-track{background:transparent}
::-webkit-scrollbar-thumb{background:rgba(255,255,255,0.1);border-radius:3px}
input,textarea,button,select{font-family:inherit}
@keyframes spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
@keyframes slideIn{from{transform:translateX(120%);opacity:0}to{transform:translateX(0);opacity:1}}
@keyframes dotPulse{0%,100%{opacity:0.3}50%{opacity:1}}`,
  input: { width:"100%", padding:"10px 14px", borderRadius:8, border:"1px solid rgba(255,255,255,0.1)", background:"rgba(255,255,255,0.04)", color:"#fff", fontSize:13, outline:"none" },
  btn: { padding:"8px 16px", borderRadius:8, border:"none", cursor:"pointer", fontSize:13, fontWeight:500, color:"#fff", background:"rgba(99,102,241,0.8)" },
  navBtn: { display:"flex", alignItems:"center", gap:8, padding:"8px 12px", border:"none", borderRadius:8, cursor:"pointer", fontSize:13, color:"rgba(255,255,255,0.75)", background:"transparent", width:"100%", textAlign:"left" as const, transition:"background 0.2s" },
};

// ─── Icons ───
const I = {
  Plus: () => <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>,
  SearchS: () => <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>,
  Lib: () => <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>,
  Send: () => <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>,
  Down: () => <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="6 9 12 15 18 9"/></svg>,
  Dots: () => <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="19" cy="12" r="2"/></svg>,
  X: () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>,
  Menu: () => <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="18" x2="21" y2="18"/></svg>,
  Archive: () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="2" y="3" width="20" height="5" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/></svg>,
  Unarchive: () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="2" y="3" width="20" height="5" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M8 12h8"/></svg>,
  Rename: () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z"/></svg>,
  Trash: () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>,
  Pin: () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L12 2z"/></svg>,
  Spinner: () => <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" style={{animation:"spin 1s linear infinite"}}><circle cx="12" cy="12" r="10" strokeDasharray="31.4 31.4" strokeLinecap="round"/></svg>,
  Refresh: () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>,
  Warning: () => <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>,
  Check: () => <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="20 6 9 17 4 12"/></svg>,
};

// ════════════════════════════════════════
//  Toast Component
// ════════════════════════════════════════
function Toast({toasts, onDismiss}:{toasts:ToastMsg[]; onDismiss:(id:string)=>void}) {
  return (
    <div style={{position:"fixed",bottom:20,right:20,zIndex:99999,display:"flex",flexDirection:"column",gap:8,maxWidth:360}}>
      {toasts.map(t => (
        <div key={t.id} style={{animation:"slideIn 0.3s ease",padding:"12px 16px",borderRadius:10,background:t.type==="error"?"rgba(239,68,68,0.15)":"rgba(34,197,94,0.15)",border:t.type==="error"?"1px solid rgba(239,68,68,0.3)":"1px solid rgba(34,197,94,0.3)",backdropFilter:"blur(8px)",display:"flex",alignItems:"flex-start",gap:10,boxShadow:"0 8px 32px rgba(0,0,0,0.4)"}}>
          <span style={{flexShrink:0,marginTop:1,color:t.type==="error"?"rgba(239,68,68,0.9)":"rgba(34,197,94,0.9)"}}>{t.type==="error"?<I.Warning/>:<I.Check/>}</span>
          <span style={{flex:1,fontSize:13,color:"rgba(255,255,255,0.85)",lineHeight:1.4}}>{t.message}</span>
          <button onClick={()=>onDismiss(t.id)} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.3)",padding:2,flexShrink:0}}><I.X/></button>
        </div>
      ))}
    </div>
  );
}

// ════════════════════════════════════════
//  Modal Component
// ════════════════════════════════════════
function Modal({open, onClose, title, children}:{open:boolean; onClose:()=>void; title:string; children:React.ReactNode}) {
  if (!open) return null;
  return (
    <div onClick={onClose} style={{position:"fixed",top:0,left:0,right:0,bottom:0,zIndex:9999,display:"flex",alignItems:"center",justifyContent:"center",background:"rgba(0,0,0,0.6)",backdropFilter:"blur(4px)"}}>
      <div onClick={e=>e.stopPropagation()} style={{background:"#1e1f36",border:"1px solid rgba(255,255,255,0.1)",borderRadius:14,boxShadow:"0 20px 60px rgba(0,0,0,0.5)",padding:24,minWidth:360,maxWidth:420,width:"90%"}}>
        <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",marginBottom:16}}>
          <div style={{fontSize:16,fontWeight:600,color:"rgba(255,255,255,0.9)"}}>{title}</div>
          <button onClick={onClose} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.3)",padding:2}}><I.X/></button>
        </div>
        {children}
      </div>
    </div>
  );
}

// ════════════════════════════════════════
//  Confirm Modal
// ════════════════════════════════════════
function ConfirmModal({open,title,message,detail,confirmLabel,onConfirm,onCancel}:{
  open:boolean;title:string;message:string;detail:string;confirmLabel:string;onConfirm:()=>void;onCancel:()=>void;
}) {
  if(!open)return null;
  return(
    <div style={{position:"fixed",top:0,left:0,right:0,bottom:0,zIndex:9999,display:"flex",alignItems:"center",justifyContent:"center",background:"rgba(0,0,0,0.6)",backdropFilter:"blur(4px)"}} onClick={onCancel}>
      <div onClick={e=>e.stopPropagation()} style={{background:"#1e1f36",border:"1px solid rgba(255,255,255,0.1)",borderRadius:14,boxShadow:"0 20px 60px rgba(0,0,0,0.5)",padding:24,maxWidth:400,width:"90%"}}>
        <div style={{display:"flex",alignItems:"center",gap:12,marginBottom:16}}>
          <div style={{color:"rgba(239,68,68,0.8)",flexShrink:0}}><I.Warning/></div>
          <div style={{fontSize:16,fontWeight:600,color:"rgba(255,255,255,0.9)"}}>{title}</div>
        </div>
        <div style={{fontSize:13,color:"rgba(255,255,255,0.7)",lineHeight:1.5,marginBottom:8}}>{message}</div>
        <div style={{fontSize:12,color:"rgba(239,68,68,0.6)",lineHeight:1.4,marginBottom:20,padding:"8px 10px",background:"rgba(239,68,68,0.08)",borderRadius:6}}>{detail}</div>
        <div style={{display:"flex",gap:8,justifyContent:"flex-end"}}>
          <button onClick={onCancel} style={{padding:"8px 16px",borderRadius:8,border:"1px solid rgba(255,255,255,0.1)",cursor:"pointer",fontSize:13,color:"rgba(255,255,255,0.7)",background:"transparent"}}>Cancel</button>
          <button onClick={onConfirm} style={{padding:"8px 16px",borderRadius:8,border:"none",cursor:"pointer",fontSize:13,fontWeight:500,color:"#fff",background:"rgba(239,68,68,0.8)"}}>{confirmLabel}</button>
        </div>
      </div>
    </div>
  );
}

// ════════════════════════════════════════
//  Main App
// ════════════════════════════════════════
export default function App() {
  const [tab, setTab] = useState<"chat"|"search"|"cols">("chat");
  const [chats, setChats] = useState<Chat[]>([]);
  const [activeChatId, setActiveChatId] = useState(0);
  const [input, setInput] = useState("");
  const [gen, setGen] = useState(false);
  // Status messages appear in the chat flow as "system" messages
  const [statusMsgs, setStatusMsgs] = useState<Message[]>([]);
  const activeChat=chats.find(c=>c.id===activeChatId);
  const isArchived=activeChat?.archived===true;

  const createdInitialChat = useRef(false);
  const [sidebarOpen, setSidebarOpen] = useState(true);

  // Search
  const [sq, setSq]=useState(""); const [sResults,setSResults]=useState<SearchResult[]>([]);
  const [sBusy,setSBusy]=useState(false); const [sDone,setSDone]=useState(false);
  const [searchFilter,setSearchFilter]=useState("all");

  // Collections
  const [cols,setCols]=useState<Collection[]>([]);
  const [activeColId,setActiveColId]=useState(1);
  const [newColName,setNewColName]=useState("");
  const [fname,setFname]=useState(""); const [fcontent,setFcontent]=useState("");
  const [isIngesting,setIsIngesting]=useState(false);
  const [ingestProgress,setIngestProgress]=useState<IngestProgress|null>(null);
  const [idocs,setIdocs]=useState<DocRecord[]>([]);

  // Toasts
  const [toasts,setToasts]=useState<ToastMsg[]>([]);
  const addToast=useCallback((type:"success"|"error"|"info",message:string)=>{
    const id=crypto.randomUUID();
    setToasts(p=>[...p,{id,type,message}]);
    setTimeout(()=>setToasts(p=>p.filter(t=>t.id!==id)),5000);
  },[]);
  const dismissToast=(id:string)=>setToasts(p=>p.filter(t=>t.id!==id));

  // Modals
  const [confirm,setConfirm]=useState<{open:boolean;title:string;message:string;detail:string;confirmLabel:string;onConfirm:()=>void}>({open:false,title:"",message:"",detail:"",confirmLabel:"Delete",onConfirm:()=>{}});
  const [renameModal,setRenameModal]=useState<{open:boolean;chatId:number;value:string}>({open:false,chatId:0,value:""});

  // Context menu
  const [ctxMenuChatId,setCtxMenuChatId]=useState<number|null>(null);
  const [ctxMenuPos,setCtxMenuPos]=useState({x:0,y:0});
  const ctxRef=useRef<HTMLDivElement>(null);

  // Collection dropdown
  const [colDropdownOpen,setColDropdownOpen]=useState(false);
  const [colSearch,setColSearch]=useState("");
  const colDropdownRef=useRef<HTMLDivElement>(null);

  // Close menus
  useEffect(()=>{const h=(e:MouseEvent)=>{if(ctxRef.current&&!ctxRef.current.contains(e.target as Node))setCtxMenuChatId(null)};document.addEventListener("mousedown",h);return()=>document.removeEventListener("mousedown",h)},[]);
  useEffect(()=>{const h=(e:MouseEvent)=>{if(colDropdownRef.current&&!colDropdownRef.current.contains(e.target as Node))setColDropdownOpen(false)};document.addEventListener("mousedown",h);return()=>document.removeEventListener("mousedown",h)},[]);

  // Load
  useEffect(()=>{loadCols();loadChats()},[]);

  // Events — status messages now appear inside the chat as "system" messages
  useEffect(()=>{
    const offT=Events.On("chat:token",(e:any)=>{
      // Clear status messages on first token
      setStatusMsgs([]);
      const sid=e.data.sessionId;
      setChats(prev=>prev.map(c=>{
        if(c.id!==sid)return c;
        const ms=[...c.messages];
        const last=ms[ms.length-1];
        if(last&&last.sender==="ai"){
          ms[ms.length-1]={...last,text:last.text+e.data.token};
        }else{
          ms.push({id:crypto.randomUUID(),sender:"ai",text:e.data.token});
        }
        return{...c,messages:ms};
      }));
    });
    const offD=Events.On("chat:done",()=>{setGen(false);setStatusMsgs([])});
    const offStatus=Events.On("chat:status",(e:any)=>{
      // Replace the single status message with the new one
      setStatusMsgs([{id:crypto.randomUUID(),sender:"system",text:e.data.label}]);
    });
    return()=>{offT();offD();offStatus()};
  },[]);

  // Ingestion progress
  useEffect(()=>{const off=Events.On("ingest:progress",(e:any)=>{setIngestProgress(e.data);if(e.data.step==="complete"||e.data.step==="error")setIsIngesting(false)});return()=>off()},[]);

  const loadCols=async()=>{try{const c:any=await GetCollections();if(c?.length){setCols(c);setActiveColId(c[0].id)}}catch(e){console.error("loadCols",e)}};
  const loadDocs=async(colId:number)=>{try{const d:any=await GetDocumentsByCollection(colId);setIdocs(d?.length?d:[])}catch(e){setIdocs([])}};
  useEffect(()=>{if(tab==="cols")loadDocs(activeColId)},[activeColId,tab]);

  const loadChats=async()=>{
    try{
      const s:any=await GetChats();
      if(s?.length){
        const loaded:Chat[]=[];
        for(const sess of s){
          const ms:any=await GetChatMessages(sess.id);
          loaded.push({id:sess.id,title:sess.title||"New Chat",messages:(ms||[]).map((m:any)=>({id:m.id.toString(),sender:m.role==="user"?"user":"ai",text:m.content})),createdAt:sess.createdAt*1000,archived:sess.archived===true,pinned:sess.pinned===true});
        }
        setChats(loaded);
        if(loaded.length&&!activeChatId)setActiveChatId(loaded[0].id);
      }else if(!createdInitialChat.current){createdInitialChat.current=true;newChat()}
    }catch(e){console.error("loadChats",e)}
  };

  // Chat
  const newChat=async()=>{
    const empty=chats.find(c=>c.messages.length===0&&!c.archived);
    if(empty){setActiveChatId(empty.id);setTab("chat");return}
    try{const id=await CreateChat("New Chat",activeColId);setChats(p=>[{id,title:"New Chat",messages:[],createdAt:Date.now(),archived:false,pinned:false},...p]);setActiveChatId(id);setTab("chat")}catch(e){console.error("newChat",e)}
  };

  const send=async()=>{
    if(!input.trim()||gen||isArchived)return;
    let tid=activeChatId;
    if(!tid){try{tid=await CreateChat("New Chat",activeColId);setChats(p=>[{id:tid,title:"New Chat",messages:[],createdAt:Date.now(),archived:false,pinned:false},...p]);setActiveChatId(tid)}catch(e){console.error(e);return}}
    const msg=input;setInput("");setGen(true);
    // Add initial "Thinking..." status message in the chat flow
    setStatusMsgs([{id:crypto.randomUUID(),sender:"system",text:"Thinking..."}]);
    const oldChat=chats.find(c=>c.id===tid);
    const isNew=oldChat?.title==="New Chat";
    const newTitle=isNew?(msg.length>25?msg.slice(0,25)+"...":msg):oldChat?.title||"New Chat";
    if(isNew&&tid){try{await UpdateChatTitle(tid,newTitle)}catch(e){}}
    setChats(p=>p.map(c=>c.id===tid?{...c,title:newTitle,messages:[...c.messages,{id:crypto.randomUUID(),sender:"user",text:msg}]}:c));
    try{await SendMessage(tid,activeColId,msg)}catch(e){console.error(e);setGen(false);setStatusMsgs([])}
  };

  const handleDeleteChatAction=async(chatId:number)=>{setConfirm({open:false,title:"",message:"",detail:"",confirmLabel:"",onConfirm:()=>{}});setCtxMenuChatId(null);try{await DeleteChat(chatId);setChats(p=>p.filter(c=>c.id!==chatId));if(activeChatId===chatId)setActiveChatId(chats.filter(c=>c.id!==chatId)[0]?.id||0)}catch(e){console.error(e)}};
  const handleArchiveChat=async(chatId:number)=>{setCtxMenuChatId(null);try{await ArchiveChat(chatId);setChats(p=>p.map(c=>c.id===chatId?{...c,archived:true,title:"[Archived] "+c.title.replace(/^\[Archived\]\s*/,"")}:c))}catch(e){console.error(e)}};
  const handleUnarchiveChat=async(chatId:number)=>{setCtxMenuChatId(null);try{await UnarchiveChat(chatId);setChats(p=>p.map(c=>c.id===chatId?{...c,archived:false,title:c.title.replace(/^\[Archived\]\s*/,"")}:c))}catch(e){console.error(e)}};
  const handlePinChat=async(chatId:number)=>{setCtxMenuChatId(null);const chat=chats.find(c=>c.id===chatId);if(!chat)return;
    const pinned=chat.pinned;
    if(pinned){try{await UnpinChat(chatId);setChats(p=>p.map(c=>c.id===chatId?{...c,pinned:false}:c))}catch(e){console.error(e)}}
    else{try{await PinChat(chatId);setChats(p=>p.map(c=>c.id===chatId?{...c,pinned:true}:c))}catch(e){console.error(e)}}
  };

  const confirmDeleteChat=(chatId:number)=>{const chat=chats.find(c=>c.id===chatId);setConfirm({open:true,title:"Delete Chat",message:"Delete this chat?",detail:`"${chat?.title||'New Chat'}" and all messages will be permanently deleted.`,confirmLabel:"Delete Chat",onConfirm:()=>{handleDeleteChatAction(chatId)}})};

  // Collections — fixed error messages to extract just the text
  const createCol=async()=>{
    if(!newColName.trim())return;
    try{const id=await CreateCollection(newColName);setCols(p=>[...p,{id,name:newColName,docCount:0}]);setNewColName("");setActiveColId(id);addToast("success",`Collection "${newColName}" created`)}catch(e:any){addToast("error",getErrMsg(e))}
  };
  const confirmDeleteCollection=(colId:number)=>{const col=cols.find(c=>c.id===colId);setConfirm({open:true,title:"Delete Collection",message:`Delete "${col?.name||'Unknown'}"?`,detail:`Permanently delete the collection and ${col?.docCount||0} documents.`,confirmLabel:"Delete Collection",onConfirm:async()=>{setConfirm({open:false,title:"",message:"",detail:"",confirmLabel:"",onConfirm:()=>{}});try{await DeleteCollection(colId);setCols(p=>p.filter(c=>c.id!==colId));if(activeColId===colId)setActiveChatId(cols.filter(c=>c.id!==colId)[0]?.id||0);setIdocs([]);addToast("success","Collection deleted")}catch(e){addToast("error",getErrMsg(e))}}})};
  const confirmDeleteDocument=(docId:number)=>{const doc=idocs.find(d=>d.id===docId);setConfirm({open:true,title:"Delete Document",message:`Delete "${doc?.filename||'Unknown'}"?`,detail:"Permanently delete document and all chunks.",confirmLabel:"Delete Document",onConfirm:async()=>{setConfirm({open:false,title:"",message:"",detail:"",confirmLabel:"",onConfirm:()=>{}});try{await DeleteDocument(docId);setIdocs(p=>p.filter(d=>d.id!==docId));setCols(p=>p.map(c=>c.id===activeColId?{...c,docCount:Math.max(0,c.docCount-1)}:c));addToast("success","Document deleted")}catch(e){addToast("error",getErrMsg(e))}}})};

  const ingest=async(e:React.FormEvent)=>{
    e.preventDefault();
    if(!fcontent.trim())return;
    let fn=fname.trim();if(fn.length<3){addToast("error","Filename must be at least 3 characters");return}
    fn=fn.replace(/[^a-zA-Z0-9.-]/g,"_");if(!fn.includes("."))fn+=".txt";
    setIsIngesting(true);setIngestProgress({step:"chunking",label:"Starting...",pct:0,detail:""});
    try{await IngestFile(activeColId,fn,fcontent);setCols(p=>p.map(c=>c.id===activeColId?{...c,docCount:c.docCount+1}:c));setFname("");setFcontent("");loadDocs(activeColId);addToast("success",`"${fn}" ingested successfully`);setTimeout(()=>setIngestProgress(null),5000)}catch(e){setIsIngesting(false);setIngestProgress({step:"error",label:"Error: "+getErrMsg(e),pct:0,detail:""});addToast("error",getErrMsg(e))}
  };

  // Search
  const doSearch=async()=>{
    if(!sq.trim())return;
    setSBusy(true);setSDone(true);
    try{const r:any=await Search(sq,0);setSResults(r||[])}catch(e){console.error(e);setSResults([])}
    setSBusy(false)
  };
  const clearSearch=()=>{setSq("");setSResults([]);setSDone(false);setSearchFilter("all")};
  const displayScore=(score:number)=>Math.min(score*100,99.9).toFixed(1);

  // Filter search results
  const filteredResults = sDone ? sResults.filter(r => {
    if (searchFilter === "all") return true;
    if (searchFilter === "keyword") return r.searchType === "keyword";
    if (searchFilter === "vector") return r.searchType === "vector";
    if (searchFilter === "hybrid") return r.searchType === "hybrid";
    return true;
  }) : [];

  // Collection selector
  const filteredCols=cols.filter(c=>c.name.toLowerCase().includes(colSearch.toLowerCase()));
  const activeCol=cols.find(c=>c.id===activeColId);
  const collSelector=(
    <div ref={colDropdownRef} style={{position:"relative",display:"inline-block"}}>
      <div onClick={()=>setColDropdownOpen(!colDropdownOpen)} style={{display:"flex",alignItems:"center",gap:6,padding:"4px 10px",borderRadius:6,border:"1px solid rgba(255,255,255,0.12)",cursor:"pointer",fontSize:12,color:"rgba(255,255,255,0.6)",background:"rgba(255,255,255,0.04)",userSelect:"none"}}>
        <span style={{color:"rgba(255,255,255,0.85)",fontWeight:500}}>{activeCol?.name||"Select"}</span><I.Down/>
      </div>
      {colDropdownOpen&&<div style={{position:"absolute",top:"100%",left:0,marginTop:4,zIndex:100,width:220,background:"#1a1b2e",border:"1px solid rgba(255,255,255,0.1)",borderRadius:8,overflow:"hidden",boxShadow:"0 8px 24px rgba(0,0,0,0.4)"}}>
        <input value={colSearch} onChange={e=>setColSearch(e.target.value)} placeholder="Search..." autoFocus style={{width:"100%",padding:"8px 10px",border:"none",borderBottom:"1px solid rgba(255,255,255,0.06)",background:"rgba(255,255,255,0.03)",color:"#fff",fontSize:12,outline:"none"}}/>
        <div style={{maxHeight:3*44,overflowY:"auto"}}>{filteredCols.length===0?<div style={{padding:10,fontSize:12,color:"rgba(255,255,255,0.3)",textAlign:"center"}}>No collections</div>:filteredCols.map(c=><div key={c.id} onClick={()=>{setActiveColId(c.id);setColDropdownOpen(false);setColSearch("")}} style={{padding:"10px 12px",cursor:"pointer",fontSize:12,background:c.id===activeColId?"rgba(99,102,241,0.15)":"transparent",color:c.id===activeColId?"rgba(255,255,255,0.9)":"rgba(255,255,255,0.65)",borderLeft:c.id===activeColId?"3px solid rgba(99,102,241,0.8)":"3px solid transparent"}}>
          <div style={{fontWeight:500,marginBottom:2}}>{c.name}</div>
          <div style={{fontSize:10,color:"rgba(255,255,255,0.35)"}}>{c.docCount} docs</div>
        </div>)}</div>
      </div>}
    </div>
  );

  // Groups
  const now=Date.now(),day=86400000;
  const pinnedChats=chats.filter(c=>c.pinned&&!c.archived);
  const unpinnedChats=chats.filter(c=>!c.pinned&&!c.archived);
  const archivedChats=chats.filter(c=>c.archived);

  const renderGroup=(t:string,list:Chat[],icon?:string)=>list.length===0?null:(
    <div style={{marginBottom:16}}>
      <div style={{fontSize:11,color:"rgba(255,255,255,0.35)",textTransform:"uppercase",letterSpacing:"0.5px",marginBottom:8,paddingLeft:12}}>{icon||""}{t}</div>
      {list.map(c=><ChatItem key={c.id} chat={c} isActive={c.id===activeChatId} onSelect={()=>{setActiveChatId(c.id);setTab("chat")}} onCtx={(x,y)=>{setCtxMenuChatId(c.id);setCtxMenuPos({x,y})}}/>)}
    </div>
  );

  // Progress bars
  const progressBar=ingestProgress&&ingestProgress.step!=="complete"&&ingestProgress.step!=="error"?(
    <div style={{marginBottom:8,padding:"10px 14px",borderRadius:8,background:"rgba(99,102,241,0.08)",border:"1px solid rgba(99,102,241,0.15)"}}>
      <div style={{display:"flex",justifyContent:"space-between",marginBottom:6}}><span style={{fontSize:12,color:"rgba(255,255,255,0.7)"}}>{ingestProgress.label}</span><span style={{fontSize:11,color:"rgba(255,255,255,0.4)"}}>{ingestProgress.pct}%</span></div>
      <div style={{width:"100%",height:4,borderRadius:2,background:"rgba(255,255,255,0.08)",overflow:"hidden"}}><div style={{width:ingestProgress.pct+"%",height:"100%",borderRadius:2,background:"rgba(99,102,241,0.7)",transition:"width 0.3s"}}/></div>
      {ingestProgress.detail&&<div style={{fontSize:11,color:"rgba(255,255,255,0.35)",marginTop:4}}>{ingestProgress.detail}</div>}
    </div>
  ):ingestProgress?.step==="complete"?(<div style={{marginBottom:8,padding:"10px 14px",borderRadius:8,background:"rgba(34,197,94,0.08)",border:"1px solid rgba(34,197,94,0.15)"}}><span style={{fontSize:12,color:"rgba(34,197,94,0.8)"}}>✓ {ingestProgress.label}</span></div>):null;

  // Rename modal
  const openRename=(chatId:number)=>{const chat=chats.find(c=>c.id===chatId);if(!chat)return;setRenameModal({open:true,chatId,value:chat.title});setCtxMenuChatId(null)};
  const submitRename=async()=>{if(!renameModal.value.trim()){setRenameModal({open:false,chatId:0,value:""});return}try{await UpdateChatTitle(renameModal.chatId,renameModal.value.trim());setChats(p=>p.map(c=>c.id===renameModal.chatId?{...c,title:renameModal.value.trim()}:c));addToast("success","Chat renamed");setRenameModal({open:false,chatId:0,value:""})}catch(e){addToast("error","Rename failed")}};

  const sidebarW=sidebarOpen?240:48;
  return(<>
    <style>{S.global}</style>
    <div style={{display:"flex",height:"100vh",width:"100vw"}}>

      {/* ─── SIDEBAR ─── */}
      <div style={{width:sidebarW,overflow:"hidden",background:"rgba(255,255,255,0.03)",borderRight:"1px solid rgba(255,255,255,0.06)",display:"flex",flexDirection:"column",height:"100%",transition:"width 0.2s ease",flexShrink:0}}>
        {sidebarOpen?(
          <>
          <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",padding:"16px 14px 12px",borderBottom:"1px solid rgba(255,255,255,0.06)"}}>
            <span style={{fontSize:14,fontWeight:600,color:"rgba(255,255,255,0.85)",whiteSpace:"nowrap"}}>LocalRAG Chat</span>
            <button onClick={()=>setSidebarOpen(false)} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.4)",padding:2}}><I.X/></button>
          </div>
          <div style={{padding:"10px 10px 6px",display:"flex",flexDirection:"column",gap:4}}>
            <button onClick={newChat} style={S.navBtn}><I.Plus/> New Chat</button>
            <button onClick={()=>setTab("search")} style={{...S.navBtn,background:tab==="search"?"rgba(255,255,255,0.08)":"transparent"}}><I.SearchS/> Search</button>
            <button onClick={()=>{setTab("cols");loadCols();loadDocs(activeColId)}} style={{...S.navBtn,background:tab==="cols"?"rgba(255,255,255,0.08)":"transparent",position:"relative"}}>
              {isIngesting&&<span style={{position:"absolute",left:8,top:"50%",marginTop:-7}}><I.Spinner/></span>}
              <span style={{marginLeft:isIngesting?22:0,display:"flex",alignItems:"center",gap:8}}><I.Lib/> Collections{isIngesting&&<span style={{fontSize:10,color:"rgba(99,102,241,0.7)"}}> ingesting...</span>}</span>
            </button>
          </div>
          <div style={{flex:1,overflowY:"auto",paddingTop:8}}>
            <div style={{fontSize:11,color:"rgba(255,255,255,0.35)",textTransform:"uppercase",letterSpacing:"0.5px",marginBottom:8,paddingLeft:12,whiteSpace:"nowrap"}}>Chat History</div>
            {renderGroup("📌 Pinned",pinnedChats)}
            {renderGroup("Today",unpinnedChats.filter(c=>now-c.createdAt<day))}
            {renderGroup("Yesterday",unpinnedChats.filter(c=>now-c.createdAt>=day&&now-c.createdAt<2*day))}
            {renderGroup("Older",unpinnedChats.filter(c=>now-c.createdAt>=2*day))}
            {archivedChats.length>0&&renderGroup("📦 Archived",archivedChats)}
            {chats.length===0&&<div style={{fontSize:12,color:"rgba(255,255,255,0.3)",padding:20,textAlign:"center",whiteSpace:"nowrap"}}>No chats yet.</div>}
          </div>
          </>
        ):(
          <div style={{display:"flex",flexDirection:"column",alignItems:"center",padding:"12px 0",gap:12}}>
            <button onClick={()=>setSidebarOpen(true)} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.5)",padding:6}} title="Expand"><I.Menu/></button>
            <button onClick={newChat} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.45)",padding:6}} title="New Chat"><I.Plus/></button>
            <button onClick={()=>setTab("search")} style={{background:"none",border:"none",cursor:"pointer",color:tab==="search"?"rgba(99,102,241,0.8)":"rgba(255,255,255,0.45)",padding:6}} title="Search"><I.SearchS/></button>
            <button onClick={()=>{setTab("cols");loadCols()}} style={{background:"none",border:"none",cursor:"pointer",color:tab==="cols"?"rgba(99,102,241,0.8)":"rgba(255,255,255,0.45)",padding:6,position:"relative"}} title="Collections">{isIngesting?<I.Spinner/>:<I.Lib/>}</button>
          </div>
        )}
      </div>

      {/* ─── CHAT ─── */}
      {tab==="chat"&&<div style={{flex:1,display:"flex",flexDirection:"column",height:"100%",minWidth:0}}>
        <div style={{padding:"14px 20px",borderBottom:"1px solid rgba(255,255,255,0.06)",display:"flex",justifyContent:"space-between",alignItems:"center"}}>
          <div style={{minWidth:0}}>
            <div style={{fontSize:15,fontWeight:600,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}}>
              {activeChat?.title||"New Chat"}{activeChat?.pinned&&" 📌"}{isArchived&&<span style={{marginLeft:8,fontSize:11,fontWeight:400,color:"rgba(255,255,255,0.35)",background:"rgba(255,255,255,0.06)",padding:"2px 8px",borderRadius:4}}>Archived</span>}
            </div>
            <div style={{fontSize:11,color:"rgba(255,255,255,0.4)",marginTop:2}}>Collection: {collSelector}</div>
          </div>
        </div>
        <div style={{flex:1,overflowY:"auto",padding:"16px 20px",display:"flex",flexDirection:"column"}}>
          {(!activeChat||(activeChat.messages.length===0&&statusMsgs.length===0))?(
            isArchived?(<div style={{display:"flex",flexDirection:"column",alignItems:"center",justifyContent:"center",flex:1,color:"rgba(255,255,255,0.3)"}}>
              <h2 style={{fontSize:20,fontWeight:300,marginBottom:8}}>📦 Archived</h2>
              <p style={{fontSize:13,textAlign:"center",maxWidth:300}}>Use <strong>⋯</strong> in the sidebar to unarchive.</p>
            </div>):(<div style={{display:"flex",flexDirection:"column",alignItems:"center",justifyContent:"center",flex:1,color:"rgba(255,255,255,0.3)"}}>
              <h2 style={{fontSize:24,fontWeight:300,marginBottom:8}}>Ask Knowledge Base</h2>
              <p style={{fontSize:13}}>Type a query to retrieve vectorized segments.</p>
            </div>)
          ):<>{/* Render messages then status messages */}
            {activeChat?.messages.map(m=>(
              <div key={m.id} style={{marginBottom:16,display:"flex",flexDirection:"column",alignItems:m.sender==="user"?"flex-end":"flex-start"}}>
                <div style={{fontSize:11,color:"rgba(255,255,255,0.35)",marginBottom:4}}>{m.sender==="user"?"You":"LocalRAG AI"}</div>
                <div style={{maxWidth:"80%",padding:"10px 14px",borderRadius:12,background:m.sender==="user"?"rgba(99,102,241,0.2)":"rgba(255,255,255,0.06)",fontSize:13,lineHeight:1.5,whiteSpace:"pre-wrap",wordBreak:"break-word"}}>{m.text}</div>
              </div>
            ))}
            {/* Status messages appear in the chat flow where the AI will respond */}
            {statusMsgs.map(sm=>(
              <div key={sm.id} style={{marginBottom:12,display:"flex",flexDirection:"column",alignItems:"flex-start",alignSelf:"flex-start"}}>
                <div style={{fontSize:11,color:"rgba(255,255,255,0.35)",marginBottom:4}}>LocalRAG AI</div>
                <div style={{display:"flex",alignItems:"center",gap:8,padding:"10px 18px",borderRadius:16,borderBottomLeftRadius:4,background:"rgba(99,102,241,0.1)",border:"1px solid rgba(99,102,241,0.15)",fontSize:12,color:"rgba(255,255,255,0.6)",letterSpacing:"0.3px"}}>
                  <I.Spinner/>{sm.text}
                </div>
              </div>
            ))}
          </>}
        </div>
        <div style={{padding:"12px 16px",borderTop:"1px solid rgba(255,255,255,0.06)"}}>
          <div style={{display:"flex",gap:8}}>
            <input value={input} onChange={e=>setInput(e.target.value)} onKeyDown={e=>e.key==="Enter"&&!e.shiftKey&&send()} placeholder={isArchived?"Archived...":"Ask a question..."} style={{...S.input,opacity:isArchived?0.4:1}} disabled={isArchived}/>
            <button onClick={send} disabled={gen||isArchived} style={{...S.btn,opacity:(gen||isArchived)?0.5:1,padding:"8px 14px",display:"flex",alignItems:"center",justifyContent:"center",minWidth:36}}>{gen?<I.Spinner/>:<I.Send/>}</button>
          </div>
        </div>
      </div>}

      {/* ─── SEARCH ─── */}
      {tab==="search"&&<div style={{flex:1,display:"flex",flexDirection:"column",padding:20,overflow:"hidden",minWidth:0}}>
        <h2 style={{fontSize:18,fontWeight:600,marginBottom:16}}>Universal Search</h2>
        <div style={{display:"flex",gap:8,marginBottom:8}}>
          <div style={{flex:1,position:"relative"}}>
            <input value={sq} onChange={e=>setSq(e.target.value)} onKeyDown={e=>e.key==="Enter"&&doSearch()} placeholder="Search across all collections..." style={{...S.input,width:"100%",paddingRight:30}}/>
            {sq&&<button onClick={()=>setSq("")} style={{position:"absolute",right:6,top:"50%",marginTop:-8,background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.3)",padding:2}}><I.X/></button>}
          </div>
          <button onClick={doSearch} disabled={sBusy} style={S.btn}>{sBusy?"Searching...":"Search"}</button>
          {sDone&&<button onClick={clearSearch} style={{...S.btn,background:"rgba(255,255,255,0.08)"}}>Clear</button>}
        </div>
        {sDone&&sResults.length>0&&<div style={{display:"flex",gap:6,marginBottom:12,alignItems:"center"}}>
          <span style={{fontSize:11,color:"rgba(255,255,255,0.4)"}}>Filter:</span>
          {["all","keyword","vector","hybrid"].map(f=>(
            <button key={f} onClick={()=>setSearchFilter(f)} style={{padding:"3px 10px",borderRadius:12,border:searchFilter===f?"1px solid rgba(99,102,241,0.5)":"1px solid rgba(255,255,255,0.1)",cursor:"pointer",fontSize:11,color:searchFilter===f?"rgba(255,255,255,0.9)":"rgba(255,255,255,0.5)",background:searchFilter===f?"rgba(99,102,241,0.2)":"transparent",textTransform:"capitalize"}}>{f}</button>
          ))}
          <span style={{fontSize:11,color:"rgba(255,255,255,0.3)",marginLeft:8}}>{filteredResults.length} of {sResults.length} results</span>
        </div>}
        <div style={{flex:1,overflowY:"auto"}}>
          {!sDone?(<div style={{textAlign:"center",color:"rgba(255,255,255,0.3)",marginTop:40,fontSize:13}}>Enter a query and press Search.</div>
          ):filteredResults.length===0?(<div style={{textAlign:"center",color:"rgba(255,255,255,0.3)",marginTop:40,fontSize:13}}>No results for "{sq}".</div>
          ):filteredResults.map((r,i)=>(
            <div key={i} style={{padding:"12px 16px",marginBottom:8,borderRadius:8,background:"rgba(255,255,255,0.04)",border:"1px solid rgba(255,255,255,0.06)"}}>
              <div style={{display:"flex",gap:8,alignItems:"center",fontSize:11,color:"rgba(255,255,255,0.4)",marginBottom:4,flexWrap:"wrap"}}>
                <span>{r.filename}</span>
                <span style={{padding:"1px 6px",borderRadius:4,background:r.searchType==="vector"?"rgba(99,102,241,0.15)":r.searchType==="hybrid"?"rgba(34,197,94,0.15)":"rgba(255,255,255,0.06)",fontSize:10,textTransform:"uppercase",color:r.searchType==="vector"?"rgba(99,102,241,0.8)":r.searchType==="hybrid"?"rgba(34,197,94,0.8)":"rgba(255,255,255,0.5)"}}>{r.searchType}</span>
                <span><strong>{displayScore(r.score)}%</strong> match</span>
                <span>in "{r.collectionName}"</span>
              </div>
              <div style={{fontSize:13,lineHeight:1.5,color:"rgba(255,255,255,0.85)"}}>{r.content}</div>
            </div>
          ))}
        </div>
      </div>}

      {/* ─── COLLECTIONS ─── */}
      {tab==="cols"&&<div style={{flex:1,display:"flex",flexDirection:"column",padding:20,overflow:"auto",minWidth:0}}>
        <h2 style={{fontSize:18,fontWeight:600,marginBottom:16}}>Knowledge Collections</h2>
        <div style={{display:"flex",gap:8,marginBottom:16,flexWrap:"wrap"}}>
          {cols.map(c=>(
            <div key={c.id} style={{padding:"10px 14px",borderRadius:8,cursor:"pointer",fontSize:13,background:c.id===activeColId?"rgba(99,102,241,0.2)":"rgba(255,255,255,0.04)",border:c.id===activeColId?"1px solid rgba(99,102,241,0.4)":"1px solid rgba(255,255,255,0.06)",display:"flex",alignItems:"center",gap:8}}>
              <div onClick={()=>setActiveColId(c.id)} style={{flex:1}}>
                <div style={{fontWeight:600,marginBottom:2}}>{c.name}</div>
                <div style={{fontSize:11,color:"rgba(255,255,255,0.4)"}}>{c.docCount} Documents</div>
              </div>
              <button onClick={e=>{e.stopPropagation();confirmDeleteCollection(c.id)}} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(239,68,68,0.5)",padding:2,flexShrink:0}} title="Delete"><I.Trash/></button>
            </div>
          ))}
        </div>
        <div style={{display:"flex",gap:8,marginBottom:16}}>
          <input value={newColName} onChange={e=>setNewColName(e.target.value)} onKeyDown={e=>e.key==="Enter"&&createCol()} placeholder="New collection name..." style={{...S.input,flex:1}}/>
          <button onClick={createCol} style={S.btn}>+ Create</button>
        </div>
        <div style={{marginBottom:12}}>
          <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",marginBottom:8}}>
            <span style={{fontSize:13,fontWeight:600}}>Documents in "{cols.find(c=>c.id===activeColId)?.name||'...'}"</span>
            <button onClick={()=>loadDocs(activeColId)} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.4)",padding:4}} title="Refresh"><I.Refresh/></button>
          </div>
          {idocs.length===0?(<div style={{fontSize:12,color:"rgba(255,255,255,0.3)",padding:"8px 0"}}>No documents in this collection.</div>
          ):<div style={{maxHeight:200,overflowY:"auto"}}>{idocs.map(d=>(
            <div key={d.id} style={{display:"flex",justifyContent:"space-between",alignItems:"center",fontSize:12,padding:"6px 0",color:"rgba(255,255,255,0.6)",borderBottom:"1px solid rgba(255,255,255,0.04)"}}>
              <span style={{color:"rgba(255,255,255,0.85)",flex:1,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}}>{d.filename}</span>
              <button onClick={()=>confirmDeleteDocument(d.id)} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(239,68,68,0.5)",padding:"2px 6px",flexShrink:0}} title="Delete"><I.Trash/></button>
            </div>
          ))}</div>}
        </div>
        <form onSubmit={ingest} style={{flex:1,display:"flex",flexDirection:"column",gap:8}}>
          <div style={{fontSize:13,fontWeight:600}}>Vectorize & Ingest Document</div>
          <div style={{fontSize:11,color:"rgba(255,255,255,0.4)",marginBottom:4}}>Target: <strong>{cols.find(c=>c.id===activeColId)?.name||"None"}</strong></div>
          {progressBar}
          <input value={fname} onChange={e=>setFname(e.target.value)} placeholder="Filename (min 3 characters)" style={S.input}/>
          <textarea value={fcontent} onChange={e=>setFcontent(e.target.value)} placeholder="Paste document content..." style={{...S.input,flex:1,minHeight:100,resize:"vertical",fontFamily:"monospace",fontSize:12}}/>
          <button type="submit" style={S.btn}>Ingest File</button>
        </form>
      </div>}

      {/* ─── CONTEXT MENU ─── */}
      {ctxMenuChatId!==null&&(()=>{const chat=chats.find(c=>c.id===ctxMenuChatId);if(!chat)return null;const a=chat.archived;return(
        <div ref={ctxRef} style={{position:"fixed",zIndex:999,left:ctxMenuPos.x,top:ctxMenuPos.y,background:"#1e1f36",border:"1px solid rgba(255,255,255,0.1)",borderRadius:10,boxShadow:"0 12px 40px rgba(0,0,0,0.5)",padding:"4px",minWidth:170}}>
          {!a&&ctxMenuItem("Rename",<I.Rename/>,()=>openRename(ctxMenuChatId!))}
          {ctxMenuItem("Delete",<I.Trash/>,()=>{confirmDeleteChat(ctxMenuChatId!)},true)}
          {a?ctxMenuItem("Unarchive",<I.Unarchive/>,()=>handleUnarchiveChat(ctxMenuChatId!)):ctxMenuItem("Archive",<I.Archive/>,()=>handleArchiveChat(ctxMenuChatId!))}
          {!a&&ctxMenuItem(chat.pinned?"Unpin":"Pin",<I.Pin/>,()=>handlePinChat(ctxMenuChatId!))}
        </div>
      );})()}

      {/* ─── RENAME MODAL ─── */}
      <Modal open={renameModal.open} onClose={()=>setRenameModal({open:false,chatId:0,value:""})} title="Rename Chat">
        <input value={renameModal.value} onChange={e=>setRenameModal(p=>({...p,value:e.target.value}))} onKeyDown={e=>e.key==="Enter"&&submitRename()} autoFocus style={{...S.input,marginBottom:12}}/>
        <div style={{display:"flex",gap:8,justifyContent:"flex-end"}}>
          <button onClick={()=>setRenameModal({open:false,chatId:0,value:""})} style={{padding:"8px 16px",borderRadius:8,border:"1px solid rgba(255,255,255,0.1)",cursor:"pointer",fontSize:13,color:"rgba(255,255,255,0.7)",background:"transparent"}}>Cancel</button>
          <button onClick={submitRename} style={S.btn}>Rename</button>
        </div>
      </Modal>

      {/* ─── CONFIRM MODAL ─── */}
      <ConfirmModal open={confirm.open} title={confirm.title} message={confirm.message} detail={confirm.detail} confirmLabel={confirm.confirmLabel} onConfirm={confirm.onConfirm} onCancel={()=>setConfirm({open:false,title:"",message:"",detail:"",confirmLabel:"",onConfirm:()=>{}})}/>

      {/* ─── TOASTS ─── */}
      <Toast toasts={toasts} onDismiss={dismissToast}/>

    </div>
  </>);
}

// ─── Chat Item ───
function ChatItem({chat,isActive,onSelect,onCtx}:{chat:Chat;isActive:boolean;onSelect:()=>void;onCtx:(x:number,y:number)=>void}) {
  const [h,setH]=useState(false);
  return (
    <div onMouseEnter={()=>setH(true)} onMouseLeave={()=>setH(false)} onClick={onSelect} style={{padding:"6px 12px",margin:"2px 8px",borderRadius:8,cursor:"pointer",fontSize:13,color:chat.archived?"rgba(255,255,255,0.4)":"rgba(255,255,255,0.7)",background:isActive?"rgba(255,255,255,0.08)":"transparent",display:"flex",alignItems:"center",justifyContent:"space-between",transition:"background 0.15s"}}>
      <div style={{flex:1,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis",fontStyle:chat.archived?"italic":"normal"}}>{chat.archived&&"📦 "}{chat.pinned&&"📌"}{chat.title}</div>
      {h&&<button onClick={e=>{e.stopPropagation();onCtx(e.clientX,e.clientY)}} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.4)",padding:"2px",flexShrink:0}}><I.Dots/></button>}
    </div>
  );
}

function ctxMenuItem(label:string,icon:React.ReactNode,onClick:()=>void,isDanger?:boolean) {
  return (
    <div onClick={onClick} style={{display:"flex",alignItems:"center",gap:10,padding:"8px 12px",borderRadius:6,cursor:"pointer",fontSize:13,color:isDanger?"rgba(239,68,68,0.85)":"rgba(255,255,255,0.75)",transition:"background 0.1s"}}
      onMouseEnter={e=>(e.target as HTMLElement).style.background="rgba(255,255,255,0.06)"}
      onMouseLeave={e=>(e.target as HTMLElement).style.background="transparent"}
    >{icon} {label}</div>
  );
}
