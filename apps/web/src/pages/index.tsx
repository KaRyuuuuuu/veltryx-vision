import Head from "next/head";
import { publishCameraRotation } from "../hooks/useCameraRotation";
import PassageTimeline from "../components/PassageTimeline";
import { useCallback, useEffect, useState } from "react";
import CameraGrid from "../components/CameraGrid";
import CameraSettings from "../components/CameraSettings";
import ROIEditor from "../components/ROIEditor";
import { api, Camera, Event, Motion, Passage } from "../services/api";
export default function Dashboard() {
  const [cameras, setCameras] = useState<Camera[]>([]);
  const [passages, setPassages] = useState<Passage[]>([]);
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
    void api<Passage[]>("/passages?limit=100")
      .then(setPassages)
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
          <PassageTimeline passages={passages} />
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
