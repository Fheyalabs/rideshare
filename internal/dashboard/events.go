// Package dashboard provides the live event bus + SSE handler for the demo dashboard.
package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Event is a lifecycle event emitted during the demo loop.
type Event struct {
	Type    string `json:"type"`    // "phase", "wire", "winner", "loop"
	Party   string `json:"party,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Payload string `json:"payload,omitempty"` // hex-encoded ciphertext or message
}

// Bus is a thread-safe broadcast channel for dashboard events.
type Bus struct {
	mu       sync.RWMutex
	subs     map[chan []byte]struct{}
	in       chan Event
	bufSize  int
}

// NewBus returns a Bus with the given event buffer size.
func NewBus(bufSize int) *Bus {
	b := &Bus{
		subs:    make(map[chan []byte]struct{}),
		in:      make(chan Event, bufSize),
		bufSize: bufSize,
	}
	go b.broadcast()
	return b
}

// Emit sends an event to all subscribers. Non-blocking (drops if buffer full).
func (b *Bus) Emit(ev Event) {
	select {
	case b.in <- ev:
	default:
	}
}

func (b *Bus) broadcast() {
	for ev := range b.in {
		data, _ := json.Marshal(ev)
		b.mu.RLock()
		for ch := range b.subs {
			select {
			case ch <- data:
			default:
			}
		}
		b.mu.RUnlock()
	}
}

// Subscribe returns a channel that receives JSON-encoded events.
func (b *Bus) Subscribe() chan []byte {
	ch := make(chan []byte, b.bufSize)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber.
func (b *Bus) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

// Handler returns an http.HandlerFunc that serves SSE.
func (b *Bus) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		ch := b.Subscribe()
		defer b.Unsubscribe(ch)

		// Send initial connected event
		fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
		flusher.Flush()

		for {
			select {
			case data, ok := <-ch:
				if !ok { return }
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
