// Package database owns the PostgreSQL connection pool, the schema and the
// location queries.
package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"transjakarta-fleet/internal/config"
	"transjakarta-fleet/internal/model"
)

// schema is applied on startup; it is idempotent so restarts are safe.
const schema = `
CREATE TABLE IF NOT EXISTS vehicle_locations (
	id         BIGSERIAL PRIMARY KEY,
	vehicle_id TEXT             NOT NULL,
	latitude   DOUBLE PRECISION NOT NULL,
	longitude  DOUBLE PRECISION NOT NULL,
	timestamp  BIGINT           NOT NULL,
	created_at TIMESTAMPTZ      NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_vehicle_locations_vehicle_ts
	ON vehicle_locations (vehicle_id, timestamp DESC);
`

// ErrNotFound reports that a vehicle has no stored location.
var ErrNotFound = errors.New("vehicle location not found")

type DB struct {
	pool *pgxpool.Pool
}

// Connect opens a pool, waits for PostgreSQL readiness and applies the
// schema. Waiting here makes local `go run` flows work even without the
// compose healthchecks.
func Connect(ctx context.Context, cfg *config.Config) (*DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB,
	)

	var pool *pgxpool.Pool
	for attempt := 1; ; attempt++ {
		p, err := pgxpool.New(ctx, dsn)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = p.Ping(pingCtx)
			cancel()
			if err == nil {
				pool = p
				break
			}
			p.Close()
		}
		if attempt >= 15 {
			return nil, fmt.Errorf("postgres unreachable after %d attempts: %w", attempt, err)
		}
		log.Printf("[postgres] not ready yet (attempt %d): %v", attempt, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	log.Printf("[postgres] connected, schema ready")
	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) InsertLocation(ctx context.Context, loc *model.LocationPayload) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO vehicle_locations (vehicle_id, latitude, longitude, timestamp)
		VALUES ($1, $2, $3, $4)`,
		loc.VehicleID, loc.Latitude, loc.Longitude, loc.Timestamp,
	)
	return err
}

// GetLatestLocation returns the most recent location of a vehicle, or
// ErrNotFound when the vehicle has never reported.
func (db *DB) GetLatestLocation(ctx context.Context, vehicleID string) (*model.LocationPayload, error) {
	row := db.pool.QueryRow(ctx, `
		SELECT vehicle_id, latitude, longitude, timestamp
		FROM vehicle_locations
		WHERE vehicle_id = $1
		ORDER BY timestamp DESC, id DESC
		LIMIT 1`,
		vehicleID,
	)

	var loc model.LocationPayload
	err := row.Scan(&loc.VehicleID, &loc.Latitude, &loc.Longitude, &loc.Timestamp)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query latest location: %w", err)
	}
	return &loc, nil
}

// GetHistory returns locations of a vehicle whose timestamp falls within
// [start, end] (inclusive), ordered oldest first.
func (db *DB) GetHistory(ctx context.Context, vehicleID string, start, end int64, limit int) ([]model.LocationPayload, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT vehicle_id, latitude, longitude, timestamp
		FROM vehicle_locations
		WHERE vehicle_id = $1 AND timestamp BETWEEN $2 AND $3
		ORDER BY timestamp ASC, id ASC
		LIMIT $4`,
		vehicleID, start, end, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	locations := make([]model.LocationPayload, 0, limit)
	for rows.Next() {
		var loc model.LocationPayload
		if err := rows.Scan(&loc.VehicleID, &loc.Latitude, &loc.Longitude, &loc.Timestamp); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		locations = append(locations, loc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history rows: %w", err)
	}
	return locations, nil
}
