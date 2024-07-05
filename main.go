package main

import (
	"context"
	"fmt"
	"ts-benchmark/dbs/data"
	"ts-benchmark/dbs/postgres"
)

func main() {
	db, err := postgres.NewConn("localhost", 5432, "postgres", "postgres", "test")
	if err != nil {
		panic(err)
	}

	defer db.Close(context.Background())

	if err := postgres.SetupTimescaleDB(db); err != nil {
		panic(err)
	}

	docs := data.CreateFakeData(4000)
	t, err := postgres.UpsertTimescaleDB(db, docs)
	if err != nil {
		panic(err)
	}

	fmt.Printf("t: %v\n", t.Seconds())
}
