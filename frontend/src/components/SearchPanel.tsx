import { SearchResult, ThemeVars } from "../types";
import { I } from "./Icons";

interface SearchPanelProps {
  sq: string; sResults: SearchResult[]; sBusy: boolean; sDone: boolean;
  searchFilter: string; filteredResults: SearchResult[]; T: ThemeVars;
  displayScore: (score: number) => string;
  onSearch: () => void; onClear: () => void; onSqChange: (v: string) => void;
  onFilterChange: (f: string) => void;
}

export function SearchPanel(props:SearchPanelProps){
  const T=props.T;
  return(
    <div style={{flex:1,display:"flex",flexDirection:"column",padding:20,overflow:"hidden",minWidth:0}}>
      <h2 style={{fontSize:18,fontWeight:600,marginBottom:16}}>Universal Search</h2>
      <div style={{display:"flex",gap:8,marginBottom:8}}>
        <div style={{flex:1,position:"relative"}}>
          <input value={props.sq} onChange={e=>props.onSqChange(e.target.value)} onKeyDown={e=>e.key==="Enter"&&props.onSearch()} placeholder="Search across all collections..." style={{width:"100%",padding:"10px 14px",paddingRight:30,borderRadius:8,border:"1px solid "+T.border,background:T.inputBg,color:T.text,fontSize:13,outline:"none"}}/>
          {props.sq&&<button onClick={()=>props.onSqChange("")} style={{position:"absolute",right:6,top:"50%",marginTop:-8,background:"none",border:"none",cursor:"pointer",color:T.text3,padding:2}}><I.X/></button>}
        </div>
        <button onClick={props.onSearch} disabled={props.sBusy} style={{padding:"8px 16px",borderRadius:8,border:"none",cursor:"pointer",fontSize:13,fontWeight:500,color:"#fff",background:"rgba(99,102,241,0.8)"}}>{props.sBusy?"Searching...":"Search"}</button>
        {props.sDone&&<button onClick={props.onClear} style={{padding:"8px 16px",borderRadius:8,border:"1px solid "+T.border,cursor:"pointer",fontSize:13,color:T.text2,background:"transparent"}}>Clear</button>}
      </div>
      {props.sDone&&props.sResults.length>0&&(
        <div style={{display:"flex",gap:6,marginBottom:12,alignItems:"center",flexWrap:"wrap"}}>
          <span style={{fontSize:11,color:T.text3}}>Filter:</span>
          {["all","keyword","vector","hybrid"].map(f=>(
            <button key={f} onClick={()=>props.onFilterChange(f)} style={{padding:"3px 10px",borderRadius:12,border:props.searchFilter===f?"1px solid rgba(99,102,241,0.5)":"1px solid rgba(128,128,128,0.15)",cursor:"pointer",fontSize:11,color:props.searchFilter===f?T.text:T.text3,background:props.searchFilter===f?"rgba(99,102,241,0.15)":"transparent",textTransform:"capitalize"}}>{f}</button>
          ))}
          <span style={{fontSize:11,color:T.text3,marginLeft:8}}>{props.filteredResults.length} of {props.sResults.length}</span>
        </div>
      )}
      <div style={{flex:1,overflowY:"auto"}}>
        {!props.sDone?(<div style={{textAlign:"center",color:T.text3,marginTop:40,fontSize:13}}>Enter a query and press Search.</div>
        ):props.filteredResults.length===0?(<div style={{textAlign:"center",color:T.text3,marginTop:40,fontSize:13}}>No results.</div>
        ):props.filteredResults.map((r,i)=>(
          <div key={i} style={{padding:"12px 16px",marginBottom:8,borderRadius:8,background:T.bg2,border:"1px solid "+T.border}}>
            <div style={{display:"flex",gap:8,alignItems:"center",fontSize:11,color:T.text3,marginBottom:4,flexWrap:"wrap"}}>
              <span>{r.filename}</span>
              <span style={{padding:"1px 6px",borderRadius:4,background:r.searchType==="vector"?"rgba(99,102,241,0.15)":r.searchType==="hybrid"?"rgba(34,197,94,0.15)":"rgba(128,128,128,0.1)",fontSize:10,textTransform:"uppercase",color:r.searchType==="vector"?"rgba(99,102,241,0.8)":r.searchType==="hybrid"?"rgba(34,197,94,0.8)":"rgba(128,128,128,0.6)"}}>{r.searchType}</span>
              <span><strong>{props.displayScore(r.score)}%</strong></span>
              <span>in "{r.collectionName}"</span>
            </div>
            <div style={{fontSize:13,lineHeight:1.5,color:T.text}}>{r.content}</div>
          </div>
        ))}
      </div>
    </div>
  );
}
