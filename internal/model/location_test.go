package model

import (
	"encoding/json"
	"testing"
	"time"
)

func validPayload(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(LocationPayload{
		VehicleID: "B1234XYZ",
		Latitude:  -6.2088,
		Longitude: 106.8456,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return body
}

func TestParseLocationValid(t *testing.T) {
	p, err := ParseLocation(validPayload(t))
	if err != nil {
		t.Fatalf("expected valid payload, got error: %v", err)
	}
	if p.VehicleID != "B1234XYZ" || p.Latitude != -6.2088 || p.Longitude != 106.8456 {
		t.Errorf("parsed payload does not match input: %+v", p)
	}
}

func TestParseLocationInvalidJSON(t *testing.T) {
	if _, err := ParseLocation([]byte(`{"vehicle_id":`)); err == nil {
		t.Error("expected error for malformed json")
	}
}

func TestParseLocationNonNumericField(t *testing.T) {
	body := `{"vehicle_id":"B1234XYZ","latitude":"not-a-number","longitude":106.8456,"timestamp":1715003456}`
	if _, err := ParseLocation([]byte(body)); err == nil {
		t.Error("expected error for non-numeric latitude")
	}
}

func TestValidateErrors(t *testing.T) {
	now := time.Now().Unix()
	cases := []struct {
		name string
		p    LocationPayload
	}{
		{"missing vehicle id", LocationPayload{Latitude: 0, Longitude: 0, Timestamp: now}},
		{"latitude too high", LocationPayload{VehicleID: "B1", Latitude: 91, Longitude: 0, Timestamp: now}},
		{"latitude too low", LocationPayload{VehicleID: "B1", Latitude: -91, Longitude: 0, Timestamp: now}},
		{"longitude too high", LocationPayload{VehicleID: "B1", Latitude: 0, Longitude: 181, Timestamp: now}},
		{"longitude too low", LocationPayload{VehicleID: "B1", Latitude: 0, Longitude: -181, Timestamp: now}},
		{"zero timestamp", LocationPayload{VehicleID: "B1", Latitude: 0, Longitude: 0, Timestamp: 0}},
		{"timestamp in the future", LocationPayload{VehicleID: "B1", Latitude: 0, Longitude: 0, Timestamp: time.Now().Add(1 * time.Hour).Unix()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.p.Validate(); err == nil {
				t.Errorf("expected validation error for %q, got nil", tc.name)
			}
		})
	}
}

func TestValidateAcceptsBoundaryCoordinates(t *testing.T) {
	// (0,0) is a legal coordinate: zero values must not be rejected.
	p := LocationPayload{VehicleID: "B1", Latitude: 0, Longitude: 0, Timestamp: time.Now().Unix()}
	if err := p.Validate(); err != nil {
		t.Errorf("expected (0,0) to be valid, got %v", err)
	}
}
