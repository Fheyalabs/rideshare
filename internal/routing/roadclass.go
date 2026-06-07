// Package routing holds the customizable weighted road graph the server builds
// from OSM, customizes with a live metric, and distributes to clients. The
// server never runs a route query — A* lives in the client/ghost libraries.
package routing

// RoadClass is the coarse OSM highway hierarchy used for the long-haul skeleton
// and for default speeds when maxspeed is absent.
type RoadClass uint8

const (
	ClassNone RoadClass = iota // not drivable
	ClassLocal
	ClassTertiary
	ClassSecondary
	ClassPrimary
	ClassTrunk
	ClassMotorway
)

// drivable maps OSM highway tag → class. Anything not here is ClassNone.
var drivable = map[string]RoadClass{
	"motorway": ClassMotorway, "motorway_link": ClassMotorway,
	"trunk": ClassTrunk, "trunk_link": ClassTrunk,
	"primary": ClassPrimary, "primary_link": ClassPrimary,
	"secondary": ClassSecondary, "secondary_link": ClassSecondary,
	"tertiary": ClassTertiary, "tertiary_link": ClassTertiary,
	"unclassified": ClassLocal, "residential": ClassLocal,
	"living_street": ClassLocal, "service": ClassLocal,
}

// Classify returns the road class and whether the way is drivable by car.
func Classify(highway string) (RoadClass, bool) {
	c, ok := drivable[highway]
	if !ok {
		return ClassNone, false
	}
	return c, true
}

// DefaultSpeedKmh is the fallback free-flow speed when maxspeed is missing.
func DefaultSpeedKmh(c RoadClass) float64 {
	switch c {
	case ClassMotorway:
		return 120
	case ClassTrunk:
		return 100
	case ClassPrimary:
		return 70
	case ClassSecondary:
		return 60
	case ClassTertiary:
		return 50
	case ClassLocal:
		return 30
	default:
		return 0
	}
}
