import Head from "next/head";
import { publishCameraRotation } from "../hooks/useCameraRotation";
import { formatTimestamp } from "../services/time";
import { useCallback, useEffect, useState } from "react";
import CameraGrid from "../components/CameraGrid";
import CameraSettings from "../components/CameraSettings";
import ROIEditor from "../components/ROIEditor";
import { api, Camera, Event, Motion, Shot } from "../services/api";
export default function Dashboard() {
  const [cameras, setCameras] = useState<Camera[]>([]);
  const [shots, setShots] = useState<Shot[]>([]);
  const [editing, setEditing] = useState<Camera | null>(null);
  const [error, setError] = useState("");
  const [connected, setConnected] = useState(false);
  const [motions, setMotions] = useState<
    Record<string, Record<string, Motion>>
  >({});
  const [tab, setTab] = useState<"cameras" | "history">("cameras");
  const onError = useCallback((e: string) => setError(e), []);
  const refresh = useCallback(() => {
    void api<Camera[]>("/cameras")
      .then(setCameras)
      .catch((e) => setError(String(e)));
  }, []);
  const refreshShots = useCallback(() => {
    void api<Shot[]>("/screenshots?limit=100")
      .then(setShots)
      .catch((e) => setError(String(e)));
  }, []);
  useEffect(() => {
    refresh();
    refreshShots();
    const events = new EventSource("/api/v1/events");
    events.onopen = () => {
      setConnected(true);
      refresh();
      refreshShots();
    };
    events.onerror = () => setConnected(false);
    let dirty = false;
    events.onmessage = (message) => {
      try {
        const e = JSON.parse(message.data) as Event;
        if (e.type === "orientation" && e.serial)
          publishCameraRotation(
            e.serial,
            (e.data as { rotation: number }).rotation,
          );
        if (e.type === "cameras") setCameras(e.data as Camera[]);
        if (e.type === "screenshot") dirty = true;
        if (e.type === "error") setError(String(e.data));
        if (e.type === "motion" && e.serial) {
          const m = e.data as Motion;
          setMotions((prev) => ({
            ...prev,
            [e.serial!]: { ...prev[e.serial!], [m.zone]: m },
          }));
        }
      } catch {}
    };
    let ticks = 0;
    const timer = setInterval(() => {
      if (dirty || ++ticks % 5 === 0) {
        dirty = false;
        refreshShots();
      }
    }, 1000);
    return () => {
      events.close();
      clearInterval(timer);
    };
  }, [refresh, refreshShots]);
  const passages = new Map<string, Shot[]>();
  for (const s of shots) {
    const group = passages.get(s.passage) || [];
    group.push(s);
    passages.set(s.passage, group);
  }
  return (
    <>
      <Head>
        <title>Veltryx Vision · Camera Control</title>
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <meta
          name="description"
          content="Supervision des caméras industrielles et détection des passages Veltryx"
        />
      </Head>
      <header>
        <a className="brand" href="/">
          V
          <span>
            VELTRYX<b>VISION / V2</b>
          </span>
        </a>
        <nav>
          <button
            className={tab === "cameras" ? "active" : ""}
            onClick={() => setTab("cameras")}
          >
            Caméras
          </button>
          <button
            className={tab === "history" ? "active" : ""}
            onClick={() => setTab("history")}
          >
            Passages
          </button>
        </nav>
        <span className={connected ? "status online" : "status"}>
          {connected ? "ÉVÉNEMENTS EN LIGNE" : "RECONNEXION SSE…"}
        </span>
      </header>
      <main>
        <div className="page-title">
          <div>
            <p className="eyebrow">RACE OPERATIONS / VISION CONTROL</p>
            <h1>
              {tab === "cameras"
                ? "Chaque passage. Chaque détail."
                : "Historique des passages"}
            </h1>
            <p className="muted">
              {cameras.filter((c) => c.connected).length} caméra(s) connectée(s)
              · Acquisition indépendante par caméra
            </p>
          </div>
          <button
            onClick={async () => {
              try {
                setCameras(await api<Camera[]>("/discovery", "POST"));
              } catch (e) {
                setError(String(e));
              }
            }}
          >
            ↻ Découvrir
          </button>
        </div>
        {error && (
          <div className="error-banner" role="alert">
            {error}
            <button onClick={() => setError("")}>Fermer</button>
          </div>
        )}
        {tab === "cameras" ? (
          cameras.length ? (
            <CameraGrid
              cameras={cameras}
              onEdit={setEditing}
              onShot={refreshShots}
              onError={onError}
            />
          ) : (
            <div className="empty">
              <h2>Aucune caméra découverte</h2>
              <p>
                Vérifiez le réseau GigE Vision ou démarrez le backend avec{" "}
                <code>-simulate 2</code>.
              </p>
              <p>
                Les caméras apparaissent automatiquement après la découverte.
              </p>
            </div>
          )
        ) : (
          <div className="passages">
            {[...passages].map(([id, group]) => (
              <article className="passage" key={id}>
                <div>
                  <h2>{formatTimestamp(group[0].timestamp, true)}</h2>
                  <p className="muted">
                    {group[0].serial} · {group.length} image(s) ·{" "}
                    {group[0].zone ? "Détection ROI" : "Capture manuelle"}
                  </p>
                  <code>{id}</code>
                  <span className="ocr-state">OCR · moteur non configuré</span>
                </div>
                <div className="thumbnails">
                  {group.map((s) => (
                    <a
                      key={s.id}
                      href={`/api/v1/screenshots/${s.id}/image`}
                      target="_blank"
                      rel="noreferrer"
                      title={formatTimestamp(s.timestamp, true)}
                    >
                      <img
                        loading="lazy"
                        src={`/api/v1/screenshots/${s.id}/image`}
                        alt={`Capture ${s.serial} à ${formatTimestamp(s.timestamp, true)}`}
                      />
                      <time dateTime={s.timestamp}>
                        {formatTimestamp(s.timestamp)}
                      </time>
                    </a>
                  ))}
                </div>
              </article>
            ))}
            {!shots.length && (
              <div className="empty">
                Les captures manuelles et les passages détectés apparaîtront
                ici.
              </div>
            )}
          </div>
        )}
      </main>
      {editing && (
        <div
          className="modal-backdrop"
          onClick={(e) => {
            if (e.target === e.currentTarget) setEditing(null);
          }}
        >
          <section
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-label="Réglages caméra"
          >
            <div className="modal-title">
              <div>
                <p className="eyebrow">CAMÉRA / {editing.serial}</p>
                <h2>{editing.name}</h2>
              </div>
              <button onClick={() => setEditing(null)}>Fermer ✕</button>
            </div>
            {error && (
              <div className="error-banner" role="alert">
                {error}
                <button onClick={() => setError("")}>Fermer</button>
              </div>
            )}
            <ROIEditor
              camera={editing}
              motions={motions[editing.serial] || {}}
              onError={onError}
            />
            <CameraSettings
              camera={editing}
              onSaved={() => {
                refresh();
                setEditing(null);
              }}
              onError={onError}
            />
          </section>
        </div>
      )}
      <footer>
        VELTRYX VISION{" "}
        <span>
          Acquisition industrielle · Les détections ne constituent pas un
          chronométrage officiel.
        </span>
      </footer>
    </>
  );
}
