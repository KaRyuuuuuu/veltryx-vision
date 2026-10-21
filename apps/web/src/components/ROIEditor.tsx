import { useEffect, useRef, useState } from "react";
import { useCameraRotation } from "../hooks/useCameraRotation";
import {
  api,
  cameraPath,
  Camera,
  Motion,
  newZone,
  Point,
  Zone,
  motionStateLabel,
} from "../services/api";
const clamp = (v: number) => Math.max(0, Math.min(1, v));
export default function ROIEditor({
  camera,
  motions,
  onError,
}: {
  camera: Camera;
  motions: Record<string, Motion>;
  onError: (s: string) => void;
}) {
  const [zones, setZones] = useState<Zone[]>([]);
  const [selected, setSelected] = useState("");
  const [tool, setTool] = useState<"select" | "rectangle" | "polygon">(
    "select",
  );
  const [draft, setDraft] = useState<Point[]>([]);
  const [rotation, rotate, rotationPending] = useCameraRotation(
    camera.serial,
    onError,
  );
  const [vertex, setVertex] = useState<number | null>(null);
  const [drag, setDrag] = useState<number | null>(null);
  const [test, setTest] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [stamp, setStamp] = useState(0);
  const svg = useRef<SVGSVGElement>(null);
  const path = cameraPath(camera.serial);
  const current = zones.find((z) => z.id === selected);
  useEffect(() => {
    let active = true;
    api<Zone[]>(path + "/roi")
      .then((z) => {
        if (active) {
          setZones(z);
          setSelected(z[0]?.id || "");
          setLoaded(true);
        }
      })
      .catch((e) => onError(String(e)));
    const timer = setInterval(() => setStamp(Date.now()), 250);
    return () => {
      active = false;
      clearInterval(timer);
      void api(path + "/motion-test", "POST", { enabled: false }).catch(
        () => {},
      );
    };
  }, [path, onError]);
  function point(e: React.PointerEvent): Point {
    const rect = svg.current!.getBoundingClientRect();
    const x = clamp((e.clientX - rect.left) / rect.width),
      y = clamp((e.clientY - rect.top) / rect.height);
    if (rotation === 90) return { x: y, y: 1 - x };
    if (rotation === 180) return { x: 1 - x, y: 1 - y };
    if (rotation === 270) return { x: 1 - y, y: x };
    return { x, y };
  }
  function update(z: Zone) {
    setZones((prev) => prev.map((v) => (v.id === z.id ? z : v)));
  }
  function add(points: Point[]) {
    const z = newZone(tool === "rectangle" ? "rectangle" : "polygon", points);
    setZones((prev) => [...prev, z]);
    setSelected(z.id);
    setDraft([]);
    setTool("select");
  }
  function down(e: React.PointerEvent<SVGSVGElement>) {
    if (!loaded) return;
    const p = point(e);
    if (tool === "rectangle") {
      if (!draft.length) setDraft([p]);
      else {
        const a = draft[0];
        if (Math.abs(a.x - p.x) < 0.005 || Math.abs(a.y - p.y) < 0.005) return;
        const x1 = Math.min(a.x, p.x),
          x2 = Math.max(a.x, p.x),
          y1 = Math.min(a.y, p.y),
          y2 = Math.max(a.y, p.y);
        add([
          { x: x1, y: y1 },
          { x: x2, y: y1 },
          { x: x2, y: y2 },
          { x: x1, y: y2 },
        ]);
      }
    } else if (tool === "polygon") setDraft([...draft, p]);
    else if (e.altKey && current?.kind === "polygon") {
      let best = 0,
        distance = Infinity;
      current.points.forEach((a, i) => {
        const b = current.points[(i + 1) % current.points.length];
        const dx = b.x - a.x,
          dy = b.y - a.y;
        const t = clamp(
          ((p.x - a.x) * dx + (p.y - a.y) * dy) / (dx * dx + dy * dy || 1),
        );
        const dist = (p.x - a.x - t * dx) ** 2 + (p.y - a.y - t * dy) ** 2;
        if (dist < distance) {
          distance = dist;
          best = i;
        }
      });
      const points = [...current.points];
      points.splice(best + 1, 0, p);
      update({ ...current, points });
      setVertex(best + 1);
    }
  }
  function move(e: React.PointerEvent<SVGSVGElement>) {
    if (drag === null || !current) return;
    const p = point(e),
      points = [...current.points];
    points[drag] = p;
    if (current.kind === "rectangle") {
      const opposite = points[(drag + 2) % 4];
      points[(drag + 1) % 4] =
        drag % 2 === 0 ? { x: opposite.x, y: p.y } : { x: p.x, y: opposite.y };
      points[(drag + 3) % 4] =
        drag % 2 === 0 ? { x: p.x, y: opposite.y } : { x: opposite.x, y: p.y };
    }
    update({ ...current, points });
  }
  return (
    <section className="roi-editor">
      <div className="toolbar">
        {(["select", "rectangle", "polygon"] as const).map((t) => (
          <button
            key={t}
            className={tool === t ? "active" : ""}
            onClick={() => {
              setTool(t);
              setDraft([]);
            }}
          >
            {t === "select"
              ? "Déplacer"
              : t === "rectangle"
                ? "Rectangle"
                : "Polygone"}
          </button>
        ))}
        {tool === "polygon" && (
          <button disabled={draft.length < 3} onClick={() => add(draft)}>
            Fermer le polygone
          </button>
        )}
        <button onClick={rotate} disabled={rotationPending}>
          ↻ {rotation}°
        </button>
        <label className="toggle">
          <input
            type="checkbox"
            checked={test}
            onChange={async (e) => {
              const enabled = e.target.checked;
              try {
                await api(path + "/motion-test", "POST", { enabled });
                setTest(enabled);
              } catch (e) {
                onError(String(e));
              }
            }}
          />{" "}
          Mode test
        </label>
      </div>
      <p className="muted">
        Rectangle : deux coins opposés. Polygone : cliquez pour poser les
        points. Déplacez les poignées ; Alt + clic ajoute un point sur une
        arête.
      </p>
      <svg
        ref={svg}
        className="roi-canvas"
        viewBox="0 0 1 1"
        preserveAspectRatio="none"
        style={{
          aspectRatio:
            rotation % 180
              ? camera.settings.height / camera.settings.width
              : camera.settings.width / camera.settings.height,
        }}
        onPointerDown={down}
        onPointerMove={move}
        onPointerUp={() => setDrag(null)}
        onPointerCancel={() => setDrag(null)}
      >
        <g transform={`rotate(${rotation} .5 .5)`}>
          <image
            href={`/api/v1${path}/frame?t=${stamp}`}
            x="0"
            y="0"
            width="1"
            height="1"
            preserveAspectRatio="none"
          />
          {zones.map((z) => (
            <polygon
              key={z.id}
              points={z.points.map((p) => `${p.x},${p.y}`).join(" ")}
              fill={selected === z.id ? "#a3e63522" : "#22d3ee11"}
              stroke={selected === z.id ? "#bef264" : "#22d3ee"}
              strokeWidth=".003"
              onPointerDown={(e) => {
                if (tool === "select") {
                  setSelected(z.id);
                  if (!e.altKey) e.stopPropagation();
                }
              }}
            />
          ))}
          {draft.length > 0 && (
            <polyline
              points={draft.map((p) => `${p.x},${p.y}`).join(" ")}
              stroke="#fff"
              strokeWidth=".003"
              fill="none"
            />
          )}
          {test &&
            Object.values(motions).flatMap((m) =>
              (m.pixels || []).map((p, i) => (
                <rect
                  key={m.zone + i}
                  x={p.x}
                  y={p.y}
                  width={(1 / camera.settings.width) * 2}
                  height={(1 / camera.settings.height) * 2}
                  fill="#ff4060"
                />
              )),
            )}
          {current?.points.map((p, i) => (
            <circle
              key={i}
              cx={p.x}
              cy={p.y}
              r=".009"
              fill={vertex === i ? "#fff" : "#bef264"}
              stroke="#101818"
              strokeWidth=".002"
              onPointerDown={(e) => {
                if (tool !== "select") return;
                e.stopPropagation();
                svg.current?.setPointerCapture(e.pointerId);
                setDrag(i);
                setVertex(i);
              }}
            />
          ))}
        </g>
      </svg>
      <div className="roi-controls">
        <label>
          Zone
          <select
            value={selected}
            onChange={(e) => {
              setSelected(e.target.value);
              setVertex(null);
            }}
          >
            <option value="">Sélectionner…</option>
            {zones.map((z) => (
              <option key={z.id} value={z.id}>
                {z.name}
              </option>
            ))}
          </select>
        </label>
        <button
          disabled={!current}
          onClick={() => {
            setZones(zones.filter((z) => z.id !== selected));
            setSelected("");
          }}
        >
          Supprimer la zone
        </button>
        <button
          disabled={
            !current ||
            current.kind !== "polygon" ||
            vertex === null ||
            current.points.length <= 3
          }
          onClick={() => {
            if (current && vertex !== null) {
              update({
                ...current,
                points: current.points.filter((_, i) => i !== vertex),
              });
              setVertex(null);
            }
          }}
        >
          Supprimer le point
        </button>
        <button
          className="primary"
          disabled={!loaded || busy}
          onClick={async () => {
            setBusy(true);
            try {
              await api(path + "/roi", "PUT", zones);
            } catch (e) {
              onError(String(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          {busy ? "Enregistrement…" : "Enregistrer les ROI"}
        </button>
      </div>
      {current && (
        <div className="zone-settings">
          <label>
            Nom
            <input
              value={current.name}
              maxLength={160}
              onChange={(e) => update({ ...current, name: e.target.value })}
            />
          </label>
          {(
            [
              ["pixel_delta", "Écart pixel", 1, 255, 1],
              ["min_pixels", "Pixels modifiés", 1, 16777216, 1],
              ["confirm_frames", "Confirmation (frames)", 1, 120, 1],
              ["quiet_frames", "Réarmement (frames)", 1, 120, 1],
              ["cooldown_ms", "Cooldown (ms)", 0, 3600000, 1],
              ["max_changed_ratio", "Variation globale max.", 0.01, 1, 0.01],
              ["pre_frames", "Frames avant", 0, 60, 1],
              ["post_frames", "Frames après", 0, 60, 1],
            ] as const
          ).map(([key, label, min, max, step]) => (
            <label key={key}>
              {label}
              <input
                type="number"
                value={current[key]}
                min={min}
                max={max}
                step={step}
                onChange={(e) =>
                  update({ ...current, [key]: Number(e.target.value) })
                }
              />
            </label>
          ))}
        </div>
      )}
      {test && (
        <div className="test-results">
          {Object.values(motions).map((m) => (
            <span key={m.zone} className={m.triggered ? "trigger" : ""}>
              {zones.find((z) => z.id === m.zone)?.name || m.zone} : {m.changed}{" "}
              / {m.threshold} px ·{" "}
              {motionStateLabel[m.state || ""] || "En attente"}
            </span>
          ))}
          <p className="muted">
            Jusqu’à 2 048 pixels affichés par zone. Appliquez les ROI pour
            tester les nouveaux paramètres.
          </p>
        </div>
      )}
    </section>
  );
}
