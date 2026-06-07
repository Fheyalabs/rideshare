package server

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/Fheyalabs/ares-core/pkg/ares/transport"
	"github.com/Fheyalabs/rideshare/internal/discovery"
	"github.com/Fheyalabs/rideshare/internal/offerpool"
	"github.com/Fheyalabs/rideshare/internal/rating"
	"github.com/Fheyalabs/rideshare/internal/session"
)

// Invite is the public ride information a discovered driver receives so it can
// decide whether to bid. It carries NO exact location — only a coarse dropoff
// hex (direction) and the rider's offered price (for the driver's on-device
// min-€ threshold filter). The driver fetches the rider's public key by PKHandle.
type Invite struct {
	SessionID    string `json:"session_id"`
	OfferedPrice int    `json:"offered_price"`
	DropoffHex   string `json:"dropoff_hex"`
	PKHandle     string `json:"pk_handle"`
}

// RideshareServer extends the base Server with discovery, sessions, and the
// offer pool. Clients compute their own H3 cell and send the cell id, so the
// server never receives raw coordinates. It never holds a secret key.
type RideshareServer struct {
	*Server
	artifacts *transport.ArtifactStore
	registry  *discovery.Registry

	mu       sync.RWMutex
	pools    map[string]*offerpool.Pool         // sessionID → held-offer pool
	sessions map[string]*session.AuctionSession // sessionID → auction session
	invites  map[string][]Invite                // driver pseudonym → pending invites
	region   regionState
	ratings  *rating.Store
}

// NewRideshareServer returns a server with the rideshare endpoints mounted.
func NewRideshareServer(cfg Config) *RideshareServer {
	rs := &RideshareServer{
		Server:    New(cfg),
		artifacts: transport.NewArtifactStore(),
		registry:  discovery.NewRegistry(),
		pools:     make(map[string]*offerpool.Pool),
		sessions:  make(map[string]*session.AuctionSession),
		invites:   make(map[string][]Invite),
		ratings:   rating.NewStore(4.3, 20),
	}
	rs.mux.HandleFunc("POST /artifacts", rs.handleArtifactPut)
	rs.mux.HandleFunc("GET /artifacts/{handle}", rs.handleArtifactGet)
	rs.mux.HandleFunc("POST /grid", rs.handleGrid)
	rs.mux.HandleFunc("POST /discover", rs.handleDiscover)
	rs.mux.HandleFunc("POST /session/open", rs.handleSessionOpen)
	rs.mux.HandleFunc("POST /session/bid", rs.handleSessionBid)
	rs.mux.HandleFunc("GET /invites/{pseudonym}", rs.handleInvites)
	rs.mux.HandleFunc("POST /offer/accept", rs.handleOfferAccept)
	rs.mux.HandleFunc("POST /offer/cancel", rs.handleOfferCancel)
	rs.mux.HandleFunc("GET /offer/held", rs.handleOfferHeld)
	rs.mux.HandleFunc("GET /region", rs.handleRegion)
	rs.mux.HandleFunc("GET /customization", rs.handleCustomization)
	rs.mux.HandleFunc("POST /slices", rs.handleSlices)
	return rs
}

// Ratings returns the server's driver rating store (for seeding test data).
func (rs *RideshareServer) Ratings() *rating.Store { return rs.ratings }

// --- artifacts ---

func (rs *RideshareServer) handleArtifactPut(w http.ResponseWriter, r *http.Request) {
	var body struct{ Data string } // base64-encoded
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	data, err := b64Decode(body.Data)
	if err != nil {
		http.Error(w, "invalid base64 data", http.StatusBadRequest)
		return
	}
	handle, err := rs.artifacts.PutContent(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"handle": hexEncode(handle[:])})
}

