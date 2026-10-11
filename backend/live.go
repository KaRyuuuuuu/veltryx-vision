package main

import (
 "context"
 "encoding/binary"
 "io"
 "log"
 "mime/multipart"
 "net/http"
 "net/textproto"
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
  // JPEG packets travel through fd 3, not stdout (libraries can print to stdout).
  readEnd, writeEnd, err := os.Pipe()
  if err != nil { http.Error(w, "cannot create frame pipe", 500); return }
  cmd.ExtraFiles = []*os.File{writeEnd}
  cmd.Stdout = os.Stderr
  cmd.Stderr = os.Stderr
  if err := cmd.Start(); err != nil {
    _ = readEnd.Close()
    _ = writeEnd.Close()
    http.Error(w, "cannot start worker", 500)
    return
  }
  _ = writeEnd.Close()
  defer func(){ _ = readEnd.Close(); cancel(); _ = cmd.Wait() }()
  boundary := "veltryxvision"
  w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
  w.Header().Set("Cache-Control", "no-store")
  w.Header().Set("X-Content-Type-Options", "nosniff")
  mw := multipart.NewWriter(w)
  _ = mw.SetBoundary(boundary)
  reader := readEnd
  flusher, _ := w.(http.Flusher)
  for {
   var size uint32
   if err := binary.Read(reader, binary.BigEndian, &size); err != nil { break }
   if size == 0 || size > 8*1024*1024 { log.Printf("invalid live frame size: %d",size); break }
   frame := make([]byte, size)
   if _, err := io.ReadFull(reader, frame); err != nil { break }
   part, err := mw.CreatePart(textproto.MIMEHeader{
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
