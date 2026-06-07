package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fheyalabs/rideshare/internal/discovery"
)

// Pure-Go (no FHE): the discovery + invite flow. Drivers push coarse cells, the
// rider opens a session from its own coarse cell, the server discovers candidates
// and creates invites carrying the offered price + coarse dropoff hex. No
// coordinates ever reach the server.
func TestDiscovery_GridDiscoverInvite(t *testing.T) {
	rs := NewRideshareServer(Config{})
	ts := httptest.NewServer(rs.Handler())
	defer ts.Close()

	// Rider uploads its public key (opaque bytes for this pure-Go test).
	pkHandle := putArtifact(t, ts.URL, []byte("fake-pk"))

	// Two drivers push their coarse cells (client-computed; server sees no GPS).
	cellA := discovery.CellToString(discovery.CellAt(51.0500, 13.7384, discovery.BaseResolution))
	cellB := discovery.CellToString(discovery.CellAt(51.0495, 13.7390, discovery.BaseResolution))
	pushGrid(t, ts.URL, "drv-A", cellA, true)
	pushGrid(t, ts.URL, "drv-B", cellB, true)

	riderCell := discovery.CellToString(discovery.CellAt(51.0493, 13.7384, discovery.BaseResolution))

	// Preview discovery.
	if cands := discoverCell(t, ts.URL, riderCell, 1, 4); len(cands) < 1 {
		t.Fatalf("discover: expected >=1 candidate, got %d", len(cands))
	}

	// Open a session — server discovers candidates and creates invites for them.
	dropoff := discovery.CellToString(discovery.CellAt(51.0600, 13.7500, discovery.BaseResolution-2))
	openCands := openSessionCell(t, ts.URL, "ride-pure-1", pkHandle, riderCell, 1, 4, 1500, dropoff)
	if len(openCands) < 1 {
		t.Fatalf("open: expected >=1 candidate, got %d", len(openCands))
	}

	// A candidate polls its invites and sees the offered price + coarse dropoff.
	invs := getInvites(t, ts.URL, openCands[0])
	if len(invs) < 1 {
		t.Fatalf("expected >=1 invite for %s, got 0", openCands[0])
	}
	if invs[0].OfferedPrice != 1500 {
		t.Errorf("offered_price = %d, want 1500", invs[0].OfferedPrice)
	}
	if invs[0].DropoffHex != dropoff {
		t.Errorf("dropoff_hex = %q, want %q", invs[0].DropoffHex, dropoff)
	}
	if invs[0].SessionID != "ride-pure-1" {
		t.Errorf("session_id = %q, want ride-pure-1", invs[0].SessionID)
	}
}

// The server takes H3 cell ids, never coordinates — a malformed cell is rejected.
func TestDiscovery_RejectsInvalidCell(t *testing.T) {
	rs := NewRideshareServer(Config{})
	ts := httptest.NewServer(rs.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{"cell": "not-a-cell", "target": 1, "max_widen": 0})
	resp, err := http.Post(ts.URL+"/discover", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid cell should be 400, got %d", resp.StatusCode)
	}
}

// --- shared HTTP helpers (also used by the openfhe e2e test) ---

func putArtifact(t *testing.T, url string, data []byte) string {
	t.Helper()
	enc := base64.StdEncoding.EncodeToString(data)
	body, _ := json.Marshal(map[string]string{"Data": enc})
	resp, err := http.Post(url+"/artifacts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("put artifact: %v", err)
	}
	defer resp.Body.Close()
	var out struct{ Handle string }
	json.NewDecoder(resp.Body).Decode(&out)
	if out.Handle == "" {
		t.Fatal("empty artifact handle")
	}
	return out.Handle
}

func getArtifact(t *testing.T, url, handle string) []byte {
	t.Helper()
	resp, err := http.Get(url + "/artifacts/" + handle)
	if err != nil {
		t.Fatalf("get artifact: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data
}

func pushGrid(t *testing.T, url, pseudonym, cell string, accepting bool) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"pseudonym": pseudonym, "cell": cell, "accepting": accepting})
	resp, err := http.Post(url+"/grid", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("grid: %v", err)
	}
	resp.Body.Close()
}

func discoverCell(t *testing.T, url, cell string, target, maxWiden int) []string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"cell": cell, "target": target, "max_widen": maxWiden})
	resp, err := http.Post(url+"/discover", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Candidates []string `json:"candidates"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Candidates
}

func openSessionCell(t *testing.T, url, sessionID, pkHandle, cell string, target, maxWiden, offeredPrice int, dropoffHex string) []string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"session_id":    sessionID,
		"pk_handle":     pkHandle,
		"cell":          cell,
		"target":        target,
		"max_widen":     maxWiden,
		"offered_price": offeredPrice,
		"dropoff_hex":   dropoffHex,
		"floor_cents":   800, "cap_cents": 5000,
		"ring_dim": 1 << 15, "depth": 5,
	})
	resp, err := http.Post(url+"/session/open", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("open session: status %d", resp.StatusCode)
	}
	var out struct {
		Candidates []string `json:"candidates"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Candidates
}

func getInvites(t *testing.T, url, pseudonym string) []Invite {
	t.Helper()
	resp, err := http.Get(url + "/invites/" + pseudonym)
	if err != nil {
		t.Fatalf("invites: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Invites []Invite `json:"invites"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Invites
}
