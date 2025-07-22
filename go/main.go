package main

import (
	"log"
	"time"
	"timeseries-benchmark/db"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	mongo, err := db.NewMongoDB("mongodb", "localhost", db.PORT_MONGO, db.DB_USERNAME, db.DB_PASSWORD)
	if err != nil {
		return err
	}
	defer mongo.Close()

	pgNative, err := db.NewPostgresDB("pg-native", "localhost", db.PORT_POSTGRES, db.DB_USERNAME, db.DB_PASSWORD, db.DB_NAME, false)
	if err != nil {
		return err
	}
	defer pgNative.Close()

	pgTimescale, err := db.NewPostgresDB("pg-timescale", "localhost", db.PORT_TIMESCALE, db.DB_USERNAME, db.DB_PASSWORD, db.DB_NAME, true)
	if err != nil {
		return err
	}
	defer pgTimescale.Close()

	// dbMysql, err := db.NewMySQLDB("mysql", "localhost", db.PORT_MYSQL, db.DB_USERNAME, db.DB_PASSWORD, db.DB_NAME)
	// if err != nil {
	// 	return err
	// }
	// defer dbMysql.Close()

	duckDb, err := db.NewDuckDB("duckdb", "./duckdb.db")
	if err != nil {
		return err
	}
	defer duckDb.Close()

	var dbs []db.Database
	dbs = append(dbs, mongo)
	dbs = append(dbs, pgNative)
	dbs = append(dbs, duckDb)
	// dbs = append(dbs, pgTimescale)

	// dbs = append(dbs, pgTimescale)
	// dbs = append(dbs, dbMysql)

	for _, dbInstance := range dbs {
		// if err := dbInstance.Setup(); err != nil {
		// 	return err
		// }

		// fake := db.GenerateFakeData(100)
		// if err := dbInstance.UpsertSingle(fake); err != nil {
		// 	return err
		// }

		// if err := dbInstance.UpsertBulk(fake); err != nil {
		// 	return err
		// }

		now := time.Now()
		data, err := dbInstance.GetOrderedWithLimit(2_000)
		if err != nil {
			return err
		}
		// calculate the rows per millisecond
		elapsed := time.Since(now)
		rpms := float64(len(data)) / float64(elapsed.Milliseconds())
		log.Printf("Rows per millisecond for %s: %.2f", dbInstance.GetName(), rpms)
	}

	return nil
}
