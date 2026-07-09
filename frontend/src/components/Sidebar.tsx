import React from "react";
import { Chat, Theme, ThemeVars, themeVars } from "../types";
import { I } from "./Icons";
import { ChatItem } from "./ChatItem";

interface SidebarProps {
  chats: Chat[]; activeChatId: number; tab: string; sidebarOpen: boolean;
  isIngesting: boolean; theme: Theme;
  onNewChat: () => void; onSelectChat: (id: number) => void;
  onTabChange: (tab: "chat"|"search"|"cols") => void;
  onToggleSidebar: () => void;
  onCtxMenu: (chatId: number, x: number, y: number) => void;
}

export function Sidebar({chats,activeChatId,tab,sidebarOpen,isIngesting,theme,onNewChat,onSelectChat,onTabChange,onToggleSidebar,onCtxMenu}:SidebarProps){
  const T=themeVars[theme];
  const now=Date.now(),day=86400000;
  const pinnedChats=chats.filter(c=>c.pinned&&!c.archived);
  const unpinned=chats.filter(c=>!c.pinned&&!c.archived);
  const archivedChats=chats.filter(c=>c.archived);

  if(!sidebarOpen)return(
    <div style={{width:48,overflow:"hidden",background:T.bg2,borderRight:"1px solid "+T.border,display:"flex",flexDirection:"column",alignItems:"center",height:"100%",transition:"width 0.2s",flexShrink:0,padding:"12px 0",gap:12}}>
      <button onClick={onToggleSidebar} style={{background:"none",border:"none",cursor:"pointer",color:T.text3,padding:6}} title="Expand"><I.Logo/></button>
      <button onClick={onNewChat} style={{background:"none",border:"none",cursor:"pointer",color:T.text3,padding:6}} title="New Chat"><I.Plus/></button>
      <button onClick={()=>onTabChange("search")} style={{background:"none",border:"none",cursor:"pointer",color:tab==="search"?"rgba(99,102,241,0.8)":T.text3,padding:6}} title="Search"><I.SearchS/></button>
      <button onClick={()=>{onTabChange("cols")}} style={{background:"none",border:"none",cursor:"pointer",color:tab==="cols"?"rgba(99,102,241,0.8)":T.text3,padding:6}} title="Collections">{isIngesting?<I.Spinner/>:<I.Lib/>}</button>
    </div>
  );

  return(
    <div style={{width:240,overflow:"hidden",background:T.bg2,borderRight:"1px solid "+T.border,display:"flex",flexDirection:"column",height:"100%",transition:"width 0.2s,background 0.3s",flexShrink:0}}>
      <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",padding:"16px 14px 12px",borderBottom:"1px solid "+T.border}}>
        <span style={{fontSize:14,fontWeight:600,color:T.text,whiteSpace:"nowrap",display:"flex",alignItems:"center",gap:8}}><I.Logo/> LocalRAG</span>
        <button onClick={onToggleSidebar} style={{background:"none",border:"none",cursor:"pointer",color:T.text3,padding:2}}><I.X/></button>
      </div>
      <div style={{padding:"10px 10px 6px",display:"flex",flexDirection:"column",gap:4}}>
        <button onClick={onNewChat} style={navBtnStyle(T)}><I.Plus/> New Chat</button>
        <button onClick={()=>onTabChange("search")} style={{...navBtnStyle(T),background:tab==="search"?"rgba(99,102,241,0.1)":"transparent"}}><I.SearchS/> Search</button>
        <button onClick={()=>onTabChange("cols")} style={{...navBtnStyle(T),background:tab==="cols"?"rgba(99,102,241,0.1)":"transparent",position:"relative"}}>
          {isIngesting&&<span style={{position:"absolute",left:8,top:"50%",marginTop:-7}}><I.Spinner/></span>}
          <span style={{marginLeft:isIngesting?22:0,display:"flex",alignItems:"center",gap:8}}><I.Lib/> Collections{isIngesting&&<span style={{fontSize:10,color:"rgba(99,102,241,0.7)"}}> ingesting...</span>}</span>
        </button>
      </div>
      <div style={{flex:1,overflowY:"auto",paddingTop:8}}>
        <div style={{fontSize:11,color:T.text3,textTransform:"uppercase",letterSpacing:"0.5px",marginBottom:8,paddingLeft:12,whiteSpace:"nowrap"}}>Chat History</div>
        <Group title="📌 Pinned" list={pinnedChats} T={T} activeChatId={activeChatId} onSelect={onSelectChat} onCtx={onCtxMenu}/>
        <Group title="Today" list={unpinned.filter(c=>now-c.createdAt<day)} T={T} activeChatId={activeChatId} onSelect={onSelectChat} onCtx={onCtxMenu}/>
        <Group title="Yesterday" list={unpinned.filter(c=>now-c.createdAt>=day&&now-c.createdAt<2*day)} T={T} activeChatId={activeChatId} onSelect={onSelectChat} onCtx={onCtxMenu}/>
        <Group title="Older" list={unpinned.filter(c=>now-c.createdAt>=2*day)} T={T} activeChatId={activeChatId} onSelect={onSelectChat} onCtx={onCtxMenu}/>
        {archivedChats.length>0&&<Group title="📦 Archived" list={archivedChats} T={T} activeChatId={activeChatId} onSelect={onSelectChat} onCtx={onCtxMenu}/>}
        {chats.length===0&&<div style={{fontSize:12,color:T.text3,padding:20,textAlign:"center",whiteSpace:"nowrap"}}>No chats yet.</div>}
      </div>
    </div>
  );
}

function Group({title,list,T,activeChatId,onSelect,onCtx}:{title:string;list:Chat[];T:ThemeVars;activeChatId:number;onSelect:(id:number)=>void;onCtx:(id:number,x:number,y:number)=>void}){
  if(list.length===0)return null;
  return(
    <div style={{marginBottom:16}}>
      <div style={{fontSize:11,color:T.text3,textTransform:"uppercase",letterSpacing:"0.5px",marginBottom:8,paddingLeft:12}}>{title}</div>
      {list.map(c=>(
        <ChatItem key={c.id} chat={c} isActive={c.id===activeChatId} T={T} onSelect={()=>onSelect(c.id)} onCtx={(x,y)=>onCtx(c.id,x,y)}/>
      ))}
    </div>
  );
}

const navBtnStyle=(T:ThemeVars):React.CSSProperties=>({display:"flex",alignItems:"center",gap:8,padding:"8px 12px",border:"none",borderRadius:8,cursor:"pointer",fontSize:13,color:T.text2,background:"transparent",width:"100%",textAlign:"left",transition:"background 0.2s"});
