//go:build openfhe

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/dashboard"
	"github.com/Fheyalabs/rideshare/internal/discovery"
	"github.com/Fheyalabs/rideshare/internal/ghost"
	"github.com/Fheyalabs/rideshare/internal/server"
)

func main() {
	port := flag.Int("port", 9000, "server port")
	n := flag.Int("n", 5, "ghost driver count")
	flag.Parse()

	params := cgo.ContractParams{RingDim: 1 << 15, Depth: 5, ScalingFactor: float64(uint64(1) << 50)}
	bus := dashboard.NewBus(256)

	rs := server.NewRideshareServer(server.Config{Addr: fmt.Sprintf(":%d", *port)})
	rs.RegisterOpenFHERoutes()
	dashboard.Mount(rs.ServerMux(), bus)

	// Blocking trigger: each phase waits until the user clicks "Next Phase".
	// POST /dashboard/trigger sends tokens; the demo loop consumes one per phase.
	// Buffer is large enough to absorb fast clicks — they queue, never drop.
	trigger := make(chan struct{}, 64)
	rs.ServerMux().HandleFunc("POST /dashboard/trigger", func(w http.ResponseWriter, r *http.Request) {
		trigger <- struct{}{}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"advanced":true}`))
	})

	go func() {
		log.Printf("[demo] server on http://localhost:%d", *port)
		log.Printf("[demo] dashboard on http://localhost:%d/dashboard", *port)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), rs.Handler()))
	}()
	time.Sleep(500 * time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%d", *port)
	client := ghost.NewClient(baseURL)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// ── Persistent ghost drivers (random positions near Dresden) ──
	type driverState struct {
		name      string
		lat, lng  float64
		cell      discovery.Cell
	}
	var driverStates []driverState
	for i := 0; i < *n; i++ {
		drvLat := 51.0493 + (rng.Float64()-0.5)*0.04
		drvLng := 13.7384 + (rng.Float64()-0.5)*0.04
		driverStates = append(driverStates, driverState{
			name: fmt.Sprintf("Cab-%02d", i+1),
			lat:  drvLat, lng: drvLng,
			cell: discovery.CellAt(drvLat, drvLng, discovery.BaseResolution),
		})
	}

	prefix := fmt.Sprintf("demo-%d", time.Now().Unix())

	for loop := 1; ; loop++ {
		sessionID := fmt.Sprintf("%s-%04d", prefix, loop)

		// ── Generate new rider pickup + dropoff each session ──
		riderLat := 51.0493 + (rng.Float64()-0.5)*0.03
		riderLng := 13.7384 + (rng.Float64()-0.5)*0.03
		riderCell := discovery.CellAt(riderLat, riderLng, discovery.BaseResolution)
		// Dropoff is ~2-4 km away in a random direction
		dropoffLat := riderLat + (rng.Float64()-0.5)*0.04
		dropoffLng := riderLng + (rng.Float64()-0.5)*0.04
		dropoffCell := discovery.CellAt(dropoffLat, dropoffLng, discovery.BaseResolution-2)

		// ── Initialise loop ──
		bus.Emit(dashboard.Event{Type: "loop", Detail: fmt.Sprintf("Session %d", loop)})
		log.Printf("[demo] === session %d: rider=(%.4f,%.4f) dropoff=(%.4f,%.4f) ===",
			loop, riderLat, riderLng, dropoffLat, dropoffLng)

		// ── PHASE 1: IDLE ──
		bus.Emit(dashboard.Event{Type: "phase", Party: "rider", Phase: "IDLE",
			Detail: "rider waiting", Lat: riderLat, Lng: riderLng})
		bus.Emit(hexEvent("pickup", "IDLE", riderCell, riderLat, riderLng))
		bus.Emit(hexEvent("dropoff", "IDLE", dropoffCell, dropoffLat, dropoffLng))
		for _, ds := range driverStates {
			bus.Emit(dashboard.Event{Type: "phase", Party: ds.name, Phase: "ACCEPTING_RIDES",
				Lat: ds.lat, Lng: ds.lng})
		}
		log.Printf("[demo] phase=IDLE — waiting for trigger")
		<-trigger

		// ── PHASE 2: DISCOVERING ──
		bus.Emit(dashboard.Event{Type: "phase", Party: "rider", Phase: "DISCOVERING",
			Detail: "finding nearby cabs", Lat: riderLat, Lng: riderLng})
		bus.Emit(hexEvent("pickup", "DISCOVERING", riderCell, riderLat, riderLng))
		bus.Emit(hexEvent("dropoff", "DISCOVERING", dropoffCell, dropoffLat, dropoffLng))
		log.Printf("[demo] phase=DISCOVERING — waiting for trigger")
		<-trigger

		// Register drivers in discovery for this loop
		drivers := make([]ghost.Driver, *n)
		signers := make([]sign.Signer, *n)
		for i, ds := range driverStates {
			drivers[i] = ghost.Driver{
				Pseudonym: ds.name, MinCents: 500,
				Strategy: ghost.PercentBelow(0.05 + rng.Float64()*0.25),
			}
			signers[i], _ = sign.NewEd25519Signer()
			rs.Ratings().Record(string(signers[i].PublicKey()), 3.5+rng.Float64()*1.5, 50+rng.Intn(200))
			client.PushGrid(ds.name, discovery.CellToString(ds.cell))
		}

		// ── PHASE 3: KEYGEN ──
		bus.Emit(dashboard.Event{Type: "phase", Party: "rider", Phase: "KEYGEN",
			Detail: "generating single-key CKKS keypair", Lat: riderLat, Lng: riderLng})
		log.Printf("[demo] phase=KEYGEN — waiting for trigger")
		<-trigger

		rider := &ghost.Rider{Params: params, Client: client}
		if err := rider.Keygen(); err != nil {
			log.Printf("[demo] keygen: %v", err)
			continue
		}
		bus.Emit(dashboard.Event{Type: "wire", Party: "rider",
			Payload: fmt.Sprintf("pk %d bytes → server", len(rider.PK))})

		// ── PHASE 4: OFFER_REVIEW ──
		offeredPrice := 1500 + rng.Intn(500)
		bus.Emit(dashboard.Event{Type: "phase", Party: "rider", Phase: "OFFER_REVIEW",
			Detail: fmt.Sprintf("offering €%.2f", float64(offeredPrice)/100),
			Lat: riderLat, Lng: riderLng})
		log.Printf("[demo] phase=OFFER_REVIEW — waiting for trigger")
		<-trigger

		cands, err := rider.OpenSession(map[string]any{
			"session_id":   sessionID,
			"cell":         discovery.CellToString(riderCell),
			"target":       2,
			"max_widen":    5,
			"offered_price": offeredPrice,
			"dropoff_hex":  discovery.CellToString(dropoffCell),
			"floor_cents":  500,
			"cap_cents":    5000,
			"ring_dim":     1 << 15,
			"depth":        5,
		})
		if err != nil || len(cands) < 2 {
			log.Printf("[demo] L%04d discover=%d (retrying)", loop, len(cands))
			<-trigger // let user see the fail and click to next
			continue
		}

		// Highlight matched drivers on map
		for _, ds := range driverStates {
			for _, c := range cands {
				if c == ds.name {
					bus.Emit(dashboard.Event{Type: "marker", Party: ds.name, Phase: "MATCHED",
						Lat: ds.lat, Lng: ds.lng, Detail: "in cell"})
				}
			}
		}
		bus.Emit(hexEvent("pickup", "OFFER_REVIEW", riderCell, riderLat, riderLng))
		bus.Emit(hexEvent("dropoff", "OFFER_REVIEW", dropoffCell, dropoffLat, dropoffLng))

		// ── PHASE 5: BIDDING ──
		for _, ds := range driverStates {
			bus.Emit(dashboard.Event{Type: "phase", Party: ds.name, Phase: "DECIDING",
				Lat: ds.lat, Lng: ds.lng, Detail: "evaluating offer"})
		}
		log.Printf("[demo] phase=DECIDING — waiting for trigger")
		<-trigger

		bidCount := 0
		for i, d := range drivers {
			invs, _ := client.Invites(d.Pseudonym)
			if len(invs) == 0 {
				continue
			}
			// Use last (most recent) invite — stale invites are replaced server-side
			bid, ok := d.DecideBid(invs[len(invs)-1])
			if !ok {
				continue
			}
			bus.Emit(dashboard.Event{Type: "phase", Party: d.Pseudonym, Phase: "BIDDING",
				Detail: fmt.Sprintf("bid €%.2f", float64(bid)/100),
				Lat: driverStates[i].lat, Lng: driverStates[i].lng})
			if err := d.EncryptSignSubmit(params, sessionID, invs[0], bid, signers[i], client); err != nil {
				log.Printf("[demo] %s submit: %v", d.Pseudonym, err)
				continue
			}
			bus.Emit(dashboard.Event{Type: "wire", Party: d.Pseudonym,
				Payload: fmt.Sprintf("encrypted bid + Ed25519 signature → server")})
			bidCount++
		}

		if bidCount < 2 {
			log.Printf("[demo] L%04d bids=%d (need >=2)", loop, bidCount)
			<-trigger
			continue
		}

		// ── PHASE 6: BLIND AUCTION ──
		bus.Emit(dashboard.Event{Type: "phase", Party: "server", Phase: "SCORING",
			Detail: fmt.Sprintf("running blind argmin on %d encrypted bids", bidCount)})
		bus.Emit(dashboard.Event{Type: "wire", Party: "server",
			Payload: fmt.Sprintf("EvalArgmax(encrypted_bids) → %d encrypted masks", bidCount),
			Detail: "ciphertexts processed homomorphically — server never sees plaintext values"})
		log.Printf("[demo] phase=SCORING — waiting for trigger")
		<-trigger

		// ── PHASE 7: DECRYPT ──
		bus.Emit(dashboard.Event{Type: "phase", Party: "rider", Phase: "DECRYPT",
			Detail: "decrypting masks locally on-device", Lat: riderLat, Lng: riderLng})
		log.Printf("[demo] phase=DECRYPT — waiting for trigger")
		<-trigger

		winner, masks, err := rider.Winner(sessionID)
		if err != nil {
			log.Printf("[demo] L%04d winner: %v", loop, err)
			<-trigger
			continue
		}

		// ── PHASE 8: WINNER → DROPOFF → COMPLETE ──
		wName := drivers[winner].Pseudonym
		wDS := driverStates[winner]
		for i, ds := range driverStates {
			if i == winner {
				bus.Emit(dashboard.Event{Type: "phase", Party: ds.name, Phase: "WON",
					Detail: fmt.Sprintf("WINNER — €%.2f", float64(offeredPrice)*0.7/100),
					Lat: ds.lat, Lng: ds.lng})
			} else {
				bus.Emit(dashboard.Event{Type: "phase", Party: ds.name, Phase: "ACCEPTING_RIDES",
					Lat: ds.lat, Lng: ds.lng})
			}
		}
		// Highlight pickup and dropoff with final colors
		bus.Emit(hexEvent("pickup", "WON", riderCell, riderLat, riderLng))
		bus.Emit(hexEvent("dropoff", "DROPOFF", dropoffCell, dropoffLat, dropoffLng))
		bus.Emit(dashboard.Event{Type: "marker", Party: wName, Phase: "WON",
			Lat: wDS.lat, Lng: wDS.lng, Detail: "picked up!"})

		// Rider → dropoff position (simulate movement to destination)
		bus.Emit(dashboard.Event{Type: "phase", Party: "rider", Phase: "IN_RIDE",
			Detail: "en route to dropoff", Lat: dropoffLat, Lng: dropoffLng})
		bus.Emit(dashboard.Event{Type: "wire", Party: "server→rider",
			Payload: fmt.Sprintf("winner=%s masks=%s", wName, fmtMasks(masks))})
		log.Printf("[demo] phase=WON — waiting for trigger")
		<-trigger

		// ── PHASE 9: SESSION COMPLETE ──
		bus.Emit(dashboard.Event{Type: "phase", Party: "rider", Phase: "COMPLETE",
			Detail: fmt.Sprintf("session complete — €%.2f", float64(offeredPrice)*0.7/100),
			Lat: dropoffLat, Lng: dropoffLng})
		for _, ds := range driverStates {
			bus.Emit(dashboard.Event{Type: "phase", Party: ds.name, Phase: "ACCEPTING_RIDES",
				Lat: ds.lat, Lng: ds.lng})
		}
		bus.Emit(hexEvent("pickup", "COMPLETE", riderCell, riderLat, riderLng))
		bus.Emit(hexEvent("dropoff", "COMPLETE", dropoffCell, dropoffLat, dropoffLng))

		// Hold winner offer
		client.Post("/offer/hold", map[string]any{
			"session_id":  sessionID,
			"offer_id":    fmt.Sprintf("offer-%d", winner),
			"driver":      wName,
			"price_cents": (offeredPrice * 7 / 10),
			"star":        4.5,
		})

		log.Printf("[demo] ✓ L%04d: winner=%-10s bids=%d/%d masks=%s",
			loop, wName, bidCount, *n, fmtMasks(masks))
		log.Printf("[demo] session complete — waiting for trigger to start next")
		<-trigger
	}
}

func fmtMasks(v []float64) string {
	s := "["
	for i, x := range v {
		if i > 0 {
			s += " "
		}
		s += fmt.Sprintf("%.4f", x)
	}
	return s + "]"
}

// hexEvent builds a dashboard hex event with a pre-computed GeoJSON geometry.
func hexEvent(party, phase string, cell discovery.Cell, lat, lng float64) dashboard.Event {
	g := dashboard.HexGeometry(uint64(cell))
	var raw json.RawMessage
	if g != nil {
		raw = json.RawMessage(g)
	}
	return dashboard.Event{
		Type: "hex", Party: party, Phase: phase,
		Detail:   discovery.CellToString(cell),
		Lat:      lat, Lng: lng,
		Geometry: &raw,
	}
}
