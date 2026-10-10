#!/usr/bin/env python3
"""Veltryx Vision headless-capable video display with persistent Cairo overlay.
Only local loopback control API; experimental, not an official timing source.
"""
import argparse
import json
import threading
from datetime import datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import gi
gi.require_version("Gst", "1.0")
gi.require_version("GLib", "2.0")
from gi.repository import Gst, GLib


class OverlayState:
    def __init__(self, label):
        self.lock = threading.Lock()
        self.label = label
        self.event = "READY"
        self.car = ""
        self.flag = "UNKNOWN"

    def update(self, data):
        if not isinstance(data, dict):
            raise ValueError("JSON object required")
        for key in ("type", "car", "flag"):
            value = data.get(key, "")
            if not isinstance(value, str) or len(value) > 40:
                raise ValueError("type/car/flag must be strings up to 40 chars")
        with self.lock:
            self.event = data.get("type", "READY") or "READY"
            self.car = data.get("car", "")
            self.flag = data.get("flag", "UNKNOWN") or "UNKNOWN"

    def snapshot(self):
        with self.lock:
            return dict(label=self.label, event=self.event, car=self.car, flag=self.flag)


def make_server(state, port):
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            if self.path != "/state":
                self.send_error(404)
                return
            data = json.dumps(state.snapshot()).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def do_POST(self):
            if self.path != "/event":
                self.send_error(404)
                return
            try:
                size = int(self.headers.get("Content-Length", "0"))
                if size < 1 or size > 2048:
                    raise ValueError("invalid payload size")
                data = json.loads(self.rfile.read(size))
                state.update(data)
            except (ValueError, json.JSONDecodeError) as err:
                self.send_error(400, str(err))
                return
            self.send_response(204)
            self.end_headers()

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", port), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server


def draw_overlay(overlay, cairo_ctx, timestamp, duration, state):
    # Cairo's video overlay units match the decoded frame resolution.
    _, _, width, height = overlay.get_property("caps").get_structure(0).get_fraction("framerate") if False else (0, 0, 0, 0)
    # Use dimensions reported by caps from the signal.
    caps = overlay.get_property("caps")
    if caps is None:
        return
    structure = caps.get_structure(0)
    ok_w, width = structure.get_int("width")
    ok_h, height = structure.get_int("height")
    if not ok_w or not ok_h:
        return
    state_data = state.snapshot()
    scale = max(0.65, width / 1280.0)
    band_h = 60 * scale
    cairo_ctx.set_source_rgba(0.02, 0.06, 0.1, 0.82)
    cairo_ctx.rectangle(0, 0, width, band_h)
    cairo_ctx.fill()
    cairo_ctx.select_font_face("Sans", 0, 1)
    cairo_ctx.set_font_size(26 * scale)
    cairo_ctx.set_source_rgb(1, 1, 1)
    now = datetime.now().strftime("%H:%M:%S")
    car = (" #" + state_data["car"]) if state_data["car"] else ""
    text = f'{now}  |  {state_data["label"]}  |  {state_data["event"]}{car}  |  {state_data["flag"]}'
    # Scale down to avoid clipping on narrow sensors.
    extents = cairo_ctx.text_extents(text)
    text_width = extents.width if hasattr(extents, "width") else extents[2]
    if text_width > width - 24 * scale:
        cairo_ctx.set_font_size(max(12, 26 * scale * (width - 24 * scale) / text_width))
    cairo_ctx.move_to(12 * scale, 40 * scale)
    cairo_ctx.show_text(text)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["window", "kms"], default="window")
    parser.add_argument("--source", default="test", help="test or rtsp://127.0.0.1:8554/h264")
    parser.add_argument("--label", default="FINISH")
    parser.add_argument("--port", type=int, default=8095)
    args = parser.parse_args()
    if args.source != "test" and not (
        args.source.startswith("rtsp://127.0.0.1:") or
        args.source.startswith("rtsp://localhost:")):
        parser.error("Only local RTSP tunnel URLs are supported in this prototype")
    Gst.init(None)
    sink = "autovideosink sync=true" if args.mode == "window" else "kmssink sync=true"
    if args.source == "test":
        source = "videotestsrc is-live=true pattern=ball ! video/x-raw,framerate=60/1"
    else:
        source = f'uridecodebin uri="{args.source}"'
    # Cairo overlay expects an appropriate raw RGB pixel format; videoconvert negotiates it.
    pipeline_text = f"{source} ! videoconvert ! cairooverlay name=overlay ! videoconvert ! {sink}"
    print("GStreamer pipeline:", pipeline_text, flush=True)
    pipeline = Gst.parse_launch(pipeline_text)
    state = OverlayState(args.label)
    server = make_server(state, args.port)
    overlay = pipeline.get_by_name("overlay")
    overlay.connect("draw", draw_overlay, state)
    bus = pipeline.get_bus()
    bus.add_signal_watch()
    loop = GLib.MainLoop()

    def on_bus(_bus, message):
        if message.type == Gst.MessageType.ERROR:
            err, debug = message.parse_error()
            print(f"GStreamer ERROR: {err} ({debug})", flush=True)
            loop.quit()
        elif message.type == Gst.MessageType.EOS:
            print("Video stream ended", flush=True)
            loop.quit()

    bus.connect("message", on_bus)
    try:
        if pipeline.set_state(Gst.State.PLAYING) == Gst.StateChangeReturn.FAILURE:
            raise RuntimeError("Cannot start video pipeline")
        print(f"Overlay API on http://127.0.0.1:{args.port}/state", flush=True)
        loop.run()
    except KeyboardInterrupt:
        pass
    finally:
        pipeline.set_state(Gst.State.NULL)
        server.shutdown()


if __name__ == "__main__":
    main()
