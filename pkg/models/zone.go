package models

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Zone struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Kind            string  `json:"kind"`
	Points          []Point `json:"points"`
	PixelDelta      int     `json:"pixel_delta"`
	MinPixels       int     `json:"min_pixels"`
	ConfirmFrames   int     `json:"confirm_frames"`
	QuietFrames     int     `json:"quiet_frames"`
	CooldownMS      int     `json:"cooldown_ms"`
	MaxChangedRatio float64 `json:"max_changed_ratio"`
	PreFrames       int     `json:"pre_frames"`
	PostFrames      int     `json:"post_frames"`
}
