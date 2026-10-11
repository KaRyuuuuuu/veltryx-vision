// Veltryx Vision Go: RTSP frame capture, movement detection and async WebP saves.
// Not a timing source. Run only on Dragonfly; use SSH tunnel to the camera.
package main

import (
 "bufio"
 "bytes"
 "context"
 "encoding/json"
 "flag"
 "fmt"
 "image"
 "image/color"
 "image/jpeg"
 "io"
 "log"
 "net"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
 "strconv"
 "strings"
 "sync"
 "sync/atomic"
 "time"
)

type Event struct {
 ID string `json:"id"`
 Time string `json:"time"`
 Path string `json:"path"`
 Pixels int `json:"pixels"`
}
type Metrics struct {
 State string `json:"state"`
 SourceFPS float64 `json:"source_fps"`
 Processed uint64 `json:"processed"`
 Drops uint64 `json:"dropped_saves"`
 MotionPixels int `json:"motion_pixels"`
 Captures int `json:"captures"`
 LastFrame string `json:"last_frame"`
}
type Snapshot struct {
 pixels []byte
 width,height int
 timestamp time.Time
 motion int
}
type Server struct {
 mu sync.RWMutex
 metrics Metrics
 events []Event
 lastJPEG []byte
 jobs chan Snapshot
 dir string
 width,height,threshold,confirm int
 bytesPerFrame int
 source string
 running atomic.Bool
}

