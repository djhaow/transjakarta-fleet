package mqtt

import "testing"

func TestVehicleIDFromTopic(t *testing.T) {
	got, err := vehicleIDFromTopic("/fleet/vehicle/B1234XYZ/location")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "B1234XYZ" {
		t.Errorf("vehicleIDFromTopic = %q, want B1234XYZ", got)
	}
}

func TestVehicleIDFromTopicRejectsBadTopics(t *testing.T) {
	bad := []string{
		"fleet/vehicle/B1234XYZ/location", // missing leading slash
		"/fleet/vehicle/B1234XYZ",         // missing trailing segment
		"/fleet/driver/B1234XYZ/location", // wrong resource
		"/fleet/vehicle//location",        // empty vehicle id
	}
	for _, topic := range bad {
		if _, err := vehicleIDFromTopic(topic); err == nil {
			t.Errorf("expected error for topic %q", topic)
		}
	}
}
