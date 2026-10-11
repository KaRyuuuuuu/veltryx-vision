#!/usr/bin/env python3
"""Low-latency RTSP -> CUDA YOLO/ByteTrack -> SSE JSONL.
Capture and OCR run outside the inference loop. 60 FPS is a target, not a guarantee.
"""
import argparse
import json
import os
import queue
import re
import sys
import threading
import time

os.environ.setdefault("OPENCV_FFMPEG_CAPTURE_OPTIONS", "rtsp_transport;tcp")
import cv2
import pytesseract
import torch
from ultralytics import YOLO

VEHICLES = {"car", "truck", "bus", "motorcycle"}


class LatestFrame:
    """Capture continuously; discard stale frames rather than build latency."""
    def __init__(self, source):
        self.cap = cv2.VideoCapture(source, cv2.CAP_FFMPEG)
        if not self.cap.isOpened():
            raise RuntimeError("Cannot open RTSP for detection")
        self.condition = threading.Condition()
        self.frame = None
        self.sequence = 0
        self.captured = 0
        self.closed = False
        self.error = None
        self.worker = threading.Thread(target=self.run, daemon=True)
        self.worker.start()

    def run(self):
        try:
            while True:
                ok, frame = self.cap.read()
                if not ok:
                    raise RuntimeError("RTSP disconnected")
                with self.condition:
                    if self.closed:
                        return
                    self.frame = frame
                    self.sequence += 1
                    self.captured += 1
                    self.condition.notify_all()
        except Exception as exc:
            with self.condition:
                self.error = exc
                self.closed = True
                self.condition.notify_all()
        finally:
            self.cap.release()

    def next(self, previous):
        with self.condition:
            self.condition.wait_for(
                lambda: self.sequence != previous or self.closed, timeout=3.0
            )
            if self.sequence == previous:
                if self.error:
                    raise self.error
                raise RuntimeError("No RTSP frames received for 3 seconds")
            return self.sequence, self.frame

    def stop(self):
        with self.condition:
            self.closed = True
            self.condition.notify_all()


def read_number(crop):
    gray = cv2.cvtColor(crop, cv2.COLOR_BGR2GRAY)
    gray = cv2.resize(gray, None, fx=2, fy=2, interpolation=cv2.INTER_CUBIC)
    data = pytesseract.image_to_data(
        gray, config="--psm 11 -c tessedit_char_whitelist=0123456789",
        output_type=pytesseract.Output.DICT,
    )
    candidates = []
    for value, conf in zip(data["text"], data["conf"]):
        digits = re.sub(r"\D", "", value)
        try:
            score = float(conf)
        except (ValueError, TypeError):
            continue
        if 1 <= len(digits) <= 4 and score >= 40:
            candidates.append((digits, score))
    return max(candidates, key=lambda item: item[1]) if candidates else None


class AsyncOCR:
    """One OCR worker with bounded queue; never block the inference thread."""
    def __init__(self):
        self.jobs = queue.Queue(maxsize=8)
        self.lock = threading.Lock()
        self.cache = {}
        self.pending = set()
        self.last_attempt = {}
        self.completed = 0
        threading.Thread(target=self.run, daemon=True).start()

    def submit(self, key, crop, interval):
        now = time.monotonic()
        with self.lock:
            if key in self.pending or now - self.last_attempt.get(key, 0) < interval:
                return
            self.pending.add(key)
            self.last_attempt[key] = now
        try:
            self.jobs.put_nowait((key, crop.copy()))
        except queue.Full:
            with self.lock:
                self.pending.discard(key)

    def lookup(self, key):
        with self.lock:
            return self.cache.get(key)

    def run(self):
        while True:
            key, crop = self.jobs.get()
            try:
                candidate = read_number(crop)
                with self.lock:
                    if candidate is not None:
                        # Keep the highest-confidence candidate observed for this track.
                        old = self.cache.get(key)
                        if old is None or candidate[1] > old[1]:
                            self.cache[key] = candidate
                    self.completed += 1
            except Exception as exc:
                print(f"OCR warning: {exc}", file=sys.stderr)
            finally:
                with self.lock:
                    self.pending.discard(key)
                self.jobs.task_done()


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--source", required=True)
    p.add_argument("--every", type=int, default=1, help="analyze every Nth captured frame (default: 1)")
    p.add_argument("--ocr-interval", type=float, default=0.7)
    p.add_argument("--imgsz", type=int, default=640)
    args = p.parse_args()
    gpu = torch.cuda.is_available()
    device = 0 if gpu else "cpu"
    print(f"YOLO device: {torch.cuda.get_device_name(0) if gpu else 'CPU'}", file=sys.stderr, flush=True)
    model = YOLO("yolo11n.pt")
    capture = LatestFrame(args.source)
    ocr = AsyncOCR()
    prev = 0
    analyzed = 0
    skipped = 0
    since = time.monotonic()
    last_ocr_completed = 0
    try:
        while True:
            sequence, frame = capture.next(prev)
            skipped += max(0, sequence - prev - 1)
            prev = sequence
            if sequence % max(1, args.every):
                continue
            height, width = frame.shape[:2]
            start = time.monotonic()
            result = model.track(
                frame, conf=0.4, imgsz=args.imgsz,
                device=device, persist=True,
                tracker="bytetrack.yaml", verbose=False,
            )[0]
            boxes = []
            for b in result.boxes:
                if model.names[int(b.cls[0])] not in VEHICLES:
                    continue
                xyxy = [int(v) for v in b.xyxy[0].tolist()]
                track_id = int(b.id[0]) if b.id is not None else None
                # OCR caching requires a stable tracker ID.
                candidate = None
                if track_id is not None:
                    x1, y1, x2, y2 = xyxy
                    crop = frame[max(0, y1):max(0, y2), max(0, x1):max(0, x2)]
                    if crop.size:
                        ocr.submit(track_id, crop, args.ocr_interval)
                    candidate = ocr.lookup(track_id)
                boxes.append({
                    "box": xyxy, "track_id": track_id,
                    "confidence": round(float(b.conf[0]), 3),
                    "number": candidate[0] if candidate else None,
                    "ocr_confidence": round(candidate[1] / 100, 3) if candidate else None,
                })
            analyzed += 1
            now = time.monotonic()
            elapsed = now - since
            if elapsed >= 5:
                print(
                    f"Capture {capture.captured / elapsed:.1f} fps | "
                    f"YOLO {analyzed / elapsed:.1f} fps | "
                    f"skipped {skipped} frames | "
                    f"OCR {ocr.completed - last_ocr_completed} jobs | "
                    f"latest inference {now - start:.3f}s",
                    file=sys.stderr, flush=True,
                )
                with capture.condition:
                    capture.captured = 0
                since = now
                analyzed = 0
                skipped = 0
                last_ocr_completed = ocr.completed
            print(json.dumps({
                "timestamp": time.time(), "frame_sequence": sequence,
                "width": width, "height": height, "detections": boxes,
            }), flush=True)
    finally:
        capture.stop()


if __name__ == "__main__":
    try:
        main()
    except (BrokenPipeError, KeyboardInterrupt):
        pass
    except Exception as exc:
        print(f"Detection worker failed: {exc}", file=sys.stderr)
        sys.exit(1)
