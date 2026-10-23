export type Point = { x: number; y: number };
export type Settings = {
  auto_exposure?: boolean;
  auto_gain?: boolean;
  fps_unlimited?: boolean;
  width: number;
  height: number;
  fps: number;
  exposure_us: number;
  gain: number;
};
export type Camera = {
  capabilities?: {
    auto_exposure?: boolean;
    auto_gain?: boolean;
    fps_unlimited?: boolean;
  };
  serial: string;
  id: string;
  name: string;
  vendor: string;
  model: string;
  ip: string;
  formats: string[];
  settings: Settings;
  connected: boolean;
  simulated: boolean;
};
export type Zone = {
  id: string;
  name: string;
  kind: "rectangle" | "polygon";
  points: Point[];
  pixel_delta: number;
  min_pixels: number;
  confirm_frames: number;
  quiet_frames: number;
  cooldown_ms: number;
  max_changed_ratio: number;
  pre_frames: number;
  post_frames: number;
};
export type Shot = {
  role?: "pre" | "trigger" | "post" | "manual";
  id: string;
  serial: string;
  passage: string;
  zone?: string;
  timestamp: string;
  format: string;
};
export type Stats = {
  screenshots?: { images: number; passages: number; last?: string };
  motion?: Motion[];
  capture: {
    frames: number;
    dropped: number;
    measured_fps: number;
    latency_ms: number;
    running: boolean;
    error?: string;
  };
  video: {
    bitrate_kbps: number;
    encoded_frames: number;
    peers: number;
    error?: string;
  };
};
export type Motion = {
  name?: string;
  state?: string;
  roi_pixels?: number;
  confirmed?: number;
  required?: number;
  timestamp?: string;
  zone: string;
  changed: number;
  threshold: number;
  triggered: boolean;
  pixels?: Point[];
};
export type Event = {
  type: string;
  serial?: string;
  time: string;
  data: unknown;
};
export const cameraPath = (id: string) => `/cameras/${encodeURIComponent(id)}`;
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    method,
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}
export const newZone = (kind: Zone["kind"], points: Point[]): Zone => ({
  id: crypto.randomUUID(),
  name: kind === "rectangle" ? "Rectangle" : "Polygone",
  kind,
  points,
  pixel_delta: 30,
  min_pixels: 150,
  confirm_frames: 2,
  quiet_frames: 3,
  cooldown_ms: 1000,
  max_changed_ratio: 0.8,
  pre_frames: 3,
  post_frames: 3,
});

export const motionStateLabel: Record<string, string> = {
  initializing: "Initialisation",
  idle: "Sous le seuil",
  confirming: "Confirmation",
  triggered: "PASSAGE DÉTECTÉ",
  global_change: "Variation globale filtrée",
  rearming: "Attente de retour au calme",
  cooldown: "Cooldown en cours",
};

export type Passage = {
  id: string;
  serial: string;
  zone?: string;
  timestamp: string;
  image_count: number;
  representative: Shot;
};
