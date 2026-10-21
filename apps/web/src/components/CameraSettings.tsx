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
          ["width", "Largeur", 16, 8192, 1],
          ["height", "Hauteur", 16, 8192, 1],
          ["fps", "Images / seconde", 1, 240, 0.1],
          ["exposure_us", "Exposition · µs", 1, 1000000, 1],
          ["gain", "Gain", 0, 100, 0.1],
        ] as const
      ).map(([key, label, min, max, step]) => (
        <label key={key}>
          {label}
          <input
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
        L’application des paramètres redémarre l’acquisition. Reconnectez
        ensuite la vidéo WebRTC.
      </p>
      <button disabled={busy} className="primary">
        {busy ? "Application…" : "Appliquer"}
      </button>
    </form>
  );
}
