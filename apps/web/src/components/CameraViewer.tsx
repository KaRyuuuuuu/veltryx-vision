import { useEffect, useRef, useState } from "react";
import { formatTimestamp } from "../services/time";
import { useCameraRotation } from "../hooks/useCameraRotation";
import {
  api,
  cameraPath,
  Camera,
  Stats,
  motionStateLabel,
} from "../services/api";
export default function CameraViewer({
  camera,
  onEdit,
  onShot,
  onError,
}: {
  camera: Camera;
  onEdit: () => void;
  onShot: () => void;
  onError: (s: string) => void;
}) {
  const [rotation, rotate, rotationPending] = useCameraRotation(
    camera.serial,
    onError,
  );
  const stage = useRef<HTMLDivElement>(null);
  const [stageSize, setStageSize] = useState({ width: 0, height: 0 });
  const aspect = camera.settings.width / camera.settings.height || 16 / 9;
  const turned = rotation % 180 !== 0;
  const mediaWidth = turned
    ? Math.min(stageSize.height, stageSize.width * aspect)
    : Math.min(stageSize.width, stageSize.height * aspect);
  useEffect(() => {
    if (!stage.current) return;
    const observer = new ResizeObserver(([entry]) => {
      setStageSize({
        width: entry.contentRect.width,
        height: entry.contentRect.height,
      });
    });
    observer.observe(stage.current);
    return () => observer.disconnect();
  }, []);
  const video = useRef<HTMLVideoElement>(null);
  const pc = useRef<RTCPeerConnection | null>(null);
  const box = useRef<HTMLElement>(null);
  const [mode, setMode] = useState<"preview" | "connecting" | "live">(
    "preview",
  );
  const [version, setVersion] = useState(0);
  const [stats, setStats] = useState<Stats | null>(null);
  const [busy, setBusy] = useState(false);
  const path = cameraPath(camera.serial);
  useEffect(() => {
    let active = true;
    const update = () => {
      api<Stats>(path + "/stats")
        .then((v) => {
          if (active) setStats(v);
        })
        .catch(() => {});
      setVersion(Date.now());
    };
    update();
    const timer = setInterval(update, 1000);
    return () => {
      active = false;
      clearInterval(timer);
      pc.current?.close();
    };
  }, [path]);
  async function connect() {
    pc.current?.close();
    setMode("connecting");
    const peer = new RTCPeerConnection();
    pc.current = peer;
    peer.addTransceiver("video", { direction: "recvonly" });
    peer.ontrack = (e) => {
      if (video.current) video.current.srcObject = new MediaStream([e.track]);
    };
    peer.onconnectionstatechange = () => {
      if (peer !== pc.current) return;
      if (peer.connectionState === "connected") setMode("live");
      if (["failed", "closed", "disconnected"].includes(peer.connectionState))
        setMode("preview");
    };
    try {
      await peer.setLocalDescription(await peer.createOffer());
      await new Promise<void>((resolve, reject) => {
        if (peer.iceGatheringState === "complete") return resolve();
        const timer = setTimeout(
          () => reject(new Error("Délai ICE dépassé")),
          8000,
        );
        peer.onicegatheringstatechange = () => {
          if (peer.iceGatheringState === "complete") {
            clearTimeout(timer);
            resolve();
          }
        };
      });
      const answer = await api<RTCSessionDescriptionInit>(
        path + "/webrtc",
        "POST",
        peer.localDescription,
      );
      if (peer !== pc.current) return;
      await peer.setRemoteDescription(answer);
    } catch (e) {
      peer.close();
      setMode("preview");
      onError(String(e));
    }
  }
  return (
    <article className="camera" ref={box}>
      <div className="camera-title">
        <div>
          <h2>{camera.name || camera.model}</h2>
          <span className="muted">
            {camera.vendor} {camera.model} · {camera.serial}
          </span>
        </div>
        <span className={camera.connected ? "status online" : "status"}>
          {camera.simulated
            ? "SIMULATION"
            : camera.connected
              ? "CONNECTÉE"
              : "HORS LIGNE"}
        </span>
      </div>
      <div className="video-stage" ref={stage}>
        <div
          className="rotated-media"
          style={{
            width: mediaWidth || "100%",
            height: mediaWidth ? mediaWidth / aspect : "100%",
            transform: `translate(-50%, -50%) rotate(${rotation}deg)`,
          }}
        >
          {mode === "preview" && camera.connected && (
            <img
              src={`/api/v1${path}/frame?t=${version}`}
              alt={camera.name}
              onError={(e) => {
                e.currentTarget.style.visibility = "hidden";
              }}
              onLoad={(e) => {
                e.currentTarget.style.visibility = "visible";
              }}
            />
          )}
          <video
            ref={video}
            autoPlay
            muted
            playsInline
            style={{ display: mode === "preview" ? "none" : "block" }}
          />
        </div>
        <button
          className="rotate-image"
          onClick={rotate}
          disabled={rotationPending}
          aria-label={`Tourner l’image de 90 degrés ; orientation actuelle ${rotation} degrés`}
          title="Tourner l’image de 90°"
        >
          ↻ {rotation}°
        </button>
        <div className="video-label">
          {mode === "live"
            ? "● WEBRTC / H.264"
            : mode === "connecting"
              ? "CONNEXION…"
              : "APERÇU JPEG · 1 FPS"}
        </div>
        <button
          className="fullscreen"
          aria-label="Plein écran"
          onClick={() =>
            box.current?.requestFullscreen().catch((e) => onError(String(e)))
          }
        >
          ⛶
        </button>
      </div>
      <div className="metrics">
        <span>
          <b>{stats?.capture.measured_fps.toFixed(1) || "0"}</b> fps
        </span>
        <span>
          <b>{stats?.video.bitrate_kbps.toFixed(0) || "0"}</b> kb/s
        </span>
        <span>
          <b>{stats?.capture.latency_ms.toFixed(1) || "0"}</b> ms
        </span>
        <span>
          <b>{stats?.capture.dropped || 0}</b> pertes abonnés
        </span>
      </div>
      <div className="detection-summary" aria-live="polite">
        {stats?.screenshots ? (
          <p>
            <strong>{stats.screenshots.images} captures</strong> ·{" "}
            {stats.screenshots.passages} passages
            {stats.screenshots.last && (
              <> · Dernière : {formatTimestamp(stats.screenshots.last)}</>
            )}
          </p>
        ) : (
          <p>Chargement des compteurs…</p>
        )}
        {stats?.motion?.map((m) => (
          <p key={m.zone}>
            <strong>{m.name || "ROI"}</strong> : {m.changed} / {m.threshold}{" "}
            pixels · {motionStateLabel[m.state || ""] || "En attente"}
            {m.state === "confirming" &&
              ` (${m.confirmed}/${m.required} frames)`}
          </p>
        ))}
        {stats?.motion?.length === 0 && (
          <p>
            Aucune ROI active. Dessinez une zone puis cliquez sur « Enregistrer
            les ROI ».
          </p>
        )}
      </div>
      {(stats?.capture.error || stats?.video.error) && (
        <p className="error">{stats.capture.error || stats.video.error}</p>
      )}
      <div className="camera-actions">
        <button
          onClick={connect}
          disabled={!camera.connected || mode === "connecting"}
        >
          Voir en direct
        </button>
        <button onClick={onEdit}>ROI & réglages</button>
        <button
          disabled={busy || !camera.connected}
          onClick={async () => {
            setBusy(true);
            try {
              await api(path + "/screenshots", "POST", { format: "jpeg" });
              onShot();
            } catch (e) {
              onError(String(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          Capture
        </button>
        <button
          onClick={async () => {
            try {
              const stop = stats?.capture.running;
              await api(path + `/stream/${stop ? "stop" : "start"}`, "POST");
              if (stop) {
                pc.current?.close();
                setMode("preview");
              }
            } catch (e) {
              onError(String(e));
            }
          }}
        >
          {stats?.capture.running ? "Arrêter" : "Démarrer"}
        </button>
      </div>
    </article>
  );
}
