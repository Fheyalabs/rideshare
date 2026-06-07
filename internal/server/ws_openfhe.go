//go:build openfhe

package server

import "net/http"

// RegisterOpenFHERoutes mounts the openfhe-gated endpoints on an existing RideshareServer.
func (rs *RideshareServer) RegisterOpenFHERoutes() {
	rs.mux.HandleFunc("GET /session/{id}/masks", rs.handleSessionMasks)
}

func (rs *RideshareServer) handleSessionMasks(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("id")
	sess, ok := rs.sessions[sid]
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	masks, err := sess.RunBlindAuction()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	handles := make([]string, len(masks))
	for i, m := range masks {
		h, _ := rs.artifacts.PutContent(m)
		handles[i] = hexEncode(h[:])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":   sid,
		"mask_handles": handles,
		"bid_count":    sess.BidCount(),
	})
}
