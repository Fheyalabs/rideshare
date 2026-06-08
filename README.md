# rideshare

Privacy-preserving ride-share app with single-key CKKS reverse auction.
The server is a **blind evaluator** — it never holds a secret key and never
sees GPS coordinates. Riders generate keypairs; drivers encrypt bids under
the rider's public key; the server runs the auction homomorphically and
returns encrypted masks.

Consumes the public [ARES-core](https://github.com/Fheyalabs/ARES-core)
framework. Design: Fheya workspace
`docs/superpowers/specs/2026-06-04-cab-app-mvp-design.md`.

## Quick start — browser demo

The demo runs a self-contained server with ghost drivers, a rider, and a
live dashboard showing H3 hex zones, car markers, and encrypted wire traffic.

### Prerequisites

- Go ≥ 1.22
- OpenFHE compiled with CKKS support (the demo links via CGo)
- `ARES-core` sibling checkout at `../ARES-core` (or edit the `replace`
  directive in `go.mod`)

### Run

```bash
# Build
go build -tags openfhe -o bin/demo ./cmd/demo/

# Start (defaults: port 9000, 5 ghost drivers)
./bin/demo -port 9000 -n 5

# Open in browser
open http://localhost:9000/dashboard
```

### How to use the dashboard

- Click **▶ Next Phase** to step through the lifecycle one phase at a time.
  Clicks are queued — you can tap rapidly without losing steps.
- **Map** shows real OSM tiles centered on Dresden.
- **Green hex** = rider pickup zone; **purple hex** = dropoff zone.
  Colors change per phase (blue=discovering, cyan=review, orange=scoring,
  bright green=won).
- **Blue circle** = rider; **orange circles** = available drivers.
  Markers move and change color as the auction progresses.
- **Sidebar** shows each party's current phase.
- **Encrypted Wire** panel shows ciphertext-only data flows — the server
  never sees plaintext bids or locations.
- After the ride completes, the next click starts a **new session** with a
  fresh random rider and destination.

### Lifecycle phases

| Phase | What happens |
|---|---|
| IDLE | Rider waiting, drivers accepting |
| DISCOVERING | Rider searching for nearby cabs |
| KEYGEN | Rider generates single-key CKKS keypair |
| OFFER_REVIEW | Rider opens session, discovers drivers |
| DECIDING | Drivers evaluate the offer |
| SCORING | Server runs blind argmin on encrypted bids |
| DECRYPT | Rider decrypts masks locally on-device |
| WON | Winner announced, pickup zone highlighted |
| COMPLETE | Ride finished, awaiting next session |

### Other binaries

```bash
# Standalone rideshare server (no demo loop)
go build -tags openfhe -o bin/server ./cmd/server/
./bin/server -region dresden

# Ghost fleet simulator (standalone drivers)
go build -tags openfhe -o bin/ghostfleet ./cmd/ghostfleet/

# OSM graph builder (pre-process OpenStreetMap PBF)
go build -o bin/buildgraph ./cmd/buildgraph/
./bin/buildgraph -pbf dresden.osm.pbf -out dresden.region
```

## Architecture

```
rider (phone)                    server (blind)              driver (phone)
─────────────                    ───────────────             ───────────────
keygen → pk                                                      (keyless)
pk → artifact store ──────────────────────────────────────────→ fetch pk
open session → discover ─────────→ H3 registry lookup
                                  ┌─ invite drivers
encrypt(price) ←──────────────────┤
sign(enc || session) ────────────→│ collect signed bids
                                  │ EvalArgmax(enc_bids)
encrypted masks ←─────────────────┤
decrypt masks → winner
verify winner signature ─────────→ hold offer
```

## Test

```bash
go test -tags openfhe ./...
```
