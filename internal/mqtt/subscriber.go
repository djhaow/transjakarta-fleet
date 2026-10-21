// Package mqtt subscribes to vehicle location topics on Eclipse Mosquitto.
package mqtt

import (
	"fmt"
	"log"
	"strings"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"

	"transjakarta-fleet/internal/config"
)

// Handler processes one raw message from the location topic, together with
// the vehicle_id parsed out of the topic itself.
type Handler func(vehicleIDFromTopic string, payload []byte)

// Start connects to the broker and subscribes to the configured topic
// (default /fleet/vehicle/+/location, QoS 1). Connection retries happen in
// the background, so Start never fails hard: the rest of the service can
// come up while the broker is still starting.
func Start(cfg *config.Config, handle Handler) pahomqtt.Client {
	opts := pahomqtt.NewClientOptions().
		AddBroker(cfg.MQTTBroker).
		SetClientID(cfg.MQTTClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(2 * time.Second).
		SetOrderMatters(false).
		SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
			log.Printf("[mqtt] connection lost: %v", err)
		}).
		SetOnConnectHandler(func(c pahomqtt.Client) {
			token := c.Subscribe(cfg.MQTTTopic, 1, messageRouter(handle))
			token.Wait()
			if token.Error() != nil {
				log.Printf("[mqtt] subscribe %q failed: %v", cfg.MQTTTopic, token.Error())
				return
			}
			log.Printf("[mqtt] subscribed to %q on %s", cfg.MQTTTopic, cfg.MQTTBroker)
		})

	client := pahomqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(15 * time.Second) {
		log.Printf("[mqtt] still connecting to %s; retrying in the background", cfg.MQTTBroker)
	} else if token.Error() != nil {
		log.Printf("[mqtt] connect failed: %v", token.Error())
	}
	return client
}

func messageRouter(handle Handler) pahomqtt.MessageHandler {
	return func(_ pahomqtt.Client, msg pahomqtt.Message) {
		vehicleID, err := vehicleIDFromTopic(msg.Topic())
		if err != nil {
			log.Printf("[mqtt] drop message on unexpected topic %q: %v", msg.Topic(), err)
			return
		}
		handle(vehicleID, msg.Payload())
	}
}

// vehicleIDFromTopic parses /fleet/vehicle/{vehicle_id}/location. The topic
// topic has a leading slash, so the first split segment is empty.
func vehicleIDFromTopic(topic string) (string, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 5 ||
		parts[1] != "fleet" ||
		parts[2] != "vehicle" ||
		parts[3] == "" ||
		parts[4] != "location" {
		return "", fmt.Errorf("topic does not match /fleet/vehicle/{vehicle_id}/location")
	}
	return parts[3], nil
}
