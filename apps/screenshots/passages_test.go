package screenshots

import (
	"testing"
	"time"

	"github.com/veltryx/vision/internal/frames"
)

func TestPassagesPreferTriggerAndKeepWholeGroups(t *testing.T) {
	dir := t.TempDir()
	m, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	f := &frames.Frame{Serial: "a", Width: 2, Height: 2, Pixels: make([]byte, 4)}
	for i, role := range []string{"pre", "trigger", "post", "post", "post"} {
		f.Timestamp = time.Unix(int64(10+i), 123000000)
		f.Sequence = uint64(i)
		if _, e = m.Save(f, "png", "new", "roi", role); e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 3; i++ {
		f.Timestamp = time.Unix(int64(i), 0)
		if _, e = m.Save(f, "png", "legacy", "roi"); e != nil {
			t.Fatal(e)
		}
	}
	restored, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	groups := restored.Passages("a", 1)
	if len(groups) != 1 || groups[0].ImageCount != 5 || groups[0].Representative.Role != "trigger" || groups[0].Representative.Sequence != 1 {
		t.Fatal(groups)
	}
	all := restored.Passages("a", 10)
	if len(all) != 2 || all[1].Representative.Timestamp.Unix() != 1 {
		t.Fatal(all)
	}
	if len(restored.Passages("other", 10)) != 0 {
		t.Fatal("camera filtering failed")
	}
}
