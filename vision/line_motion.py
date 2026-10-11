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

os.environ.setdefault("OPENCV_FFMPEG_CAPTURE_OPTIONS", "rtsp_transport;tcp")
import cv2
import numpy as np

REFERENCE_W, REFERENCE_H = 1649, 928
# Approximate painted checkered line, with some margin for passing cars.
DEFAULT_POLYGON = [(215, 485), (355, 485), (575, 905), (345, 905)]


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

            if now - start < args.warmup:
                continue
            moving = motion_pixels >= args.min_pixels
            consecutive = consecutive + 1 if moving else 0

            # One capture per transition into the ROI; avoid duplicates while occupied.
            if not moving:
                ready = True
            if ready and consecutive >= args.consecutive and now - last_saved >= args.cooldown:
                stamp = datetime.now(timezone.utc)
                folder = output / stamp.strftime("%Y-%m-%d")
                folder.mkdir(parents=True, exist_ok=True)
                filename = stamp.strftime("%Y%m%dT%H%M%S.%f")[:-3] + "Z.webp"
                destination = folder / filename
                if cv2.imwrite(str(destination), frame, [cv2.IMWRITE_WEBP_QUALITY, 85]):
                    saved += 1
                    last_saved = now
                    ready = False
                    print(f"[{saved}] {destination} (motion_pixels={motion_pixels})", flush=True)
                    if args.limit and saved >= args.limit:
                        break
                else:
                    print(f"Cannot save {destination}", flush=True)

            if args.preview:
                display = frame.copy()
                cv2.polylines(display, [pts], True, (0, 255, 255), 2)
                cv2.putText(display, f"Motion in ROI: {motion_pixels}", (15, 40),
                            cv2.FONT_HERSHEY_SIMPLEX, 0.8, (0, 255, 255), 2)
                cv2.imshow("Veltryx line motion ROI (q to quit)", display)
                if cv2.waitKey(1) & 0xFF == ord("q"):
                    break
    except KeyboardInterrupt:
        pass
    finally:
        capture.release()
        if args.preview:
            cv2.destroyAllWindows()
    print(f"Stopped; {saved} image(s) saved", flush=True)


if __name__ == "__main__":
    main()