func (rs *RideshareServer) handleArtifactGet(w http.ResponseWriter, r *http.Request) {
	hHex := r.PathValue("handle")
	var h [32]byte
	if err := hexDecode(hHex, h[:]); err != nil {
		http.Error(w, "invalid handle", http.StatusBadRequest)
		return
	}
	data, err := rs.artifacts.GetContent(h)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// --- discovery (cell-based; the server never sees coordinates) ---

// handleGrid records a driver's current coarse cell on a grid-change event
// (no heartbeat). accepting=false removes the driver (off-duty / mid-ride).
// The body carries an H3 cell id, never coordinates.
func (rs *RideshareServer) handleGrid(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pseudonym string `json:"pseudonym"`
		Cell      string `json:"cell"`
		Accepting bool   `json:"accepting"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Pseudonym == "" {
		http.Error(w, "pseudonym required", http.StatusBadRequest)
		return
	}
	if !body.Accepting {
		rs.registry.Remove(body.Pseudonym)
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
		return
	}
	cell, err := discovery.CellFromString(body.Cell)
	if err != nil {
		http.Error(w, "invalid cell", http.StatusBadRequest)
		return
	}
	rs.registry.SetCell(body.Pseudonym, cell)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// discover runs the hierarchical registry lookup from a coarse cell id, widening
// up to maxWiden times until target candidates are found. Shared by /discover
// and /session/open.
func (rs *RideshareServer) discover(cellID string, target, maxWiden int) ([]string, discovery.Cell, error) {
	cell, err := discovery.CellFromString(cellID)
	if err != nil {
		return nil, 0, err
	}
	if target <= 0 {
		target = 1
	}
	cands, final := rs.registry.Discover(cell, target, maxWiden)
	return cands, final, nil
}

func (rs *RideshareServer) handleDiscover(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Cell     string `json:"cell"`
		Target   int    `json:"target"`
		MaxWiden int    `json:"max_widen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	cands, final, err := rs.discover(body.Cell, body.Target, body.MaxWiden)
	if err != nil {
		http.Error(w, "invalid cell", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"candidates": cands,
		"cell":       discovery.CellToString(final),
	})
}

// --- session ---

type sessionOpenReq struct {
	SessionID    string `json:"session_id"`
	PKHandle     string `json:"pk_handle"`
	Cell         string `json:"cell"`      // rider's coarse pickup cell (client-computed)
	Target       int    `json:"target"`    // desired candidate count
	MaxWiden     int    `json:"max_widen"` // hierarchical widen cap
	OfferedPrice int    `json:"offered_price"`
	DropoffHex   string `json:"dropoff_hex"` // coarse h3ToParent of dropoff (direction only)
	FloorCents   int    `json:"floor_cents"`
	CapCents     int    `json:"cap_cents"`
	RingDim      uint32 `json:"ring_dim"`
	Depth        uint32 `json:"depth"`
}

func (rs *RideshareServer) handleSessionOpen(w http.ResponseWriter, r *http.Request) {
	var body sessionOpenReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	var pkHandle [32]byte
	if err := hexDecode(body.PKHandle, pkHandle[:]); err != nil {
		http.Error(w, "invalid pk_handle", http.StatusBadRequest)
		return
	}
	pk, err := rs.artifacts.GetContent(pkHandle)
	if err != nil {
		http.Error(w, "pk not found", http.StatusNotFound)
		return
	}
	// Hierarchical discovery from the rider's coarse cell — server never sees GPS.
	cands, final, err := rs.discover(body.Cell, body.Target, body.MaxWiden)
	if err != nil {
		http.Error(w, "invalid cell", http.StatusBadRequest)
		return
	}

	sess := session.NewAuctionSession([]byte(body.SessionID),
		defaultSessionParams(body.RingDim, body.Depth),
		auctionPriceBand(body.FloorCents, body.CapCents),
		session.DefaultWeights(), 1,
		rs.ratings)
	sess.SetRiderPK(pk)

	inv := Invite{
		SessionID:    body.SessionID,
		OfferedPrice: body.OfferedPrice,
		DropoffHex:   body.DropoffHex,
		PKHandle:     body.PKHandle,
	}

	rs.mu.Lock()
	rs.sessions[body.SessionID] = sess
	rs.pools[body.SessionID] = offerpool.New(3, holdTTL)
	for _, p := range cands {
		rs.invites[p] = append(rs.invites[p], inv)
	}
	rs.mu.Unlock()

	writeJSON(w, http.StatusCreated, map[string]any{
		"session_id": body.SessionID,
		"candidates": cands,
		"cell":       discovery.CellToString(final),
	})
}

type sessionBidReq struct {
	SessionID string  `json:"session_id"`
	BidHandle string  `json:"bid_handle"`
	Nonce     []byte  `json:"nonce"`
	Pubkey    []byte  `json:"pubkey"`
	Sig       []byte  `json:"sig"`
	StarNorm  float64 `json:"star_norm"`
	DistSq    float64 `json:"dist_sq"`
}

func (rs *RideshareServer) handleSessionBid(w http.ResponseWriter, r *http.Request) {
	var body sessionBidReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	rs.mu.RLock()
	sess, ok := rs.sessions[body.SessionID]
	rs.mu.RUnlock()
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	var bidHandle [32]byte
	if err := hexDecode(body.BidHandle, bidHandle[:]); err != nil {
		http.Error(w, "invalid bid_handle", http.StatusBadRequest)
		return
	}
	encBid, err := rs.artifacts.GetContent(bidHandle)
	if err != nil {
		http.Error(w, "bid not found", http.StatusNotFound)
		return
	}
	sb := auctionSignedBid{EncBid: encBid, Nonce: body.Nonce, Pubkey: body.Pubkey, Sig: body.Sig}
	if err := sess.SubmitBid(sb); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// handleInvites returns the pending invites for a driver pseudonym. The driver
// polls this, applies its on-device min-€ threshold filter, then decides to bid.
func (rs *RideshareServer) handleInvites(w http.ResponseWriter, r *http.Request) {
	pseudonym := r.PathValue("pseudonym")
	rs.mu.RLock()
	invs := append([]Invite(nil), rs.invites[pseudonym]...)
	rs.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"invites": invs})
}

// --- offer pool ---

func (rs *RideshareServer) pool(sessionID string) (*offerpool.Pool, bool) {
	rs.mu.RLock()
	p, ok := rs.pools[sessionID]
	rs.mu.RUnlock()
	return p, ok
}

func (rs *RideshareServer) handleOfferAccept(w http.ResponseWriter, r *http.Request) {
	var body struct{ SessionID, OfferID string }
	json.NewDecoder(r.Body).Decode(&body)
	p, ok := rs.pool(body.SessionID)
	if !ok {
		http.Error(w, "pool not found", http.StatusNotFound)
		return
	}
	if err := p.Accept(body.OfferID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (rs *RideshareServer) handleOfferCancel(w http.ResponseWriter, r *http.Request) {
	var body struct{ SessionID, OfferID string }
	json.NewDecoder(r.Body).Decode(&body)
	p, ok := rs.pool(body.SessionID)
	if !ok {
		http.Error(w, "pool not found", http.StatusNotFound)
		return
	}
	if err := p.Cancel(body.OfferID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (rs *RideshareServer) handleOfferHeld(w http.ResponseWriter, r *http.Request) {
	var body struct{ SessionID string }
	json.NewDecoder(r.Body).Decode(&body)
	p, ok := rs.pool(body.SessionID)
	if !ok {
		http.Error(w, "pool not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"held": p.Held()})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
