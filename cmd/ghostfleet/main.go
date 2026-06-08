package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/discovery"
	"github.com/Fheyalabs/rideshare/internal/ghost"
	"github.com/Fheyalabs/rideshare/internal/movement"
	"github.com/Fheyalabs/rideshare/internal/routing"
)

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "rideshare server URL")
	regionPath := flag.String("region", "", "serialized region.bin")
	n := flag.Int("n", 5, "ghost driver count")
	strat := flag.String("price", "pct:0.1", "price strategy: pct:X.X or fixed:NNNN")
	seed := flag.Int64("seed", 42, "PRNG seed")
	tick := flag.Float64("tick", 5, "sim seconds per tick")
	riderMode := flag.Bool("rider", false, "run one test-rider loop")
	flag.Parse()

	// Load region graph.
	if *regionPath == "" {
		log.Fatal("-region required")
	}
	data, err := os.ReadFile(*regionPath)
	if err != nil {
		log.Fatalf("read region: %v", err)
	}
	g, err := routing.Deserialize(data)
	if err != nil {
		log.Fatalf("deserialize region: %v", err)
	}
	log.Printf("region: %d nodes, %d edges", len(g.Nodes), len(g.Edges))

	w := routing.Customize(g, routing.Metric{})
	eng := movement.NewEngine(g, w, *seed)

	// Parse price strategy.
	var stratFn ghost.PriceStrategy
	if _, err := fmt.Sscanf(*strat, "pct:%f", new(float64)); err == nil {
		var pct float64
		fmt.Sscanf(*strat, "pct:%f", &pct)
		stratFn = ghost.PercentBelow(pct)
	} else {
		var fixed int
		fmt.Sscanf(*strat, "fixed:%d", &fixed)
		stratFn = ghost.Fixed(fixed)
	}

	drivers := make([]ghost.Driver, *n)
	signers := make([]sign.Signer, *n)
	client := ghost.NewClient(*serverURL)

	eng.OnCellChange = func(id string, cell discovery.Cell) {
		cStr := discovery.CellToString(cell)
		if err := client.PushGrid(id, cStr); err != nil {
			log.Printf("[%s] grid push: %v", id, err)
		}
	}

	for i := 0; i < *n; i++ {
		id := fmt.Sprintf("ghost-%02d", i)
		drivers[i] = ghost.Driver{Pseudonym: id, MinCents: 800, Strategy: stratFn}
		signers[i], _ = sign.NewEd25519Signer()
		startNode := int64((i * 7919) % len(g.Nodes)) // deterministic spread
		eng.Add(id, startNode)
		if c := eng.Actor(id).Cell(); c != 0 {
			client.PushGrid(id, discovery.CellToString(c))
		}
	}
	log.Printf("started %d ghosts", *n)

	// Main loop: tick engine, poll invites, bid.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		eng.Tick(*tick)
		for i, d := range drivers {
			invs, err := client.Invites(d.Pseudonym)
			if err != nil {
				continue
			}
			for _, inv := range invs {
				bid, ok := d.DecideBid(inv)
				if !ok {
					continue
				}
				log.Printf("[%s] bidding %d (offer %d)", d.Pseudonym, bid, inv.OfferedPrice)
				submitBidReal(d, inv, bid, signers[i], client, 1<<15, 5, inv.SessionID)
			}
		}
		if *riderMode {
			// test rider loop (openfhe only)
		}
	}
}
