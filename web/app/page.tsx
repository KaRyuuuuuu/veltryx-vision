"use client";
import {useEffect,useState} from "react";
type Detection={frame:number;seconds:number;number:string;confidence:number;box:number[]};
type Job={id:string;source:string;status:string;error?:string;detections:Detection[];created_at:string};
const api="http://127.0.0.1:8088";
export default function Home(){
 const [live,setLive]=useState(false);
 const [path,setPath]=useState("");const [jobs,setJobs]=useState<Job[]>([]);const [error,setError]=useState("");
 async function refresh(){try{const r=await fetch(api+"/api/jobs");if(!r.ok)throw Error("API indisponible");setJobs(await r.json())}catch{setError("API Go indisponible sur le port 8088")}}
 useEffect(()=>{refresh();const t=setInterval(refresh,2000);return()=>clearInterval(t)},[]);
 async function launch(){setError("");try{const r=await fetch(api+"/api/jobs",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({source:path})});if(!r.ok)throw Error(await r.text());setPath("");refresh()}catch(e){setError(String(e))}}
 return <main><header><div><span className="eyebrow">VELTRYX / R&D</span><h1>Vision Control</h1><p>Détection YOLO + lecture OCR · analyse locale, non officielle</p></div><span className="pill">Development</span></header>
 <section className="panel"><div className="jobhead"><h2>Caméra en direct</h2><button onClick={()=>setLive(!live)}>{live?"Arrêter le direct":"Démarrer le direct"}</button></div><p>Aperçu local depuis le tunnel RTSP de Dragonfly (localhost:8554). Cadence limitée, détections expérimentales et OCR non validé.</p>{live&&<img className="liveframe" alt="Flux caméra annoté par YOLO et OCR" src={api+"/api/live?view=1"}/>}</section>
 <section className="panel"><h2>Nouvelle analyse</h2><p>Chemin absolu vers un fichier vidéo dans ~/Videos sur Dragonfly. Aucune caméra de production n'est sollicitée.</p><div className="form"><input aria-label="Chemin vidéo" placeholder="/home/karyuu/Videos/veltryx-vision/basler.mkv" value={path} onChange={e=>setPath(e.target.value)}/><button disabled={!path.trim()} onClick={launch}>Analyser</button></div>{error&&<div className="error">{error}</div>}</section>
 <section className="panel"><h2>Analyses <span>{jobs.length}</span></h2>{jobs.length===0?<p>Aucune analyse pour l'instant.</p>:jobs.slice().reverse().map(job=><article key={job.id}><div className="jobhead"><strong>{job.source.split("/").pop()}</strong><span className={"status "+job.status}>{job.status}</span></div><small>{new Date(job.created_at).toLocaleString("fr-BE")} · {job.detections.length} candidats OCR</small>{job.error&&<pre>{job.error}</pre>}{job.detections.length>0&&<table><thead><tr><th>Temps vidéo</th><th>Numéro candidat</th><th>Confiance OCR</th></tr></thead><tbody>{job.detections.slice(0,50).map((d,i)=><tr key={i}><td>{d.seconds.toFixed(2)} s</td><td>#{d.number}</td><td>{Math.round(d.confidence*100)}%</td></tr>)}</tbody></table>}</article>)}</section>
 <footer>Veltryx Vision v0.1 · Ne pas utiliser pour le chronométrage officiel</footer></main>
}
