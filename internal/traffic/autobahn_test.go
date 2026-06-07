package traffic

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fheyalabs/rideshare/internal/routing"
)

func TestAutobahn_ClosuresToEdges(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"closure":[{"identifier":"x","coordinate":{"lat":"51.050","long":"13.740"}}]}`))
	}))
	defer ts.Close()

	pts, err := FetchClosures(ts.URL, "A17")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(pts) != 1 {
		t.Fatalf("want 1 closure, got %d", len(pts))
	}

	g := routing.NewGraph()
	g.AddNode(1, 51.0500, 13.7400)
	g.AddNode(2, 51.0505, 13.7400)
	g.AddEdge(1, 2, routing.ClassMotorway, 120, true)
	g.Edges[0].Ref = "A17"

	closed := MatchClosures(g, pts, "A17", 200)
	if !closed[routing.EdgeKey{From: 1, To: 2}] {
		t.Error("A17 edge near the closure point must be closed")
	}
}
