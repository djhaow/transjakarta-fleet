// Package rabbitmq publishes geofence events to the fleet.events exchange
// and consumes the geofence_alerts queue.
package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"transjakarta-fleet/internal/config"
	"transjakarta-fleet/internal/model"
)

const EventGeofenceEntry = "geofence_entry"

// Location mirrors the nested location object of the event contract.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Event is the message published to the fleet.events exchange.
type Event struct {
	VehicleID string   `json:"vehicle_id"`
	Event     string   `json:"event"`
	Location  Location `json:"location"`
	Timestamp int64    `json:"timestamp"`
}

type Publisher struct {
	conn       *amqp.Connection
	ch         *amqp.Channel
	exchange   string
	routingKey string
}

// ConnectPublisher dials RabbitMQ (retrying until ctx is cancelled) and
// declares the exchange, queue and binding.
func ConnectPublisher(ctx context.Context, cfg *config.Config) (*Publisher, error) {
	conn, err := dialWithRetry(ctx, cfg.RabbitURL)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	if err := declareTopology(ch, cfg); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &Publisher{conn: conn, ch: ch, exchange: cfg.RabbitExchange, routingKey: cfg.RabbitRoutingKey}, nil
}

func (p *Publisher) PublishGeofenceEntry(loc *model.LocationPayload) error {
	body, err := json.Marshal(Event{
		VehicleID: loc.VehicleID,
		Event:     EventGeofenceEntry,
		Location:  Location{Latitude: loc.Latitude, Longitude: loc.Longitude},
		Timestamp: loc.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return p.ch.PublishWithContext(context.Background(), p.exchange, p.routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         body,
	})
}

func (p *Publisher) Close() {
	p.ch.Close()
	p.conn.Close()
}

// RunConsumer consumes the geofence_alerts queue until ctx is cancelled,
// logging each event and acknowledging it. It reconnects on failure.
func RunConsumer(ctx context.Context, cfg *config.Config) error {
	for {
		err := consumeOnce(ctx, cfg)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("[worker] consume interrupted: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func consumeOnce(ctx context.Context, cfg *config.Config) error {
	conn, err := dialWithRetry(ctx, cfg.RabbitURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	if err := declareTopology(ch, cfg); err != nil {
		return err
	}

	deliveries, err := ch.Consume(cfg.RabbitQueue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %q: %w", cfg.RabbitQueue, err)
	}
	log.Printf("[worker] waiting for messages on queue %q (exchange %q)", cfg.RabbitQueue, cfg.RabbitExchange)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("delivery channel closed by the broker")
			}
			handleDelivery(d)
		}
	}
}

func handleDelivery(d amqp.Delivery) {
	var ev Event
	if err := json.Unmarshal(d.Body, &ev); err != nil {
		// Poison message: log it and drop it instead of requeueing forever.
		log.Printf("[worker] cannot decode message (delivery %d): %v", d.DeliveryTag, err)
		_ = d.Reject(false)
		return
	}

	pretty, _ := json.MarshalIndent(ev, "", "  ")
	log.Printf("[worker] geofence event received:\n%s", pretty)

	_ = d.Ack(false)
}

// declareTopology is idempotent; publisher and consumer both call it.
func declareTopology(ch *amqp.Channel, cfg *config.Config) error {
	if err := ch.ExchangeDeclare(cfg.RabbitExchange, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %q: %w", cfg.RabbitExchange, err)
	}
	if _, err := ch.QueueDeclare(cfg.RabbitQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %q: %w", cfg.RabbitQueue, err)
	}
	if err := ch.QueueBind(cfg.RabbitQueue, cfg.RabbitRoutingKey, cfg.RabbitExchange, false, nil); err != nil {
		return fmt.Errorf("bind queue %q to exchange %q with key %q: %w",
			cfg.RabbitQueue, cfg.RabbitExchange, cfg.RabbitRoutingKey, err)
	}
	return nil
}

// dialWithRetry keeps retrying until the broker accepts the connection or
// ctx is cancelled, logging every few attempts.
func dialWithRetry(ctx context.Context, url string) (*amqp.Connection, error) {
	for attempt := 1; ; attempt++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			return conn, nil
		}
		if attempt%5 == 1 {
			log.Printf("[rabbitmq] dial failed (attempt %d): %v; retrying every 2s...", attempt, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
