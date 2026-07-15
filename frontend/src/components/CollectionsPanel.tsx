import { useState, useMemo } from "react";
import type { CSSProperties } from "react";
import { Collection, DocRecord, ThemeVars, IncompleteJob, IngestLogEntry, ChunkRecord } from "../types";
import { I } from "./Icons";

type SortKey = "filename" | "chunks" | "date";
type SortDir = "asc" | "desc";

interface CollectionsPanelProps {
  cols: Collection[]; activeColId: number; activeCollection?: Collection; idocs: DocRecord[];
  selectedDocId: number|null; selectedDocContent: string; selectedDocChunks: ChunkRecord[];
  incompleteJobs: IncompleteJob[];
  ingestLogs: IngestLogEntry[];
  isIngesting: boolean;
  T: ThemeVars;
  onSelectCol: (id: number) => void; onDeleteCol: (id: number) => void;
  onDeleteDoc: (id: number) => void; onViewDoc: (id: number) => void; onInspectChunk: (chunkId: number, filename: string) => void;
  onEditCollectionProfile: () => void;
  onRefresh: () => void;
  newColName: string; onNewColNameChange: (v: string) => void; onCreateCol: () => void;
  onOpenUploadModal: () => void;
  onResumeQueue: () => void;
  onDiscardQueue: () => void;
  onCancelIngest: () => void;
}


const queueBtn = (T: ThemeVars, danger?: boolean): CSSProperties => ({
  padding: "5px 10px",
  borderRadius: 999,
  border: "1px solid " + (danger ? "rgba(239,68,68,0.25)" : T.border),
  background: danger ? "rgba(239,68,68,0.08)" : "transparent",
  color: danger ? "rgba(239,68,68,0.9)" : T.text2,
  cursor: "pointer",
  fontSize: 11,
});

