package screenshots

import (
	"errors"
	"sync/atomic"

	"github.com/veltryx/vision/internal/frames"
)

type task struct {
	frame         *frames.Frame
	passage, zone string
	role          string
}

// Queue keeps disk I/O outside acquisition and motion loops. Enqueue never blocks.
// One camera owns each queue; Close must follow the last Enqueue.
type Queue struct {
	tasks  chan task
	bytes  atomic.Int64
	done   chan struct{}
	report func(Capture, error)
}

func NewQueue(m *Manager, report func(Capture, error)) *Queue {
	q := &Queue{tasks: make(chan task, 64), done: make(chan struct{}), report: report}
	go func() {
		defer close(q.done)
		for t := range q.tasks {
			c, e := m.Save(t.frame, "jpeg", t.passage, t.zone, t.role)
			q.bytes.Add(-int64(len(t.frame.Pixels)))
			report(c, e)
		}
	}()
	return q
}
func (q *Queue) Enqueue(f *frames.Frame, passage, zone string, role ...string) bool {
	kind := ""
	if len(role) > 0 {
		kind = role[0]
	}
	n := int64(len(f.Pixels))
	if q.bytes.Add(n) > 64*1024*1024 {
		q.bytes.Add(-n)
		q.report(Capture{Serial: f.Serial}, errors.New("screenshot queue memory budget exceeded; image dropped"))
		return false
	}
	select {
	case q.tasks <- task{f, passage, zone, kind}:
		return true
	default:
		q.bytes.Add(-n)
		q.report(Capture{Serial: f.Serial}, errors.New("screenshot queue full; image dropped"))
		return false
	}
}
func (q *Queue) Close() { close(q.tasks); <-q.done }
