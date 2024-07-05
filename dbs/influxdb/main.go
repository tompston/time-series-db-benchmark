package influxdb

/*

brew install influxdb
influxd

brew services start influxdb

*/

import (
	"context"
	"fmt"
	"time"
	"ts-benchmark/dbs/data"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

func NewConn(url, token string) influxdb2.Client {
	client := influxdb2.NewClient(url, token)
	return client
}

func Setup(client influxdb2.Client, org, bucket string) error {
	deleteAPI := client.DeleteAPI()

	// Define the time range for deletion (from the beginning of time to now)
	start := time.Unix(0, 0)
	end := time.Now().AddDate(2, 0, 0)

	// Delete previous data
	err := deleteAPI.DeleteWithName(context.Background(), org, bucket, start, end, `_measurement="timeseries"`)
	if err != nil {
		return fmt.Errorf("failed to delete previous data: %v", err)
	}

	return nil
}

func Upsert(client influxdb2.Client, org, bucket string, docs []data.Timeseries) error {
	writeAPI := client.WriteAPIBlocking(org, bucket)

	// Upsert new data
	for _, doc := range docs {
		p := influxdb2.NewPointWithMeasurement("timeseries").
			// AddTag("interval", fmt.Sprintf("%d", doc.Interval)).
			AddTag("area", doc.Area).
			AddField("value", doc.Value).
			SetTime(doc.StartTime)

		err := writeAPI.WritePoint(context.Background(), p)
		if err != nil {
			return err
		}
	}

	return nil
}
