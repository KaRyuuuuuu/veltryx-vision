import { useCallback, useEffect, useState } from "react";
import { api, cameraPath } from "../services/api";

const changeEvent = "veltryx:camera-rotation";
type Orientation = { rotation: number; configured: boolean };

export function publishCameraRotation(serial: string, rotation: number) {
  window.dispatchEvent(
    new CustomEvent(changeEvent, { detail: { serial, rotation } }),
  );
}

// The same persisted orientation controls display and saved screenshot pixels.
export function useCameraRotation(
  serial: string,
  onError: (message: string) => void,
) {
  const key = `veltryx:rotation:${serial}`;
  const path = cameraPath(serial) + "/orientation";
  const [rotation, setRotation] = useState(0);
  const [pending, setPending] = useState(true);

  useEffect(() => {
    let active = true;
    setPending(true);
    const changed = (event: Event) => {
      const { detail } = event as CustomEvent<{
        serial: string;
        rotation: number;
      }>;
      if (detail.serial === serial) setRotation(detail.rotation);
    };
    window.addEventListener(changeEvent, changed);
    async function load() {
      try {
        let saved = await api<Orientation>(path);
        if (!active) return;
        // Migrate an existing browser preference once, without replacing server settings.
        let previous: string | null = null;
        try {
          previous = localStorage.getItem(key);
        } catch {}
        if (
          !saved.configured &&
          previous !== null &&
          [0, 90, 180, 270].includes(Number(previous))
        ) {
          saved = await api<Orientation>(path, "PUT", {
            rotation: Number(previous),
          });
        }
        if (active) publishCameraRotation(serial, saved.rotation);
      } catch (error) {
        if (active) onError(`Orientation : ${String(error)}`);
      } finally {
        if (active) setPending(false);
      }
    }
    void load();
    return () => {
      active = false;
      window.removeEventListener(changeEvent, changed);
    };
  }, [key, path, serial, onError]);

  const rotate = useCallback(async () => {
    if (pending) return;
    setPending(true);
    try {
      const saved = await api<Orientation>(path, "PUT", {
        rotation: (rotation + 90) % 360,
      });
      publishCameraRotation(serial, saved.rotation);
      try {
        localStorage.setItem(key, String(saved.rotation));
      } catch {}
    } catch (error) {
      onError(`Impossible d’enregistrer l’orientation : ${String(error)}`);
    } finally {
      setPending(false);
    }
  }, [key, path, rotation, serial, pending, onError]);

  return [rotation, rotate, pending] as const;
}
