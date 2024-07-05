package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"ts-benchmark/dbs/data"

	"github.com/jackc/pgx/v5"
)

// CREATE DATABASE test;
func NewConn(host string, port int, username, password, dbname string) (*pgx.Conn, error) {
	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", username, password, host, port, dbname)
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %v", err)
	}

	return conn, nil
}

func SetupVanilla(conn *pgx.Conn) error {
	// drop the table if it exists to refresh the data
	if _, err := conn.Exec(context.Background(), `DROP TABLE IF EXISTS timeseries`); err != nil {
		return err
	}

	if _, err := conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS timeseries (
			id 			SERIAL 				PRIMARY KEY,
			start_time 	TIMESTAMP			NOT NULL,
			interval 	BIGINT				NOT NULL,
			area 		VARCHAR(64) 		NOT NULL,
			value 		DOUBLE PRECISION	NOT NULL,
			UNIQUE (start_time, interval, area)
		)
	`); err != nil {
		return err
	}

	// Create index on start_time
	if _, err := conn.Exec(context.Background(), `
		CREATE INDEX IF NOT EXISTS idx_start_time ON timeseries(start_time);
		CREATE INDEX IF NOT EXISTS idx_interval ON timeseries(interval);
		CREATE INDEX IF NOT EXISTS idx_area ON timeseries(area);
		CREATE INDEX IF NOT EXISTS idx_timeseries_area_compound ON timeseries(start_time, interval, area);
	`); err != nil {
		return err
	}

	return nil
}

func UpsertVanilla(conn *pgx.Conn, docs []data.Timeseries) (time.Duration, error) {
	now := time.Now()
	for _, doc := range docs {
		_, err := conn.Exec(context.Background(), `
			INSERT INTO timeseries (start_time, interval, area, value)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (start_time, interval, area)
			DO UPDATE SET value = EXCLUDED.value
		`, doc.StartTime, doc.Interval, doc.Area, doc.Value)
		if err != nil {
			return 0, err
		}
	}

	return time.Since(now), nil
}

func SetupTimescaleDB(conn *pgx.Conn) error {
	// // Create the TimescaleDB extension if it doesn't exist
	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS timescaledb`); err != nil {
		return err
	}

	// drop the table if it exists to refresh the data
	if _, err := conn.Exec(context.Background(), `DROP TABLE IF EXISTS timeseries_timescale`); err != nil {
		return err
	}

	// Create the table
	if _, err := conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS timeseries_timescale (
			id 				SERIAL,
			start_time 		TIMESTAMP 			NOT NULL,
			interval 		BIGINT 				NOT NULL,
			area 			VARCHAR(64) 		NOT NULL,
			value 			DOUBLE PRECISION	NOT NULL,
			PRIMARY KEY (start_time, interval, area)
		)
	`); err != nil {
		return fmt.Errorf("SetupTimescaleDB: %v", err)
	}

	// Convert the table to a hypertable
	if _, err := conn.Exec(context.Background(), `SELECT create_hypertable('timeseries_timescale', 'start_time');`); err != nil {
		return fmt.Errorf("SetupTimescaleDB:create_hypertable : %v", err)
	}

	// // Create additional indexes
	if _, err := conn.Exec(context.Background(), `
		CREATE INDEX IF NOT EXISTS idx_interval ON timeseries_timescale(interval);
		CREATE INDEX IF NOT EXISTS idx_area ON timeseries_timescale(area);
		CREATE INDEX IF NOT EXISTS idx_timeseries_timescale_area_compound ON timeseries_timescale(start_time, interval, area);
	`); err != nil {
		return fmt.Errorf("SetupTimescaleDB:idx_interval : %v", err)
	}

	return nil
}

func UpsertTimescaleDB(conn *pgx.Conn, docs []data.Timeseries) (time.Duration, error) {
	now := time.Now()
	for _, doc := range docs {
		_, err := conn.Exec(context.Background(), `
			INSERT INTO timeseries_timescale (start_time, interval, area, value)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (start_time, interval, area)
			DO UPDATE SET value = EXCLUDED.value
		`, doc.StartTime, doc.Interval, doc.Area, doc.Value)
		if err != nil {
			return 0, err
		}
	}

	return time.Since(now), nil
}

func UpsertBatchTimescaleDB(conn *pgx.Conn, docs []data.Timeseries) (time.Duration, error) {
	now := time.Now()

	batch := &pgx.Batch{}
	for _, doc := range docs {
		batch.Queue(`
			INSERT INTO timeseries_timescale (start_time, interval, area, value)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (start_time, interval, area)
			DO UPDATE SET value = EXCLUDED.value
		`, doc.StartTime, doc.Interval, doc.Area, doc.Value)
	}

	br := conn.SendBatch(context.Background(), batch)
	defer br.Close()

	for i := 0; i < len(docs); i++ {
		_, err := br.Exec()
		if err != nil {
			return 0, err
		}
	}

	return time.Since(now), nil
}

func EnableTimescaleDB(conn *pgx.Conn) error {
	// Create the TimescaleDB extension if it doesn't exist
	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS timescaledb`); err != nil {
		return err
	}
	return nil
}

func DisableTimescaleDB(conn *pgx.Conn) error {
	// Drop the TimescaleDB extension if it exists
	if _, err := conn.Exec(context.Background(), `DROP EXTENSION IF EXISTS timescaledb CASCADE`); err != nil {
		return err
	}
	return nil
}

// GetTableSize returns the size of a specified table including its indexes in a human-readable format
func GetTableSize(conn *pgx.Conn, tableName string) (int, error) {
	var totalSize string
	query := `
		SELECT pg_size_pretty(pg_total_relation_size($1)) AS total_size;
	`
	err := conn.QueryRow(context.Background(), query, tableName).Scan(&totalSize)
	if err != nil {
		return 0, fmt.Errorf("GetTableSize: %v", err)
	}

	// parse the string to a int
	return parseSize(totalSize)
	// return totalSize, nil
}

// parseSize parses the size string and returns the size in kilobytes.
func parseSize(sizeStr string) (int, error) {
	// Remove any leading/trailing whitespace
	sizeStr = strings.TrimSpace(sizeStr)

	// Split the string into the numeric part and the unit part
	parts := strings.Fields(sizeStr)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	// Convert the numeric part to a float
	sizeValue, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric value: %s", parts[0])
	}

	// Determine the unit and convert to kilobytes
	unit := parts[1]
	unit = strings.ToLower(unit)

	switch unit {
	case "kb":
		return int(sizeValue), nil
	case "mb":
		return int(sizeValue * 1024), nil
	case "gb":
		return int(sizeValue * 1024 * 1024), nil
	case "tb":
		return int(sizeValue * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}
