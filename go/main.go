package main

import (
	"log"
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

	var dbs []db.Database
	// dbs = append(dbs, mongo)
	dbs = append(dbs, pgNative)
	dbs = append(dbs, pgTimescale)
	// dbs = append(dbs, dbMysql)

	for _, dbInstance := range dbs {
		if err := dbInstance.Setup(); err != nil {
			return err
		}

		// fake := db.GenerateFakeData(100)
		// if err := dbInstance.UpsertSingle(fake); err != nil {
		// 	return err
		// }

		// if err := dbInstance.UpsertBulk(fake); err != nil {
		// 	return err
		// }

		// _, err = dbInstance.GetOrderedWithLimit(100)
		// if err != nil {
		// 	return err
		// }

		// size, err := dbInstance.TableSizeInKB()
		// if err != nil {
		// 	return err
		// }

		// log.Printf("Table size: %v KB", size)
	}

	return nil
}
