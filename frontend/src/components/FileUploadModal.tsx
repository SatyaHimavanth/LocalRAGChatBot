import { useState, useRef, useEffect, ChangeEvent } from "react";
import { Events } from "@wailsio/runtime";
import { FileUploadItem, Theme, themeVars } from "../types";
import { Modal } from "./Modal";
import { I } from "./Icons";

interface FileUploadModalProps {
  open: boolean;
  onClose: () => void;
  collectionId: number;
  collectionName: string;
  onUpload: (file: File, replace: boolean) => Promise<string>;
  onIngestPaste?: (filename: string, content: string) => Promise<string>;
  theme: Theme;
}

export function FileUploadModal({open,onClose,collectionId,collectionName,onUpload,onIngestPaste,theme}:FileUploadModalProps){
  const [mode, setMode] = useState<"upload"|"paste">("upload");
  const [files,setFiles]=useState<FileUploadItem[]>([]);
  const [uploading,setUploading]=useState(false);
  const [pasting, setPasting] = useState(false);
  const [pasteFilename, setPasteFilename] = useState("");
  const [pasteContent, setPasteContent] = useState("");
  const [pasteStatus, setPasteStatus] = useState("");
  const [pastePct, setPastePct] = useState(0);
  const fileRef = useRef<HTMLInputElement>(null);
  const [pasteLabel, setPasteLabel] = useState("");
  const T = themeVars[theme];

  useEffect(() => {
    if (!uploading && !pasting) return;
    const off = Events.On("ingest:progress", (e: any) => {
      if (!e.data) return;
      const pct = e.data.pct || 0;
      const step = e.data.step;
      if (mode === "upload" && uploading) {
        setFiles(prev => {
          const processing = [...prev].reverse().find(f => f.status === "processing");
          if (!processing) return prev;
          let msg = step === "chunking" ? "Extracting text..." : step === "chunked" ? `Splitting into ${e.data.detail}` : step === "embedding" ? `Embedding ${e.data.detail || e.data.label}` : "✓ Done";
          return prev.map(f => f.id === processing.id ? { ...f, progressMsg: msg, progressPct: pct } : f);
        });
      }
      if (mode === "paste" && pasting) {
        if (step === "chunking") { setPasteLabel("Extracting text..."); setPastePct(2); }
        else if (step === "chunked") { setPasteLabel(`Splitting into ${e.data.detail}`); setPastePct(5); }
        else if (step === "embedding") { setPasteLabel(`Embedding ${e.data.detail || e.data.label}`); setPastePct(pct); }
        else if (step === "complete") { setPasteLabel("✓ Done!"); setPastePct(100); }
      }
    });
    return () => off();
  }, [uploading, pasting, mode]);

  useEffect(() => {
    if (open) {
      setFiles([]); setPasteFilename(""); setPasteContent(""); setPasteStatus("");
      setUploading(false); setPasting(false); setPastePct(0); setPasteLabel("");
    }
  }, [open]);

  const handleSelect=(e:ChangeEvent<HTMLInputElement>)=>{
    const selected=Array.from(e.target.files||[]);
    setFiles(p=>[...p,...selected.map(f=>({id:crypto.randomUUID(),file:f,status:"pending" as const,progressPct:0}))]);
    if(e.target)e.target.value="";
  };

  const uploadAll=async()=>{
    setUploading(true);
    for(const item of files){
      if(item.status==="pending"||item.status==="duplicate"){
        setFiles(p=>p.map(f=>f.id===item.id?{...f,status:"processing",progressMsg:"Extracting text...",progressPct:0}:f));
        try {
          const result = await onUpload(item.file, false);
          const isError = result !== "success" && result !== "duplicate" && result !== "replaced";
          setFiles(p=>p.map(f=>f.id===item.id?{...f,status:isError?"error":result as any,progressMsg:isError?result:"✓ Done",progressPct:isError?0:100}:f));
        } catch(e) {
          setFiles(p=>p.map(f=>f.id===item.id?{...f,status:"error",progressMsg:"Upload failed"}:f));
        }
      }
    }
    setUploading(false);
  };

  const removeFile=(id:string)=>setFiles(p=>p.filter(f=>f.id!==id));
  const pendingCount=files.filter(f=>f.status==="pending"||f.status==="duplicate").length;
  const errorCount=files.filter(f=>f.status==="error").length;
  const successCount=files.filter(f=>f.status==="success"||f.status==="replaced").length;
  const hasCompleted = files.length > 0 && pendingCount === 0 && !uploading;

  const handlePasteSubmit = async () => {
    if (!pasteContent.trim() || !onIngestPaste || pasting) return;
    const fn = pasteFilename.trim() || "pasted.txt";
    if (fn.length < 3) { setPasteStatus("Filename must be at least 3 characters"); return; }
    setPasting(true); setPasteStatus("Processing..."); setPastePct(0); setPasteLabel("Extracting text...");
    try {
      const result = await onIngestPaste(fn, pasteContent);
      if (result === "success") { setPasteStatus("✓ Ingested!"); setPasteLabel("✓ Done!"); setPastePct(100); setPasteFilename(""); setPasteContent(""); }
      else { setPasteStatus(result || "Failed"); setPasteLabel(""); setPastePct(0); }
    } catch(e: any) { setPasteStatus(e?.message || "Failed"); setPasteLabel(""); setPastePct(0); }
    setPasting(false);
  };

  const B = { background: T.bg2, border: "1px solid "+T.border, color: T.text, fontSize: 13, outline: "none" as const, width: "100%", padding: "10px 14px", borderRadius: 8 };

  return(
    <Modal open={open} onClose={onClose} title="Add Document" wide theme={theme}>
      <div style={{fontSize:12,color:T.text3,marginBottom:12}}>
        Target: <strong style={{color:T.text}}>{collectionName}</strong>
      </div>

      <div style={{display:"flex",gap:4,marginBottom:12,background:T.bg2,borderRadius:8,padding:3}}>
        {(["upload","paste"] as const).map(m => (
          <button key={m} onClick={()=>!uploading&&!pasting&&setMode(m)} style={{flex:1,padding:"6px",borderRadius:6,border:"none",cursor:uploading||pasting?"default":"pointer",fontSize:12,fontWeight:500,color:mode===m?"#fff":T.text3,background:mode===m?"rgba(99,102,241,0.6)":"transparent"}}>
            {m==="upload"?<><I.Paperclip/> Upload Files</>:<>Paste Text</>}
          </button>
        ))}
      </div>

      {mode === "upload" && (
        <>
      <div style={{fontSize:11,color:T.text3,marginBottom:8}}>PDF, DOCX, TXT supported</div>
      <input ref={fileRef} type="file" multiple accept=".pdf,.docx,.txt" onChange={handleSelect} style={{display:"none"}} disabled={uploading}/>
      {!uploading && (
        <div style={{padding:"24px",borderRadius:8,border:"2px dashed "+T.border,textAlign:"center",color:T.text3,fontSize:13,marginBottom:12,cursor:"pointer"}}
          onClick={()=>fileRef.current?.click()}
          onDragOver={e=>e.preventDefault()}
          onDrop={e=>{e.preventDefault();const dt=e.dataTransfer?.files;if(dt)handleSelect({target:{files:dt}} as any)}}
        >
          <I.Paperclip/><br/>Drop files here or click to browse
        </div>
      )}

      {files.length>0&&(
        <div style={{maxHeight:300,overflowY:"auto",marginBottom:8}}>
          {files.map(f=>(
            <div key={f.id} style={{padding:"6px 0",marginBottom:4,borderBottom:"1px solid "+T.border}}>
              <div style={{display:"flex",alignItems:"center",gap:8,fontSize:12}}>
                <span style={{flex:1,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap",color:T.text}}>{f.file.name}</span>
                {f.status==="pending"&&<span style={{color:T.text3,flexShrink:0}}>Pending</span>}
                {f.status==="processing"&&<span style={{flexShrink:0}}><I.Spinner/></span>}
                {f.status==="success"&&<span style={{color:"rgba(34,197,94,0.8)",flexShrink:0}}>✓</span>}
                {f.status==="replaced"&&<span style={{color:"rgba(34,197,94,0.8)",flexShrink:0}}>↻</span>}
                {f.status==="duplicate"&&<span style={{color:"rgba(234,179,8,0.8)",fontSize:10,flexShrink:0}}>Duplicate</span>}
                {f.status==="error"&&<span style={{color:"rgba(239,68,68,0.8)",fontSize:10,flexShrink:0}} title={f.message}>{f.message||"Failed"}</span>}
                {f.status==="pending"&&<button onClick={()=>removeFile(f.id)} style={{background:"none",border:"none",cursor:"pointer",color:T.text3,padding:2,flexShrink:0}}><I.X/></button>}
              </div>
              {f.status==="processing" && (
                <div style={{marginTop:4,marginLeft:4,display:"flex",alignItems:"center",gap:6}}>
                  <div style={{flex:1,height:4,borderRadius:2,background:T.border,overflow:"hidden"}}>
                    <div style={{width:Math.max(2, Math.min(100, f.progressPct||0))+"%",height:"100%",borderRadius:2,background:"rgba(99,102,241,0.7)",transition:"width 0.3s ease"}}/>
                  </div>
                  <span style={{fontSize:10,color:T.text3}}>{f.progressMsg||""}</span>
                </div>
              )}
              {f.status==="success" && f.progressMsg && (
                <div style={{marginTop:4,marginLeft:4,fontSize:10,color:"rgba(34,197,94,0.8)"}}>{f.progressMsg}</div>
              )}
            </div>
          ))}
        </div>
      )}

      {pendingCount>0&&!uploading&&(
        <button onClick={uploadAll} style={btnStyle}>Embed {pendingCount} File{pendingCount>1?'s':''}</button>
      )}
      {hasCompleted&&(
        <div style={{display:"flex",flexDirection:"column",gap:6}}>
          {successCount > 0 && <div style={{fontSize:12,color:"rgba(34,197,94,0.8)",textAlign:"center"}}>✓ {successCount} succeeded{errorCount>0?`, ${errorCount} failed`:''}</div>}
          <button onClick={onClose} style={{...btnStyle,background:errorCount===0?"rgba(34,197,94,0.8)":"rgba(99,102,241,0.8)"}}>
            {errorCount===0 ? "✓ Done" : "Close"}
          </button>
        </div>
      )}
        </>
      )}

      {mode === "paste" && onIngestPaste && (
        <div style={{display:"flex",flexDirection:"column",gap:8}}>
          <input value={pasteFilename} onChange={e=>{if (pasteStatus.startsWith("✓")) setPasteStatus(""); setPasteFilename(e.target.value);}} placeholder="Filename (min 3 chars)" style={{...B,border:"1px solid "+T.border,background:T.bg2,color:T.text,opacity:pasting?0.5:1}} disabled={pasting}/>
          {pasteFilename.trim().length > 0 && pasteFilename.trim().length < 3 && !pasting && (
            <div style={{fontSize:10,color:"rgba(239,68,68,0.7)",marginTop:-6}}>Filename must be at least 3 characters</div>
          )}
          <textarea value={pasteContent} onChange={e=>{if (pasteStatus.startsWith("✓")) setPasteStatus(""); setPasteContent(e.target.value);}} placeholder="Paste document content..." style={{...B,minHeight:100,resize:"vertical",fontFamily:"monospace",fontSize:12}} disabled={pasting}/>

          {pasting && pasteLabel && (
            <div style={{padding:"8px 12px",borderRadius:6,background:"rgba(99,102,241,0.08)",display:"flex",alignItems:"center",gap:8}}>
              <div style={{flex:1,height:4,borderRadius:2,background:T.border,overflow:"hidden"}}>
                <div style={{width:pastePct+"%",height:"100%",borderRadius:2,background:"rgba(99,102,241,0.7)",transition:"width 0.3s ease"}}/>
              </div>
              <span style={{fontSize:10,color:T.text3,whiteSpace:"nowrap"}}>{pasteLabel}</span>
            </div>
          )}

          {!pasting && !pasteStatus.startsWith("✓") && (
            <button onClick={handlePasteSubmit} disabled={!pasteContent.trim() || pasteFilename.trim().length < 3} style={{...btnStyle,opacity:(!pasteContent.trim()||pasteFilename.trim().length<3)?0.5:1}}>
              Ingest Text
            </button>
          )}
          {pasteStatus && !pasting && (
            <div style={{fontSize:12,padding:"8px",borderRadius:6,background:pasteStatus.startsWith("✓")?"rgba(34,197,94,0.1)":"rgba(239,68,68,0.1)",color:pasteStatus.startsWith("✓")?"rgba(34,197,94,0.9)":"rgba(239,68,68,0.9)",textAlign:"center"}}>
              {pasteStatus}
              {pasteStatus.startsWith("✓") && <button onClick={onClose} style={{marginLeft:8,padding:"2px 10px",borderRadius:4,border:"none",cursor:"pointer",fontSize:11,color:"#fff",background:"rgba(34,197,94,0.8)"}}>Close</button>}
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}

const btnStyle: React.CSSProperties = { width:"100%", padding:"10px", borderRadius:8, border:"none", cursor:"pointer", fontSize:13, fontWeight:500, color:"#fff", background:"rgba(99,102,241,0.8)", display:"flex", alignItems:"center", justifyContent:"center", gap:8 };
