import { useState, useMemo } from "react";
import { Collection, DocRecord, ThemeVars } from "../types";
import { I } from "./Icons";

type SortKey = "filename" | "chunks" | "date";
type SortDir = "asc" | "desc";

interface CollectionsPanelProps {
  cols: Collection[]; activeColId: number; idocs: DocRecord[];
  selectedDocId: number|null; selectedDocContent: string;
  T: ThemeVars;
  onSelectCol: (id: number) => void; onDeleteCol: (id: number) => void;
  onDeleteDoc: (id: number) => void; onViewDoc: (id: number) => void;
  onRefresh: () => void;
  newColName: string; onNewColNameChange: (v: string) => void; onCreateCol: () => void;
  onOpenUploadModal: () => void;
}

export function CollectionsPanel(props:CollectionsPanelProps){
  const T=props.T;
  const [sortKey, setSortKey] = useState<SortKey>("date");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) setSortDir(d => d === "asc" ? "desc" : "asc");
    else { setSortKey(key); setSortDir("desc"); }
  };

  const sortedDocs = useMemo(() => {
    const docs = [...props.idocs];
    docs.sort((a, b) => {
      let cmp = 0;
      if (sortKey === "filename") cmp = a.filename.localeCompare(b.filename);
      else if (sortKey === "chunks") cmp = a.chunkCount - b.chunkCount;
      else if (sortKey === "date") cmp = a.createdAt - b.createdAt;
      return sortDir === "asc" ? cmp : -cmp;
    });
    return docs;
  }, [props.idocs, sortKey, sortDir]);

  const sortIcon = (key: SortKey) => {
    if (sortKey !== key) return " ↕";
    return sortDir === "asc" ? " ↑" : " ↓";
  };

  return(
    <div style={{flex:1,display:"flex",flexDirection:"column",padding:20,overflow:"auto",minWidth:0}}>
      <h2 style={{fontSize:18,fontWeight:600,marginBottom:16,flexShrink:0}}>Knowledge Collections</h2>
      <div style={{display:"flex",gap:8,marginBottom:16,flexWrap:"wrap",flexShrink:0}}>
        {props.cols.map(c=>(
          <div key={c.id} style={{padding:"8px 12px",borderRadius:8,cursor:"pointer",fontSize:13,background:c.id===props.activeColId?"rgba(99,102,241,0.15)":T.bg2,border:c.id===props.activeColId?"1px solid rgba(99,102,241,0.4)":"1px solid "+T.border,display:"flex",alignItems:"center",gap:8}}>
            <div onClick={()=>props.onSelectCol(c.id)} style={{flex:1}}>
              <div style={{fontWeight:600,marginBottom:2,color:T.text}}>{c.name}</div>
              <div style={{fontSize:11,color:T.text3}}>{c.docCount} Documents</div>
            </div>
            <button onClick={e=>{e.stopPropagation();props.onDeleteCol(c.id)}} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(239,68,68,0.5)",padding:2,flexShrink:0}}><I.Trash/></button>
          </div>
        ))}
      </div>
      <div style={{display:"flex",gap:8,marginBottom:16,flexShrink:0}}>
        <input value={props.newColName} onChange={e=>props.onNewColNameChange(e.target.value)} onKeyDown={e=>e.key==="Enter"&&props.onCreateCol()} placeholder="New collection..." style={{flex:1,padding:"10px 14px",borderRadius:8,border:"1px solid "+T.border,background:T.inputBg,color:T.text,fontSize:13,outline:"none"}}/>
        <button onClick={props.onCreateCol} style={{padding:"8px 16px",borderRadius:8,border:"none",cursor:"pointer",fontSize:13,fontWeight:500,color:"#fff",background:"rgba(99,102,241,0.8)"}}>+ Create</button>
      </div>

      {/* Document table + content preview */}
      <div style={{flex:1,display:"flex",gap:16,overflow:"hidden",flexWrap:"wrap",alignContent:"flex-start"}}>
        {/* Left: Sortable document table */}
        <div style={{flex:"1 1 350px",minWidth:260,display:"flex",flexDirection:"column",overflow:"hidden"}}>
          <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",marginBottom:8,flexShrink:0}}>
            <span style={{fontSize:13,fontWeight:600}}>Documents</span>
            <div style={{display:"flex",gap:4}}>
              <button onClick={props.onOpenUploadModal} style={{padding:"3px 10px",borderRadius:6,border:"1px solid rgba(99,102,241,0.4)",cursor:"pointer",fontSize:11,color:"rgba(99,102,241,0.8)",background:"transparent",display:"flex",alignItems:"center",gap:4}}><I.Paperclip/> Add</button>
              <button onClick={props.onRefresh} style={{background:"none",border:"none",cursor:"pointer",color:T.text3,padding:4}}><I.Refresh/></button>
            </div>
          </div>
          {/* Table header */}
          <div style={{display:"flex",fontSize:11,color:T.text3,borderBottom:"1px solid "+T.border,padding:"4px 8px",flexShrink:0,userSelect:"none"}}>
            <div style={{flex:3,display:"flex",alignItems:"center",gap:4,cursor:"pointer"}} onClick={()=>toggleSort("filename")}>
              Filename<span style={{fontSize:10,opacity:0.6}}>{sortIcon("filename")}</span>
            </div>
            <div style={{flex:1,textAlign:"center",cursor:"pointer"}} onClick={()=>toggleSort("chunks")}>
              Chunks<span style={{fontSize:10,opacity:0.6}}>{sortIcon("chunks")}</span>
            </div>
            <div style={{flex:2,textAlign:"right",cursor:"pointer"}} onClick={()=>toggleSort("date")}>
              Uploaded<span style={{fontSize:10,opacity:0.6}}>{sortIcon("date")}</span>
            </div>
            <div style={{width:24}}/>
          </div>
          {/* Table body */}
          <div style={{flex:1,overflowY:"auto"}}>
            {sortedDocs.length===0?<div style={{fontSize:12,color:T.text3,padding:"8px"}}>No documents.</div>
            :sortedDocs.map(d=>(
              <div key={d.id} onClick={()=>props.onViewDoc(d.id)} style={{display:"flex",fontSize:12,padding:"6px 8px",color:T.text2,borderRadius:4,cursor:"pointer",background:props.selectedDocId===d.id?"rgba(99,102,241,0.1)":"transparent",borderBottom:"1px solid "+T.border,alignItems:"center"}}>
                <div style={{flex:3,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap",color:T.text}}>{d.filename}</div>
                <div style={{flex:1,textAlign:"center",color:T.text3}}>{d.chunkCount}</div>
                <div style={{flex:2,textAlign:"right",color:T.text3,fontSize:10}}>{new Date(d.createdAt*1000).toLocaleDateString([],{month:"short",day:"numeric",year:"numeric"})}</div>
                <button onClick={e=>{e.stopPropagation();props.onDeleteDoc(d.id)}} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(239,68,68,0.5)",padding:"2px 0 2px 6px",flexShrink:0}}><I.Trash/></button>
              </div>
            ))}
          </div>
        </div>

        {/* Right: Content preview (scrolls independently) */}
        <div style={{flex:"2 1 450px",minWidth:300,display:"flex",flexDirection:"column",overflow:"hidden"}}>
          <div style={{fontSize:13,fontWeight:600,marginBottom:8,flexShrink:0}}>{props.selectedDocId?"Content Preview":"Select a document"}</div>
          <div style={{flex:1,overflowY:"auto",padding:"12px",borderRadius:8,border:"1px solid "+T.border,background:T.inputBg,fontSize:12,lineHeight:1.6,whiteSpace:"pre-wrap",wordBreak:"break-word",fontFamily:"monospace"}}>
            {props.selectedDocContent||(props.selectedDocId?"Loading...":"Click a document to view its extracted content.")}
          </div>
        </div>
      </div>
    </div>
  );
}
