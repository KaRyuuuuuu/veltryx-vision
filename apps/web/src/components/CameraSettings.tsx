import { useState } from "react";
import { api, cameraPath, Camera, Settings } from "../services/api";
export default function CameraSettings({
  camera,
  onSaved,
  onError,
}: {
  camera: Camera;
  onSaved: () => void;
  onError: (s: string) => void;
}) {
  const [values, setValues] = useState<Settings>(camera.settings);
  const [busy, setBusy] = useState(false);
  return (
    <form
      className="settings"
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        try {
          await api(cameraPath(camera.serial) + "/settings", "PUT", values);
          onSaved();
        } catch (e) {
          onError(String(e));
        } finally {
          setBusy(false);
        }
      }}
    >
      <h3>Acquisition</h3>
      {(
        [
          [
            "auto_exposure",
            "Exposition automatique",
            camera.capabilities?.auto_exposure,
          ],
          ["auto_gain", "Gain automatique", camera.capabilities?.auto_gain],
          [
            "fps_unlimited",
            "FPS libres · maximum permis par la caméra",
            camera.capabilities?.fps_unlimited,
          ],
        ] as const
      ).map(([key, label, supported]) => (
        <label key={key} className="auto-setting">
          <input
            type="checkbox"
            checked={!!values[key]}
            disabled={supported === false}
            onChange={(e) => setValues({ ...values, [key]: e.target.checked })}
          />
          {label}
          {supported === false && " (non pris en charge)"}
        </label>
      ))}
      {(
        [
          ["width", "Largeur", 16, 8192, 1],
          ["height", "Hauteur", 16, 8192, 1],
          ["fps", "Limite FPS manuelle", 0.1, undefined, 0.1],
          ["exposure_us", "Exposition manuelle · µs", 1, undefined, 1],
          ["gain", "Gain manuel", 0, undefined, 0.1],
        ] as const
      ).map(([key, label, min, max, step]) => (
        <label key={key}>
          {label}
          <input
            disabled={
              (key === "fps" && values.fps_unlimited) ||
              (key === "exposure_us" && values.auto_exposure) ||
              (key === "gain" && values.auto_gain)
            }
            required
            type="number"
            value={values[key]}
            min={min}
            max={max}
            step={step}
            onChange={(e) =>
              setValues({ ...values, [key]: Number(e.target.value) })
            }
          />
        </label>
      ))}
      <p className="muted">
        Les FPS libres désactivent la limite de fréquence de la caméra.
        L’exposition, le capteur et le réseau peuvent toujours limiter la
        cadence réelle. L’application des paramètres redémarre l’acquisition.
        Reconnectez ensuite la vidéo WebRTC.
      </p>
      <button disabled={busy} className="primary">
        {busy ? "Application…" : "Appliquer"}
      </button>
    </form>
  );
}
