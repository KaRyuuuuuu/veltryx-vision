"""Local-only SQLite snapshot index and asynchronous WebP persistence.

One snapshot per observed ByteTrack ID, delayed briefly for OCR classification.
Never blocks CUDA inference on SQLite or disk I/O.
"""
from datetime import datetime, timezone
from pathlib import Path
import queue
import sqlite3
import sys
import threading
import time
import uuid

import cv2

SESSION = uuid.uuid4().hex[:8]


class CaptureStore:
    def __init__(self, root, wait_seconds=1.5):
        self.root = Path(root).expanduser().resolve()
        self.root.mkdir(parents=True, exist_ok=True)
        self.wait_seconds = wait_seconds
        self.first_seen = {}
        self.saved = set()
        self.pending = set()
        self.jobs = queue.Queue(maxsize=16)
        self.lock = threading.Lock()
        self.thread = threading.Thread(target=self._writer, daemon=True)
        self.thread.start()

    def observe(self, track_id, image, bbox, number, confidence, utc_ms):
        if track_id is None:
            return
        key = (SESSION, track_id)
        now = time.monotonic()
        with self.lock:
            self.first_seen.setdefault(key, now)
            if key in self.saved or key in self.pending:
                return
            if not number and now - self.first_seen[key] < self.wait_seconds:
                return
            self.pending.add(key)
        try:
            self.jobs.put_nowait((key, image.copy(), bbox, number, confidence, utc_ms))
        except queue.Full:
            with self.lock:
                self.pending.discard(key)

    def _writer(self):
        con = sqlite3.connect(self.root / "detections.sqlite3", timeout=10)
        try:
            con.execute("PRAGMA journal_mode=WAL")
            con.execute("""CREATE TABLE IF NOT EXISTS captures (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                session_id TEXT NOT NULL,
                track_id INTEGER NOT NULL,
                captured_utc_ms INTEGER NOT NULL,
                number_candidate TEXT,
                ocr_confidence REAL,
                image_path TEXT NOT NULL,
                box_json TEXT NOT NULL,
                UNIQUE(session_id, track_id)
            )""")
            con.commit()
            while True:
                key, frame, bbox, number, confidence, utc_ms = self.jobs.get()
                try:
                    import json
                    name = f"car-{number}" if number else f"car-unknown-{key[1]}"
                    folder = self.root / "captures" / name
                    folder.mkdir(parents=True, exist_ok=True)
                    stamp = datetime.fromtimestamp(utc_ms / 1000, timezone.utc).strftime("%Y%m%dT%H%M%S")
                    relative = Path("captures") / name / f"{stamp}.{utc_ms % 1000:03d}Z-{SESSION}-{key[1]}.webp"
                    absolute = self.root / relative
                    ok = cv2.imwrite(str(absolute), frame, [cv2.IMWRITE_WEBP_QUALITY, 82])
                    if not ok:
                        raise RuntimeError("WebP encoding failed")
                    try:
                        con.execute("""INSERT INTO captures
                            (session_id, track_id, captured_utc_ms, number_candidate,
                             ocr_confidence, image_path, box_json)
                            VALUES (?, ?, ?, ?, ?, ?, ?)""",
                            (key[0], key[1], utc_ms, number, confidence,
                             relative.as_posix(), json.dumps(bbox)))
                        con.commit()
                    except Exception:
                        absolute.unlink(missing_ok=True)
                        raise
                    with self.lock:
                        self.saved.add(key)
                except Exception as exc:
                    print(f"Capture save warning: {exc}", file=sys.stderr, flush=True)
                finally:
                    with self.lock:
                        self.pending.discard(key)
                    self.jobs.task_done()
        finally:
            con.close()
