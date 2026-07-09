import { useEffect, useRef, useState, useCallback } from "react";
import { Message, ThemeVars, Theme } from "../types";
import { I } from "./Icons";

interface ChatPanelProps {
  activeChat: { id: number; title: string; messages: Message[]; pinned?: boolean; archived?: boolean } | undefined;
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

  // Check if user has scrolled up — show down arrow if so
  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const distFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    setShowScrollDown(distFromBottom > 200);
  }, []);

  // Auto-scroll on new user message: position it at 50% from top
  useEffect(() => {
    const el = scrollRef.current;
    const msgs = activeChat?.messages || [];
    const msgCount = msgs.length;

    // Only scroll on NEW messages (not on initial load)
    if (msgCount <= prevMsgCount.current) {
      prevMsgCount.current = msgCount;
      return;
    }
    prevMsgCount.current = msgCount;

    // Small delay to let the DOM render the new message
    requestAnimationFrame(() => {
      if (!lastUserMsgRef.current || !el) return;

      // Scroll so the user message is at 50% of the viewport
      const msgTop = lastUserMsgRef.current.offsetTop;
      const containerHeight = el.clientHeight;
      const targetScroll = msgTop - containerHeight * 0.5;

      el.scrollTo({ top: Math.max(0, targetScroll), behavior: "smooth" });
    });
  }, [activeChat?.messages]);

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

      {/* Scroll container */}
      <div ref={scrollRef} onScroll={handleScroll} style={{flex:1,overflowY:"auto",padding:"16px 20px",display:"flex",flexDirection:"column",position:"relative"}}>
        {(!activeChat||activeChat.messages.length===0)&&statusMsgs.length===0?(
          isArchived?(<div style={{display:"flex",flexDirection:"column",alignItems:"center",justifyContent:"center",flex:1,color:T.text3}}><h2 style={{fontSize:20,fontWeight:300,marginBottom:8}}>📦 Archived</h2><p style={{fontSize:13,textAlign:"center",maxWidth:300}}>Use ⋯ to unarchive.</p></div>)
          :(<div style={{display:"flex",flexDirection:"column",alignItems:"center",justifyContent:"center",flex:1,color:T.text3}}><h2 style={{fontSize:24,fontWeight:300,marginBottom:8}}>Ask Knowledge Base</h2><p style={{fontSize:13}}>Type a query or upload a document.</p></div>)
        ):<>{activeChat?.messages.map((m,idx)=>{
          const isLastUser = m.sender==="user" && idx === activeChat.messages.length - 1;
          return (
          <div key={m.id} ref={isLastUser ? lastUserMsgRef : undefined} style={{marginBottom:16,display:"flex",flexDirection:"column",alignItems:m.sender==="user"?"flex-end":"flex-start"}}>
            <div style={{fontSize:11,color:T.text3,marginBottom:4}}>{m.sender==="user"?"You":"LocalRAG AI"}</div>
            <div style={{maxWidth:"80%",padding:"10px 14px",borderRadius:12,background:m.sender==="user"?T.bubbleUser:T.bubbleAI,fontSize:13,lineHeight:1.5,whiteSpace:"pre-wrap",wordBreak:"break-word"}}>{m.text}</div>
          </div>
        )})}
        {/* Status messages appear AFTER user messages, where AI response goes */}
        {statusMsgs.map(sm=>(
          <div key={sm.id} style={{marginBottom:12,display:"flex",flexDirection:"column",alignItems:"flex-start"}}>
            <div style={{fontSize:11,color:T.text3,marginBottom:4}}>LocalRAG AI</div>
            <StatusBubble label={sm.text}/>
          </div>
        ))}
        {/* Spacer so content doesn't hug the bottom — fills remaining space */}
        <div style={{flex:1,minHeight:0}}/>
        </>}

        {/* Scroll-to-bottom arrow */}
        {showScrollDown && (
          <button onClick={() => scrollRef.current?.scrollTo({top: scrollRef.current.scrollHeight, behavior:"smooth"})}
            style={{position:"sticky",bottom:8,alignSelf:"center",zIndex:10,padding:"6px 14px",borderRadius:20,border:"none",cursor:"pointer",fontSize:11,fontWeight:500,color:"#fff",background:"rgba(99,102,241,0.8)",boxShadow:"0 4px 12px rgba(0,0,0,0.3)",display:"flex",alignItems:"center",gap:4}}>
            <I.Down/> Scroll to bottom
          </button>
        )}
      </div>

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
