//go:build x264 && cgo

package encoder

/*
#cgo pkg-config: x264
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <x264.h>
typedef struct { x264_t *encoder; x264_picture_t pic; int64_t pts; } vx_encoder;
static vx_encoder *vx_open(int w,int h,double fps,int bitrate,const char *preset,const char *tune) {
 x264_param_t p;
 if(x264_param_default_preset(&p,preset,tune)<0) return NULL;
 p.i_width=w; p.i_height=h; p.i_csp=X264_CSP_I420;
 p.i_fps_num=(int)(fps*1000); p.i_fps_den=1000;
 p.i_timebase_num=1000; p.i_timebase_den=p.i_fps_num;
 p.b_vfr_input=0; p.b_repeat_headers=1; p.b_annexb=1;
 p.i_keyint_max=(int)(fps*2); p.i_bframe=0;
 p.rc.i_rc_method=X264_RC_ABR; p.rc.i_bitrate=bitrate;
 p.rc.i_vbv_max_bitrate=bitrate; p.rc.i_vbv_buffer_size=bitrate;
 p.i_threads=1; p.i_log_level=X264_LOG_ERROR;
 if(x264_param_apply_profile(&p,"baseline")<0) return NULL;
 vx_encoder *v=calloc(1,sizeof(*v)); if(!v) return NULL;
 if(x264_picture_alloc(&v->pic,X264_CSP_I420,w,h)<0){free(v);return NULL;}
 v->encoder=x264_encoder_open(&p); if(!v->encoder){x264_picture_clean(&v->pic);free(v);return NULL;}
 return v;
}
static int vx_encode(vx_encoder *v,uint8_t *gray,int w,int h,uint8_t **data) {
 for(int y=0;y<h;y++) memcpy(v->pic.img.plane[0]+y*v->pic.img.i_stride[0],gray+y*w,w);
 for(int y=0;y<h/2;y++){memset(v->pic.img.plane[1]+y*v->pic.img.i_stride[1],128,w/2);memset(v->pic.img.plane[2]+y*v->pic.img.i_stride[2],128,w/2);}
 v->pic.i_pts=v->pts++; x264_nal_t *nal; int count; x264_picture_t out;
 int size=x264_encoder_encode(v->encoder,&nal,&count,&v->pic,&out);
 if(size>0) *data=nal[0].p_payload; return size;
}
static void vx_close(vx_encoder *v){if(v){x264_encoder_close(v->encoder);x264_picture_clean(&v->pic);free(v);}}
*/
import "C"
import (
	"errors"
	"time"
	"unsafe"

	"github.com/veltryx/vision/internal/frames"
)

type native struct {
	ptr           *C.vx_encoder
	width, height int
	duration      time.Duration
}

func New(c Config, w, h int) (Encoder, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	if w < 2 || h < 2 || w%2 != 0 || h%2 != 0 {
		return nil, errors.New("x264 requires even dimensions")
	}
	p, t := C.CString(c.Preset), C.CString(c.Tune)
	defer C.free(unsafe.Pointer(p))
	defer C.free(unsafe.Pointer(t))
	v := C.vx_open(C.int(w), C.int(h), C.double(c.FPS), C.int(c.BitrateKbps), p, t)
	if v == nil {
		return nil, errors.New("x264 initialization failed")
	}
	return &native{ptr: v, width: w, height: h, duration: time.Duration(float64(time.Second) / c.FPS)}, nil
}
func (n *native) Encode(f *frames.Frame) (Sample, error) {
	if n.ptr == nil || !f.Valid() || f.Width != n.width || f.Height != n.height {
		return Sample{}, errors.New("encoder/frame dimension mismatch")
	}
	var out *C.uint8_t
	size := C.vx_encode(n.ptr, (*C.uint8_t)(unsafe.Pointer(&f.Pixels[0])), C.int(f.Width), C.int(f.Height), &out)
	if size < 0 {
		return Sample{}, errors.New("x264 encoding failed")
	}
	return Sample{Data: C.GoBytes(unsafe.Pointer(out), size), Timestamp: f.Timestamp, Duration: n.duration}, nil
}
func (n *native) Close() error {
	if n.ptr != nil {
		C.vx_close(n.ptr)
		n.ptr = nil
	}
	return nil
}
