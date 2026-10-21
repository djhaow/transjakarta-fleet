// Package geofence implements geofence entry detection on top of the
// Haversine great-circle distance.
package geofence

import (
	"math"
	"sync"
)

const earthRadiusM = 6_371_000.0

// Point is a WGS84 coordinate.
type Point struct {
	Latitude  float64
	Longitude float64
}

// HaversineDistanceMeters returns the great-circle distance between two
// points in meters.
func HaversineDistanceMeters(a, b Point) float64 {
	lat1 := a.Latitude * math.Pi / 180
	lat2 := b.Latitude * math.Pi / 180
	dLat := (b.Latitude - a.Latitude) * math.Pi / 180
	dLon := (b.Longitude - a.Longitude) * math.Pi / 180

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusM * c
}

// Tracker detects geofence entries per vehicle. An entry fires only on the
// outside -> inside transition. State is in-memory, which is fine for a
// single backend instance; move it to a shared store when scaling out.
type Tracker struct {
	mu      sync.Mutex
	inside  map[string]bool
	center  Point
	radiusM float64
}

func NewTracker(center Point, radiusM float64) *Tracker {
	return &Tracker{
		inside:  make(map[string]bool),
		center:  center,
		radiusM: radiusM,
	}
}

// Process records one location and reports whether it constitutes a geofence
// entry (an outside -> inside transition, including the first fix).
func (t *Tracker) Process(vehicleID string, p Point) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	wasInside := t.inside[vehicleID]
	isInside := HaversineDistanceMeters(p, t.center) <= t.radiusM
	t.inside[vehicleID] = isInside

	return isInside && !wasInside
}
