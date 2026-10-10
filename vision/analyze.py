#!/usr/bin/env python3
"""Offline prototype: vehicle bounding boxes from YOLO, tentative number OCR.
Outputs JSON on stdout only; stderr is used for logs."""
import argparse
import json
import re
import sys

import cv2
import pytesseract
from ultralytics import YOLO


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--stride", type=int, default=10)
    parser.add_argument("--max-frames", type=int, default=300)
    args = parser.parse_args()
    if args.stride < 1 or args.max_frames < 1:
        parser.error("stride and max-frames must be positive")
    model = YOLO("yolo11n.pt")
    video = cv2.VideoCapture(args.input)
    if not video.isOpened():
        raise RuntimeError("Cannot open video")
    fps = video.get(cv2.CAP_PROP_FPS) or 0
    candidates = []
    idx = 0
    try:
        while idx < args.max_frames:
            ok, frame = video.read()
            if not ok:
                break
            if idx % args.stride == 0:
                results = model.predict(frame, imgsz=640, conf=0.35, verbose=False)
                for box in results[0].boxes:
                    cls = int(box.cls[0])
                    if model.names[cls] not in ("car", "truck", "bus", "motorcycle"):
                        continue
                    x1, y1, x2, y2 = [int(x) for x in box.xyxy[0].tolist()]
                    crop = frame[max(0,y1):max(0,y2), max(0,x1):max(0,x2)]
                    if crop.size == 0:
                        continue
                    gray = cv2.cvtColor(crop, cv2.COLOR_BGR2GRAY)
                    gray = cv2.resize(gray, None, fx=2, fy=2, interpolation=cv2.INTER_CUBIC)
                    gray = cv2.equalizeHist(gray)
                    data = pytesseract.image_to_data(
                        gray, config="--psm 11 -c tessedit_char_whitelist=0123456789",
                        output_type=pytesseract.Output.DICT)
                    for txt, confidence in zip(data["text"], data["conf"]):
                        number = re.sub(r"\D", "", txt)
                        if 1 <= len(number) <= 4 and float(confidence) >= 35:
                            candidates.append({
                                "frame":idx, "seconds":round(idx/fps,3) if fps else 0,
                                "number":number, "confidence":round(float(confidence)/100,3),
                                "box":[x1,y1,x2,y2]
                            })
            idx += 1
    finally:
        video.release()
    print(json.dumps({"detections":candidates}))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"Vision worker error: {exc}", file=sys.stderr)
        sys.exit(1)
