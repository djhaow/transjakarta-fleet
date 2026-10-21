// Package service wires MQTT ingestion to storage and geofence processing.
package service

import (
	"context"
	"log"

	"transjakarta-fleet/internal/database"
	"transjakarta-fleet/internal/geofence"
	"transjakarta-fleet/internal/model"
	"transjakarta-fleet/internal/rabbitmq"
)

// IngestService is the pipeline behind every MQTT location message:
// validate -> store -> geofence check -> publish event on entry.
type IngestService struct {
	db        *database.DB
	tracker   *geofence.Tracker
	publisher *rabbitmq.Publisher
}

func NewIngestService(db *database.DB, tracker *geofence.Tracker, publisher *rabbitmq.Publisher) *IngestService {
	return &IngestService{db: db, tracker: tracker, publisher: publisher}
}

// HandleLocation is invoked for every MQTT message on the location topic.
// Invalid messages are logged and dropped; a geofence_entry event is published
// only on an outside -> inside fence transition.
func (s *IngestService) HandleLocation(vehicleIDFromTopic string, payload []byte) {
	loc, err := model.ParseLocation(payload)
	if err != nil {
		log.Printf("[ingest] invalid payload: %v", err)
		return
	}
	if loc.VehicleID != vehicleIDFromTopic {
		log.Printf("[ingest] drop: payload vehicle_id %q does not match topic vehicle_id %q",
			loc.VehicleID, vehicleIDFromTopic)
		return
	}

	if err := s.db.InsertLocation(context.Background(), loc); err != nil {
		log.Printf("[ingest] insert failed for %s: %v", loc.VehicleID, err)
	}

	point := geofence.Point{Latitude: loc.Latitude, Longitude: loc.Longitude}
	if s.tracker.Process(loc.VehicleID, point) {
		log.Printf("[ingest] geofence entry detected for %s", loc.VehicleID)
		if err := s.publisher.PublishGeofenceEntry(loc); err != nil {
			log.Printf("[ingest] publish geofence event failed: %v", err)
		}
	}
}
