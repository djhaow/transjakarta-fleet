# Transjakarta Fleet Management

Backend service for a vehicle fleet management system: it ingests GPS
locations over MQTT, stores them in PostgreSQL, serves them through a REST
API, and publishes geofence entry events through RabbitMQ.

## Architecture

    mock-publisher -> Mosquitto (MQTT) -> server -> RabbitMQ -> worker
                                             |
                                         PostgreSQL
                                             |
                                         REST API (:8080)

- `cmd/server` subscribes to `/fleet/vehicle/+/location` (QoS 1), validates
  and stores every location, detects geofence entries and publishes a
  `geofence_entry` event to the `fleet.events` exchange.
- `cmd/worker` consumes the `geofence_alerts` queue and logs each event.
- `cmd/mock-publisher` simulates a GPS tracker on a north-south route that
  passes through the geofence center.

## Running

Requires Docker.

    docker compose up -d --build

| Service                | Address                                       |
|------------------------|-----------------------------------------------|
| REST API               | http://localhost:8080                         |
| RabbitMQ               | amqp://localhost:5672                         |
| RabbitMQ management UI | http://localhost:15672 (fleet / fleet123)     |
| Mosquitto              | tcp://localhost:1883                          |
| PostgreSQL             | localhost:5432, user/pass/db: fleet/fleet/fleet |

To also start the mock publisher:

    docker compose --profile demo up -d

or straight from source:

    go run ./cmd/mock-publisher -vehicle B1234XYZ

The route crosses the geofence center, so each pass produces one
`geofence_entry` event in the worker logs.

## API

Latest location:

    curl http://localhost:8080/vehicles/B1234XYZ/location

History (unix seconds, inclusive range):

    curl "http://localhost:8080/vehicles/B1234XYZ/history?start=1715000000&end=1715009999"

`404` when the vehicle has never reported, `400` on bad parameters.

## Tests

    go test ./...

Covers payload validation, topic parsing and geofence transition logic.

## Configuration

Credentials are read from `.env` when the file exists (template in
`.env.example`). Every variable also has a fallback default, both in
`docker-compose.yml` (`${VAR:-default}`) and in `internal/config` for plain
`go run`, so the stack runs without a `.env` too. Variables already set in
the environment always win.

## Design notes

- Geofence state lives in memory, which is fine for a single backend
  instance; move it to a shared store (e.g. Redis) before scaling out.
- The MQTT subscriber and the RabbitMQ clients reconnect in the background,
  so the stack tolerates brokers starting out of order.
