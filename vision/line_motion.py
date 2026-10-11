#!/usr/bin/env python3
"""Lightweight motion-triggered full-frame snapshots at the timing line.
No AI, OCR, GPU inference or official timing functionality.
ROI defaults are approximate for the user's 1649x928 reference image.
"""
import argparse
from datetime import datetime, timezone
from pathlib import Path
import os
import time
from collections import deque

os.environ.setdefault("OPENCV_FFMPEG_CAPTURE_OPTIONS", "rtsp_transport;tcp")
import cv2
import numpy as np

REFERENCE_W, REFERENCE_H = 1649, 928
# Approximate painted checkered line, with some margin for passing cars.
DEFAULT_POLYGON = [(215, 485), (355, 485), (575, 905), (345, 905)]


# OpenCV-only monitoring UI: no extra GUI libraries or GPU workloads.
def render_dashboard(frame, points, motion_pixels, threshold, saved, last_capture, state, recent):
    scale = min(1.0, 1020 / frame.shape[1], 650 / frame.shape[0])
    width = max(1, round(frame.shape[1] * scale))
    height = max(1, round(frame.shape[0] * scale))
    video = cv2.resize(frame, (width, height), interpolation=cv2.INTER_AREA)
    sidebar = 280
    top = 63
    bottom = 60
    canvas = np.full((height + top + bottom, width + sidebar, 3), (22, 17, 12), dtype=np.uint8)
    canvas[top:top + height, :width] = video
    cv2.polylines(canvas, [np.round(points * scale).astype(np.int32) + [0, top]], True, (248, 196, 39), 2, cv2.LINE_AA)
    cv2.rectangle(canvas, (0, 0), (width + sidebar, top), (30, 25, 18), -1)
    cv2.putText(canvas, "VELTRYX / LINE MONITOR", (18, 39), cv2.FONT_HERSHEY_SIMPLEX, 0.72, (255, 221, 168), 2, cv2.LINE_AA)
    cv2.putText(canvas, datetime.now().strftime("%H:%M:%S"), (max(20, width - 145), 39), cv2.FONT_HERSHEY_SIMPLEX, 0.76, (245, 245, 245), 2, cv2.LINE_AA)
    x = width + 16
    cv2.putText(canvas, "CAPTURE STATUS", (x, 40), cv2.FONT_HERSHEY_SIMPLEX, 0.57, (255, 221, 168), 1, cv2.LINE_AA)
    state_color = (85, 200, 115) if state == "READY" else ((30, 186, 255) if state == "MOTION" else (150, 150, 150))
    cv2.circle(canvas, (x + 9, top + 34), 7, state_color, -1, cv2.LINE_AA)
    cv2.putText(canvas, state, (x + 27, top + 41), cv2.FONT_HERSHEY_SIMPLEX, 0.72, state_color, 2, cv2.LINE_AA)
    cv2.putText(canvas, f"Motion: {motion_pixels} px", (x, top + 86), cv2.FONT_HERSHEY_SIMPLEX, 0.56, (230, 230, 230), 1, cv2.LINE_AA)
    cv2.putText(canvas, f"Threshold: {threshold} px", (x, top + 114), cv2.FONT_HERSHEY_SIMPLEX, 0.52, (172, 172, 172), 1, cv2.LINE_AA)
    cv2.putText(canvas, f"Saved: {saved}", (x, top + 160), cv2.FONT_HERSHEY_SIMPLEX, 0.75, (255, 221, 168), 2, cv2.LINE_AA)
    cv2.putText(canvas, "RECENT CAPTURES", (x, top + 207), cv2.FONT_HERSHEY_SIMPLEX, 0.55, (215, 215, 215), 1, cv2.LINE_AA)
    thumb_w, thumb_h = 108, 67
    for index, (thumb, label) in enumerate(list(recent)[:4]):
        row_y = top + 220 + index * 82
        if row_y + thumb_h >= top + height - 8:
            break
        canvas[row_y:row_y + thumb_h, x:x + thumb_w] = cv2.resize(thumb, (thumb_w, thumb_h), interpolation=cv2.INTER_AREA)
        cv2.putText(canvas, label, (x + thumb_w + 8, row_y + 33), cv2.FONT_HERSHEY_SIMPLEX, 0.4, (215, 215, 215), 1, cv2.LINE_AA)
    if last_capture and time.monotonic() - last_capture < 1.0:
        cv2.rectangle(canvas, (0, top), (width - 1, top + height - 1), (75, 215, 116), 5)
    cv2.putText(canvas, "MOTION TRIGGER / NOT OFFICIAL TIMING", (16, top + height + 23), cv2.FONT_HERSHEY_SIMPLEX, 0.51, (175, 181, 187), 1, cv2.LINE_AA)
    cv2.putText(canvas, "Q quit   S snapshot", (16, top + height + 47), cv2.FONT_HERSHEY_SIMPLEX, 0.56, (245, 245, 245), 1, cv2.LINE_AA)
    return canvas



