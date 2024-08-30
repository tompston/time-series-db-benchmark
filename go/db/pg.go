package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type PostgresDB struct {
	conn           *pgx.Conn
	usingTimescale bool
	name           string
}

func NewPostgresDB(name, host string, port int, username, password, dbname string, usingTimescale bool) (*PostgresDB, error) {
	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", username, password, host, port, dbname)
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %v", err)
	}

	return &PostgresDB{
		name:           name,
		conn:           conn,
		usingTimescale: usingTimescale,
	}, nil
}

func (db *PostgresDB) GetName() string {
	return db.name
}

func (db *PostgresDB) Setup() error {
	if _, err := db.conn.Exec(ctx, `DROP TABLE IF EXISTS `+DB_TABLE_NAME); err != nil {
		return err
	}

	if _, err := db.conn.Exec(ctx, fmt.Sprintf(`
                CREATE TABLE IF NOT EXISTS %v (
                    created_at  TIMESTAMPTZ         NOT NULL,
                    updated_at  TIMESTAMPTZ         NOT NULL,
                    start_time  TIMESTAMPTZ         NOT NULL,
                    interval    BIGINT     			NOT NULL,
                    area        TEXT         		NOT NULL,
                    source      TEXT         		NOT NULL,
                    value       DOUBLE PRECISION    NOT NULL,
					PRIMARY KEY (start_time, interval, area)
                )
	`, DB_TABLE_NAME)); err != nil {
		return err
	}

	if db.usingTimescale {
		if _, err := db.conn.Exec(ctx, fmt.Sprintf(`SELECT create_hypertable('%v', by_range('start_time', INTERVAL '30 days'));`, DB_TABLE_NAME)); err != nil {
			return err
		}

		if _, err := db.conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %v SET (timescaledb.compress);`, DB_TABLE_NAME)); err != nil {
			return err
		}

		if _, err := db.conn.Exec(ctx, fmt.Sprintf(`
		ALTER TABLE %v 
		SET (
			timescaledb.compress
		);
		`, DB_TABLE_NAME)); err != nil {
			return err
		}
	}

	// // create compound index on the "start_time", "interval" and "area" fields
	// if _, err := db.conn.Exec(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_start_time_interval_area ON %v (start_time, interval, area)`, DB_TABLE_NAME)); err != nil {
	// 	return fmt.Errorf("failed to create unique compound index: %v", err)
	// }

	if _, err := db.conn.Exec(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_start_time ON %v (start_time)`, DB_TABLE_NAME)); err != nil {
		return fmt.Errorf("failed to create index: %v", err)
	}

	return nil
}

func (db *PostgresDB) Close() error {
	return db.conn.Close(ctx)
}

func (db *PostgresDB) UpsertSingle(docs []DataObject) error {
	for _, doc := range docs {

		query := fmt.Sprintf(`
			INSERT INTO ` + DB_TABLE_NAME + ` (created_at, updated_at, start_time, interval, area, source, value)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (start_time, interval, area) DO UPDATE
			SET updated_at = $2, source = $6, value = $7
		`)

		if _, err := db.conn.Exec(ctx, query,
			doc.CreatedAt, doc.UpdatedAt, doc.StartTime, doc.Interval, doc.Area, doc.Source, doc.Value); err != nil {
			return fmt.Errorf("UpsertSingle: %v", err)
		}
	}

	return nil
}

func (db *PostgresDB) UpsertBulk(docs []DataObject) error {

	batch := &pgx.Batch{}
	for _, doc := range docs {

		query := fmt.Sprintf(`
			INSERT INTO ` + DB_TABLE_NAME + ` (created_at, updated_at, start_time, interval, area, source, value)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (start_time, interval, area) DO UPDATE
			SET updated_at = $2, source = $6, value = $7
		`)

		batch.Queue(query,
			doc.CreatedAt, doc.UpdatedAt, doc.StartTime, doc.Interval, doc.Area, doc.Source, doc.Value)
	}

	br := db.conn.SendBatch(context.Background(), batch)
	defer br.Close()

	for i := 0; i < len(docs); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("UpsertBulk: %v", err)
		}
	}

	return nil
}

func (db *PostgresDB) GetOrderedWithLimit(limit int) ([]DataObject, error) {
	query := fmt.Sprintf(`SELECT * FROM %v ORDER BY start_time DESC LIMIT %v`, DB_TABLE_NAME, limit)

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DataObject
	for rows.Next() {
		var obj DataObject
		if err := rows.Scan(&obj.CreatedAt, &obj.UpdatedAt, &obj.StartTime, &obj.Interval, &obj.Area, &obj.Source, &obj.Value); err != nil {
			return nil, err
		}

		results = append(results, obj)
	}

	return results, nil
}

func (db *PostgresDB) TableSizeInKB() (int, error) {

	var totalSize string

	if db.usingTimescale {
		query := `SELECT hypertable_size($1) AS total_size;`
		err := db.conn.QueryRow(context.Background(), query, DB_TABLE_NAME).Scan(&totalSize)
		if err != nil {
			return 0, err
		}

		sizeInBytes, err := strconv.Atoi(totalSize)
		if err != nil {
			return 0, err
		}

		return sizeInBytes / 1024, nil
	}

	query := `SELECT pg_size_pretty(pg_total_relation_size($1)) AS total_size;`
	err := db.conn.QueryRow(context.Background(), query, DB_TABLE_NAME).Scan(&totalSize)
	if err != nil {
		return 0, err
	}

	parseSize := func(sizeStr string) (int, error) {
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

	return parseSize(totalSize)
}

// It seems like, without triggering the manual compression, the compression is not applied
// after upserrting the data. So to get the table size with the compression applied, we
// need to run this manually in the benchmarks.
func (db *PostgresDB) ExecManualCompression() error {
	if !db.usingTimescale {
		return fmt.Errorf("compression is only supported for TimescaleDB")
	}

	if _, err := db.conn.Exec(ctx, fmt.Sprintf(`SELECT compress_chunk(c) from show_chunks('%v') c;`, DB_TABLE_NAME)); err != nil {
		return err
	}

	return nil
}
