// Package server wires the rideshare HTTP (REST) endpoints over the discovery,
// session, and offer-pool components. Big ciphertexts move by content handle via
// a content-addressed artifact store; clients poll. The server is BLIND — it
// never holds a secret key and never sees coordinates (clients send H3 cell ids,
// never GPS); it relays ciphertext handles and runs the blind auction.
package server

import "net/http"

// Config holds the server's startup parameters.
type Config struct {
	Addr string // listen address, e.g. ":8080"
}

// Server is the rideshare HTTP service.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New returns a Server ready to handle requests.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return s
}

// Handler returns the HTTP handler (for httptest + ListenAndServe).
func (s *Server) Handler() http.Handler { return s.mux }

// ServerMux returns the underlying ServeMux so callers can mount additional
// routes (e.g. the dashboard) on the same server.
func (s *Server) ServerMux() *http.ServeMux { return s.mux }
