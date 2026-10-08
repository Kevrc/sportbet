package eventbus

import (
	"context"
	"sync"
)

// Recorder es un Bus síncrono para pruebas: guarda lo publicado y llama a los
// handlers en la misma goroutine.
type Recorder struct {
	mu     sync.Mutex
	events []Event
	subs   map[Topic][]Handler
}

func NewRecorder() *Recorder { return &Recorder{subs: map[Topic][]Handler{}} }

func (r *Recorder) Publish(ctx context.Context, e Event) {
	r.mu.Lock()
	r.events = append(r.events, e)
	hs := append([]Handler(nil), r.subs[e.Topic()]...)
	r.mu.Unlock()
	for _, h := range hs {
		h(ctx, e)
	}
}

func (r *Recorder) Subscribe(t Topic, _ string, h Handler) func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs[t] = append(r.subs[t], h)
	return func() {}
}

func (r *Recorder) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}
