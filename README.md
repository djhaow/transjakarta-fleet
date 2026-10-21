# Transjakarta Fleet Management

Backend service for a vehicle fleet management system: it ingests GPS
locations over MQTT, stores them in PostgreSQL, serves them through a REST
API, and publishes geofence entry events through RabbitMQ.

## Architecture

    GPS tracker (mock-publisher)
        |
        | MQTT /fleet/vehicle/{vehicle_id}/location (QoS 1)
        v
    Mosquitto :1883
        |
        v
    server (cmd/server)
        |--> validate + store --> PostgreSQL (vehicle_locations)
        |--> geofence check (Haversine, radius 50 m)
        |         |
        |         | on an outside -> inside transition
        |         v
        |    RabbitMQ exchange fleet.events -> queue geofence_alerts
        |                                           |
        |                                           v
        |                                      worker (cmd/worker)
        v
    REST API :8080 (GET /vehicles/{id}/location, /history)

Locations are pushed by the trackers and pulled once by the backend; the
REST API reads PostgreSQL only and never touches MQTT or RabbitMQ.

### Data flow

1. A tracker publishes a JSON location to `/fleet/vehicle/{vehicle_id}/location`.
2. `server` takes the vehicle id from the topic, validates the payload
   (required fields, coordinate ranges, timestamp skew) and stores the fix.
3. The same message runs through the geofence tracker: Haversine distance
   against the configured center. Only an outside -> inside transition
   publishes a `geofence_entry` event to the `fleet.events` exchange, so a
   vehicle parked inside the fence does not spam alerts.
4. `worker` consumes the `geofence_alerts` queue, logs the event and acks it.
5. Clients read the latest location and the history straight from PostgreSQL.

### Layout

    cmd/server           MQTT subscriber, geofence publisher, REST API
    cmd/worker           geofence_alerts consumer
    cmd/mock-publisher   GPS simulator, publishes every 2 s
    internal/config      environment and .env loading, all values defaulted
    internal/mqtt        subscriber, topic parsing, background reconnect
    internal/model       payload contract and validation
    internal/database    pgx pool, schema, location queries
    internal/geofence    Haversine distance, entry transition tracking
    internal/rabbitmq    event contract, publisher, consumer
    internal/service     ingest pipeline wiring
    internal/api         gin routes and parameter handling

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
