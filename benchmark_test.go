package main

import (
	"context"
	"fmt"
	"log"
	"testing"

	"ts-benchmark/dbs/data"
	"ts-benchmark/dbs/mongodb"
	"ts-benchmark/dbs/postgres"

	"go.mongodb.org/mongo-driver/bson"
)

const (
	TABLE = "timeseries"
)

func BenchmarkDbs(b *testing.B) {

	pgdb, err := postgres.NewConn("localhost", 5432, "postgres", "postgres", "test")
	if err != nil {
		panic(err)
	}
	defer pgdb.Close(context.Background())

	const NUM_RECORDS = 10000

	// Generate fake data which is reused for both MongoDB and Postgres
	fake := data.CreateFakeData(NUM_RECORDS)

	var kbMongo, kbPostgresNative, kbPostgresTimescale int32

	b.Run("MongoDB-compound-single-upsert", func(b *testing.B) {
		db, err := mongodb.NewConn("localhost", 27017, "", "")
		if err != nil {
			b.Fatalf("Failed to connect to MongoDB: %v", err)
		}
		defer db.Disconnect(context.Background())

		coll := db.Database("test").Collection(TABLE)
		if err := mongodb.Setup(coll); err != nil {
			b.Fatalf("Failed to setup MongoDB: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			t, err := mongodb.Upsert(coll, fake)
			if err != nil {
				b.Fatalf("Failed to Upsert in MongoDB: %v", err)
			}
			_ = t
		}
		b.StopTimer()

		// Get the statistics of the collection
		var stats bson.M
		command := bson.D{{Key: "collStats", Value: "timeseries"}}
		err = db.Database("test").RunCommand(context.TODO(), command).Decode(&stats)
		if err != nil {
			log.Fatal(err)
		}

		bytes, ok := stats["totalSize"].(int32)
		if !ok {
			log.Fatal("Failed to convert to int32")
		}

		kbMongo = bytes / 1024
	})

	b.Run("Postgres-native-single", func(b *testing.B) {

		if err := postgres.SetupVanilla(pgdb); err != nil {
			b.Fatalf("Failed to setup Postgres: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			t, err := postgres.UpsertVanilla(pgdb, fake)
			if err != nil {
				b.Fatalf("Failed to Upsert in Postgres: %v", err)
			}
			_ = t
		}
		b.StopTimer()

		tableSize, err := postgres.GetTableSize(pgdb, "timeseries")
		if err != nil {
			log.Fatal(err)
		}

		kbPostgresNative = int32(tableSize)
	})

	b.Run("Postgres-timescale-single uspert", func(b *testing.B) {
		if err := postgres.SetupTimescaleDB(pgdb); err != nil {
			b.Fatalf("Failed to setup Postgres: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			t, err := postgres.UpsertTimescaleDB(pgdb, fake)
			if err != nil {
				b.Fatalf("Failed to Upsert in Postgres: %v", err)
			}

			_ = t
		}
		b.StopTimer()

		tableSize, err := postgres.GetTableSize(pgdb, "timeseries_timescale")
		if err != nil {
			log.Fatal(err)
		}
		kbPostgresTimescale = int32(tableSize)
	})

	b.Run("Postgres-timescale-batch upsert", func(b *testing.B) {
		if err := postgres.SetupTimescaleDB(pgdb); err != nil {
			b.Fatalf("Failed to setup Postgres: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			t, err := postgres.UpsertBatchTimescaleDB(pgdb, fake)
			if err != nil {
				b.Fatalf("Failed to Upsert in Postgres: %v", err)
			}
			_ = t
		}
		b.StopTimer()

	})

	fmt.Printf(`
Table size summary for %v records:
------------------------
MongoDB: 			%v kb
Postgres (native): 		%v kb
Postgres (timescale): 		%v kb
`, NUM_RECORDS, kbMongo, kbPostgresNative, kbPostgresTimescale)
}
