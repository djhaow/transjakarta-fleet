package geofence

import (
	"math"
	"testing"
)

func TestHaversineSamePointIsZero(t *testing.T) {
	p := Point{Latitude: -6.2088, Longitude: 106.8456}
	if d := HaversineDistanceMeters(p, p); d > 1e-9 {
		t.Errorf("distance to self = %v m, want 0", d)
	}
}

func TestHaversineKnownDistance(t *testing.T) {
	// Monas -> Bundaran HI, Jakarta: roughly 2.2 km.
	monas := Point{Latitude: -6.1754, Longitude: 106.8272}
	bundaranHI := Point{Latitude: -6.1949, Longitude: 106.8230}

	d := HaversineDistanceMeters(monas, bundaranHI)
	if d < 2100 || d > 2350 {
		t.Errorf("Monas -> Bundaran HI = %.0f m, want ~2.2 km", d)
	}
}

func TestHaversineOneDegreeLatitude(t *testing.T) {
	// One degree of latitude is ~111.2 km everywhere on Earth.
	a := Point{Latitude: 0, Longitude: 100}
	b := Point{Latitude: 1, Longitude: 100}

	d := HaversineDistanceMeters(a, b)
	if math.Abs(d-111_200) > 500 {
		t.Errorf("1 degree of latitude = %.0f m, want ~111200 m", d)
	}
}

func TestTrackerEntryIsEdgeTriggered(t *testing.T) {
	tr := NewTracker(Point{Latitude: -6.2088, Longitude: 106.8456}, 50)
	inside := Point{Latitude: -6.2088, Longitude: 106.8456}
	// ~111 m north of the center: safely outside the 50 m fence.
	outside := Point{Latitude: -6.2088 - 0.001, Longitude: 106.8456}

	steps := []struct {
		point     Point
		wantEntry bool
		why       string
	}{
		{outside, false, "starts outside: no entry"},
		{inside, true, "crosses in: entry fires"},
		{inside, false, "still inside: no duplicate entry"},
		{inside, false, "still inside: no duplicate entry"},
		{outside, false, "leaves the fence"},
		{inside, true, "re-enters: entry fires again"},
	}
	for i, s := range steps {
		if got := tr.Process("B1234XYZ", s.point); got != s.wantEntry {
			t.Errorf("step %d (%s): Process = %v, want %v", i, s.why, got, s.wantEntry)
		}
	}
}

func TestTrackerVehiclesAreIndependent(t *testing.T) {
	tr := NewTracker(Point{Latitude: -6.2088, Longitude: 106.8456}, 50)
	inside := Point{Latitude: -6.2088, Longitude: 106.8456}

	if !tr.Process("B1", inside) {
		t.Error("B1 entering should fire")
	}
	if !tr.Process("B2", inside) {
		t.Error("B2 is a different vehicle: its entry should also fire")
	}
	if tr.Process("B1", inside) {
		t.Error("B1 still inside: should not fire again")
	}
}
