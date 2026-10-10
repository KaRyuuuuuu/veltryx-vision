package main

import (
 "bufio"
 "context"
 "encoding/binary"
 "io"
 "log"
 "mime/multipart"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "strconv"
 "strings"
)

func registerLive(mux *http.ServeMux, python string) {
 source := os.Getenv("VISION_LIVE_SOURCE")
 if source == "" { source = "rtsp://127.0.0.1:8554/h264" }
 // Development-only: fixed, server-configured source; never accept arbitrary URLs from HTTP clients.
 if !strings.HasPrefix(source, "rtsp://127.0.0.1:") && !strings.HasPrefix(source, "rtsp://localhost:") {
  log.Fatal("VISION_LIVE_SOURCE must be a loopback RTSP URL (via SSH tunnel)")
 }
 script, err := filepath.Abs("../vision/live.py")
 if err != nil { log.Fatal(err) }
 mux.HandleFunc("GET /api/live", func(w http.ResponseWriter, r *http.Request) {
  ctx, cancel := context.WithCancel(r.Context())
  defer cancel()
  cmd := exec.CommandContext(ctx, python, script, "--source", source)
  stdout, err := cmd.StdoutPipe()
  if err != nil { http.Error(w, "cannot start stream", 500); return }
  cmd.Stderr = os.Stderr
  if err := cmd.Start(); err != nil { http.Error(w, "cannot start worker", 500); return }
  defer func(){ cancel(); _ = cmd.Wait() }()
  boundary := "veltryxvision"
  w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
  w.Header().Set("Cache-Control", "no-store")
  w.Header().Set("X-Content-Type-Options", "nosniff")
  mw := multipart.NewWriter(w)
  _ = mw.SetBoundary(boundary)
  reader := bufio.NewReader(stdout)
  flusher, _ := w.(http.Flusher)
  for {
   var size uint32
   if err := binary.Read(reader, binary.BigEndian, &size); err != nil { break }
   if size == 0 || size > 8*1024*1024 { log.Print("invalid live frame size"); break }
   frame := make([]byte, size)
   if _, err := io.ReadFull(reader, frame); err != nil { break }
   part, err := mw.CreatePart(map[string][]string{
    "Content-Type":{"image/jpeg"},
    "Content-Length":{strconv.Itoa(len(frame))},
   })
   if err != nil { break }
   if _, err := part.Write(frame); err != nil { break }
   if flusher != nil { flusher.Flush() }
  }
  _ = mw.Close()
 })
}
