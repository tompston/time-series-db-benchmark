package db

import (
	"context"
	"math/rand"
	"time"
)

type Database interface {
	GetName() string
	Setup() error
	Close() error
	TableSizeInKB() (int, error)
	UpsertSingle(docs []DataObject) error
	UpsertBulk(docs []DataObject) error
	GetOrderedWithLimit(limit int) ([]DataObject, error)
}

const (
	DB_NAME       = "timeseries_benchmark"
	DB_USERNAME   = "test"
	DB_PASSWORD   = "test"
	DB_TABLE_NAME = "data_objects"

	PORT_MONGO     = 5551
	PORT_POSTGRES  = 5552
	PORT_TIMESCALE = 5553
	PORT_MYSQL     = 5554
)

type DataObject struct {
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
	StartTime time.Time `bson:"start_time"`
	Interval  int64     `bson:"interval"`
	Area      string    `bson:"area"`
	Source    string    `bson:"source"`
	Value     float64   `bson:"value"`
}

var (
	BaseTime = time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx      = context.Background()
)

func GenerateFakeData(numObjects int) []DataObject {
	rows := make([]DataObject, numObjects)

	for i := range numObjects {
		now := time.Now().UTC()
		startTime := BaseTime.Add(time.Duration(i) * time.Hour)

		obj := DataObject{
			CreatedAt: now,
			UpdatedAt: now,
			StartTime: startTime,
			Interval:  3600000, // 1 hour in milliseconds
			Area:      "lv",
			Source:    "source-of-data",
			Value:     rand.Float64(),
		}

		rows[i] = obj
	}

	return rows
}
