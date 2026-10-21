// Command mock-publisher simulates a GPS tracker: it publishes the location
// of a vehicle to /fleet/vehicle/{vehicle_id}/location every 2 seconds. The
// route passes through the configured geofence center.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"

	"transjakarta-fleet/internal/config"
	"transjakarta-fleet/internal/model"
)

func main() {
	broker := flag.String("broker", "", "MQTT broker URL (default: MQTT_BROKER env or tcp://localhost:1883)")
	vehicleID := flag.String("vehicle", "B1234XYZ", "vehicle id to simulate")
	interval := flag.Duration("interval", 2*time.Second, "time between location updates")
	flag.Parse()

	cfg := config.Load()
	if *broker != "" {
		cfg.MQTTBroker = *broker
	}

	sim := newSimulator(*vehicleID, cfg.GeofenceLat, cfg.GeofenceLon)

	opts := pahomqtt.NewClientOptions().
		AddBroker(cfg.MQTTBroker).
		SetClientID("fleet-mock-publisher").
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(2 * time.Second)
	client := pahomqtt.NewClient(opts)

	token := client.Connect()
	if !token.WaitTimeout(30 * time.Second) {
		log.Fatalf("[mock] could not connect to %s within 30s", cfg.MQTTBroker)
	}
	if token.Error() != nil {
		log.Fatalf("[mock] connect: %v", token.Error())
	}
	log.Printf("[mock] connected to %s; publishing to %s every %s",
		cfg.MQTTBroker, sim.topic, *interval)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	published := 0
	for {
		select {
		case <-ticker.C:
			payload := sim.nextMessage()
			client.Publish(sim.topic, 1, false, payload)
			published++
			log.Printf("[mock] #%d -> %s %s", published, sim.topic, payload)
		case <-stop:
			log.Printf("[mock] stopped after %d messages", published)
			client.Disconnect(250)
			return
		}
	}
}

// simulator walks a vehicle along a ping-pong (north -> south -> north)
// route through the geofence center.
type simulator struct {
	topic     string
	vehicleID string
	lat       float64
	lon       float64
	step      float64 // signed degrees of latitude per tick
	minLat    float64
	maxLat    float64
}

func newSimulator(vehicleID string, centerLat, centerLon float64) *simulator {
	const (
		halfRoute  = 0.0150 // degrees either side of the center (~1.67 km)
		stepMeters = 45.0   // distance per tick (~81 km/h at a 2 s interval)
	)
	degPerMeter := 1.0 / 111_320.0 // approx degrees of latitude per meter
	return &simulator{
		topic:     "/fleet/vehicle/" + vehicleID + "/location",
		vehicleID: vehicleID,
		lat:       centerLat - halfRoute,
		lon:       centerLon,
		step:      stepMeters * degPerMeter,
		minLat:    centerLat - halfRoute,
		maxLat:    centerLat + halfRoute,
	}
}

// nextMessage advances the simulator and renders one JSON payload.
func (s *simulator) nextMessage() []byte {
	s.lat += s.step
	if s.lat > s.maxLat {
		s.lat = s.maxLat
		s.step = -s.step
	} else if s.lat < s.minLat {
		s.lat = s.minLat
		s.step = -s.step
	}

	payload, err := json.Marshal(model.LocationPayload{
		VehicleID: s.vehicleID,
		Latitude:  s.lat,
		Longitude: s.lon,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		log.Fatalf("[mock] marshal payload: %v", err)
	}
	return payload
}
