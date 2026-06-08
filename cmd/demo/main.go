//go:build openfhe

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/discovery"
	"github.com/Fheyalabs/rideshare/internal/ghost"
	"github.com/Fheyalabs/rideshare/internal/server"
)

func main() {
	port := flag.Int("port", 8080, "server port")
	n := flag.Int("n", 5, "ghost driver count")
	interval := flag.Duration("interval", 5*time.Second, "time between lifecycle loops")
	maxLoops := flag.Int("loops", 0, "max loops (0 = infinite)")
	flag.Parse()

	prefix := fmt.Sprintf("demo-%d", time.Now().Unix())
	params := cgo.ContractParams{RingDim: 1 << 15, Depth: 5, ScalingFactor: float64(uint64(1) << 50)}

	rs := server.NewRideshareServer(server.Config{Addr: fmt.Sprintf(":%d", *port)})
	rs.RegisterOpenFHERoutes()
	go func() {
		log.Printf("[demo] server on :%d", *port)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), rs.Handler()))
	}()
	time.Sleep(500 * time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%d", *port)
	client := ghost.NewClient(baseURL)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	dresdenLat, dresdenLng := 51.0493, 13.7384
	dresdenCell := discovery.CellAt(dresdenLat, dresdenLng, discovery.BaseResolution)
	dropoff := discovery.CellAt(51.0600, 13.7500, discovery.BaseResolution-2)

	for loop := 1; *maxLoops == 0 || loop <= *maxLoops; loop++ {
		sessionID := fmt.Sprintf("%s-%04d", prefix, loop)

		// Fresh drivers each loop (prevents registry accumulation)
		drivers := make([]ghost.Driver, *n)
		signers := make([]sign.Signer, *n)
		for i := 0; i < *n; i++ {
			drvLat := dresdenLat + (rng.Float64()-0.5)*0.02
			drvLng := dresdenLng + (rng.Float64()-0.5)*0.02
			drivers[i] = ghost.Driver{
				Pseudonym: fmt.Sprintf("d%04d-%02d", loop, i),
				MinCents:  500,
				Strategy:  ghost.PercentBelow(0.05 + rng.Float64()*0.25),
			}
			signers[i], _ = sign.NewEd25519Signer()
			rs.Ratings().Record(string(signers[i].PublicKey()), 3.5+rng.Float64()*1.5, 50+rng.Intn(200))
			client.PushGrid(drivers[i].Pseudonym,
				discovery.CellToString(discovery.CellAt(drvLat, drvLng, discovery.BaseResolution)))
		}

		rider := &ghost.Rider{Params: params, Client: client}
		if err := rider.Keygen(); err != nil {
			log.Printf("[demo] L%04d keygen: %v", loop, err); time.Sleep(*interval); continue
		}

		offeredPrice := 1500 + rng.Intn(500)
		cands, err := rider.OpenSession(map[string]any{
			"session_id": sessionID, "cell": discovery.CellToString(dresdenCell),
			"target": 2, "max_widen": 5, "offered_price": offeredPrice,
			"dropoff_hex": discovery.CellToString(dropoff),
			"floor_cents": 500, "cap_cents": 5000,
			"ring_dim": 1 << 15, "depth": 5,
		})
		if err != nil || len(cands) < 2 {
			log.Printf("[demo] L%04d discover=%d (retrying)", loop, len(cands))
			time.Sleep(*interval); continue
		}

		bidCount := 0
		for i, d := range drivers {
			invs, _ := client.Invites(d.Pseudonym)
			if len(invs) == 0 { continue }
			bid, ok := d.DecideBid(invs[0])
			if !ok { continue }
			if err := d.EncryptSignSubmit(params, sessionID, invs[0], bid, signers[i], client); err != nil {
				continue
			}
			bidCount++
		}
		if bidCount < 2 {
			log.Printf("[demo] L%04d bids=%d (retrying)", loop, bidCount)
			time.Sleep(*interval); continue
		}

		winner, masks, err := rider.Winner(sessionID)
		if err != nil {
			log.Printf("[demo] L%04d winner: %v", loop, err)
			time.Sleep(*interval); continue
		}
		log.Printf("[demo] ✓ L%04d: winner=%-10s bids=%d/%d masks=%s",
			loop, drivers[winner].Pseudonym, bidCount, *n, fmtMasks(masks))

		time.Sleep(*interval)
	}
}

func fmtMasks(v []float64) string {
	s := "["
	for i, x := range v {
		if i > 0 { s += " " }
		s += fmt.Sprintf("%.4f", x)
	}
	return s + "]"
}
