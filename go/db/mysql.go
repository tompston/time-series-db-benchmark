package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLDB struct {
	conn *sql.DB
	name string
}

// mysql -u test -p -h localhost -P 5554
/*

SELECT
    table_schema AS `Database`,
    table_name AS `Table`,
    ROUND((data_length + index_length) / 1024, 2) AS `Size (KB)`
FROM
    information_schema.tables
WHERE
    table_name = 'data_objects' AND table_schema = 'timeseries_benchmark';

*/
func NewMySQLDB(name, host string, port int, username, password, dbname string) (*MySQLDB, error) {
	connStr := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", username, password, host, port, dbname)
	conn, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %v", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %v", err)
	}

	conn.SetMaxOpenConns(100)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(time.Hour)

	return &MySQLDB{
		name: name,
		conn: conn,
	}, nil
}

func (db *MySQLDB) GetName() string {
	return db.name
}

func (db *MySQLDB) Setup() error {

	_, err := db.conn.ExecContext(ctx, `DROP TABLE IF EXISTS `+DB_TABLE_NAME)
	if err != nil {
		return err
	}

	_, err = db.conn.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %v (
			created_at  TIMESTAMP 			NOT NULL,
			updated_at  TIMESTAMP 			NOT NULL,
			start_time  TIMESTAMP 			NOT NULL,
			resolution  BIGINT    			NOT NULL,
			area        VARCHAR(50)      	NOT NULL,
			source      VARCHAR(50)      	NOT NULL,
			value       DOUBLE    			NOT NULL,
			PRIMARY KEY (start_time, resolution, area(50))
		)
	`, DB_TABLE_NAME))
	if err != nil {
		return err
	}

	// _, err = db.conn.ExecContext(ctx, fmt.Sprintf(`CREATE INDEX idx_start_time_resolution_area ON %v (start_time, resolution, area)`, DB_TABLE_NAME))
	// if err != nil {
	// 	return fmt.Errorf("failed to create unique compound index: %v", err)
	// }

	_, err = db.conn.ExecContext(ctx, fmt.Sprintf(`CREATE INDEX idx_start_time ON %v (start_time)`, DB_TABLE_NAME))
	if err != nil {
		return fmt.Errorf("failed to create index: %v", err)
	}

	return nil
}

func (db *MySQLDB) Close() error {
	return db.conn.Close()
}

func (db *MySQLDB) UpsertSingle(docs []DataObject) error {
	query := fmt.Sprintf(`
		INSERT INTO %v (created_at, updated_at, start_time, resolution, area, source, value)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		updated_at = VALUES(updated_at), source = VALUES(source), value = VALUES(value)
	`, DB_TABLE_NAME)

	for _, doc := range docs {
		_, err := db.conn.ExecContext(ctx, query,
			doc.CreatedAt, doc.UpdatedAt, doc.StartTime, doc.Interval, doc.Area, doc.Source, doc.Value)
		if err != nil {
			return fmt.Errorf("UpsertSingle: %v", err)
		}
	}

	return nil
}

func (db *MySQLDB) UpsertBulk(docs []DataObject) error {

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("UpsertBulk: %v", err)
	}

	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(`
		INSERT INTO %v (created_at, updated_at, start_time, resolution, area, source, value)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		updated_at = VALUES(updated_at), source = VALUES(source), value = VALUES(value)
	`, DB_TABLE_NAME))
	if err != nil {
		return fmt.Errorf("UpsertBulk: %v", err)
	}
	defer stmt.Close()

	for _, doc := range docs {
		_, err = stmt.ExecContext(ctx,
			doc.CreatedAt, doc.UpdatedAt, doc.StartTime, doc.Interval, doc.Area, doc.Source, doc.Value)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("UpsertBulk: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("UpsertBulk: %v", err)
	}

	return nil
}

func (db *MySQLDB) GetOrderedWithLimit(limit int) ([]DataObject, error) {

	query := fmt.Sprintf(`SELECT * FROM %v ORDER BY start_time DESC LIMIT ?`, DB_TABLE_NAME)

	rows, err := db.conn.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DataObject
	for rows.Next() {
		var obj DataObject
		err := rows.Scan(&obj.CreatedAt, &obj.UpdatedAt, &obj.StartTime, &obj.Interval, &obj.Area, &obj.Source, &obj.Value)
		if err != nil {
			return nil, err
		}

		results = append(results, obj)
	}

	return results, nil
}

func (db *MySQLDB) TableSizeInKB() (int, error) {

	var totalSize string

	query := fmt.Sprintf(`SELECT ROUND(SUM(data_length + index_length) / 1024) AS total_size FROM information_schema.TABLES WHERE table_name = '%v' AND table_schema = DATABASE();`, DB_TABLE_NAME)
	err := db.conn.QueryRowContext(ctx, query).Scan(&totalSize)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(totalSize)
}
