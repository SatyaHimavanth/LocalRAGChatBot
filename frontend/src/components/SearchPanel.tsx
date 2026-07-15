import { SearchResult, ThemeVars, SearchScope } from "../types";
import { I } from "./Icons";

interface SearchPanelProps {
  sq: string; sResults: SearchResult[]; sBusy: boolean; sDone: boolean;
  searchFilter: string; filteredResults: SearchResult[]; T: ThemeVars;
  searchScope: SearchScope;
  searchLimit: number;
  searchMinScore: number;
  displayScore: (score: number) => string;
  onSearch: () => void; onClear: () => void; onSqChange: (v: string) => void;
  onFilterChange: (f: string) => void;
  onScopeChange: (scope: SearchScope) => void;
  onLimitChange: (limit: number) => void;
  onMinScoreChange: (score: number) => void;
  onInspectChunk: (chunkId: number, filename: string) => void;
}

export function SearchPanel(props:SearchPanelProps){
  const T=props.T;
  const scopes: { key: SearchScope; label: string; hint: string }[] = [
    { key: "collection", label: "Collection", hint: "active collection" },
    { key: "all", label: "All", hint: "all collections" },
    { key: "workspace", label: "Workspace", hint: "documents + session sources" },
    { key: "metadata", label: "Metadata", hint: "titles / filenames / summaries" },
  ];
  return(
    <div style={{flex:1,display:"flex",flexDirection:"column",padding:20,overflow:"hidden",minWidth:0}}>
      <h2 style={{fontSize:18,fontWeight:600,marginBottom:16}}>Universal Search</h2>
      <div style={{display:"flex",gap:8,marginBottom:10,flexWrap:"wrap"}}>
        {scopes.map(scope => (
          <button key={scope.key} onClick={()=>props.onScopeChange(scope.key)} style={{padding:"6px 12px",borderRadius:999,border:props.searchScope===scope.key?"1px solid rgba(99,102,241,0.6)":"1px solid "+T.border,cursor:"pointer",fontSize:12,color:props.searchScope===scope.key?T.text:T.text3,background:props.searchScope===scope.key?"rgba(99,102,241,0.15)":"transparent",display:"flex",flexDirection:"column",alignItems:"flex-start",gap:2}}>
            <span>{scope.label}</span>
            <span style={{fontSize:10,opacity:0.75}}>{scope.hint}</span>
          </button>
        ))}
      </div>
      <div style={{display:"flex",gap:10,marginBottom:10,flexWrap:"wrap",alignItems:"center"}}>
        <label style={{display:"flex",alignItems:"center",gap:6,fontSize:11,color:T.text3}}>
          Max results
          <input type="number" min={1} max={50} value={props.searchLimit} onChange={e=>props.onLimitChange(Number(e.target.value||20))} style={{width:72,padding:"6px 8px",borderRadius:8,border:"1px solid "+T.border,background:T.inputBg,color:T.text,fontSize:12,outline:"none"}}/>
        </label>
        <label style={{display:"flex",alignItems:"center",gap:8,fontSize:11,color:T.text3}}>
          Min score {props.searchMinScore}%
          <input type="range" min={0} max={100} step={5} value={props.searchMinScore} onChange={e=>props.onMinScoreChange(Number(e.target.value))} style={{width:180}}/>
        </label>
      </div>
      <div style={{display:"flex",gap:8,marginBottom:8}}>
        <div style={{flex:1,position:"relative"}}>
          <input value={props.sq} onChange={e=>props.onSqChange(e.target.value)} onKeyDown={e=>e.key==="Enter"&&props.onSearch()} placeholder="Search across documents and workspace..." style={{width:"100%",padding:"10px 14px",paddingRight:30,borderRadius:8,border:"1px solid "+T.border,background:T.inputBg,color:T.text,fontSize:13,outline:"none"}}/>
          {props.sq&&<button onClick={()=>props.onSqChange("")} style={{position:"absolute",right:6,top:"50%",marginTop:-8,background:"none",border:"none",cursor:"pointer",color:T.text3,padding:2}}><I.X/></button>}
        </div>
        <button onClick={props.onSearch} disabled={props.sBusy} style={{padding:"8px 16px",borderRadius:8,border:"none",cursor:"pointer",fontSize:13,fontWeight:500,color:"#fff",background:"rgba(99,102,241,0.8)"}}>{props.sBusy?"Searching...":"Search"}</button>
        {props.sDone&&<button onClick={props.onClear} style={{padding:"8px 16px",borderRadius:8,border:"1px solid "+T.border,cursor:"pointer",fontSize:13,color:T.text2,background:"transparent"}}>Clear</button>}
      </div>
      {props.sDone&&props.sResults.length>0&&(
        <div style={{display:"flex",gap:6,marginBottom:12,alignItems:"center",flexWrap:"wrap"}}>
          <span style={{fontSize:11,color:T.text3}}>Filter:</span>
          {["all","keyword","vector","hybrid","metadata","workspace"].map(f=>(
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
              <span style={{padding:"1px 6px",borderRadius:4,background:r.searchType==="vector"?"rgba(99,102,241,0.15)":r.searchType==="hybrid"?"rgba(34,197,94,0.15)":r.searchType==="metadata"?"rgba(14,165,233,0.15)":r.searchType==="workspace"?"rgba(245,158,11,0.15)":"rgba(128,128,128,0.1)",fontSize:10,textTransform:"uppercase",color:r.searchType==="vector"?"rgba(99,102,241,0.8)":r.searchType==="hybrid"?"rgba(34,197,94,0.8)":r.searchType==="metadata"?"rgba(14,165,233,0.8)":r.searchType==="workspace"?"rgba(245,158,11,0.85)":"rgba(128,128,128,0.6)"}}>{r.searchType}</span>
              <span><strong>{props.displayScore(r.score)}%</strong> match</span>
              <span>in "{r.collectionName}"</span>
              <button disabled={r.chunkId <= 0} onClick={()=>r.chunkId > 0 && props.onInspectChunk(r.chunkId, r.filename)} style={{marginLeft:"auto",padding:"2px 8px",borderRadius:10,border:"1px solid "+T.border,background:"transparent",color:r.chunkId > 0 ? T.text2 : T.text3,cursor:r.chunkId > 0 ? "pointer" : "not-allowed",fontSize:10}}>Inspect context</button>
            </div>
            <div style={{fontSize:13,lineHeight:1.5,color:T.text}}>{r.content}</div>
          </div>
        ))}
      </div>
    </div>
  );
}