func (s *Server) setState(state string) {
 s.mu.Lock();s.metrics.State=state;s.mu.Unlock()
}
func (s *Server) update(pixels int, timestamp time.Time) {
 s.mu.Lock();s.metrics.MotionPixels=pixels;s.metrics.Processed++;s.metrics.LastFrame=timestamp.UTC().Format(time.RFC3339Nano);s.mu.Unlock()
}
func (s *Server) save(ctx context.Context) {
 for {
  select {
  case <-ctx.Done():return
  case snap:=<-s.jobs:
   stamp:=snap.timestamp.UTC()
   folder:=filepath.Join(s.dir,stamp.Format("2006-01-02"))
   if err:=os.MkdirAll(folder,0755);err!=nil {log.Printf("mkdir: %v",err);continue}
   id:=stamp.Format("20060102T150405.000000000Z")
   name:=id+".webp"
   target:=filepath.Join(folder,name)
   // Keep image encoding off the RTSP acquisition loop.
   cmd:=exec.CommandContext(ctx,"ffmpeg","-nostdin","-hide_banner","-loglevel","error",
    "-f","rawvideo","-pixel_format","bgr24","-video_size",fmt.Sprintf("%dx%d",snap.width,snap.height),
    "-i","pipe:0","-frames:v","1","-c:v","libwebp","-quality","82","-y",target)
   cmd.Stdin=bytes.NewReader(snap.pixels)
   if output,err:=cmd.CombinedOutput();err!=nil {log.Printf("WebP save failed: %v: %s",err,output);continue}
   ev:=Event{ID:id,Time:stamp.Format(time.RFC3339Nano),Path:"/"+stamp.Format("2006-01-02")+"/"+name,Pixels:snap.motion}
   s.mu.Lock()
   s.events=append([]Event{ev},s.events...)
   if len(s.events)>300{s.events=s.events[:300]}
   s.metrics.Captures++
   s.mu.Unlock()
   f,err:=os.OpenFile(filepath.Join(s.dir,"events.jsonl"),os.O_CREATE|os.O_WRONLY|os.O_APPEND,0644)
   if err==nil {b,_:=json.Marshal(ev);_,_=f.Write(append(b,'
'));_ = f.Close()}
  }
 }
}
func (s *Server) capture(ctx context.Context, fps int) {
 // Constrain resource consumption and avoid writing full frames on the capture goroutine.
 frameSize:=s.width*s.height*3
 roi:=make([]int,0,18000)
 // Approximate the start/finish checkered strip from the 1649x928 reference image.
 // Scaled ROI: narrow polygon around stripe in lower-left image.
 for y:=int(float64(s.height)*485/928);y<int(float64(s.height)*905/928);y+=2{
  for x:=int(float64(s.width)*195/1649);x<int(float64(s.width)*575/1649);x+=2{
   // Check if point lies inside slanted stripe; generous margin.
   yf:=float64(y)*928/float64(s.height)
   xf:=float64(x)*1649/float64(s.width)
   left:=215+(yf-485)*130/420
   right:=355+(yf-485)*220/420
   if xf>=float64(left)&&xf<=float64(right) {
    roi=append(roi,(y*s.width+x)*3)
   }
  }
 }
 if len(roi)==0 {log.Println("ROI empty; check video dimensions");return}
 consecutive:=0
 quiet:=0
 ready:=true
 var prev []byte
 warmUntil:=time.Now().Add(3*time.Second)
 for ctx.Err()==nil {
  s.setState("CONNECTING")
  // RTSP over TCP through the existing SSH tunnel; raw frames to stdout.
  cmd:=exec.CommandContext(ctx,"ffmpeg","-nostdin","-hide_banner","-loglevel","error",
   "-rtsp_transport","tcp","-i",s.source,"-an",
   "-vf",fmt.Sprintf("fps=%d",fps),"-f","rawvideo","-pix_fmt","bgr24","pipe:1")
  stream,err:=cmd.StdoutPipe()
  if err!=nil {log.Printf("RTSP pipe: %v",err);return}
  cmd.Stderr=os.Stderr
  if err=cmd.Start();err!=nil {log.Printf("RTSP start: %v",err);return}
  reader:=bufio.NewReaderSize(stream,frameSize)
  lastStats:=time.Now()
  frames:=0
  // Reset state after reconnection to avoid false positives from disjoint frames.
  prev=nil;consecutive=0;quiet=0;ready=true
  warmUntil=time.Now().Add(3*time.Second)
  s.setState("WARMUP")
  for ctx.Err()==nil {
   frame:=make([]byte,frameSize)
   if _,err=io.ReadFull(reader,frame);err!=nil {break}
   now:=time.Now()
   frames++
   pixels:=0
   if prev!=nil {
    // BGR grayscale approximated by green channel; sparse 2x sampling.
    for _,i:=range roi {
     d:=int(frame[i+1])-int(prev[i+1])
     if d<0 {d=-d}
     if d>28 {pixels++}
    }
   }
   prev=frame
   s.update(pixels,now)
   if time.Since(lastStats)>=5*time.Second {
    s.mu.Lock();s.metrics.SourceFPS=float64(frames)/time.Since(lastStats).Seconds();s.mu.Unlock()
    lastStats=time.Now();frames=0
   }
   active:=pixels>=s.threshold
   if active {consecutive++;quiet=0}else{consecutive=0;quiet++;if quiet>=1{ready=true}}
   if now.Before(warmUntil){s.setState("WARMUP")}else if active {s.setState("MOTION")}else{s.setState("READY")}
   if ready&&consecutive>=s.confirm&&now.After(warmUntil) {
    job:=Snapshot{pixels:frame,width:s.width,height:s.height,timestamp:now,motion:pixels}
    select {
    case s.jobs<-job: ready=false
    default:
     s.mu.Lock();s.metrics.Drops++;s.mu.Unlock()
     // Keep armed; retry capture while object is still present.
    }
   }
   // Preview encoding occurs at most 5 times/sec; may cost CPU but does not include disk I/O.
   if frames%6==0 {
    img:=image.NewRGBA(image.Rect(0,0,s.width,s.height))
    for i:=0;i<s.width*s.height;i++ {j:=i*3;img.Pix[i*4]=frame[j+2];img.Pix[i*4+1]=frame[j+1];img.Pix[i*4+2]=frame[j];img.Pix[i*4+3]=255}
    var buf bytes.Buffer
    _=jpeg.Encode(&buf,img,&jpeg.Options{Quality:70})
    s.mu.Lock();s.lastJPEG=buf.Bytes();s.mu.Unlock()
   }
  }
  // Cancel FFmpeg and clean up before reconnecting.
  _=cmd.Process.Kill()
  _=cmd.Wait()
  if ctx.Err()!=nil{return}
  s.setState("RECONNECTING")
  log.Printf("RTSP interrupted: %v; reconnect in 1s",err)
  select {case <-ctx.Done():return;case <-time.After(time.Second):}
 }
}
func main(){
 var source,dir,addr string
 var width,height,fps,threshold,confirm int
 flag.StringVar(&source,"source","rtsp://127.0.0.1:8554/h264","local SSH-forwarded RTSP URL")
 flag.StringVar(&dir,"output","../local-data/go-line-crossings","local snapshot directory")
 flag.StringVar(&addr,"listen","127.0.0.1:8098","HTTP listener (local only)")
 flag.IntVar(&width,"width",960,"decoded source width")
 flag.IntVar(&height,"height",480,"decoded source height")
 flag.IntVar(&fps,"fps",30,"target decoded frame rate; never exceeds camera stream")
 flag.IntVar(&threshold,"threshold",170,"motion pixel threshold in sampled ROI")
 flag.IntVar(&confirm,"confirm",2,"number of consecutive frames to trigger")
 flag.Parse()
 if width<=0||height<=0||width*height>1920*1080||fps<1||fps>60||confirm<1||threshold<1{
  log.Fatal("invalid dimensions/fps/detection settings")
 }
 if !strings.HasPrefix(source,"rtsp://127.0.0.1:")&&!strings.HasPrefix(source,"rtsp://localhost:"){log.Fatal("source must be local SSH tunnel")}
 if _,_,err:=net.SplitHostPort(addr);err!=nil{log.Fatal(err)}
 absolute,err:=filepath.Abs(dir);if err!=nil{log.Fatal(err)}
 if err=os.MkdirAll(absolute,0755);err!=nil{log.Fatal(err)}
 ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 s:=&Server{source:source,width:width,height:height,threshold:threshold,confirm:confirm,dir:absolute,jobs:make(chan Snapshot,24)}
 s.metrics.State="STARTING"
 // Populate gallery from the event ledger for restarts.
 if f,err:=os.Open(filepath.Join(absolute,"events.jsonl"));err==nil{
  scanner:=bufio.NewScanner(f)
  for scanner.Scan(){var ev Event;if json.Unmarshal(scanner.Bytes(),&ev)==nil{s.events=append(s.events,ev)}}
  _=f.Close()
  for i,j:=0,len(s.events)-1;i<j;i,j=i+1,j-1{s.events[i],s.events[j]=s.events[j],s.events[i]}
  if len(s.events)>300{s.events=s.events[:300]}
 }
 go s.save(ctx)
 go s.capture(ctx,fps)
 mux:=http.NewServeMux()
 mux.HandleFunc("GET /api/status",func(w http.ResponseWriter,r *http.Request){
  s.mu.RLock();m:=s.metrics;s.mu.RUnlock();w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(m)
 })
 mux.HandleFunc("GET /api/events",func(w http.ResponseWriter,r *http.Request){
  s.mu.RLock();events:=append([]Event(nil),s.events...);s.mu.RUnlock();w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(events)
 })
 mux.HandleFunc("GET /api/frame.jpg",func(w http.ResponseWriter,r *http.Request){
  s.mu.RLock();frame:=append([]byte(nil),s.lastJPEG...);s.mu.RUnlock()
  if len(frame)==0{http.Error(w,"waiting for camera",503);return}
  w.Header().Set("Content-Type","image/jpeg");w.Header().Set("Cache-Control","no-store");_,_=w.Write(frame)
 })
 mux.HandleFunc("GET /api/captures/",func(w http.ResponseWriter,r *http.Request){
  path:=strings.TrimPrefix(r.URL.Path,"/api/captures/")
  if path==""||strings.Contains(path,"..")||strings.HasPrefix(path,"/"){http.Error(w,"invalid path",400);return}
  http.ServeFile(w,r,filepath.Join(absolute,filepath.FromSlash(path)))
 })
 mux.HandleFunc("GET /api/info",func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Content-Type","application/json")
  _,_=w.Write([]byte(`{"version":"go-v2","mode":"motion-only","official_timing":false}`))
 })
 server:=&http.Server{Addr:addr,Handler:mux,ReadHeaderTimeout:5*time.Second}
 log.Printf("Veltryx Go Line Monitor on http://%s; RTSP %s; output %s",addr,source,absolute)
 _=strconv.Itoa // keep explicit import for stable gofmt later
 log.Fatal(server.ListenAndServe())
}
