package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/Fheyalabs/rideshare/internal/routing"
	"github.com/Fheyalabs/rideshare/internal/traffic"
)

type regionState struct {
	mu       sync.RWMutex
	graph    *routing.Graph
	serial   []byte
	provider traffic.TrafficProvider
}

// SetRegion installs the customizable region graph + traffic provider.
func (rs *RideshareServer) SetRegion(g *routing.Graph, p traffic.TrafficProvider) {
	b, _ := routing.Serialize(g)
	rs.region.mu.Lock()
	rs.region.graph, rs.region.serial, rs.region.provider = g, b, p
	rs.region.mu.Unlock()
}

func (rs *RideshareServer) handleRegion(w http.ResponseWriter, r *http.Request) {
	rs.region.mu.RLock()
	b := rs.region.serial
	rs.region.mu.RUnlock()
	if b == nil {
		http.Error(w, "no region loaded", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func (rs *RideshareServer) handleCustomization(w http.ResponseWriter, r *http.Request) {
	rs.region.mu.RLock()
	p := rs.region.provider
	rs.region.mu.RUnlock()
	if p == nil {
		http.Error(w, "no provider", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, p.Metric(time.Now()))
}

func (rs *RideshareServer) handleSlices(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NodeSets [][]int64 `json:"node_sets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(body.NodeSets) > 32 {
		http.Error(w, "too many node sets (max 32)", http.StatusBadRequest)
		return
	}
	for _, set := range body.NodeSets {
		if len(set) > 4096 {
			http.Error(w, "node set too large (max 4096)", http.StatusBadRequest)
			return
		}
	}
	rs.region.mu.RLock()
	g := rs.region.graph
	rs.region.mu.RUnlock()
	if g == nil {
		http.Error(w, "no region loaded", http.StatusServiceUnavailable)
		return
	}
	slices := make([]json.RawMessage, len(body.NodeSets))
	for i, set := range body.NodeSets {
		keep := make(map[int64]bool, len(set))
		for _, id := range set {
			keep[id] = true
		}
		b, _ := routing.Serialize(routing.Slice(g, keep))
		slices[i] = b
	}
	writeJSON(w, http.StatusOK, map[string]any{"slices": slices})
}
