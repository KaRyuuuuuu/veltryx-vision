package ocr

import (
	"context"
	"errors"
	"math"

	"github.com/veltryx/vision/apps/screenshots"
)

type Result struct {
	Passage string `json:"passage"`
	Reading
	Samples int `json:"samples"`
}

// Consensus combines independent frame readings; an injected engine owns image loading.
func Consensus(ctx context.Context, e Engine, shots []screenshots.Capture) (Result, error) {
	if len(shots) == 0 {
		return Result{}, errors.New("empty passage")
	}
	scores := map[string]float64{}
	r := Result{Passage: shots[0].Passage}
	for _, s := range shots {
		if s.Passage != r.Passage {
			return Result{}, errors.New("mixed passages")
		}
		v, err := e.Recognize(ctx, s)
		if err != nil {
			return r, err
		}
		if v.Number == "" || math.IsNaN(v.Confidence) || v.Confidence < 0 || v.Confidence > 1 {
			continue
		}
		scores[v.Number] += v.Confidence
		r.Samples++
	}
	best := ""
	for n, score := range scores {
		if score > scores[best] || (score == scores[best] && (best == "" || n < best)) {
			best = n
		}
	}
	if best != "" {
		r.Number = best
		r.Confidence = scores[best] / float64(len(shots))
	}
	return r, nil
}
