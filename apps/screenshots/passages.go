package screenshots

import (
	"sort"
	"time"
)

type Passage struct {
	ID             string    `json:"id"`
	Serial         string    `json:"serial"`
	Zone           string    `json:"zone,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	ImageCount     int       `json:"image_count"`
	Representative Capture   `json:"representative"`
}

// Passages selects the trigger frame, or the chronological middle for legacy captures.
// Group before limiting so an API page never cuts the pre/post frames of a passage.
func (m *Manager) Passages(serial string, limit int) []Passage {
	m.mu.Lock()
	records := append([]Capture(nil), m.records...)
	m.mu.Unlock()
	groups := map[string][]Capture{}
	for _, c := range records {
		if serial == "" || serial == c.Serial {
			groups[c.Passage] = append(groups[c.Passage], c)
		}
	}
	out := make([]Passage, 0, len(groups))
	for id, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			if group[i].Timestamp.Equal(group[j].Timestamp) {
				return group[i].Sequence < group[j].Sequence
			}
			return group[i].Timestamp.Before(group[j].Timestamp)
		})
		best := group[len(group)/2]
		for _, c := range group {
			if c.Role == "trigger" {
				best = c
				break
			}
		}
		out = append(out, Passage{ID: id, Serial: best.Serial, Zone: best.Zone, Timestamp: best.Timestamp, ImageCount: len(group), Representative: best})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Timestamp.Equal(out[j].Timestamp) {
			return out[i].ID < out[j].ID
		}
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
