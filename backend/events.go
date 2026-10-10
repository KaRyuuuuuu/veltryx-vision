package main

import (
 "bufio"
 "context"
 "encoding/json"
 "log"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "time"
)

func registerEvents(mux *http.ServeMux, python string) {
 source:=os.Getenv("VISION_LIVE_SOURCE")
 if source=="" {source="rtsp://127.0.0.1:8554/h264"}
 if !strings.HasPrefix(source,"rtsp://127.0.0.1:") && !strings.HasPrefix(source,"rtsp://localhost:") {
  log.Fatal("VISION_LIVE_SOURCE must use local SSH tunnel")
 }
 script,err:=filepath.Abs("../vision/events.py");if err!=nil{log.Fatal(err)}
 mux.HandleFunc("GET /api/detections",func(w http.ResponseWriter,r *http.Request){
  ctx,cancel:=context.WithCancel(r.Context());defer cancel()
  cmd:=exec.CommandContext(ctx,python,script,"--source",source)
  pipe,err:=cmd.StdoutPipe();if err!=nil{http.Error(w,"worker pipe",500);return}
  cmd.Stderr=os.Stderr
  if err:=cmd.Start();err!=nil{http.Error(w,"worker start",500);return}
  defer func(){cancel();_ = cmd.Wait()}()
  w.Header().Set("Content-Type","text/event-stream")
  w.Header().Set("Cache-Control","no-cache")
  w.Header().Set("X-Accel-Buffering","no")
  f,_:=w.(http.Flusher)
  scanner:=bufio.NewScanner(pipe)
  scanner.Buffer(make([]byte,8192),2*1024*1024)
  for scanner.Scan(){
   line:=scanner.Bytes()
   if !json.Valid(line){continue}
   _,err=w.Write(append(append([]byte("data: "),line...),[]byte("\n\n")...))
   if err!=nil{break}
   if f!=nil{f.Flush()}
  }
  if err:=scanner.Err();err!=nil{log.Printf("detections stream: %v",err)}
  _=time.Now() // placeholder for future timestamp alignment
 })
}
