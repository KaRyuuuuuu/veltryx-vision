package config

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/veltryx/vision/internal/roi"
	"github.com/veltryx/vision/pkg/models"
)

type Camera struct {
	Rotation *int             `json:"screenshot_rotation,omitempty"`
	Settings *models.Settings `json:"settings,omitempty"`
	Zones    []models.Zone    `json:"zones"`
}
type Data struct {
	Cameras map[string]Camera `json:"cameras"`
}
type Store struct {
	mu   sync.RWMutex
	path string
	data Data
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, data: Data{Cameras: map[string]Camera{}}}
	b, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return s, nil
	}
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(b, &s.data); e != nil {
		return nil, e
	}
	if s.data.Cameras == nil {
		s.data.Cameras = map[string]Camera{}
	}
	for _, c := range s.data.Cameras {
		if c.Rotation != nil && !ValidRotation(*c.Rotation) {
			return nil, errors.New("invalid screenshot rotation")
		}
		if c.Settings != nil {
			if e = ValidateSettings(*c.Settings); e != nil {
				return nil, e
			}
		}
		if e = ValidateZones(c.Zones); e != nil {
			return nil, e
		}
	}
	return s, nil
}
func clone(c Camera) Camera {
	b, _ := json.Marshal(c)
	var out Camera
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Store) Get(serial string) Camera {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.data.Cameras[serial])
}
func (s *Store) Update(serial string, fn func(*Camera)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := clone(s.data.Cameras[serial])
	fn(&c)
	if c.Rotation != nil && !ValidRotation(*c.Rotation) {
		return errors.New("rotation must be 0, 90, 180 or 270")
	}
	if c.Settings != nil {
		if e := ValidateSettings(*c.Settings); e != nil {
			return e
		}
	}
	if e := ValidateZones(c.Zones); e != nil {
		return e
	}
	c = clone(c)
	previous, exists := s.data.Cameras[serial]
	s.data.Cameras[serial] = c
	if e := s.save(); e != nil {
		if exists {
			s.data.Cameras[serial] = previous
		} else {
			delete(s.data.Cameras, serial)
		}
		return e
	}
	return nil
}
func (s *Store) save() error {
	b, e := json.MarshalIndent(s.data, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(s.path), 0750); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(s.path), ".config-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), s.path)
}
func ValidateSettings(v models.Settings) error {
	if v.Width < 16 || v.Height < 16 || v.Width > 8192 || v.Height > 8192 || int64(v.Width)*int64(v.Height) > 16777216 || (math.IsNaN(v.FPS) || math.IsInf(v.FPS, 0) || (!v.FPSUnlimited && v.FPS <= 0)) || (math.IsNaN(v.Exposure) || math.IsInf(v.Exposure, 0) || (!v.AutoExposure && v.Exposure <= 0)) || (math.IsNaN(v.Gain) || math.IsInf(v.Gain, 0) || (!v.AutoGain && v.Gain < 0)) {
		return errors.New("invalid camera settings")
	}
	return nil
}
func ValidateZones(zones []models.Zone) error {
	if len(zones) > 16 {
		return errors.New("too many ROIs")
	}
	ids := map[string]bool{}
	for _, z := range zones {
		if e := roi.Validate(z); e != nil {
			return e
		}
		if ids[z.ID] {
			return errors.New("duplicate ROI id")
		}
		ids[z.ID] = true
	}
	return nil
}

func ValidRotation(v int) bool { return v == 0 || v == 90 || v == 180 || v == 270 }
func (s *Store) Rotation(serial string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v := s.data.Cameras[serial].Rotation; v != nil {
		return *v
	}
	return 0
}
