package mongodb

import (
	"context"
	"fmt"
	"time"
	"ts-benchmark/dbs/data"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewConn(host string, port int, username, password string) (*mongo.Client, error) {
	opt := options.Client().
		SetMaxPoolSize(20).                  // Set the maximum number of connections in the connection pool
		SetMaxConnIdleTime(10 * time.Minute) // Close idle connections after the specified time

	// If both the username and password exists, use it as the credentials. Else use the non-authenticated url.
	var url string
	if username != "" && password != "" {
		opt.SetAuth(options.Credential{Username: username, Password: password})
		url = fmt.Sprintf("mongodb://%s:%s@%s:%d", username, password, host, port)
	} else {
		url = fmt.Sprintf("mongodb://%s:%d", host, port)
	}

	opt.ApplyURI(url)

	conn, err := mongo.Connect(context.Background(), opt)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(context.Background(), nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb: %v", err)
	}

	return conn, nil
}

func Setup(coll *mongo.Collection) error {
	// Delete previous data
	if _, err := coll.DeleteMany(context.Background(), bson.M{}); err != nil {
		return err
	}

	// Create index on the "start_time", "interval" and "area" fields
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "start_time", Value: -1},
			{Key: "interval", Value: -1},
			{Key: "area", Value: -1},
		},
	}

	if _, err := coll.Indexes().CreateOne(context.Background(), indexModel); err != nil {
		return err
	}

	return nil
}

func Upsert(coll *mongo.Collection, docs []data.Timeseries) (time.Duration, error) {
	now := time.Now()

	for _, doc := range docs {
		filter := map[string]interface{}{
			"start_time": doc.StartTime,
			"interval":   doc.Interval,
			"area":       doc.Area,
		}

		update := map[string]interface{}{
			"$set": map[string]interface{}{
				"value": doc.Value,
			},
		}

		_, err := coll.UpdateOne(context.Background(), filter, update, options.Update().SetUpsert(true))
		if err != nil {
			return 0, err
		}
	}

	return time.Since(now), nil
}
