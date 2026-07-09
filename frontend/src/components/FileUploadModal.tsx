import { useState, useRef, useEffect, ChangeEvent } from "react";
import { Events } from "@wailsio/runtime";
import { FileUploadItem } from "../types";
import { Modal } from "./Modal";
import { I } from "./Icons";

interface FileUploadModalProps {
  open: boolean;
  onClose: () => void;
  collectionId: number;
  collectionName: string;
  onUpload: (file: File, replace: boolean) => Promise<string>;
  onIngestPaste?: (filename: string, content: string) => Promise<string>;
}

export function FileUploadModal({open,onClose,collectionId,collectionName,onUpload,onIngestPaste}:FileUploadModalProps){
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

  // Listen for ingest:progress events — updates per-file progress bars with actual pct
  useEffect(() => {
    if (!uploading && !pasting) return;
    const off = Events.On("ingest:progress", (e: any) => {
      if (!e.data) return;
      const pct = e.data.pct || 0;
      const step = e.data.step;
      const label = e.data.label || "";

      if (mode === "upload" && uploading) {
        setFiles(prev => {
          const processing = [...prev].reverse().find(f => f.status === "processing");
          if (!processing) return prev;
          let msg = label;
          if (step === "chunking") msg = "Extracting text...";
          else if (step === "chunked") msg = `Splitting into ${e.data.detail}`;
          else if (step === "embedding") msg = `Embedding ${e.data.detail || label}`;
          else if (step === "complete") msg = "✓ Done";
          return prev.map(f => f.id === processing.id ? { ...f, progressMsg: msg, progressPct: pct } : f);
        });
      }

      if (mode === "paste" && pasting) {
        if (step === "chunking") { setPasteLabel("Extracting text..."); setPastePct(2); }
        else if (step === "chunked") { setPasteLabel(`Splitting into ${e.data.detail}`); setPastePct(5); }
        else if (step === "embedding") { setPasteLabel(`Embedding ${e.data.detail || label}`); setPastePct(pct); }
        else if (step === "complete") { setPasteLabel("✓ Done!"); setPastePct(100); }
      }
    });
    return () => off();
  }, [uploading, pasting, mode]);

  // Reset when opened
  useEffect(() => {
    if (open) {
      setFiles([]); setPasteFilename(""); setPasteContent(""); setPasteStatus("");
      setUploading(false); setPasting(false); setPastePct(0); setPasteLabel("");
    }
  }, [open]);

  const handleSelect=(e:ChangeEvent<HTMLInputElement>)=>{
    const selected=Array.from(e.target.files||[]);
    const items:FileUploadItem[]=selected.map(f=>({id:crypto.randomUUID(),file:f,status:"pending",progressPct:0}));
    setFiles(p=>[...p,...items]);
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

  return(
    <Modal open={open} onClose={onClose} title="Add Document" wide>
      <div style={{fontSize:12,color:"rgba(255,255,255,0.5)",marginBottom:12}}>
        Target: <strong style={{color:"rgba(255,255,255,0.85)"}}>{collectionName}</strong>
      </div>

      <div style={{display:"flex",gap:4,marginBottom:12,background:"rgba(255,255,255,0.04)",borderRadius:8,padding:3}}>
        <button onClick={()=>!uploading&&!pasting&&setMode("upload")} style={{flex:1,padding:"6px",borderRadius:6,border:"none",cursor:uploading||pasting?"default":"pointer",fontSize:12,fontWeight:500,color:mode==="upload"?"#fff":"rgba(255,255,255,0.5)",background:mode==="upload"?"rgba(99,102,241,0.6)":"transparent"}}>
          <I.Paperclip/> Upload Files
        </button>
        <button onClick={()=>!uploading&&!pasting&&setMode("paste")} style={{flex:1,padding:"6px",borderRadius:6,border:"none",cursor:uploading||pasting?"default":"pointer",fontSize:12,fontWeight:500,color:mode==="paste"?"#fff":"rgba(255,255,255,0.5)",background:mode==="paste"?"rgba(99,102,241,0.6)":"transparent"}}>
          Paste Text
        </button>
      </div>

      {mode === "upload" && (
        <>
      <div style={{fontSize:11,color:"rgba(255,255,255,0.4)",marginBottom:8}}>PDF, DOCX, TXT supported</div>
      <input ref={fileRef} type="file" multiple accept=".pdf,.docx,.txt" onChange={handleSelect} style={{display:"none"}} disabled={uploading}/>
      {!uploading && (
        <div style={{padding:"24px",borderRadius:8,border:"2px dashed rgba(255,255,255,0.15)",textAlign:"center",color:"rgba(255,255,255,0.4)",fontSize:13,marginBottom:12,cursor:"pointer"}}
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
            <div key={f.id} style={{padding:"6px 0",marginBottom:4,borderBottom:"1px solid rgba(255,255,255,0.04)"}}>
              <div style={{display:"flex",alignItems:"center",gap:8,fontSize:12}}>
                <span style={{flex:1,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap",color:"rgba(255,255,255,0.85)"}}>{f.file.name}</span>
                {f.status==="pending"&&<span style={{color:"rgba(255,255,255,0.4)",flexShrink:0}}>Pending</span>}
                {f.status==="processing"&&<span style={{flexShrink:0}}><I.Spinner/></span>}
                {f.status==="success"&&<span style={{color:"rgba(34,197,94,0.8)",flexShrink:0}}>✓</span>}
                {f.status==="replaced"&&<span style={{color:"rgba(34,197,94,0.8)",flexShrink:0}}>↻</span>}
                {f.status==="duplicate"&&<span style={{color:"rgba(234,179,8,0.8)",fontSize:10,flexShrink:0}}>Duplicate</span>}
                {f.status==="error"&&<span style={{color:"rgba(239,68,68,0.8)",fontSize:10,flexShrink:0}} title={f.message}>{f.message||"Failed"}</span>}
                {f.status==="pending"&&<button onClick={()=>removeFile(f.id)} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.3)",padding:2,flexShrink:0}}><I.X/></button>}
              </div>
              {/* Per-file progress bar — only during processing, uses actual pct */}
              {f.status==="processing" && (
                <div style={{marginTop:4,marginLeft:4,display:"flex",alignItems:"center",gap:6}}>
                  <div style={{flex:1,height:4,borderRadius:2,background:"rgba(255,255,255,0.08)",overflow:"hidden"}}>
                    <div style={{width:Math.max(2, Math.min(100, f.progressPct||0))+"%",height:"100%",borderRadius:2,background:"rgba(99,102,241,0.7)",transition:"width 0.3s ease"}}/>
                  </div>
                  <span style={{fontSize:10,color:"rgba(255,255,255,0.5)"}}>{f.progressMsg||""}</span>
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
        <button onClick={uploadAll} style={{width:"100%",padding:"10px",borderRadius:8,border:"none",cursor:"pointer",fontSize:13,fontWeight:500,color:"#fff",background:"rgba(99,102,241,0.8)",display:"flex",alignItems:"center",justifyContent:"center",gap:8}}>
          Embed {pendingCount} File{pendingCount>1?'s':''}
        </button>
      )}
      {hasCompleted&&(
        <div style={{display:"flex",flexDirection:"column",gap:6}}>
          {successCount > 0 && <div style={{fontSize:12,color:"rgba(34,197,94,0.8)",textAlign:"center"}}>✓ {successCount} succeeded{errorCount>0?`, ${errorCount} failed`:''}</div>}
          <button onClick={onClose} style={{width:"100%",padding:"10px",borderRadius:8,border:"none",cursor:"pointer",fontSize:13,fontWeight:500,color:"#fff",background:errorCount===0?"rgba(34,197,94,0.8)":"rgba(99,102,241,0.8)"}}>
            {errorCount===0 ? "✓ Done" : "Close"}
          </button>
        </div>
      )}
        </>
      )}

      {mode === "paste" && onIngestPaste && (
        <div style={{display:"flex",flexDirection:"column",gap:8}}>
          <input value={pasteFilename} onChange={e=>setPasteFilename(e.target.value)} placeholder="Filename (min 3 chars)" style={{width:"100%",padding:"10px 14px",borderRadius:8,border:"1px solid rgba(255,255,255,0.1)",background:"rgba(255,255,255,0.04)",color:"#fff",fontSize:13,outline:"none"}} disabled={pasting}/>
          <textarea value={pasteContent} onChange={e=>setPasteContent(e.target.value)} placeholder="Paste document content..." style={{width:"100%",minHeight:100,padding:"10px 14px",borderRadius:8,border:"1px solid rgba(255,255,255,0.1)",background:"rgba(255,255,255,0.04)",color:"#fff",fontSize:12,outline:"none",resize:"vertical",fontFamily:"monospace"}} disabled={pasting}/>

          {/* Paste progress bar — uses actual pct from events */}
          {pasting && pasteLabel && (
            <div style={{padding:"8px 12px",borderRadius:6,background:"rgba(99,102,241,0.08)",display:"flex",alignItems:"center",gap:8}}>
              <div style={{flex:1,height:4,borderRadius:2,background:"rgba(255,255,255,0.08)",overflow:"hidden"}}>
                <div style={{width:pastePct+"%",height:"100%",borderRadius:2,background:"rgba(99,102,241,0.7)",transition:"width 0.3s ease"}}/>
              </div>
              <span style={{fontSize:10,color:"rgba(255,255,255,0.5)",whiteSpace:"nowrap"}}>{pasteLabel}</span>
            </div>
          )}

          {!pasting && !pasteStatus.startsWith("✓") && (
            <button onClick={handlePasteSubmit} disabled={!pasteContent.trim()} style={{width:"100%",padding:"10px",borderRadius:8,border:"none",cursor:"pointer",fontSize:13,fontWeight:500,color:"#fff",background:"rgba(99,102,241,0.8)",opacity:!pasteContent.trim()?0.5:1}}>
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

      <style>{`@keyframes progressPulse{0%,100%{opacity:0.6}50%{opacity:1}}`}</style>
    </Modal>
  );
}
