package db

import (
	"database/sql"
	"fmt"

	_ "github.com/marcboeker/go-duckdb"
)

type DuckDB struct {
	db   *sql.DB
	name string
}

func NewDuckDB(name, filepath string) (*DuckDB, error) {
	db, err := sql.Open("duckdb", filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open DuckDB: %v", err)
	}

	return &DuckDB{
		db:   db,
		name: name,
	}, nil
}

func (d *DuckDB) GetName() string {
	return d.name
}

func (d *DuckDB) Setup() error {
	_, err := d.db.Exec(`DROP TABLE IF EXISTS ` + DB_TABLE_NAME)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(fmt.Sprintf(`
		CREATE TABLE %v (
			created_at  TIMESTAMP NOT NULL,
			updated_at  TIMESTAMP NOT NULL,
			start_time  TIMESTAMP NOT NULL,
			interval    BIGINT    NOT NULL,
			area        TEXT      NOT NULL,
			source      TEXT      NOT NULL,
			value       DOUBLE    NOT NULL,
			UNIQUE(start_time, interval, area)
		);
	`, DB_TABLE_NAME))
	return err
}

func (d *DuckDB) Close() error {
	return d.db.Close()
}

func (d *DuckDB) UpsertSingle(docs []DataObject) error {
	query := fmt.Sprintf(`
		INSERT INTO %v (created_at, updated_at, start_time, interval, area, source, value)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(start_time, interval, area) DO UPDATE SET
			updated_at = EXCLUDED.updated_at,
			source = EXCLUDED.source,
			value = EXCLUDED.value;
	`, DB_TABLE_NAME)

	for _, doc := range docs {
		_, err := d.db.Exec(query,
			doc.CreatedAt, doc.UpdatedAt, doc.StartTime, doc.Interval, doc.Area, doc.Source, doc.Value)
		if err != nil {
			return fmt.Errorf("UpsertSingle: %w", err)
		}
	}

	return nil
}

func (d *DuckDB) UpsertBulk(docs []DataObject) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := fmt.Sprintf(`
		INSERT INTO %v (created_at, updated_at, start_time, interval, area, source, value)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(start_time, interval, area) DO UPDATE SET
			updated_at = EXCLUDED.updated_at,
			source = EXCLUDED.source,
			value = EXCLUDED.value;
	`, DB_TABLE_NAME)

	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, doc := range docs {
		if _, err := stmt.Exec(
			doc.CreatedAt, doc.UpdatedAt, doc.StartTime, doc.Interval, doc.Area, doc.Source, doc.Value); err != nil {
			return fmt.Errorf("UpsertBulk: %w", err)
		}
	}

	return tx.Commit()
}

func (d *DuckDB) GetOrderedWithLimit(limit int) ([]DataObject, error) {
	query := fmt.Sprintf(`SELECT created_at, updated_at, start_time, interval, area, source, value FROM %v ORDER BY start_time DESC LIMIT %d`, DB_TABLE_NAME, limit)
	rows, err := d.db.Query(query)
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

func (d *DuckDB) TableSizeInKB() (int, error) {
	return 0, nil
}
