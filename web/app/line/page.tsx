"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
type Status={state:string;source_fps:number;processed:number;dropped_saves:number;motion_pixels:number;captures:number;last_frame:string};
type Capture={id:string;time:string;path:string;pixels:number};
const api="/api/line";
export default function LineMonitor(){
 const [status,setStatus]=useState<Status|null>(null);
 const [captures,setCaptures]=useState<Capture[]>([]);
 const [selected,setSelected]=useState<string|null>(null);
 const [error,setError]=useState("");
 const [tick,setTick]=useState(0);
 useEffect(()=>{
  let active=true;
  const update=async()=>{
   try{
    const [s,e]=await Promise.all([fetch(api+"/status",{cache:"no-store"}),fetch(api+"/events",{cache:"no-store"})]);
    if(!s.ok||!e.ok)throw Error("Moteur Go indisponible sur le port 8098");
    const metrics=await s.json() as Status;
    const events=(await e.json() as Capture[]|null)??[];
    if(active){setStatus(metrics);setCaptures(events);setError("");}
   }catch(err){if(active){setStatus(null);setError(String(err));}}
  };
  update();const timer=setInterval(()=>{setTick(v=>v+1);update()},1200);
  return()=>{active=false;clearInterval(timer)};
 },[]);
 const current=captures.find(c=>c.path===selected)??captures[0];
 return <main className="line-page">
  <header><div><span className="eyebrow">VELTRYX / VISION GO</span><h1>Line Monitor</h1><p>Acquisition RTSP · détection de mouvement · galerie des passages</p></div><Link href="/">← Vision Control</Link></header>
  <div className="line-stats">
   <div><span>État du flux</span><strong className={status?.state==="READY"?"line-ready":""}>{status?.state??"OFFLINE"}</strong></div>
   <div><span>Images reçues</span><strong>{status?.source_fps.toFixed(1)??"—"} FPS</strong></div>
   <div><span>Captures session</span><strong>{status?.captures??"—"}</strong></div>
   <div><span>Sauvegardes perdues</span><strong>{status?.dropped_saves??"—"}</strong></div>
  </div>
  {error&&<p className="error">{error}. Lance le service dans le dossier vision-go.</p>}
  <div className="line-layout">
   <section className="panel"><div className="jobhead"><h2>Caméra — direct léger</h2><span className="pill">{status?.motion_pixels??0} pixels en mouvement</span></div>
    {status?<img className="line-video" src={api+"/frame.jpg?t="+tick} alt="Dernière image caméra" />:<div className="line-placeholder">En attente du moteur Go...</div>}
    <p>Aperçu actualisé environ une fois par seconde, sans seconde connexion RTSP. Horaires indicatifs, non officiels.</p>
   </section>
   <section className="panel line-gallery"><h2>Captures récentes ({captures.length})</h2>
    {current?<><img className="line-detail" src={api+"/captures"+current.path} alt={"Passage du "+current.time}/><small>{new Date(current.time).toLocaleString("fr-BE")} · {current.pixels} pixels</small></>:<p>Aucune capture pour le moment.</p>}
    <div className="line-thumbs">{captures.map(c=><button key={c.id} className={current?.id===c.id?"active":""} onClick={()=>setSelected(c.path)}><img src={api+"/captures"+c.path} alt="Capture de passage" loading="lazy"/><small>{new Date(c.time).toLocaleTimeString("fr-BE")}</small></button>)}</div>
   </section>
  </div>
  <footer>Prototype de détection de mouvement — sans IA, OCR ni chronométrage officiel.</footer>
 </main>;
}
