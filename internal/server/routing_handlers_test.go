package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fheyalabs/rideshare/internal/routing"
	"github.com/Fheyalabs/rideshare/internal/traffic"
)

func TestRoutingHandlers(t *testing.T) {
	g := routing.NewGraph()
	g.AddNode(1, 51.05, 13.74)
	g.AddNode(2, 51.06, 13.74)
	g.AddEdge(1, 2, routing.ClassPrimary, 50, false)

	rs := NewRideshareServer(Config{})
	rs.SetRegion(g, traffic.SimulatedProvider{})
	ts := httptest.NewServer(rs.Handler())
	defer ts.Close()

	// region download deserializes
	resp, _ := http.Get(ts.URL + "/region")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if g2, err := routing.Deserialize(body); err != nil || len(g2.Edges) != 2 {
		t.Fatalf("region download bad: err=%v", err)
	}
	// customization is present
	resp, _ = http.Get(ts.URL + "/customization")
	var cm routing.Metric
	json.NewDecoder(resp.Body).Decode(&cm)
	resp.Body.Close()
	if cm.ClassMult == nil {
		t.Fatal("customization missing class multipliers")
	}
	// decoy-padded slices: 2 node-sets (1 real + 1 decoy); server returns 2 slices
	reqBody, _ := json.Marshal(map[string]any{"node_sets": [][]int64{{1, 2}, {1}}})
	resp, _ = http.Post(ts.URL+"/slices", "application/json", bytes.NewReader(reqBody))
	var out struct {
		Slices []json.RawMessage `json:"slices"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	if len(out.Slices) != 2 {
		t.Fatalf("want 2 slices, got %d", len(out.Slices))
	}
}
