// Package dashboard provides the live event bus + SSE handler for the demo dashboard.
package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	h3 "github.com/uber/h3-go/v4"
)

// Event is a lifecycle event emitted during the demo loop.
type Event struct {
	Type     string           `json:"type"`              // "phase", "wire", "marker", "hex", "loop", "connected"
	Party    string           `json:"party,omitempty"`   // rider, driver pseudonym, server, cell
	Phase    string           `json:"phase,omitempty"`   // lifecycle phase (IDLE, DISCOVERING, KEYGEN, BIDDING, etc.)
	Detail   string           `json:"detail,omitempty"`  // human-readable detail
	Payload  string           `json:"payload,omitempty"` // wire payload (hex ciphertext, message)
	Lat      float64          `json:"lat,omitempty"`     // GPS latitude (for markers + hex overlay)
	Lng      float64          `json:"lng,omitempty"`     // GPS longitude
	Geometry *json.RawMessage `json:"geometry,omitempty"` // pre-computed GeoJSON polygon (for hex cells)
}

// HexGeometry computes a GeoJSON Polygon for an H3 cell.
func HexGeometry(cell uint64) []byte {
	b, err := h3.CellToBoundary(h3.Cell(cell))
	if err != nil || len(b) == 0 {
		return nil
	}
	// Build GeoJSON Polygon: [[[lng,lat],[lng,lat],...]]
	ring := make([][]float64, 0, len(b)+1)
	for _, v := range b {
		ring = append(ring, []float64{v.Lng, v.Lat})
	}
	ring = append(ring, ring[0]) // close ring
	geom := struct {
		Type        string        `json:"type"`
		Coordinates [][][]float64 `json:"coordinates"`
	}{Type: "Polygon", Coordinates: [][][]float64{ring}}
	data, _ := json.Marshal(geom)
	return data
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
