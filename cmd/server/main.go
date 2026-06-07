package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Fheyalabs/rideshare/internal/routing"
	"github.com/Fheyalabs/rideshare/internal/server"
	"github.com/Fheyalabs/rideshare/internal/traffic"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	regionPath := flag.String("region", "", "serialized region.bin (from cmd/buildgraph)")
	autobahnBase := flag.String("autobahn", "", "Autobahn API base URL (empty = no closures)")
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
		// Wire Autobahn closure poller.
		var closureFn func() map[routing.EdgeKey]bool
		if *autobahnBase != "" {
			poller := traffic.NewClosurePoller(*autobahnBase,
				[]string{"A4", "A13", "A17"}, g, 5*time.Minute, 300)
			poller.Start()
			closureFn = poller.Closed
		}
		srv.SetRegion(g, traffic.CombinedProvider{
			Base:    traffic.SimulatedProvider{},
			Closure: closureFn,
		})
		log.Printf("region loaded: %d nodes, %d edges (autobahn=%q)", len(g.Nodes), len(g.Edges), *autobahnBase)
	}

	log.Printf("rideshare server listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
