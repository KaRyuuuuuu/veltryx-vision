#!/usr/bin/env python3
"""Local-only MJPEG frames with YOLO bounding boxes and tentative OCR labels.
FD 3 protocol: 4-byte network-endian JPEG length followed by JPEG bytes.
Each browser viewer starts its own worker; intended for ONE development viewer.
"""
import argparse
import os
import re
import struct
import sys
import time

os.environ.setdefault("OPENCV_FFMPEG_CAPTURE_OPTIONS", "rtsp_transport;tcp")
import cv2
import pytesseract
import torch
from ultralytics import YOLO


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", required=True)
    args = parser.parse_args()
    model = YOLO("yolo11n.pt")
    gpu = torch.cuda.is_available()
    device = 0 if gpu else "cpu"
    print(f"Live YOLO on {torch.cuda.get_device_name(0) if gpu else 'CPU'}", file=sys.stderr)
    cap = cv2.VideoCapture(args.source, cv2.CAP_FFMPEG)
    if not cap.isOpened():
        raise RuntimeError("RTSP stream unavailable. Check SSH tunnel on :8554")
    frame_out = os.fdopen(3, "wb", buffering=0)
    last_boxes = []
    n = 0
    last_emit = 0.0
    try:
        while True:
            ok, image = cap.read()
            if not ok:
                raise RuntimeError("RTSP stream closed")
            n += 1
            if n % 5 == 1:
                last_boxes = []
                prediction = model.predict(image, imgsz=640, conf=0.35, device=device, verbose=False)[0]
                for b in prediction.boxes:
                    cls = int(b.cls[0])
                    if model.names[cls] not in ("car", "truck", "bus", "motorcycle"):
                        continue
                    x1, y1, x2, y2 = map(int, b.xyxy[0].tolist())
                    label = "Vehicule"
                    if n % 20 == 1:
                        crop = image[max(y1, 0):max(y2, 0), max(x1, 0):max(x2, 0)]
                        if crop.size:
                            gray = cv2.cvtColor(crop, cv2.COLOR_BGR2GRAY)
                            gray = cv2.resize(gray, None, fx=2, fy=2)
                            try:
                                data = pytesseract.image_to_data(gray,
                                    config="--psm 11 -c tessedit_char_whitelist=0123456789",
                                    output_type=pytesseract.Output.DICT)
                                ranked = [(re.sub(r"\D", "", t), float(c))
                                          for t, c in zip(data["text"], data["conf"])]
                                ranked = [(t, c) for t, c in ranked if 1 <= len(t) <= 4 and c >= 40]
                                if ranked:
                                    value, score = max(ranked, key=lambda x: x[1])
                                    label = f"#{value}? {score:.0f}%"
                            except Exception as e:
                                print(f"OCR warning: {e}", file=sys.stderr)
                    last_boxes.append((x1, y1, x2, y2, label))
            # Annotate only the preview copy, never overwrite raw footage.
            for x1, y1, x2, y2, label in last_boxes:
                cv2.rectangle(image, (x1,y1), (x2,y2), (0,220,120), 2)
                cv2.putText(image,label,(x1,max(20,y1-7)),cv2.FONT_HERSHEY_SIMPLEX,
                            0.6,(0,220,120),2,cv2.LINE_AA)
            now = time.monotonic()
            if now - last_emit < 0.15:
                continue
            last_emit = now
            ok, encoded = cv2.imencode(".jpg", image, [cv2.IMWRITE_JPEG_QUALITY, 75])
            if not ok:
                continue
            payload = encoded.tobytes()
            frame_out.write(struct.pack("!I",len(payload)))
            frame_out.write(payload)
            frame_out.flush()
    finally:
        cap.release()
        frame_out.close()


if __name__ == "__main__":
    try:
        main()
    except (BrokenPipeError, KeyboardInterrupt):
        pass
    except Exception as error:
        print(f"Live worker error: {error}", file=sys.stderr)
        sys.exit(1)
