package traffic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Fheyalabs/rideshare/internal/routing"
)

// ClosurePoint is a closure location from the Autobahn API.
type ClosurePoint struct{ Lat, Lon float64 }

// FetchClosures GETs the Autobahn closure list for a road (e.g. "A17").
func FetchClosures(base, road string) ([]ClosurePoint, error) {
	url := fmt.Sprintf("%s/%s/services/closure", base, road)
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body struct {
		Closure []struct {
			Coordinate struct {
				Lat  string `json:"lat"`
				Long string `json:"long"`
			} `json:"coordinate"`
		} `json:"closure"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	pts := make([]ClosurePoint, 0, len(body.Closure))
	for _, c := range body.Closure {
		lat, e1 := strconv.ParseFloat(c.Coordinate.Lat, 64)
		lon, e2 := strconv.ParseFloat(c.Coordinate.Long, 64)
		if e1 == nil && e2 == nil {
			pts = append(pts, ClosurePoint{Lat: lat, Lon: lon})
		}
	}
	return pts, nil
}

// MatchClosures closes edges whose Ref matches and whose midpoint is within
// radiusM of any closure point.
func MatchClosures(g *routing.Graph, pts []ClosurePoint, ref string, radiusM float64) map[routing.EdgeKey]bool {
	closed := map[routing.EdgeKey]bool{}
	for _, e := range g.Edges {
		if e.Ref != ref {
			continue
		}
		a, b := g.Nodes[e.From], g.Nodes[e.To]
		mlat, mlon := (a.Lat+b.Lat)/2, (a.Lon+b.Lon)/2
		for _, p := range pts {
			if routing.DistM(mlat, mlon, p.Lat, p.Lon) <= radiusM {
				closed[routing.EdgeKey{From: e.From, To: e.To}] = true
				break
			}
		}
	}
	return closed
}
