#!/usr/bin/env python3
"""Veltryx Line Monitor: Qt desktop UI for motion-triggered RTSP captures.
No YOLO, OCR, or CUDA. Candidate snapshots are not official timing.
"""
import os
os.environ.setdefault("OPENCV_FFMPEG_CAPTURE_OPTIONS", "rtsp_transport;tcp")
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
import cv2
import numpy as np
from PySide6.QtCore import Qt, QThread, Signal, QTimer, QSize
from PySide6.QtGui import QImage, QPixmap, QIcon
from PySide6.QtWidgets import (
    QApplication, QMainWindow, QWidget, QLabel, QPushButton, QHBoxLayout,
    QVBoxLayout, QGridLayout, QFrame, QSplitter, QListWidget, QListWidgetItem,
    QSpinBox, QMessageBox, QLineEdit, QGroupBox, QSizePolicy
)

ROOT = Path(__file__).resolve().parent.parent
CAPTURES = ROOT / "local-data" / "line-crossings"
REFERENCE = (1649, 928)
POLYGON = np.array([(215, 485), (355, 485), (575, 905), (345, 905)], dtype=np.float32)


def image_to_pixmap(image):
    rgb = cv2.cvtColor(image, cv2.COLOR_BGR2RGB)
    h, w, _ = rgb.shape
    return QPixmap.fromImage(QImage(rgb.data, w, h, 3*w, QImage.Format_RGB888).copy())


class Detector(QThread):
    frame = Signal(object)
    captured = Signal(str)
    metrics = Signal(int, str)
    failure = Signal(str)

    def __init__(self, source, threshold, confirm):
        super().__init__()
        self.source = source
        self.threshold = threshold
        self.confirm = confirm
        self.running = True

    def stop(self):
        self.running = False

    def run(self):
        cap = cv2.VideoCapture(self.source, cv2.CAP_FFMPEG)
        if not cap.isOpened():
            self.failure.emit("Flux RTSP inaccessible. Vérifie le tunnel SSH.")
            return
        bg = cv2.createBackgroundSubtractorMOG2(history=180, varThreshold=38, detectShadows=True)
        mask = None
        points = None
        dims = None
        consecutive = 0
        ready = True
        warmup_until = time.monotonic() + 3
        last_preview = 0.0
        kernel = np.ones((3,3), np.uint8)
        try:
            while self.running:
                ok, img = cap.read()
                if not ok:
                    self.failure.emit("Connexion RTSP interrompue.")
                    break
                h, w = img.shape[:2]
                if dims != (w,h):
                    dims = (w,h)
                    points = (POLYGON * [w/REFERENCE[0], h/REFERENCE[1]]).round().astype(np.int32)
                    mask = np.zeros((h,w), np.uint8)
                    cv2.fillPoly(mask, [points], 255)
                fg = bg.apply(cv2.cvtColor(img, cv2.COLOR_BGR2GRAY))
                fg = cv2.bitwise_and(fg, mask)
                _, fg = cv2.threshold(fg, 250, 255, cv2.THRESH_BINARY)
                fg = cv2.morphologyEx(fg, cv2.MORPH_OPEN, kernel)
                pixels = cv2.countNonZero(fg)
                motion = pixels >= self.threshold
                consecutive = consecutive + 1 if motion else 0
                if not motion:
                    ready = True
                state = "WARMUP" if time.monotonic() < warmup_until else ("MOTION" if motion else "READY")
                if ready and consecutive >= self.confirm and state != "WARMUP":
                    ts = datetime.now(timezone.utc)
                    folder = CAPTURES / ts.strftime("%Y-%m-%d")
                    folder.mkdir(parents=True, exist_ok=True)
                    path = folder / (ts.strftime("%Y%m%dT%H%M%S.%f")[:-3] + "Z.webp")
                    if cv2.imwrite(str(path), img, [cv2.IMWRITE_WEBP_QUALITY, 85]):
                        self.captured.emit(str(path))
                        ready = False
                now = time.monotonic()
                if now-last_preview > 1/15:
                    # Draw on preview only; persist the pristine camera frame.
                    view = img.copy()
                    cv2.polylines(view, [points], True, (0, 208, 255), 2, cv2.LINE_AA)
                    self.frame.emit(view)
                    self.metrics.emit(pixels, state)
                    last_preview = now
        except Exception as exc:
            self.failure.emit(str(exc))
        finally:
            cap.release()


