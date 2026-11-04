package event

import "sync"

// Event is an invalidation hint, not data: clients refetch the topic.
type Event struct {
	Topic       string `json:"topic" enum:"scores,candidates,rubrics,interviews,users"`
	InterviewID string `json:"interviewId,omitempty"`
}

type Sub struct {
	C chan Event
	// Lost is closed when the subscriber fell behind. The stream must end so
	// the client reconnects and resyncs, instead of silently missing events.
	Lost chan struct{}
}

type Bus struct {
	mu   sync.Mutex
	subs map[*Sub]struct{}
}

func NewBus() *Bus { return &Bus{subs: map[*Sub]struct{}{}} }

func (b *Bus) Subscribe() *Sub {
	s := &Sub{C: make(chan Event, 64), Lost: make(chan struct{})}
	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()
	return s
}

func (b *Bus) Unsubscribe(s *Sub) {
	b.mu.Lock()
	delete(b.subs, s)
	b.mu.Unlock()
}

// Publish never blocks. A full subscriber is dropped and its Lost is closed.
// A nil bus is a no-op so slices work without one in tests.
func (b *Bus) Publish(e Event) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs {
		select {
		case s.C <- e:
		default:
			delete(b.subs, s)
			close(s.Lost)
		}
	}
}