const decodeHeadingPath = (raw?: string) => {
  if (!raw) return "";
  try {
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) return parsed.filter(Boolean).join(" > ");
  } catch {}
  return raw;
};

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
    <div style={{flex:1,display:"flex",flexDirection:"column",padding:20,overflow:"auto",minWidth:0,minHeight:0}}>
      <h2 style={{fontSize:18,fontWeight:600,marginBottom:16,flexShrink:0}}>Knowledge Collections</h2>
      <div style={{display:"flex",gap:8,marginBottom:12,flexWrap:"wrap",flexShrink:0}}>
        <button onClick={props.onEditCollectionProfile} style={{padding:"8px 12px",borderRadius:8,border:"1px solid rgba(99,102,241,0.35)",cursor:"pointer",fontSize:12,color:"rgba(99,102,241,0.85)",background:"transparent"}}>Collection profile</button>
      </div>
      {props.activeCollection && (
        <div style={{padding:12,borderRadius:8,border:"1px solid "+T.border,background:T.bg2,marginBottom:16,flexShrink:0}}>
          <div style={{display:"flex",justifyContent:"space-between",gap:8,alignItems:"center",marginBottom:8}}>
            <div>
              <div style={{fontSize:13,fontWeight:600}}>{props.activeCollection.name}</div>
              <div style={{fontSize:11,color:T.text3}}>{props.activeCollection.docCount} ready document{props.activeCollection.docCount === 1 ? "" : "s"}</div>
            </div>
            <div style={{display:"flex",flexDirection:"column",alignItems:"flex-end",gap:4,fontSize:11,color:T.text3}}>
              <span>{props.activeCollection.vectorBackend || "sqlite-vec"}</span>
              <span>{props.activeCollection.embeddingModel || "local embedding"}</span>
              <span>dims {props.activeCollection.embeddingDims ?? 0}</span>
            </div>
          </div>
          <div style={{display:"flex",gap:8,flexWrap:"wrap"}}>
            <span style={{padding:"2px 8px",borderRadius:999,border:"1px solid "+T.border,fontSize:10,color:T.text3}}>backend {props.activeCollection.vectorBackend || "sqlite-vec"}</span>
            <span style={{padding:"2px 8px",borderRadius:999,border:"1px solid "+T.border,fontSize:10,color:T.text3}}>model {props.activeCollection.embeddingModel || "local embedding"}</span>
            <span style={{padding:"2px 8px",borderRadius:999,border:"1px solid "+T.border,fontSize:10,color:T.text3}}>dims {props.activeCollection.embeddingDims ?? 0}</span>
          </div>
        </div>
      )}
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
      <div style={{display:"flex",gap:16,flexWrap:"wrap",alignContent:"flex-start",minHeight:0}}>
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
            :sortedDocs.map(d=>{
              const st = d.status || "ready";
              const statusColor = st === "ready" ? "rgba(34,197,94,0.75)"
                : st === "failed" ? "rgba(239,68,68,0.75)"
                : st === "embedding" || st === "queued" ? "rgba(234,179,8,0.85)"
                : T.text3;
              return (
              <div key={d.id} onClick={()=>props.onViewDoc(d.id)} style={{display:"flex",fontSize:12,padding:"6px 8px",color:T.text2,borderRadius:4,cursor:"pointer",background:props.selectedDocId===d.id?"rgba(99,102,241,0.1)":"transparent",borderBottom:"1px solid "+T.border,alignItems:"center"}}>
                <div style={{flex:3,overflow:"hidden",minWidth:0}}>
                  <div style={{overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap",color:T.text}}>{d.filename}</div>
                  <div style={{fontSize:10,color:T.text3,marginTop:1,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}}>
                    {(d.title || d.sourceType || "document").trim()}{d.wordCount ? ` · ${d.wordCount} words` : ""}{d.sourceSizeBytes ? ` · ${(d.sourceSizeBytes/1024).toFixed(d.sourceSizeBytes > 1024*1024 ? 1 : 0)} KB` : ""}
                  </div>
                  {d.summary && (
                    <div style={{fontSize:10,color:T.text3,marginTop:1,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}} title={d.summary}>
                      {d.summary}
                    </div>
                  )}
                  {st !== "ready" && (
                    <div style={{fontSize:10,color:statusColor,marginTop:1}}>
                      {st}{d.expectedChunks ? ` · ${d.chunkCount}/${d.expectedChunks}` : ""}
                    </div>
                  )}
                </div>
                <div style={{flex:1,textAlign:"center",color:T.text3}}>{d.chunkCount}</div>
                <div style={{flex:2,textAlign:"right",color:T.text3,fontSize:10}}>{new Date(d.createdAt*1000).toLocaleDateString([],{month:"short",day:"numeric",year:"numeric"})}</div>
                <button onClick={e=>{e.stopPropagation();props.onDeleteDoc(d.id)}} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(239,68,68,0.5)",padding:"2px 0 2px 6px",flexShrink:0}}><I.Trash/></button>
              </div>
            );})}
          </div>
        </div>

        {/* Right: Queue, logs, and content preview */}
        <div style={{flex:"2 1 450px",minWidth:300,display:"flex",flexDirection:"column",gap:12,overflow:"hidden"}}>
          <div style={{display:"grid",gridTemplateColumns:"repeat(auto-fit, minmax(240px, 1fr))",gap:12,flexShrink:0}}>
            <section style={{padding:12,borderRadius:8,border:"1px solid "+T.border,background:T.bg2}}>
              <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",gap:8,marginBottom:8}}>
                <div>
                  <div style={{fontSize:13,fontWeight:600}}>Ingestion Queue</div>
                  <div style={{fontSize:11,color:T.text3}}>{props.incompleteJobs.length} incomplete job{props.incompleteJobs.length === 1 ? "" : "s"}</div>
                </div>
                <div style={{display:"flex",gap:6,flexWrap:"wrap",justifyContent:"flex-end"}}>
                  {props.isIngesting && <button onClick={props.onCancelIngest} style={queueBtn(T, true)}>Cancel</button>}
                  {props.incompleteJobs.length > 0 && <button onClick={props.onResumeQueue} style={queueBtn(T)}>Resume</button>}
                  {props.incompleteJobs.length > 0 && <button onClick={props.onDiscardQueue} style={queueBtn(T, true)}>Discard</button>}
                </div>
              </div>
              <div style={{maxHeight:160,overflowY:"auto",display:"flex",flexDirection:"column",gap:8}}>
                {props.incompleteJobs.length === 0 ? (
                  <div style={{fontSize:12,color:T.text3}}>No incomplete jobs.</div>
                ) : props.incompleteJobs.slice(0, 8).map(job => (
                  <div key={job.docId} style={{padding:"8px 10px",borderRadius:6,border:"1px solid "+T.border,background:T.inputBg}}>
                    <div style={{display:"flex",justifyContent:"space-between",gap:8,alignItems:"center",fontSize:12}}>
                      <span style={{color:T.text,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}}>{job.filename}</span>
                      <span style={{color:job.status === "failed" ? "rgba(239,68,68,0.85)" : "rgba(234,179,8,0.85)",fontSize:10,textTransform:"uppercase"}}>{job.status}</span>
                    </div>
                    <div style={{marginTop:6,height:4,borderRadius:2,background:T.border,overflow:"hidden"}}>
                      <div style={{width: `${Math.max(0, Math.min(100, job.progressPct))}%`,height:"100%",background:"rgba(99,102,241,0.7)"}} />
                    </div>
                    <div style={{marginTop:6,fontSize:10,color:T.text3}}>{job.progressPct}% · {job.chunkCount}/{job.expectedChunks} chunks{job.errorMessage ? ` · ${job.errorMessage}` : ""}</div>
                  </div>
                ))}
              </div>
            </section>

            <section style={{padding:12,borderRadius:8,border:"1px solid "+T.border,background:T.bg2}}>
              <div style={{fontSize:13,fontWeight:600,marginBottom:8}}>Recent Ingest Logs</div>
              <div style={{maxHeight:160,overflowY:"auto",display:"flex",flexDirection:"column",gap:6}}>
                {props.ingestLogs.length === 0 ? (
                  <div style={{fontSize:12,color:T.text3}}>No ingest logs yet.</div>
                ) : props.ingestLogs.slice(0, 10).map(log => (
                  <div key={log.id} style={{padding:"8px 10px",borderRadius:6,border:"1px solid "+T.border,background:T.inputBg}}>
                    <div style={{display:"flex",justifyContent:"space-between",gap:8,alignItems:"center",fontSize:10,color:T.text3}}>
                      <span style={{textTransform:"uppercase",color:log.level === "error" ? "rgba(239,68,68,0.85)" : log.level === "warn" ? "rgba(234,179,8,0.85)" : T.text3}}>{log.level}</span>
                      <span>{new Date(log.timestamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</span>
                    </div>
                    <div style={{marginTop:4,fontSize:12,color:T.text}}>{log.message}</div>
                    <div style={{marginTop:4,fontSize:10,color:T.text3}}>{log.stage}{log.filename ? ` · ${log.filename}` : ""}</div>
                  </div>
                ))}
              </div>
            </section>
          </div>
          <div style={{fontSize:13,fontWeight:600,marginBottom:0,flexShrink:0}}>{props.selectedDocId?"Content Preview":"Select a document"}</div>
          <div style={{flex:0.9,overflowY:"auto",padding:"12px",borderRadius:8,border:"1px solid "+T.border,background:T.inputBg,fontSize:12,lineHeight:1.6,whiteSpace:"pre-wrap",wordBreak:"break-word",fontFamily:"monospace"}}>
            {props.selectedDocContent||(props.selectedDocId?"Loading...":"Click a document to view its extracted content.")}
          </div>

          <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",gap:8,marginTop:2,flexShrink:0}}>
            <div>
              <div style={{fontSize:13,fontWeight:600}}>Chunk Browser</div>
              <div style={{fontSize:11,color:T.text3}}>{props.selectedDocId ? selectedDocName : "No document selected"} · {props.selectedDocChunks.length} chunk{props.selectedDocChunks.length === 1 ? "" : "s"}</div>
            </div>
            {props.selectedDocId && props.selectedDocChunks.length > 0 && (
              <button onClick={() => props.onViewDoc(props.selectedDocId!)} style={queueBtn(T)}>Reload</button>
            )}
          </div>

          <div style={{flex:1.1,overflowY:"auto",display:"flex",flexDirection:"column",gap:8,paddingRight:2}}>
            {!props.selectedDocId ? (
              <div style={{fontSize:12,color:T.text3}}>Select a document to browse its chunks.</div>
            ) : props.selectedDocChunks.length === 0 ? (
              <div style={{fontSize:12,color:T.text3}}>No chunk records were found for this document.</div>
            ) : (
              props.selectedDocChunks.map(chunk => (
                <div key={chunk.id} style={{padding:"10px 12px",borderRadius:8,border:"1px solid "+T.border,background:T.bg2}}>
                  <div style={{display:"flex",justifyContent:"space-between",gap:8,flexWrap:"wrap",alignItems:"center",marginBottom:6}}>
                    <div style={{display:"flex",alignItems:"center",gap:8,flexWrap:"wrap"}}>
                      <span style={{fontSize:10,fontWeight:700,textTransform:"uppercase",color:chunk.role === "summary" ? "rgba(34,197,94,0.85)" : T.text3}}>{getChunkBadge(chunk)}</span>
                      <span style={{fontSize:11,color:T.text3}}>ord {chunk.ord}</span>
                      <span style={{fontSize:11,color:T.text3}}>level {chunk.level}</span>
                      <span style={{fontSize:11,color:T.text3}}>#{chunk.id}</span>
                    </div>
                    <button onClick={() => props.onInspectChunk(chunk.id, selectedDocName)} style={{padding:"4px 10px",borderRadius:999,border:"1px solid "+T.border,background:"transparent",color:T.text2,cursor:"pointer",fontSize:11}}>Inspect context</button>
                  </div>
                  {chunk.headingPath ? (
                    <div style={{fontSize:11,color:T.text3,marginBottom:6}}>
                      <strong>Heading:</strong> {decodeHeadingPath(chunk.headingPath)}
                    </div>
                  ) : null}
                  {chunk.summary ? (
                    <div style={{fontSize:12,color:T.text2,marginBottom:6}}>
                      <strong>Summary:</strong> {chunk.summary}
                    </div>
                  ) : null}
                  <div style={{fontSize:12,color:T.text,whiteSpace:"pre-wrap",wordBreak:"break-word",fontFamily:"monospace",lineHeight:1.5}}>
                    {summarizeChunk(chunk.content)}
                  </div>
                  <div style={{display:"flex",gap:10,flexWrap:"wrap",marginTop:8,fontSize:10,color:T.text3}}>
                    <span>parent {chunk.parentOrd >= 0 ? chunk.parentOrd : "—"}</span>
                    <span>prev {chunk.prevOrd >= 0 ? chunk.prevOrd : "—"}</span>
                    <span>next {chunk.nextOrd >= 0 ? chunk.nextOrd : "—"}</span>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
