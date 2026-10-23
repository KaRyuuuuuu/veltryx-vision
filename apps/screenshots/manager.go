package screenshots

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/veltryx/vision/internal/frames"
)

type Capture struct {
	Role            string    `json:"role,omitempty"`
	Rotation        int       `json:"rotation"`
	Width           int       `json:"width,omitempty"`
	Height          int       `json:"height,omitempty"`
	ID              string    `json:"id"`
	Serial          string    `json:"serial"`
	Passage         string    `json:"passage"`
	Zone            string    `json:"zone,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
	DeviceTimestamp uint64    `json:"device_timestamp"`
	Sequence        uint64    `json:"sequence"`
	Format          string    `json:"format"`
	File            string    `json:"file"`
}
type Manager struct {
	mu       sync.Mutex
	rotation func(string) int
	dir      string
	records  []Capture
	totals   map[string]Stats
	passages map[string]map[string]bool
}

func ID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func New(dir string) (*Manager, error) {
	if e := os.MkdirAll(dir, 0750); e != nil {
		return nil, e
	}
	m := &Manager{dir: dir, totals: map[string]Stats{}, passages: map[string]map[string]bool{}}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil, e
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		b, e := os.ReadFile(filepath.Join(dir, entry.Name()))
		if e != nil {
			return nil, e
		}
		var c Capture
		if e = json.Unmarshal(b, &c); e != nil {
			return nil, e
		}
		m.add(c)
	}
	return m, nil
}
func (m *Manager) Save(f *frames.Frame, format, passage, zone string, role ...string) (Capture, error) {
	if !f.Valid() {
		return Capture{}, errors.New("no valid frame available")
	}
	if format != "jpeg" && format != "png" {
		return Capture{}, errors.New("format must be jpeg or png")
	}
	m.mu.Lock()
	source := m.rotation
	m.mu.Unlock()
	rotation := 0
	if source != nil {
		rotation = source(f.Serial)
	}
	image, e := Rotate(f, rotation)
	if e != nil {
		return Capture{}, e
	}
	if passage == "" {
		passage = ID()
	}
	c := Capture{Rotation: rotation, Width: image.Width, Height: image.Height, ID: ID(), Serial: f.Serial, Passage: passage, Zone: zone, Timestamp: f.Timestamp, DeviceTimestamp: f.DeviceTimestamp, Sequence: f.Sequence, Format: format}
	if len(role) > 0 {
		c.Role = role[0]
	} else if zone == "" {
		c.Role = "manual"
	}
	c.File = c.ID + "." + format
	tmp, e := os.CreateTemp(m.dir, ".image-*")
	if e != nil {
		return c, e
	}
	defer os.Remove(tmp.Name())
	e = WriteCapture(tmp, image, format)
	if e == nil {
		e = tmp.Sync()
	}
	ce := tmp.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return c, e
	}
	if e = os.Rename(tmp.Name(), filepath.Join(m.dir, c.File)); e != nil {
		return c, e
	}
	b, _ := json.Marshal(c)
	if e = atomicMetadata(m.dir, c.ID+".json", b); e != nil {
		_ = os.Remove(filepath.Join(m.dir, c.File))
		return c, e
	}
	m.mu.Lock()
	m.add(c)
	m.mu.Unlock()
	return c, nil
}
func (m *Manager) List(serial string, limit int) []Capture {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Capture, 0)
	for _, c := range m.records {
		if serial == "" || serial == c.Serial {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
func (m *Manager) Path(id string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.records {
		if c.ID == id {
			return filepath.Join(m.dir, c.File), true
		}
	}
	return "", false
}

func atomicMetadata(dir, name string, b []byte) error {
	f, e := os.CreateTemp(dir, ".metadata-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0640); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}

// Stats includes persisted captures, not just events received by this browser.
type Stats struct {
	Images   int        `json:"images"`
	Passages int        `json:"passages"`
	Last     *time.Time `json:"last,omitempty"`
}

func (m *Manager) add(c Capture) {
	m.records = append(m.records, c)
	v := m.totals[c.Serial]
	v.Images++
	if m.passages[c.Serial] == nil {
		m.passages[c.Serial] = map[string]bool{}
	}
	if !m.passages[c.Serial][c.Passage] {
		m.passages[c.Serial][c.Passage] = true
		v.Passages++
	}
	if v.Last == nil || c.Timestamp.After(*v.Last) {
		t := c.Timestamp
		v.Last = &t
	}
	m.totals[c.Serial] = v
}
func (m *Manager) Stats(serial string) Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := m.totals[serial]
	if v.Last != nil {
		t := *v.Last
		v.Last = &t
	}
	return v
}

func (m *Manager) SetRotationSource(source func(string) int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rotation = source
}
