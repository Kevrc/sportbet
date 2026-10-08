package eventbus

import (
	"context"
	"log/slog"
	"sync"
)

type subscription struct {
	id   int
	name string
	ch   chan Event
	done chan struct{}
}

// InMemory entrega cada evento a cada suscriptor en su propia goroutine, con
// un canal con búfer. Si el búfer está lleno, el evento se descarta con un
// warning. Un pánico en un handler se recupera.
type InMemory struct {
	log    *slog.Logger
	buffer int

	mu     sync.RWMutex
	subs   map[Topic][]*subscription
	nextID int
	closed bool
	wg     sync.WaitGroup
}

func NewInMemory(log *slog.Logger, buffer int) *InMemory {
	if buffer <= 0 {
		buffer = 256
	}
	return &InMemory{log: log, buffer: buffer, subs: map[Topic][]*subscription{}}
}

func (b *InMemory) Publish(ctx context.Context, e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for _, s := range b.subs[e.Topic()] {
		select {
		case s.ch <- e:
		default:
			b.log.Warn("eventbus: búfer lleno, evento descartado", "topic", e.Topic(), "subscriber", s.name)
		}
	}
}

func (b *InMemory) Subscribe(t Topic, name string, h Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	s := &subscription{id: b.nextID, name: name, ch: make(chan Event, b.buffer), done: make(chan struct{})}
	b.subs[t] = append(b.subs[t], s)
	b.wg.Add(1)
	go b.loop(s, h)

	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			list := b.subs[t]
			for i, x := range list {
				if x.id == s.id {
					b.subs[t] = append(list[:i:i], list[i+1:]...)
					close(s.ch)
					break
				}
			}
			b.mu.Unlock()
		})
	}
}

func (b *InMemory) loop(s *subscription, h Handler) {
	defer b.wg.Done()
	for e := range s.ch {
		func() {
			defer func() {
				if r := recover(); r != nil {
					b.log.Error("eventbus: pánico en handler", "subscriber", s.name, "topic", e.Topic(), "panic", r)
				}
			}()
			h(context.Background(), e)
		}()
	}
}

// Close deja de aceptar eventos y espera a que los handlers vacíen sus búferes.
func (b *InMemory) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	for t, list := range b.subs {
		for _, s := range list {
			close(s.ch)
		}
		delete(b.subs, t)
	}
	b.mu.Unlock()
	b.wg.Wait()
}
