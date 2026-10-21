// Command server is the main backend: MQTT subscriber, geofence publisher
// and REST API in one process.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"transjakarta-fleet/internal/api"
	"transjakarta-fleet/internal/config"
	"transjakarta-fleet/internal/database"
	"transjakarta-fleet/internal/geofence"
	"transjakarta-fleet/internal/mqtt"
	"transjakarta-fleet/internal/rabbitmq"
	"transjakarta-fleet/internal/service"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// storage
	db, err := database.Connect(ctx, cfg)
	if err != nil {
		log.Fatalf("[server] postgres: %v", err)
	}
	defer db.Close()

	// rabbitmq publisher
	publisher, err := rabbitmq.ConnectPublisher(ctx, cfg)
	if err != nil {
		log.Fatalf("[server] rabbitmq: %v", err)
	}
	defer publisher.Close()
	log.Printf("[server] rabbitmq ready (exchange %q, queue %q)",
		cfg.RabbitExchange, cfg.RabbitQueue)

	// geofence
	tracker := geofence.NewTracker(
		geofence.Point{Latitude: cfg.GeofenceLat, Longitude: cfg.GeofenceLon},
		cfg.GeofenceRadiusM,
	)
	log.Printf("[server] geofence center (%.6f, %.6f), radius %.0f m",
		cfg.GeofenceLat, cfg.GeofenceLon, cfg.GeofenceRadiusM)

	// mqtt ingestion
	svc := service.NewIngestService(db, tracker, publisher)
	mqttClient := mqtt.Start(cfg, svc.HandleLocation)
	defer mqttClient.Disconnect(250)

	// http api
	httpServer := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: api.NewServer(db).Router(),
	}
	go func() {
		log.Printf("[server] http listening on :%s", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[server] http: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("[server] shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[server] http shutdown: %v", err)
	}
	log.Println("[server] bye")
}
