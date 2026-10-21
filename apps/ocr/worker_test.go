package ocr

import (
	"context"
	"errors"
	"github.com/veltryx/vision/apps/screenshots"
	"testing"
)

type engine struct{}

func (engine) Recognize(_ context.Context, s screenshots.Capture) (Reading, error) {
	if s.ID == "different" {
		return Reading{Number: "12", Confidence: 0.8}, nil
	}
	return Reading{Number: "42", Confidence: 0.9}, nil
}
func TestConsensusAndUnavailable(t *testing.T) {
	shots := []screenshots.Capture{{Passage: "p", ID: "one"}, {Passage: "p", ID: "two"}, {Passage: "p", ID: "different"}}
	r, e := Consensus(context.Background(), engine{}, shots)
	if e != nil || r.Number != "42" || r.Confidence < 0.59 || r.Confidence > 0.61 {
		t.Fatal(r, e)
	}
	if _, e = Consensus(context.Background(), Disabled{}, shots); !errors.Is(e, ErrUnavailable) {
		t.Fatal(e)
	}
}
