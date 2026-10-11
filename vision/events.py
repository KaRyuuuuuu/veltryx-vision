#!/usr/bin/env python3
"""Real-time vehicle tracking on CUDA with throttled candidate OCR.
SSE JSONL output on stdout; logs to stderr. Preview is not video-synchronized.
"""
import argparse
import json
import os
import re
import sys
import time
os.environ.setdefault("OPENCV_FFMPEG_CAPTURE_OPTIONS", "rtsp_transport;tcp")
import cv2
import pytesseract
import torch
from ultralytics import YOLO

VEHICLES = {"car", "truck", "bus", "motorcycle"}

def read_number(frame, xyxy):
    x1,y1,x2,y2 = xyxy
    crop = frame[max(0,y1):max(0,y2), max(0,x1):max(0,x2)]
    if crop.size == 0:
        return None
    gray = cv2.cvtColor(crop,cv2.COLOR_BGR2GRAY)
    gray = cv2.resize(gray,None,fx=2,fy=2,interpolation=cv2.INTER_CUBIC)
    data = pytesseract.image_to_data(
        gray,config="--psm 11 -c tessedit_char_whitelist=0123456789",
        output_type=pytesseract.Output.DICT)
    candidates = [(re.sub(r"\D","",v),float(c)) for v,c in zip(data["text"],data["conf"])]
    candidates = [(n,c) for n,c in candidates if 1 <= len(n) <= 4 and c >= 40]
    return max(candidates,key=lambda item:item[1]) if candidates else None

def main():
    p=argparse.ArgumentParser()
    p.add_argument("--source",required=True)
    p.add_argument("--every",type=int,default=3,help="analyze every Nth frame")
    p.add_argument("--ocr-interval",type=float,default=0.7)
    args=p.parse_args()
    gpu=torch.cuda.is_available()
    device=0 if gpu else "cpu"
    print(f"YOLO device: {torch.cuda.get_device_name(0) if gpu else 'CPU'}",file=sys.stderr)
    model=YOLO("yolo11n.pt")
    cap=cv2.VideoCapture(args.source,cv2.CAP_FFMPEG)
    if not cap.isOpened(): raise RuntimeError("Cannot open RTSP for detection")
    frame_index=0
    last_ocr={}
    cache={}
    next_stats=time.monotonic()+5
    measured=0
    try:
        while True:
            ok,frame=cap.read()
            if not ok: raise RuntimeError("RTSP disconnected")
            frame_index+=1
            if frame_index%max(1,args.every): continue
            height,width=frame.shape[:2]
            start=time.monotonic()
            prediction=model.track(frame,conf=0.4,imgsz=640,device=device,
                half=gpu,persist=True,tracker="bytetrack.yaml",verbose=False)[0]
            boxes=[]
            now=time.monotonic()
            for b in prediction.boxes:
                if model.names[int(b.cls[0])] not in VEHICLES: continue
                xyxy=[int(v) for v in b.xyxy[0].tolist()]
                track_id=int(b.id[0]) if b.id is not None else None
                key=track_id if track_id is not None else f"anonymous-{len(boxes)}"
                if key not in last_ocr or now-last_ocr[key]>=args.ocr_interval:
                    last_ocr[key]=now
                    candidate=read_number(frame,xyxy)
                    if candidate is not None:cache[key]=candidate
                candidate=cache.get(key)
                boxes.append({"box":xyxy,"track_id":track_id,
                    "confidence":round(float(b.conf[0]),3),
                    "number":candidate[0] if candidate else None,
                    "ocr_confidence":round(candidate[1]/100,3) if candidate else None})
            measured+=1
            if now>=next_stats:
                fps=measured/5
                print(f"YOLO processed {fps:.1f} fps, last frame {time.monotonic()-start:.3f}s",file=sys.stderr)
                next_stats=now+5
                measured=0
                # Prevent unbounded OCR cache if track ids disappear.
                active={b["track_id"] for b in boxes if b["track_id"] is not None}
                last_ocr={k:v for k,v in last_ocr.items() if not isinstance(k,int) or k in active}
                cache={k:v for k,v in cache.items() if not isinstance(k,int) or k in active}
            print(json.dumps({"timestamp":time.time(),"width":width,"height":height,
                "detections":boxes}),flush=True)
    finally:
        cap.release()

if __name__=="__main__":
    try: main()
    except (BrokenPipeError,KeyboardInterrupt):pass
    except Exception as e:
        print(f"Detection worker failed: {e}",file=sys.stderr)
        sys.exit(1)