def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", default="rtsp://127.0.0.1:8554/h264")
    parser.add_argument("--output", default=str(Path(__file__).resolve().parent.parent / "local-data" / "line-crossings"))
    parser.add_argument("--polygon", default=";".join(f"{x},{y}" for x, y in DEFAULT_POLYGON),
                        help="ROI polygon vertices x,y;x,y;... in 1649x928 reference pixels")
    parser.add_argument("--min-pixels", type=int, default=900,
                        help="minimum foreground pixels inside ROI")
    parser.add_argument("--consecutive", type=int, default=2)
    parser.add_argument("--cooldown", type=float, default=2.0)
    parser.add_argument("--warmup", type=float, default=3.0)
    parser.add_argument("--preview", action="store_true", help="show ROI and foreground pixels in window")
    parser.add_argument("--limit", type=int, default=0, help="stop after this many images (0=unlimited)")
    args = parser.parse_args()

    if args.min_pixels < 1 or args.consecutive < 1 or args.cooldown < 0:
        parser.error("invalid thresholds")
    try:
        polygon = [tuple(map(int, item.split(","))) for item in args.polygon.split(";")]
        if len(polygon) < 3 or any(len(p) != 2 for p in polygon):
            raise ValueError
    except ValueError:
        parser.error("polygon must be x,y;x,y;x,y[;...]")

    capture = cv2.VideoCapture(args.source, cv2.CAP_FFMPEG)
    if not capture.isOpened():
        raise RuntimeError("Cannot open camera RTSP. Is the SSH tunnel active?")

    output = Path(args.output).expanduser().resolve()
    bg = cv2.createBackgroundSubtractorMOG2(history=180, varThreshold=38, detectShadows=True)
    start = time.monotonic()
    last_saved = 0.0
    consecutive = 0
    ready = True
    saved = 0
    recent = deque(maxlen=4)
    mask_roi = None
    h = w = 0
    kernel = np.ones((3, 3), dtype=np.uint8)
    print("Motion capture started — no YOLO/OCR/CUDA. Ctrl+C to stop.", flush=True)
    try:
        while True:
            ok, frame = capture.read()
            if not ok:
                raise RuntimeError("RTSP ended or disconnected")
            height, width = frame.shape[:2]
            if mask_roi is None or (h, w) != (height, width):
                h, w = height, width
                pts = np.array(
                    [[round(x * w / REFERENCE_W), round(y * h / REFERENCE_H)]
                     for x, y in polygon], dtype=np.int32
                )
                mask_roi = np.zeros((h, w), dtype=np.uint8)
                cv2.fillPoly(mask_roi, [pts], 255)
                print(f"Image {w}x{h}; ROI scaled from {REFERENCE_W}x{REFERENCE_H}", flush=True)

            # Restrict background subtraction to low-resolution ROI to reduce CPU load.
            roi_gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
            foreground = bg.apply(roi_gray)
            foreground = cv2.bitwise_and(foreground, mask_roi)
            foreground = cv2.threshold(foreground, 250, 255, cv2.THRESH_BINARY)[1]
            foreground = cv2.morphologyEx(foreground, cv2.MORPH_OPEN, kernel)
            motion_pixels = cv2.countNonZero(foreground)
            now = time.monotonic()

            warming_up = now - start < args.warmup
            moving = motion_pixels >= args.min_pixels
            consecutive = consecutive + 1 if moving else 0

            # One capture per transition into the ROI; avoid duplicates while occupied.
            if not moving:
                ready = True
            if not warming_up and ready and consecutive >= args.consecutive and now - last_saved >= args.cooldown:
                stamp = datetime.now(timezone.utc)
                folder = output / stamp.strftime("%Y-%m-%d")
                folder.mkdir(parents=True, exist_ok=True)
                filename = stamp.strftime("%Y%m%dT%H%M%S.%f")[:-3] + "Z.webp"
                destination = folder / filename
                if cv2.imwrite(str(destination), frame, [cv2.IMWRITE_WEBP_QUALITY, 85]):
                    saved += 1
                    recent.appendleft((frame.copy(), stamp.strftime("%H:%M:%S")))
                    last_saved = now
                    ready = False
                    print(f"[{saved}] {destination} (motion_pixels={motion_pixels})", flush=True)
                    if args.limit and saved >= args.limit:
                        break
                else:
                    print(f"Cannot save {destination}", flush=True)

            if args.preview:
                state = "WARMUP" if warming_up else ("MOTION" if moving else "READY")
                display = render_dashboard(frame, pts, motion_pixels, args.min_pixels,
                                           saved, last_saved, state, recent)
                cv2.imshow("Veltryx Vision - Line Monitor", display)
                key = cv2.waitKey(1) & 0xFF
                if key == ord("q") or key == 27:
                    break
                if key == ord("s"):
                    stamp = datetime.now(timezone.utc)
                    folder = output / stamp.strftime("%Y-%m-%d")
                    folder.mkdir(parents=True, exist_ok=True)
                    dest = folder / (stamp.strftime("%Y%m%dT%H%M%S.%f")[:-3] + "Z-manual.webp")
                    if cv2.imwrite(str(dest), frame, [cv2.IMWRITE_WEBP_QUALITY, 85]):
                        saved += 1
                        last_saved = now
                        recent.appendleft((frame.copy(), stamp.strftime("%H:%M:%S")))
                        print(f"[manual {saved}] {dest}", flush=True)
    except KeyboardInterrupt:
        pass
    finally:
        capture.release()
        if args.preview:
            cv2.destroyAllWindows()
    print(f"Stopped; {saved} image(s) saved", flush=True)


if __name__ == "__main__":
    main()
