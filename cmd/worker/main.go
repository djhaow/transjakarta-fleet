// Command worker consumes geofence_alerts from RabbitMQ and logs every event.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"transjakarta-fleet/internal/config"
	"transjakarta-fleet/internal/rabbitmq"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("[worker] consuming queue %q from %s", cfg.RabbitQueue, cfg.RabbitURL)
	if err := rabbitmq.RunConsumer(ctx, cfg); err != nil && ctx.Err() == nil {
		log.Fatalf("[worker] exited with error: %v", err)
	}
	log.Println("[worker] shutdown complete")
}
