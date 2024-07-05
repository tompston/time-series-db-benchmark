package data

import (
	"math"
	"time"
)

type Timeseries struct {
	StartTime time.Time `json:"start_time" bson:"start_time"`
	Interval  int       `json:"interval" bson:"interval"`
	Area      string    `json:"area" bson:"area"`
	Value     float64   `json:"value" bson:"value"`
}

func CreateFakeData(numRows int) []Timeseries {
	var docs []Timeseries

	for i := 0; i < numRows; i++ {
		// random value
		val := math.Round(math.Sin(float64(i)) * 100)

		doc := Timeseries{
			StartTime: time.Now().Add(time.Duration(i) * time.Hour).Truncate(time.Minute),
			Interval:  36000000,
			Area:      "area1",
			Value:     val,
		}
		docs = append(docs, doc)
	}

	return docs
}
