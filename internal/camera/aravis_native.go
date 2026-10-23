//go:build aravis && cgo

package camera

/*
#cgo pkg-config: aravis-0.10
#include <arv.h>
#include <stdlib.h>
static ArvStream *vx_stream(ArvCamera *c,GError **e) { return arv_camera_create_stream(c,NULL,NULL,NULL,e); }
static const char *vx_user(ArvCamera *c) {GError *e=NULL; const char *s=arv_device_get_string_feature_value(arv_camera_get_device(c),"DeviceUserID",&e);if(e){g_error_free(e);return "";}return s;}
static const char *vx_format(const char **p,int i){return p[i];}
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/veltryx/vision/internal/frames"
	"github.com/veltryx/vision/pkg/models"
)

// Aravis' global device list must be serialized across discovery and opens.
var arvGlobal sync.Mutex

type aravis struct{}

func NewAravis() (Provider, error) { return &aravis{}, nil }
func arvError(e *C.GError) error {
	if e == nil {
		return nil
	}
	s := C.GoString(e.message)
	C.g_error_free(e)
	return errors.New(s)
}
func (a *aravis) Discover(ctx context.Context) ([]models.Camera, error) {
	arvGlobal.Lock()
	defer arvGlobal.Unlock()
	C.arv_update_device_list()
	out := make([]models.Camera, 0, int(C.arv_get_n_devices()))
	for i := C.uint(0); i < C.arv_get_n_devices(); i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if C.GoString(C.arv_get_device_protocol(i)) != "GigEVision" {
			continue
		}
		c := models.Camera{ID: C.GoString(C.arv_get_device_id(i)), Serial: C.GoString(C.arv_get_device_serial_nbr(i)), IP: C.GoString(C.arv_get_device_address(i)), Vendor: C.GoString(C.arv_get_device_vendor(i)), Model: C.GoString(C.arv_get_device_model(i)), Connected: true}
		if c.Serial == "" {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

type arvDevice struct {
	mu       sync.Mutex
	cam      *C.ArvCamera
	info     models.Camera
	lastRead time.Time
	running  bool
}

func (a *aravis) Open(ctx context.Context, c models.Camera) (Device, error) {
	arvGlobal.Lock()
	defer arvGlobal.Unlock()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	id := C.CString(c.ID)
	defer C.free(unsafe.Pointer(id))
	var e *C.GError
	cam := C.arv_camera_new(id, &e)
	if err := arvError(e); err != nil {
		return nil, err
	}
	if cam == nil {
		return nil, errors.New("Aravis camera unavailable")
	}
	d := &arvDevice{cam: cam, info: c}
	d.info.Name = C.GoString(C.vx_user(cam))
	if d.info.Name == "" {
		d.info.Name = c.Model + " " + c.Serial
	}
	var n C.uint
	e = nil
	formats := C.arv_camera_dup_available_pixel_formats_as_strings(cam, &n, &e)
	if err := arvError(e); err != nil {
		d.Close()
		return nil, err
	}
	for i := 0; i < int(n); i++ {
		d.info.Formats = append(d.info.Formats, C.GoString(C.vx_format(formats, C.int(i))))
	}
	C.g_free(C.gpointer(unsafe.Pointer(formats)))
	if err := d.readSettings(); err != nil {
		d.Close()
		return nil, err
	}
	if err := d.readCapabilities(); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}
func (d *arvDevice) readSettings() error {
	var e *C.GError
	var x, y, w, h C.gint
	C.arv_camera_get_region(d.cam, &x, &y, &w, &h, &e)
	if err := arvError(e); err != nil {
		return err
	}
	d.info.Settings.Width = int(w)
	d.info.Settings.Height = int(h)
	e = nil
	d.info.Settings.FPS = float64(C.arv_camera_get_frame_rate(d.cam, &e))
	if err := arvError(e); err != nil {
		return err
	}
	e = nil
	d.info.Settings.Exposure = float64(C.arv_camera_get_exposure_time(d.cam, &e))
	if err := arvError(e); err != nil {
		return err
	}
	e = nil
	d.info.Settings.Gain = float64(C.arv_camera_get_gain(d.cam, &e))
	if err := arvError(e); err != nil {
		return err
	}
	return d.readAutomatic()
}
func (d *arvDevice) Info() models.Camera {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running && time.Since(d.lastRead) >= time.Second {
		_ = d.readSettings()
		d.lastRead = time.Now()
	}
	return d.info
}
func (d *arvDevice) Configure(s models.Settings) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		return errors.New("stop acquisition before configuring camera")
	}
	var e *C.GError
	C.arv_camera_set_region(d.cam, 0, 0, C.gint(s.Width), C.gint(s.Height), &e)
	if err := arvError(e); err != nil {
		return err
	}

	if err := d.configureAutomatic(s); err != nil {
		return err
	}

	if e := d.readSettings(); e != nil {
		return e
	}
	return d.readCapabilities()
}
func (d *arvDevice) Run(ctx context.Context, publish func(*frames.Frame)) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return errors.New("acquisition already running")
	}
	d.running = true
	d.mu.Unlock()
	defer func() { d.mu.Lock(); d.running = false; d.mu.Unlock() }()
	// Explicitly select Mono8: unsupported formats are reported, never interpreted as Gray8.
	var e *C.GError
	format := C.CString("Mono8")
	C.arv_camera_set_pixel_format_from_string(d.cam, format, &e)
	C.free(unsafe.Pointer(format))
	if err := arvError(e); err != nil {
		return fmt.Errorf("Mono8 required: %w", err)
	}
	e = nil
	stream := C.vx_stream(d.cam, &e)
	if err := arvError(e); err != nil {
		return err
	}
	if stream == nil {
		return errors.New("stream creation failed")
	}
	defer C.g_object_unref(C.gpointer(unsafe.Pointer(stream)))
	e = nil
	payload := C.arv_camera_get_payload(d.cam, &e)
	if err := arvError(e); err != nil {
		return err
	}
	if payload == 0 || payload > 64*1024*1024 {
		return errors.New("invalid camera payload")
	}
	for i := 0; i < 8; i++ {
		b := C.arv_buffer_new_allocate(C.size_t(payload))
		if b == nil {
			return errors.New("Aravis buffer allocation failed")
		}
		C.arv_stream_push_buffer(stream, b)
	}
	e = nil
	C.arv_camera_start_acquisition(d.cam, &e)
	if err := arvError(e); err != nil {
		return err
	}
	defer func() { var err *C.GError; C.arv_camera_stop_acquisition(d.cam, &err); _ = arvError(err) }()
	last := time.Now()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		b := C.arv_stream_timeout_pop_buffer(stream, 200000)
		if b == nil {
			if time.Since(last) > 3*time.Second {
				return errors.New("camera acquisition timeout")
			}
			continue
		}
		if C.arv_buffer_get_status(b) != C.ARV_BUFFER_STATUS_SUCCESS {
			C.arv_stream_push_buffer(stream, b)
			if time.Since(last) > 3*time.Second {
				return errors.New("camera has no valid buffers")
			}
			continue
		}
		var size C.size_t
		data := C.arv_buffer_get_image_data(b, &size)
		w, h := int(C.arv_buffer_get_image_width(b)), int(C.arv_buffer_get_image_height(b))
		var xp, yp C.gint
		C.arv_buffer_get_image_padding(b, &xp, &yp)
		if w <= 0 || h <= 0 || uint64(w)*uint64(h) > 16777216 || int(size) < (w+int(xp))*h {
			C.arv_stream_push_buffer(stream, b)
			return errors.New("invalid Mono8 image layout")
		}
		if data == nil || C.arv_buffer_get_image_pixel_format(b) != C.ARV_PIXEL_FORMAT_MONO_8 {
			C.arv_stream_push_buffer(stream, b)
			return errors.New("camera returned unsupported pixel format")
		}
		raw := unsafe.Slice((*byte)(unsafe.Pointer(data)), int(size))
		pixels := make([]byte, w*h)
		for y := 0; y < h; y++ {
			copy(pixels[y*w:(y+1)*w], raw[y*(w+int(xp)):y*(w+int(xp))+w])
		}
		f := &frames.Frame{Serial: d.info.Serial, Sequence: uint64(C.arv_buffer_get_frame_id(b)), Timestamp: time.Now().UTC(), DeviceTimestamp: uint64(C.arv_buffer_get_timestamp(b)), Width: w, Height: h, Pixels: pixels}
		C.arv_stream_push_buffer(stream, b)
		last = time.Now()
		publish(f)
	}
}
func (d *arvDevice) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		return errors.New("cannot close running camera")
	}
	if d.cam != nil {
		C.g_object_unref(C.gpointer(unsafe.Pointer(d.cam)))
		d.cam = nil
	}
	return nil
}

func (d *arvDevice) readCapabilities() error {
	caps := &models.Capabilities{}
	var lo, hi C.gint
	var e *C.GError
	C.arv_camera_get_width_bounds(d.cam, &lo, &hi, &e)
	if err := arvError(e); err != nil {
		return err
	}
	caps.Width = models.Range{Min: float64(lo), Max: float64(hi)}
	e = nil
	caps.Width.Increment = float64(C.arv_camera_get_width_increment(d.cam, &e))
	if err := arvError(e); err != nil {
		return err
	}
	e = nil
	C.arv_camera_get_height_bounds(d.cam, &lo, &hi, &e)
	if err := arvError(e); err != nil {
		return err
	}
	caps.Height = models.Range{Min: float64(lo), Max: float64(hi)}
	e = nil
	caps.Height.Increment = float64(C.arv_camera_get_height_increment(d.cam, &e))
	if err := arvError(e); err != nil {
		return err
	}
	var min, max C.double
	e = nil
	C.arv_camera_get_frame_rate_bounds(d.cam, &min, &max, &e)
	if err := arvError(e); err != nil {
		return err
	}
	caps.FPS = models.Range{Min: float64(min), Max: float64(max)}
	e = nil
	C.arv_camera_get_exposure_time_bounds(d.cam, &min, &max, &e)
	if err := arvError(e); err != nil {
		return err
	}
	caps.Exposure = models.Range{Min: float64(min), Max: float64(max)}
	e = nil
	C.arv_camera_get_gain_bounds(d.cam, &min, &max, &e)
	if err := arvError(e); err != nil {
		return err
	}
	caps.Gain = models.Range{Min: float64(min), Max: float64(max)}
	autoExposure, autoGain, freeFPS := d.autoCapabilities()
	caps.AutoExposure = autoExposure
	caps.AutoGain = autoGain
	caps.FPSUnlimited = freeFPS
	d.info.Capabilities = caps
	return nil
}

func (d *arvDevice) autoCapabilities() (bool, bool, bool) {
	var e *C.GError
	exposure := C.arv_camera_is_exposure_auto_available(d.cam, &e) != 0
	if e != nil {
		_ = arvError(e)
		exposure = false
	}
	e = nil
	gain := C.arv_camera_is_gain_auto_available(d.cam, &e) != 0
	if e != nil {
		_ = arvError(e)
		gain = false
	}
	e = nil
	name := C.CString("AcquisitionFrameRateEnable")
	defer C.free(unsafe.Pointer(name))
	fps := C.arv_camera_is_feature_available(d.cam, name, &e) != 0
	if e != nil {
		_ = arvError(e)
		fps = false
	}
	return exposure, gain, fps
}
func (d *arvDevice) readAutomatic() error {
	exposure, gain, fps := d.autoCapabilities()
	var e *C.GError
	if exposure {
		d.info.Settings.AutoExposure = C.arv_camera_get_exposure_time_auto(d.cam, &e) == C.ARV_AUTO_CONTINUOUS
		if err := arvError(e); err != nil {
			return err
		}
		e = nil
	}
	if gain {
		d.info.Settings.AutoGain = C.arv_camera_get_gain_auto(d.cam, &e) == C.ARV_AUTO_CONTINUOUS
		if err := arvError(e); err != nil {
			return err
		}
		e = nil
	}
	if fps {
		d.info.Settings.FPSUnlimited = C.arv_camera_get_frame_rate_enable(d.cam, &e) == 0
		return arvError(e)
	}
	return nil
}
func (d *arvDevice) configureAutomatic(s models.Settings) error {
	exposure, gain, fps := d.autoCapabilities()
	if s.AutoExposure && !exposure {
		return errors.New("camera does not support continuous automatic exposure")
	}
	if s.AutoGain && !gain {
		return errors.New("camera does not support continuous automatic gain")
	}
	if s.FPSUnlimited && !fps {
		return errors.New("camera does not support disabling its FPS limit")
	}
	var e *C.GError
	// Turn automatic controls off before applying manual values or enabling continuous auto.
	if exposure {
		C.arv_camera_set_exposure_time_auto(d.cam, C.ARV_AUTO_OFF, &e)
		if err := arvError(e); err != nil {
			return err
		}
		e = nil
	}
	if gain {
		C.arv_camera_set_gain_auto(d.cam, C.ARV_AUTO_OFF, &e)
		if err := arvError(e); err != nil {
			return err
		}
		e = nil
	}
	if fps {
		enabled := C.gboolean(1)
		if s.FPSUnlimited {
			enabled = 0
		}
		C.arv_camera_set_frame_rate_enable(d.cam, enabled, &e)
		if err := arvError(e); err != nil {
			return err
		}
		e = nil
	}
	if !s.FPSUnlimited {
		C.arv_camera_set_frame_rate(d.cam, C.double(s.FPS), &e)
		if err := arvError(e); err != nil {
			return err
		}
		e = nil
	}
	if !s.AutoExposure {
		C.arv_camera_set_exposure_time(d.cam, C.double(s.Exposure), &e)
		if err := arvError(e); err != nil {
			return err
		}
		e = nil
	}
	if !s.AutoGain {
		C.arv_camera_set_gain(d.cam, C.double(s.Gain), &e)
		if err := arvError(e); err != nil {
			return err
		}
		e = nil
	}
	if s.AutoExposure {
		C.arv_camera_set_exposure_time_auto(d.cam, C.ARV_AUTO_CONTINUOUS, &e)
		if err := arvError(e); err != nil {
			return err
		}
		e = nil
	}
	if s.AutoGain {
		C.arv_camera_set_gain_auto(d.cam, C.ARV_AUTO_CONTINUOUS, &e)
		return arvError(e)
	}
	return nil
}
