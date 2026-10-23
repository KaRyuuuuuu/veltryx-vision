package models

type Settings struct {
	AutoExposure bool    `json:"auto_exposure"`
	AutoGain     bool    `json:"auto_gain"`
	FPSUnlimited bool    `json:"fps_unlimited"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	FPS          float64 `json:"fps"`
	Exposure     float64 `json:"exposure_us"`
	Gain         float64 `json:"gain"`
}
type Range struct {
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	Increment float64 `json:"increment,omitempty"`
}
type Capabilities struct {
	AutoExposure bool  `json:"auto_exposure"`
	AutoGain     bool  `json:"auto_gain"`
	FPSUnlimited bool  `json:"fps_unlimited"`
	Width        Range `json:"width"`
	Height       Range `json:"height"`
	FPS          Range `json:"fps"`
	Exposure     Range `json:"exposure_us"`
	Gain         Range `json:"gain"`
}
type Camera struct {
	Capabilities *Capabilities `json:"capabilities,omitempty"`

	Serial    string   `json:"serial"`
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Vendor    string   `json:"vendor"`
	Model     string   `json:"model"`
	IP        string   `json:"ip"`
	Formats   []string `json:"formats"`
	Settings  Settings `json:"settings"`
	Connected bool     `json:"connected"`
	Simulated bool     `json:"simulated"`
}
