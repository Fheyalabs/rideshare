package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/Fheyalabs/rideshare/internal/routing"
	"github.com/Fheyalabs/rideshare/internal/server"
	"github.com/Fheyalabs/rideshare/internal/traffic"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	regionPath := flag.String("region", "", "serialized region.bin (from cmd/buildgraph)")
	flag.Parse()

	srv := server.NewRideshareServer(server.Config{Addr: *addr})

	// Load routing region if provided.
	if *regionPath != "" {
		data, err := os.ReadFile(*regionPath)
		if err != nil {
			log.Fatalf("read region %s: %v", *regionPath, err)
		}
		g, err := routing.Deserialize(data)
		if err != nil {
			log.Fatalf("deserialize region: %v", err)
		}
		srv.SetRegion(g, traffic.CombinedProvider{
			Base: traffic.SimulatedProvider{},
			// Autobahn closures wired in when the graph is present.
		})
		log.Printf("region loaded: %d nodes, %d edges", len(g.Nodes), len(g.Edges))
	}

	log.Printf("rideshare server listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
