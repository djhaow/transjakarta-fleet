// Package model defines the data contracts shared by the MQTT subscriber,
// the REST API and the RabbitMQ events.
package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// LocationPayload is the JSON contract shared by MQTT and the REST API.
type LocationPayload struct {
	VehicleID string  `json:"vehicle_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp int64   `json:"timestamp"` // unix seconds
}

// maxFutureSkew tolerates small clock drift between devices and servers.
const maxFutureSkew = 10 * time.Minute

// ErrMismatchedVehicle is returned when the vehicle_id inside the payload
// does not match the vehicle_id carried by the MQTT topic.
var ErrMismatchedVehicle = errors.New("payload vehicle_id does not match the topic")

// ParseLocation decodes and validates one MQTT payload.
func ParseLocation(raw []byte) (*LocationPayload, error) {
	var p LocationPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate enforces the payload contract.
func (p *LocationPayload) Validate() error {
	if p.VehicleID == "" {
		return errors.New("vehicle_id is required")
	}
	if len(p.VehicleID) > 32 {
		return errors.New("vehicle_id must be at most 32 characters")
	}
	if p.Latitude < -90 || p.Latitude > 90 {
		return fmt.Errorf("latitude %f out of range [-90, 90]", p.Latitude)
	}
	if p.Longitude < -180 || p.Longitude > 180 {
		return fmt.Errorf("longitude %f out of range [-180, 180]", p.Longitude)
	}
	if p.Timestamp <= 0 {
		return errors.New("timestamp is required (unix seconds)")
	}
	if time.Unix(p.Timestamp, 0).After(time.Now().Add(maxFutureSkew)) {
		return fmt.Errorf("timestamp %d is too far in the future", p.Timestamp)
	}
	return nil
}
