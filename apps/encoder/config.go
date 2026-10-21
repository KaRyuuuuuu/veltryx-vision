package encoder

import "errors"

type Config struct {
	Preset      string
	Tune        string
	BitrateKbps int
	FPS         float64
}

func Default() Config {
	return Config{Preset: "veryfast", Tune: "zerolatency", BitrateKbps: 2500, FPS: 25}
}
func (c Config) Validate() error {
	valid := false
	for _, p := range []string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow"} {
		if c.Preset == p {
			valid = true
		}
	}
	if !valid || c.Tune != "zerolatency" || c.BitrateKbps < 64 || c.BitrateKbps > 100000 || c.FPS < 1 || c.FPS > 240 {
		return errors.New("invalid x264 configuration")
	}
	return nil
}
