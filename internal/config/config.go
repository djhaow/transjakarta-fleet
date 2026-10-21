// Package config loads runtime configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config groups every setting the services need. All values have working
// defaults so the stack runs with a plain `docker compose up`.
type Config struct {
	HTTPPort string

	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string

	MQTTBroker   string
	MQTTClientID string
	MQTTTopic    string

	RabbitURL        string
	RabbitExchange   string
	RabbitQueue      string
	RabbitRoutingKey string

	GeofenceLat     float64
	GeofenceLon     float64
	GeofenceRadiusM float64
}

// Load reads the configuration, falling back to defaults for anything unset.
// A .env file in the working directory is applied first; variables already
// present in the environment win.
func Load() *Config {
	applyEnvFile(".env")

	return &Config{
		HTTPPort:         getenv("HTTP_PORT", "8080"),
		PostgresHost:     getenv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getenv("POSTGRES_PORT", "5432"),
		PostgresUser:     getenv("POSTGRES_USER", "fleet"),
		PostgresPassword: getenv("POSTGRES_PASSWORD", "fleet"),
		PostgresDB:       getenv("POSTGRES_DB", "fleet"),

		MQTTBroker:   getenv("MQTT_BROKER", "tcp://localhost:1883"),
		MQTTClientID: getenv("MQTT_CLIENT_ID", "fleet-backend"),
		MQTTTopic:    getenv("MQTT_TOPIC", "/fleet/vehicle/+/location"),

		RabbitURL:        getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitExchange:   getenv("RABBITMQ_EXCHANGE", "fleet.events"),
		RabbitQueue:      getenv("RABBITMQ_QUEUE", "geofence_alerts"),
		RabbitRoutingKey: getenv("RABBITMQ_ROUTING_KEY", "geofence.entry"),

		GeofenceLat:     getfloat("GEOFENCE_LAT", -6.2088),
		GeofenceLon:     getfloat("GEOFENCE_LON", 106.8456),
		GeofenceRadiusM: getfloat("GEOFENCE_RADIUS_M", 50),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getfloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

// applyEnvFile sets variables from a KEY=VALUE file, skipping blank and
// commented lines. A missing file is not an error: the getenv defaults are
// the last resort.
func applyEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		if key != "" && os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
