package server

import (
	"encoding/json"
	"net/http"

	"github.com/Fheyalabs/ares-core/pkg/ares/transport"
	"github.com/Fheyalabs/rideshare/internal/discovery"
	"github.com/Fheyalabs/rideshare/internal/offerpool"
	"github.com/Fheyalabs/rideshare/internal/session"
)

// RideshareServer extends the base Server with discovery, session, and offer-pool.
type RideshareServer struct {
	*Server
	artifacts *transport.ArtifactStore
	registry  *discovery.Registry
	pools     map[string]*offerpool.Pool // sessionID → pool
	sessions  map[string]*session.AuctionSession
}

// NewRideshareServer returns a server with rideshare endpoints mounted.
func NewRideshareServer(cfg Config) *RideshareServer {
	rs := &RideshareServer{
		Server:    New(cfg),
		artifacts: transport.NewArtifactStore(),
		registry:  discovery.NewRegistry(),
		pools:     make(map[string]*offerpool.Pool),
		sessions:  make(map[string]*session.AuctionSession),
	}
	rs.mux.HandleFunc("POST /artifacts", rs.handleArtifactPut)
	rs.mux.HandleFunc("GET /artifacts/{handle}", rs.handleArtifactGet)
	rs.mux.HandleFunc("POST /heartbeat", rs.handleHeartbeat)
	rs.mux.HandleFunc("POST /discover", rs.handleDiscover)
	rs.mux.HandleFunc("POST /session/open", rs.handleSessionOpen)
	rs.mux.HandleFunc("POST /session/bid", rs.handleSessionBid)
	rs.mux.HandleFunc("POST /offer/accept", rs.handleOfferAccept)
	rs.mux.HandleFunc("POST /offer/cancel", rs.handleOfferCancel)
	rs.mux.HandleFunc("GET /offer/held", rs.handleOfferHeld)
	return rs
}

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

// --- discovery ---

func (rs *RideshareServer) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pseudonym string `json:"pseudonym"`
		Lat       float64
		Lng       float64
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	rs.registry.Heartbeat(body.Pseudonym, discovery.CellAt(body.Lat, body.Lng, discovery.BaseResolution))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (rs *RideshareServer) handleDiscover(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Lat      float64
		Lng      float64
		Target   int `json:"target"`
		MaxWiden int `json:"max_widen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	cands, cell := rs.registry.Discover(
		discovery.CellAt(body.Lat, body.Lng, discovery.BaseResolution),
		body.Target, body.MaxWiden,
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"candidates": cands,
		"cell":       cell,
	})
}

// --- session ---

type sessionOpenReq struct {
	SessionID    string  `json:"session_id"`
	PKHandle     string  `json:"pk_handle"`
	OfferedPrice int     `json:"offered_price"`
	DropoffHex   string  `json:"dropoff_hex"`
	FloorCents   int     `json:"floor_cents"`
	CapCents     int     `json:"cap_cents"`
	RingDim      uint32  `json:"ring_dim"`
	Depth        uint32  `json:"depth"`
}

func (rs *RideshareServer) handleSessionOpen(w http.ResponseWriter, r *http.Request) {
	var body sessionOpenReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	// Retrieve pk from artifact store
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
	// Create session
	sess := session.NewAuctionSession([]byte(body.SessionID),
		defaultSessionParams(body.RingDim, body.Depth),
		auctionPriceBand(body.FloorCents, body.CapCents),
		session.DefaultWeights(), 1)
	sess.SetRiderPK(pk)

	rs.sessions[body.SessionID] = sess
	rs.pools[body.SessionID] = offerpool.New(3, holdTTL)

	writeJSON(w, http.StatusCreated, map[string]string{
		"session_id":    body.SessionID,
		"offered_price": itoa(body.OfferedPrice),
		"dropoff_hex":   body.DropoffHex,
	})
}

type sessionBidReq struct {
	SessionID string `json:"session_id"`
	BidHandle string `json:"bid_handle"`
	Nonce     []byte `json:"nonce"`
	Pubkey    []byte `json:"pubkey"`
	Sig       []byte `json:"sig"`
	StarNorm  float64
	DistSq    float64
}

func (rs *RideshareServer) handleSessionBid(w http.ResponseWriter, r *http.Request) {
	var body sessionBidReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	sess, ok := rs.sessions[body.SessionID]
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	// Retrieve encrypted bid from artifact store
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
	if err := sess.SubmitBid(sb, body.StarNorm, body.DistSq); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// --- offer pool ---

func (rs *RideshareServer) handleOfferAccept(w http.ResponseWriter, r *http.Request) {
	var body struct{ SessionID, OfferID string }
	json.NewDecoder(r.Body).Decode(&body)
	p, ok := rs.pools[body.SessionID]
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
	p, ok := rs.pools[body.SessionID]
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
	p, ok := rs.pools[body.SessionID]
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
