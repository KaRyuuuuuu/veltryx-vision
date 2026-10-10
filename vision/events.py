#!/usr/bin/env python3
"""Independent inference feed; stdout JSONL, stderr diagnostics.
Not frame-synchronized to WebRTC playback (preview only)."""
import argparse
import json
import os
import re
import sys
import time

os.environ.setdefault("OPENCV_FFMPEG_CAPTURE_OPTIONS", "rtsp_transport;tcp")
import cv2
import pytesseract
from ultralytics import YOLO


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--source", required=True)
    p.add_argument("--every", type=int, default=6)
    args = p.parse_args()
    model = YOLO("yolo11n.pt")
    cap = cv2.VideoCapture(args.source, cv2.CAP_FFMPEG)
    if not cap.isOpened():
        raise RuntimeError("Cannot open RTSP for detection")
    i = 0
    try:
        while True:
            ok, frame = cap.read()
            if not ok:
                raise RuntimeError("RTSP disconnected")
            i += 1
            if i % max(1,args.every):
                continue
            h,w = frame.shape[:2]
            boxes = []
            prediction = model.predict(frame, conf=0.4, imgsz=640, verbose=False)[0]
            for b in prediction.boxes:
                if model.names[int(b.cls[0])] not in ("car","truck","bus","motorcycle"):
                    continue
                x1,y1,x2,y2 = [int(v) for v in b.xyxy[0].tolist()]
                item = {"box":[x1,y1,x2,y2], "confidence":round(float(b.conf[0]),3), "number":None, "ocr_confidence":None}
                crop = frame[max(0,y1):max(0,y2),max(0,x1):max(0,x2)]
                if crop.size:
                    gray = cv2.cvtColor(crop,cv2.COLOR_BGR2GRAY)
                    gray = cv2.resize(gray,None,fx=2,fy=2,interpolation=cv2.INTER_CUBIC)
                    data = pytesseract.image_to_data(gray,config="--psm 11 -c tessedit_char_whitelist=0123456789",output_type=pytesseract.Output.DICT)
                    nums = [(re.sub(r"\D","",t),float(c)) for t,c in zip(data["text"],data["conf"])]
                    nums = [(t,c) for t,c in nums if 1 <= len(t) <= 4 and c >= 40]
                    if nums:
                        item["number"],score=max(nums,key=lambda x:x[1])
                        item["ocr_confidence"]=round(score/100,3)
                boxes.append(item)
            print(json.dumps({"timestamp":time.time(),"width":w,"height":h,"detections":boxes}),flush=True)
    finally:
        cap.release()


if __name__=="__main__":
    try:
        main()
    except (BrokenPipeError,KeyboardInterrupt):
        pass
    except Exception as e:
        print(f"Detection worker failed: {e}",file=sys.stderr)
        sys.exit(1)
