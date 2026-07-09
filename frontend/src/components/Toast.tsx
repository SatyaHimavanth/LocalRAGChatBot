import { ToastMsg, Theme, themeVars } from "../types";
import { I } from "./Icons";

export function Toast({toasts,onDismiss,theme}:{toasts:ToastMsg[];onDismiss:(id:string)=>void;theme?:Theme}){
  const T = theme ? themeVars[theme] : null;
  const textColor = T ? T.text : "rgba(255,255,255,0.85)";
  return(
  <div style={{position:"fixed",bottom:20,right:20,zIndex:99999,display:"flex",flexDirection:"column",gap:8,maxWidth:360}}>
    {toasts.map(t=>(
      <div key={t.id} style={{animation:"slideIn 0.3s ease",padding:"12px 16px",borderRadius:10,
        background:t.type==="error"?"rgba(239,68,68,0.15)":t.type==="info"?"rgba(99,102,241,0.15)":"rgba(34,197,94,0.15)",
        border:t.type==="error"?"1px solid rgba(239,68,68,0.3)":t.type==="info"?"1px solid rgba(99,102,241,0.3)":"1px solid rgba(34,197,94,0.3)",
        backdropFilter:"blur(8px)",display:"flex",alignItems:"flex-start",gap:10,boxShadow:"0 8px 32px rgba(0,0,0,0.4)",color:textColor}}>
        <span style={{flexShrink:0,marginTop:1,
          color:t.type==="error"?"rgba(239,68,68,0.9)":t.type==="info"?"rgba(99,102,241,0.9)":"rgba(34,197,94,0.9)"}}>
          {t.type==="error"?<I.Warning/>:<I.Check/>}
        </span>
        <span style={{flex:1,fontSize:13,lineHeight:1.4}}>{t.message}</span>
        <button onClick={()=>onDismiss(t.id)} style={{background:"none",border:"none",cursor:"pointer",color:T?T.text3:"rgba(128,128,128,0.5)",padding:2,flexShrink:0}}><I.X/></button>
      </div>
    ))}
  </div>
);}