class Monitor(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("Veltryx Vision · Line Monitor")
        self.resize(1440, 880)
        self.worker = None
        self.last_path = None
        self.known_paths = []
        self.make_ui()
        self.gallery_timer = QTimer(self)
        self.gallery_timer.timeout.connect(self.refresh_gallery)
        self.gallery_timer.start(1500)
        self.refresh_gallery()

    def make_ui(self):
        self.setStyleSheet("""
        QMainWindow,QWidget {background:#0a101c;color:#e5eef9;font-family:'Inter','Sans Serif';font-size:13px}
        QFrame#panel,QGroupBox {background:#101d2e;border:1px solid #263a53;border-radius:12px}
        QGroupBox {margin-top:14px;padding:16px 10px 10px 10px;font-weight:600}
        QGroupBox::title {subcontrol-origin:margin;left:15px;padding:0 6px;color:#93bedc}
        QLineEdit,QSpinBox,QListWidget {background:#0a1525;border:1px solid #30465e;border-radius:7px;padding:7px}
        QPushButton {background:#167cc2;border:0;border-radius:8px;padding:11px 17px;font-weight:700}
        QPushButton:hover {background:#2196e3}
        QPushButton#secondary {background:#21344c}
        QPushButton#secondary:hover {background:#31506e}
        QLabel#muted {color:#8fa6bd}
        QLabel#metric {font-size:24px;font-weight:800;color:#b8e6ff}
        QListWidget::item {padding:6px;border-bottom:1px solid #203146}
        QListWidget::item:selected {background:#21527a}
        """)
        root = QWidget()
        self.setCentralWidget(root)
        vertical = QVBoxLayout(root)
        vertical.setContentsMargins(20,18,20,18)
        vertical.setSpacing(12)
        header = QHBoxLayout()
        title = QLabel("VELTRYX   /   LINE MONITOR")
        title.setStyleSheet("font-size:23px;font-weight:800;letter-spacing:1px")
        self.status = QLabel("●  STOPPED")
        self.status.setStyleSheet("color:#f8ba77;font-size:16px;font-weight:700")
        header.addWidget(title)
        header.addStretch()
        header.addWidget(self.status)
        vertical.addLayout(header)
        subtitle = QLabel("Surveillance de la ligne · captures locales WebP · sans IA / OCR · non homologué pour le chronométrage")
        subtitle.setObjectName("muted")
        vertical.addWidget(subtitle)
        split = QSplitter(Qt.Horizontal)
        vertical.addWidget(split,1)
        left = QFrame()
        left.setObjectName("panel")
        left_l = QVBoxLayout(left)
        self.camera = QLabel("CAMÉRA HORS LIGNE\n\nDémarre le tunnel RTSP, puis la surveillance")
        self.camera.setAlignment(Qt.AlignCenter)
        self.camera.setStyleSheet("background:#040b14;border-radius:8px;color:#8099b2;font-size:17px")
        self.camera.setMinimumHeight(280)
        self.camera.setSizePolicy(QSizePolicy.Expanding,QSizePolicy.Expanding)
        left_l.addWidget(self.camera,1)
        metrics = QHBoxLayout()
        self.motion = QLabel("0 px")
        self.motion.setObjectName("metric")
        self.saved = QLabel("0")
        self.saved.setObjectName("metric")
        for name, metric in (("MOUVEMENT",self.motion),("CAPTURES SESSION",self.saved)):
            container=QVBoxLayout()
            label=QLabel(name)
            label.setObjectName("muted")
            container.addWidget(label)
            container.addWidget(metric)
            metrics.addLayout(container)
        metrics.addStretch()
        left_l.addLayout(metrics)
        controls = QGroupBox("Contrôle de l'acquisition")
        ctl = QGridLayout(controls)
        ctl.addWidget(QLabel("Source RTSP"),0,0)
        self.source=QLineEdit("rtsp://127.0.0.1:8554/h264")
        ctl.addWidget(self.source,0,1,1,3)
        ctl.addWidget(QLabel("Seuil pixels"),1,0)
        self.threshold=QSpinBox()
        self.threshold.setRange(100,100000)
        self.threshold.setValue(900)
        ctl.addWidget(self.threshold,1,1)
        ctl.addWidget(QLabel("Images de confirmation"),1,2)
        self.confirm=QSpinBox()
        self.confirm.setRange(1,10)
        self.confirm.setValue(2)
        ctl.addWidget(self.confirm,1,3)
        self.startbtn=QPushButton("Démarrer la surveillance")
        self.startbtn.clicked.connect(self.toggle)
        ctl.addWidget(self.startbtn,2,0,1,4)
        left_l.addWidget(controls)
        split.addWidget(left)
        right=QFrame()
        right.setObjectName("panel")
        rl=QVBoxLayout(right)
        label=QLabel("GALERIE   /   CAPTURES")
        label.setStyleSheet("font-size:17px;font-weight:800;color:#b8e6ff")
        rl.addWidget(label)
        hint=QLabel("Actualisation automatique · sélectionne une image pour l'agrandir")
        hint.setObjectName("muted")
        hint.setWordWrap(True)
        rl.addWidget(hint)
        self.gallery=QListWidget()
        self.gallery.setIconSize(QSize(90,55))
        self.gallery.currentItemChanged.connect(self.show_capture)
        rl.addWidget(self.gallery,1)
        self.detail=QLabel("Aucune capture sélectionnée")
        self.detail.setAlignment(Qt.AlignCenter)
        self.detail.setStyleSheet("background:#050e19;border-radius:9px;color:#8da4bd")
        self.detail.setMinimumHeight(190)
        rl.addWidget(self.detail,1)
        self.selection=QLabel(" ")
        self.selection.setObjectName("muted")
        self.selection.setWordWrap(True)
        rl.addWidget(self.selection)
        openbtn=QPushButton("Ouvrir le dossier de captures")
        openbtn.setObjectName("secondary")
        openbtn.clicked.connect(self.open_folder)
        rl.addWidget(openbtn)
        split.addWidget(right)
        split.setSizes([880,470])
        foot=QLabel("Une image par transition de mouvement. Deux voitures sans intervalle libre peuvent être fusionnées. Horodatage de sauvegarde, pas temps officiel.")
        foot.setObjectName("muted")
        foot.setWordWrap(True)
        vertical.addWidget(foot)
        self.count=0

    def toggle(self):
        if self.worker:
            self.stop_worker()
            return
        self.worker=Detector(self.source.text().strip(),self.threshold.value(),self.confirm.value())
        self.worker.frame.connect(self.show_frame)
        self.worker.metrics.connect(self.show_metrics)
        self.worker.captured.connect(self.new_capture)
        self.worker.failure.connect(self.worker_failed)
        self.worker.finished.connect(self.worker_finished)
        self.startbtn.setText("Arrêter la surveillance")
        self.source.setEnabled(False)
        self.threshold.setEnabled(False)
        self.confirm.setEnabled(False)
        self.status.setText("●  CONNECTING")
        self.worker.start()

    def stop_worker(self):
        if self.worker:
            self.worker.stop()
            # Keep Qt thread alive until OpenCV releases its reader.
            # A stuck RTSP read may delay shutdown; never destroy a running QThread.
            self.worker.wait()
        self.status.setText("●  STOPPED")
        self.startbtn.setText("Démarrer la surveillance")
        self.source.setEnabled(True)
        self.threshold.setEnabled(True)
        self.confirm.setEnabled(True)

    def worker_finished(self):
        self.worker=None
        self.startbtn.setText("Démarrer la surveillance")
        self.source.setEnabled(True)
        self.threshold.setEnabled(True)
        self.confirm.setEnabled(True)
        self.status.setText("●  STOPPED")

    def worker_failed(self, message):
        self.status.setText("●  ERROR")
        QMessageBox.warning(self,"Problème de capture",message)

    def show_frame(self, frame):
        self.current_frame=frame
        self.resize_image()

    def resize_image(self):
        if hasattr(self, "current_frame"):
            pix=image_to_pixmap(self.current_frame)
            self.camera.setPixmap(pix.scaled(self.camera.size(),Qt.KeepAspectRatio,Qt.SmoothTransformation))

    def show_metrics(self,pixels,state):
        self.motion.setText(f"{pixels:,} px".replace(","," "))
        color={"READY":"#55d9ab","MOTION":"#ffd166","WARMUP":"#93a7ba"}.get(state,"#9dd3ff")
        self.status.setStyleSheet(f"color:{color};font-size:16px;font-weight:700")
        self.status.setText("●  "+state)

    def new_capture(self,path):
        self.count+=1
        self.saved.setText(str(self.count))
        self.last_path=path
        self.refresh_gallery(select_new=True)

    def refresh_gallery(self,select_new=False):
        paths=sorted(CAPTURES.glob("*/*.webp"),key=lambda p:p.stat().st_mtime,reverse=True)[:200]
        labels=[str(p) for p in paths]
        if labels==self.known_paths:
            return
        previous=self.gallery.currentItem().data(Qt.UserRole) if self.gallery.currentItem() else None
        self.known_paths=labels
        self.gallery.blockSignals(True)
        self.gallery.clear()
        for p in paths:
            pix=QPixmap(str(p))
            item=QListWidgetItem(QIcon(pix.scaled(90,55,Qt.KeepAspectRatio,Qt.SmoothTransformation)),p.name)
            item.setData(Qt.UserRole,str(p))
            self.gallery.addItem(item)
        self.gallery.blockSignals(False)
        selected=self.last_path if select_new else previous
        for i,path in enumerate(labels):
            if path==selected:
                self.gallery.setCurrentRow(i)
                break
        else:
            if self.gallery.count():
                self.gallery.setCurrentRow(0)

    def show_capture(self,current,previous):
        if not current:
            return
        path=current.data(Qt.UserRole)
        self.selected_pix=QPixmap(path)
        self.detail.setPixmap(self.selected_pix.scaled(self.detail.size(),Qt.KeepAspectRatio,Qt.SmoothTransformation))
        self.selection.setText(str(path))

    def resizeEvent(self,event):
        super().resizeEvent(event)
        self.resize_image()
        if hasattr(self,"selected_pix"):
            self.detail.setPixmap(self.selected_pix.scaled(self.detail.size(),Qt.KeepAspectRatio,Qt.SmoothTransformation))

    def open_folder(self):
        from PySide6.QtCore import QUrl
        from PySide6.QtGui import QDesktopServices
        CAPTURES.mkdir(parents=True,exist_ok=True)
        QDesktopServices.openUrl(QUrl.fromLocalFile(str(CAPTURES)))

    def closeEvent(self,event):
        self.stop_worker()
        event.accept()


if __name__=="__main__":
    app=QApplication(sys.argv)
    window=Monitor()
    window.show()
    sys.exit(app.exec())
