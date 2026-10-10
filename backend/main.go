package main

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "log"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "sync"
 "time"
)

type Detection struct {
 Frame int `json:"frame"`
 Seconds float64 `json:"seconds"`
 Number string `json:"number"`
 Confidence float64 `json:"confidence"`
 Box []int `json:"box"`
}
type Job struct {
 ID string `json:"id"`
 Source string `json:"source"`
 Status string `json:"status"`
 Error string `json:"error,omitempty"`
 Detections []Detection `json:"detections"`
 CreatedAt time.Time `json:"created_at"`
}
var state = struct {sync.Mutex; jobs map[string]*Job}{jobs:make(map[string]*Job)}

func main() {
 root := os.Getenv("VISION_MEDIA_ROOT")
 if root=="" { home,_:=os.UserHomeDir(); root=filepath.Join(home,"Videos") }
 absRoot,err:=filepath.EvalSymlinks(root)
 if err!=nil {log.Fatalf("media root unavailable %q: %v",root,err)}
 py:=os.Getenv("VISION_PYTHON"); if py=="" { py="python" }
 worker:=os.Getenv("VISION_WORKER"); if worker=="" {worker="../vision/analyze.py"}
 worker,err=filepath.Abs(worker); if err!=nil {log.Fatal(err)}
 mux:=http.NewServeMux()
 mux.HandleFunc("GET /api/health",func(w http.ResponseWriter,r *http.Request){respond(w,map[string]string{"status":"ok"})})
 mux.HandleFunc("GET /api/jobs",func(w http.ResponseWriter,r *http.Request){
  state.Lock();defer state.Unlock()
  out:=make([]Job,0,len(state.jobs));for _,j:=range state.jobs {out=append(out,*j)}
  respond(w,out)
 })
 mux.HandleFunc("POST /api/jobs",func(w http.ResponseWriter,r *http.Request){
  var req struct{Source string `json:"source"`}
  if err:=json.NewDecoder(http.MaxBytesReader(w,r.Body,8192)).Decode(&req);err!=nil{http.Error(w,"invalid request",400);return}
  src,err:=secureVideo(absRoot,req.Source)
  if err!=nil{http.Error(w,err.Error(),400);return}
  id:=fmt.Sprintf("%d",time.Now().UnixNano())
  job:=&Job{ID:id,Source:src,Status:"queued",CreatedAt:time.Now(),Detections:[]Detection{}}
  state.Lock();state.jobs[id]=job;state.Unlock()
  go analyze(py,worker,id,src)
  w.WriteHeader(http.StatusAccepted);respond(w,job)
 })
 handler:=http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Access-Control-Allow-Origin","http://localhost:3000")
  w.Header().Set("Access-Control-Allow-Methods","GET, POST, OPTIONS")
  w.Header().Set("Access-Control-Allow-Headers","Content-Type")
  if r.Method=="OPTIONS"{w.WriteHeader(204);return}
  mux.ServeHTTP(w,r)
 })
 log.Printf("Veltryx Vision API on http://127.0.0.1:8088, media root %s",absRoot)
 log.Fatal(http.ListenAndServe("127.0.0.1:8088",handler))
}
func secureVideo(root,src string)(string,error){
 if !filepath.IsAbs(src){return "",errors.New("provide an absolute video path")}
 abs,err:=filepath.EvalSymlinks(src);if err!=nil{return "",err}
 rel,err:=filepath.Rel(root,abs);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(os.PathSeparator)){return "",errors.New("video must be inside VISION_MEDIA_ROOT")}
 stat,err:=os.Stat(abs);if err!=nil||!stat.Mode().IsRegular(){return "",errors.New("not a regular file")}
 switch strings.ToLower(filepath.Ext(abs)){case ".mp4",".mkv",".mov",".avi":return abs,nil}
 return "",errors.New("unsupported video extension")
}
func analyze(py,worker,id,src string){
 state.Lock();state.jobs[id].Status="running";state.Unlock()
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Minute);defer cancel()
 cmd:=exec.CommandContext(ctx,py,worker,"--input",src,"--stride","10","--max-frames","300")
 raw,err:=cmd.CombinedOutput()
 state.Lock();defer state.Unlock()
 j:=state.jobs[id]
 if err!=nil {j.Status="failed";j.Error=fmt.Sprintf("%v: %s",err,limit(string(raw),1200));return}
 var data struct{Detections []Detection `json:"detections"`}
 if err=json.Unmarshal(raw,&data);err!=nil{j.Status="failed";j.Error="bad worker JSON: "+limit(string(raw),300);return}
 j.Detections=data.Detections
 if j.Detections==nil{j.Detections=[]Detection{}}
 j.Status="completed"
}
func limit(s string,n int)string{if len(s)>n{return s[:n]};return s}
func respond(w http.ResponseWriter,v any){w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(v)}
