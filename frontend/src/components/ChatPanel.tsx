import { useEffect, useRef, useState, useCallback } from "react";
import { Message, ThemeVars, Theme, SourceRef } from "../types";
import { I } from "./Icons";
import { Markdown } from "./Markdown";

interface ChatPanelProps {
  activeChat: { id: number; title: string; messages: Message[]; pinned?: boolean; archived?: boolean; messageSources?: Record<number, SourceRef[]> } | undefined;
  isArchived: boolean; input: string; gen: boolean;
  statusMsgs: Message[]; T: ThemeVars; theme: Theme;
  collSelector: React.ReactNode;
  onInputChange: (v: string) => void; onSend: () => void;
  onThemeToggle: () => void;
  onOpenUploadModal: () => void;
}

export function ChatPanel({activeChat,isArchived,input,gen,statusMsgs,T,theme,collSelector,onInputChange,onSend,onThemeToggle,onOpenUploadModal}:ChatPanelProps){
  const scrollRef = useRef<HTMLDivElement>(null);
  const lastUserMsgRef = useRef<HTMLDivElement>(null);
  const prevMsgCount = useRef(activeChat?.messages.length || 0);
  const [showScrollDown, setShowScrollDown] = useState(false);
  const [sourceModal, setSourceModal] = useState<{sources: SourceRef[]; refNum: number} | null>(null);
  const messagesRef = useRef<HTMLDivElement>(null);

  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setShowScrollDown(el.scrollHeight - el.scrollTop - el.clientHeight > 200);
  }, []);

  useEffect(() => {
    const el = scrollRef.current;
    const msgs = activeChat?.messages || [];
    const msgCount = msgs.length;
    if (msgCount <= prevMsgCount.current) { prevMsgCount.current = msgCount; return; }
    prevMsgCount.current = msgCount;
    requestAnimationFrame(() => {
      if (!lastUserMsgRef.current || !el) return;
      el.scrollTo({ top: Math.max(0, lastUserMsgRef.current.offsetTop - el.clientHeight * 0.5), behavior: "smooth" });
    });
  }, [activeChat?.messages]);

  // Handle clicks on source reference badges within markdown content
  const handleMessagesClick = useCallback((e: React.MouseEvent) => {
    const target = e.target as HTMLElement;
    const refBadge = target.closest("[data-ref]");
    if (!refBadge) return;
    const refNum = parseInt(refBadge.getAttribute("data-ref") || "0");
    if (!refNum) return;
    // Find which message this belongs to by walking up
    const msgDiv = target.closest("[data-msg-id]");
    if (!msgDiv) return;
    const msgId = parseInt(msgDiv.getAttribute("data-msg-id") || "0");
    if (!msgId || !activeChat?.messageSources) return;
    const sources = activeChat.messageSources[msgId];
    if (sources) setSourceModal({sources, refNum});
  }, [activeChat?.messageSources]);

  return(
    <div style={{flex:1,display:"flex",flexDirection:"column",height:"100%",minWidth:0}}>
      <div style={{padding:"12px 20px",borderBottom:"1px solid "+T.border,display:"flex",justifyContent:"space-between",alignItems:"center"}}>
        <div style={{minWidth:0}}>
          <div style={{fontSize:15,fontWeight:600,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}}>
            {activeChat?.title||"New Chat"}{activeChat?.pinned&&" 📌"}{isArchived&&<span style={{marginLeft:8,fontSize:11,color:T.text3,background:"rgba(128,128,128,0.1)",padding:"2px 8px",borderRadius:4}}>Archived</span>}
          </div>
          <div style={{fontSize:11,color:T.text3,marginTop:2}}>Collection: {collSelector}</div>
        </div>
        <button onClick={onThemeToggle} style={{background:"none",border:"none",cursor:"pointer",color:T.text3,padding:6}} title="Toggle theme">{theme==="dark"?<I.Sun/>:<I.Moon/>}</button>
      </div>

      <div ref={scrollRef} onScroll={handleScroll} style={{flex:1,overflowY:"auto",padding:"16px 20px",display:"flex",flexDirection:"column",position:"relative"}}>
        {(!activeChat||activeChat.messages.length===0)&&statusMsgs.length===0?(
          isArchived?(<div style={{display:"flex",flexDirection:"column",alignItems:"center",justifyContent:"center",flex:1,color:T.text3}}><h2 style={{fontSize:20,fontWeight:300,marginBottom:8}}>📦 Archived</h2><p style={{fontSize:13,textAlign:"center",maxWidth:300}}>Use ⋯ to unarchive.</p></div>)
          :(<div style={{display:"flex",flexDirection:"column",alignItems:"center",justifyContent:"center",flex:1,color:T.text3}}><h2 style={{fontSize:24,fontWeight:300,marginBottom:8}}>Ask Knowledge Base</h2><p style={{fontSize:13}}>Type a query or upload a document.</p></div>)
        ):<div ref={messagesRef} onClick={handleMessagesClick}>{activeChat?.messages.map((m,idx)=>{
          const isLastUser = m.sender==="user" && idx === activeChat.messages.length - 1;
          const msgId = parseInt(m.id);
          const msgSources = activeChat.messageSources?.[msgId];
          const hasSources = msgSources && msgSources.length > 0;
          const processed = hasSources ? insertSourceBadges(m.text, msgSources) : m.text;
          return (
          <div key={m.id} ref={isLastUser ? lastUserMsgRef : undefined} data-msg-id={msgId} style={{marginBottom:16,display:"flex",flexDirection:"column",alignItems:m.sender==="user"?"flex-end":"flex-start"}}>
            <div style={{fontSize:11,color:T.text3,marginBottom:4}}>{m.sender==="user"?"You":"LocalRAG AI"}</div>
            <div style={{maxWidth:"80%",padding:"10px 14px",borderRadius:12,background:m.sender==="user"?T.bubbleUser:T.bubbleAI,fontSize:13,lineHeight:1.5,wordBreak:"break-word",overflow:"hidden"}}>
              <Markdown text={processed} hasPreformattedHtml={hasSources}/>
            </div>
            {m.sender==="ai" && !gen && !hasSources && idx === activeChat.messages.length - 1 && (
              <div style={{marginTop:6,fontSize:10,color:T.text3,fontStyle:"italic"}}>
                ⚡ Answered from general knowledge
              </div>
            )}
          </div>
        )})}</div>}
        {statusMsgs.map(sm=>(
          <div key={sm.id} style={{marginBottom:12,display:"flex",flexDirection:"column",alignItems:"flex-start"}}>
            <div style={{fontSize:11,color:T.text3,marginBottom:4}}>LocalRAG AI</div>
            <StatusBubble label={sm.text}/>
          </div>
        ))}
        <div style={{flex:1,minHeight:0}}/>
        {showScrollDown && (
          <button onClick={() => scrollRef.current?.scrollTo({top: scrollRef.current.scrollHeight, behavior:"smooth"})}
            style={{position:"sticky",bottom:8,alignSelf:"center",zIndex:10,padding:"6px 14px",borderRadius:20,border:"none",cursor:"pointer",fontSize:11,fontWeight:500,color:"#fff",background:"rgba(99,102,241,0.8)",boxShadow:"0 4px 12px rgba(0,0,0,0.3)",display:"flex",alignItems:"center",gap:4}}>
            <I.Down/> Scroll to bottom
          </button>
        )}
      </div>

      {/* Source Reference Modal */}
      {sourceModal && (
        <div onClick={()=>setSourceModal(null)} style={{position:"fixed",top:0,left:0,right:0,bottom:0,zIndex:9999,display:"flex",alignItems:"center",justifyContent:"center",background:"rgba(0,0,0,0.6)",backdropFilter:"blur(4px)"}}>
          <div onClick={e=>e.stopPropagation()} style={{background:"#1e1f36",border:"1px solid rgba(255,255,255,0.1)",borderRadius:14,boxShadow:"0 20px 60px rgba(0,0,0,0.5)",padding:24,width:"90%",maxWidth:600,maxHeight:"80vh",display:"flex",flexDirection:"column"}}>
            <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",marginBottom:16,flexShrink:0}}>
              <div style={{fontSize:16,fontWeight:600,color:"rgba(255,255,255,0.9)"}}>Source [{sourceModal.refNum}]</div>
              <button onClick={()=>setSourceModal(null)} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.3)",padding:2}}><I.X/></button>
            </div>
            {(()=>{
              const src = sourceModal.sources.find(s => s.refNumber === sourceModal.refNum);
              if (!src) return <div style={{color:"rgba(255,255,255,0.5)",fontSize:13}}>Source not found.</div>;
              return (
                <div style={{overflowY:"auto",flex:1}}>
                  <div style={{display:"flex",gap:8,marginBottom:12,flexWrap:"wrap"}}>
                    <span style={{fontSize:11,padding:"3px 8px",borderRadius:4,background:"rgba(99,102,241,0.15)",color:"rgba(99,102,241,0.8)"}}>{src.filename}</span>
                    <span style={{fontSize:11,padding:"3px 8px",borderRadius:4,background:"rgba(34,197,94,0.12)",color:"rgba(34,197,94,0.8)"}}>{src.collectionName}</span>
                    <span style={{fontSize:11,padding:"3px 8px",borderRadius:4,background:"rgba(255,255,255,0.06)",color:"rgba(255,255,255,0.6)"}}>{(src.similarity * 100).toFixed(1)}% match</span>
                  </div>
                  <div style={{fontSize:12,lineHeight:1.7,color:"rgba(255,255,255,0.8)",whiteSpace:"pre-wrap",fontFamily:"monospace",padding:"12px",borderRadius:8,border:"1px solid rgba(255,255,255,0.06)",background:"rgba(255,255,255,0.03)",wordBreak:"break-word",overflowX:"auto"}}>
                    {src.content}
                  </div>
                  {sourceModal.sources.length > 1 && (
                    <div style={{display:"flex",gap:6,marginTop:12,flexWrap:"wrap"}}>
                      {sourceModal.sources.map(s => (
                        <span key={s.refNumber} onClick={()=>setSourceModal({sources: sourceModal.sources, refNum: s.refNumber})}
                          style={{fontSize:11,padding:"3px 8px",borderRadius:4,cursor:"pointer",
                            background:s.refNumber===sourceModal.refNum?"rgba(99,102,241,0.25)":"rgba(255,255,255,0.04)",
                            color:s.refNumber===sourceModal.refNum?"rgba(99,102,241,0.9)":"rgba(255,255,255,0.5)",
                            border:s.refNumber===sourceModal.refNum?"1px solid rgba(99,102,241,0.4)":"1px solid transparent"}}>
                          [{s.refNumber}] {s.filename}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              );
            })()}
          </div>
        </div>
      )}

      <div style={{padding:"12px 16px",borderTop:"1px solid "+T.border}}>
        <div style={{display:"flex",gap:8,alignItems:"center"}}>
          <button onClick={onOpenUploadModal} style={{background:"none",border:"none",cursor:"pointer",color:T.text3,padding:"6px",display:"flex",flexShrink:0}} title="Upload documents"><I.Paperclip/></button>
          <input value={input} onChange={e=>onInputChange(e.target.value)} onKeyDown={e=>e.key==="Enter"&&!e.shiftKey&&onSend()} placeholder={isArchived?"Archived...":"Ask a question..."} style={{flex:1,padding:"10px 14px",borderRadius:8,border:"1px solid "+T.border,background:T.inputBg,color:T.text,fontSize:13,outline:"none",opacity:isArchived?0.4:1,transition:"background 0.3s"}} disabled={isArchived}/>
          <button onClick={onSend} disabled={gen||isArchived} style={{padding:"8px 14px",borderRadius:8,border:"none",cursor:"pointer",fontSize:13,fontWeight:500,color:"#fff",background:"rgba(99,102,241,0.8)",opacity:(gen||isArchived)?0.5:1,display:"flex",alignItems:"center",justifyContent:"center",minWidth:36}}>{gen?<I.Spinner/>:<I.Send/>}</button>
        </div>
      </div>
    </div>
  );
}

function StatusBubble({label}:{label:string}){return(
  <div style={{display:"flex",alignItems:"center",gap:8,padding:"10px 18px",marginBottom:12,borderRadius:16,borderBottomLeftRadius:4,background:"rgba(99,102,241,0.1)",border:"1px solid rgba(99,102,241,0.15)",alignSelf:"flex-start",maxWidth:"fit-content"}}>
    <I.Spinner/><span style={{fontSize:12,color:"rgba(255,255,255,0.6)"}}>{label}</span>
  </div>
);}

// Pre-process text to replace [N] references with HTML source badges
function insertSourceBadges(text: string, sources: SourceRef[]): string {
  return text.replace(/\[(\d+)\]/g, (_m, num) => {
    const refNum = parseInt(num);
    const src = sources.find(s => s.refNumber === refNum);
    return `<sup><span data-ref="${refNum}" style="cursor:pointer;color:rgba(99,102,241,0.9);font-weight:600;font-size:11px;background:rgba(99,102,241,0.12);padding:1px 5px;border-radius:3px;margin:0 1px;display:inline-block" title="${src ? `Source: ${src.filename}` : `Reference ${refNum}`}">[${refNum}]</span></sup>`;
  });
}
