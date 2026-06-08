package dashboard

import "net/http"

// Mount registers the dashboard routes on a mux.
func Mount(mux *http.ServeMux, bus *Bus) {
	mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(PageHTML))
	})
	mux.HandleFunc("/dashboard/events", bus.Handler())
}
