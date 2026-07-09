import React from "react";
import { I } from "./Icons";

export function Modal({open,onClose,title,children,wide}:{open:boolean;onClose:()=>void;title:string;children:React.ReactNode;wide?:boolean}){if(!open)return null;return(
  <div onClick={onClose} style={{position:"fixed",top:0,left:0,right:0,bottom:0,zIndex:9999,display:"flex",alignItems:"center",justifyContent:"center",background:"rgba(0,0,0,0.6)",backdropFilter:"blur(4px)"}}>
    <div onClick={e=>e.stopPropagation()} style={{background:"#1e1f36",border:"1px solid rgba(255,255,255,0.1)",borderRadius:14,boxShadow:"0 20px 60px rgba(0,0,0,0.5)",padding:24,minWidth:wide?500:360,maxWidth:wide?700:420,width:"90%",maxHeight:"80vh",display:"flex",flexDirection:"column"}}>
      <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",marginBottom:16,flexShrink:0}}>
        <div style={{fontSize:16,fontWeight:600,color:"rgba(255,255,255,0.9)"}}>{title}</div>
        <button onClick={onClose} style={{background:"none",border:"none",cursor:"pointer",color:"rgba(255,255,255,0.3)",padding:2}}><I.X/></button>
      </div>
      <div style={{flex:1,overflowY:"auto"}}>{children}</div>
    </div>
  </div>
);}

export function ConfirmModal({open,title,message,detail,confirmLabel,onConfirm,onCancel}:{open:boolean;title:string;message:string;detail:string;confirmLabel:string;onConfirm:()=>void;onCancel:()=>void}){
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
