package frames

// Buffer is a bounded ring; the owning worker serializes access.
type Buffer struct {
	data        []*Frame
	next, count int
}

func NewBuffer(capacity int) *Buffer {
	if capacity < 1 {
		capacity = 1
	}
	return &Buffer{data: make([]*Frame, capacity)}
}
func (b *Buffer) Push(f *Frame) {
	b.data[b.next] = f
	b.next = (b.next + 1) % len(b.data)
	if b.count < len(b.data) {
		b.count++
	}
}
func (b *Buffer) Snapshot() []*Frame {
	out := make([]*Frame, 0, b.count)
	start := (b.next - b.count + len(b.data)) % len(b.data)
	for i := 0; i < b.count; i++ {
		out = append(out, b.data[(start+i)%len(b.data)])
	}
	return out
}
