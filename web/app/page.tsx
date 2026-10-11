"use client";
import {useEffect,useState} from "react";
type Detection={frame:number;seconds:number;number:string;confidence:number;box:number[]};
type Job={id:string;source:string;status:string;error?:string;detections:Detection[];created_at:string};
const api="http://127.0.0.1:8088";
export default function Home(){
 const [live,setLive]=useState(false);
 const [previewMode,setPreviewMode]=useState<"precise"|"smooth">("precise");
 const [detections,setDetections]=useState<Array<{box:number[];confidence:number;number:string|null;ocr_confidence:number|null}>>([]);
 const [feedStatus,setFeedStatus]=useState("Arrêté");
 const [frameSize,setFrameSize]=useState({width:960,height:480});
 const [lastDetectionAt,setLastDetectionAt]=useState(0);
 useEffect(()=>{if(!live){setFeedStatus("Arrêté");setDetections([]);return;}setFeedStatus("Connexion YOLO...");const ev=new EventSource(api+"/api/detections");ev.onmessage=e=>{try{const d=JSON.parse(e.data);setDetections(d.detections ?? []);if(d.width>0&&d.height>0)setFrameSize({width:d.width,height:d.height});setLastDetectionAt(Date.now());setFeedStatus("Analyse active");}catch{}};ev.onerror=()=>setFeedStatus("IA indisponible : vérifier le terminal Go");return()=>ev.close()},[live]);
 useEffect(()=>{if(!live)return;const t=setInterval(()=>{if(lastDetectionAt&&Date.now()-lastDetectionAt>2500)setDetections([])},500);return()=>clearInterval(t)},[live,lastDetectionAt]);
 const [path,setPath]=useState("");const [jobs,setJobs]=useState<Job[]>([]);const [error,setError]=useState("");
 async function refresh(){try{const r=await fetch(api+"/api/jobs");if(!r.ok)throw Error("API indisponible");setJobs(await r.json())}catch{setError("API Go indisponible sur le port 8088")}}
 useEffect(()=>{refresh();const t=setInterval(refresh,2000);return()=>clearInterval(t)},[]);
 async function launch(){setError("");try{const r=await fetch(api+"/api/jobs",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({source:path})});if(!r.ok)throw Error(await r.text());setPath("");refresh()}catch(e){setError(String(e))}}
 return <main><header><div><span className="eyebrow">VELTRYX / R&D</span><h1>Vision Control</h1><p>Détection YOLO + lecture OCR · analyse locale, non officielle</p></div><span className="pill">Development</span></header>
 <section className="panel">
 <div className="jobhead"><h2>Caméra en direct</h2><button onClick={()=>setLive(!live)}>{live?"Arrêter le direct":"Démarrer le direct"}</button></div>
 <div className="preview-modes"><button className={previewMode==="precise"?"selected":""} onClick={()=>setPreviewMode("precise")}>Précision · YOLO + OCR</button><button className={previewMode==="smooth"?"selected":""} onClick={()=>setPreviewMode("smooth")}>Fluidité · WebRTC 60 FPS</button></div>
 <p>{previewMode==="precise"?"Les cadres sont dessinés directement sur l'image analysée : pas de décalage géométrique ou temporel. L'aperçu MJPEG est volontairement limité en FPS.":"Vidéo WebRTC fluide à la cadence de la caméra. Les rectangles ne sont pas affichés, car l'analyse séparée n'est pas synchronisée au flux."}</p>
 {live&&(previewMode==="precise"?<div className="precise-preview"><img src={api+"/api/live"} alt="Caméra avec rectangles YOLO et numéros OCR, synchronisés avec chaque image affichée" /></div>:<div className="videoarea" style={{aspectRatio:`${frameSize.width} / ${frameSize.height}`}}><iframe title="Basler direct WebRTC" src="http://localhost:8889/cam" allow="autoplay; fullscreen" /></div>)}
 <p>Analyse : {feedStatus} · {detections.length} véhicule(s) sur la dernière image analysée</p>
 {live&&detections.length>0&&<div className="detections">{detections.map((d,i)=><div key={i}>Véhicule {i+1} — détection {Math.round(d.confidence*100)} % — {d.number ? "Numéro candidat #"+d.number+" ("+Math.round((d.ocr_confidence??0)*100)+" % OCR)" : "Numéro inconnu"}</div>)}</div>}
 </section>
 <section className="panel"><h2>Nouvelle analyse</h2><p>Chemin absolu vers un fichier vidéo dans ~/Videos sur Dragonfly. Aucune caméra de production n'est sollicitée.</p><div className="form"><input aria-label="Chemin vidéo" placeholder="/home/karyuu/Videos/veltryx-vision/basler.mkv" value={path} onChange={e=>setPath(e.target.value)}/><button disabled={!path.trim()} onClick={launch}>Analyser</button></div>{error&&<div className="error">{error}</div>}</section>
 <section className="panel"><h2>Analyses <span>{jobs.length}</span></h2>{jobs.length===0?<p>Aucune analyse pour l'instant.</p>:jobs.slice().reverse().map(job=><article key={job.id}><div className="jobhead"><strong>{job.source.split("/").pop()}</strong><span className={"status "+job.status}>{job.status}</span></div><small>{new Date(job.created_at).toLocaleString("fr-BE")} · {job.detections.length} candidats OCR</small>{job.error&&<pre>{job.error}</pre>}{job.detections.length>0&&<table><thead><tr><th>Temps vidéo</th><th>Numéro candidat</th><th>Confiance OCR</th></tr></thead><tbody>{job.detections.slice(0,50).map((d,i)=><tr key={i}><td>{d.seconds.toFixed(2)} s</td><td>#{d.number}</td><td>{Math.round(d.confidence*100)}%</td></tr>)}</tbody></table>}</article>)}</section>
 <footer>Veltryx Vision v0.1 · Ne pas utiliser pour le chronométrage officiel</footer></main>
}
