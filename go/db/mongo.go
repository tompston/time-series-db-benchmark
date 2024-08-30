package db

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	conn *mongo.Client
	coll *mongo.Collection
	name string
}

func NewMongoDB(name, host string, port int, username, password string) (*MongoDB, error) {
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

	conn, err := mongo.Connect(ctx, opt)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb: %v", err)
	}

	return &MongoDB{
		name: name,
		conn: conn,
		coll: conn.Database(DB_NAME).Collection(DB_TABLE_NAME),
	}, nil
}

func (db *MongoDB) GetName() string {
	return db.name
}

func (db *MongoDB) Close() error {
	return db.conn.Disconnect(ctx)
}

func (db *MongoDB) Setup() error {
	if _, err := db.coll.DeleteMany(ctx, bson.M{}); err != nil {
		return err
	}

	// Create compound index on the "start_time", "interval" and "area" fields
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "start_time", Value: -1},
			{Key: "interval", Value: -1},
			{Key: "area", Value: -1},
		},
	}

	if _, err := db.coll.Indexes().CreateOne(ctx, indexModel); err != nil {
		return err
	}

	// Create index on the "start_time" field
	if _, err := db.coll.Indexes().CreateOne(ctx,
		mongo.IndexModel{Keys: bson.D{{Key: "start_time", Value: -1}}}); err != nil {
		return err
	}

	return nil
}

func (db *MongoDB) UpsertSingle(docs []DataObject) error {
	for _, doc := range docs {
		filter := bson.M{"start_time": doc.StartTime, "interval": doc.Interval, "area": doc.Area}
		update := bson.M{"$set": doc}
		opt := options.Update().SetUpsert(true)

		if _, err := db.coll.UpdateOne(ctx, filter, update, opt); err != nil {
			return err
		}
	}

	return nil
}

func (db *MongoDB) UpsertBulk(docs []DataObject) error {
	var models []mongo.WriteModel

	for _, doc := range docs {
		filter := bson.M{"start_time": doc.StartTime, "interval": doc.Interval, "area": doc.Area}
		update := bson.M{"$set": doc}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}

	_, err := db.coll.BulkWrite(ctx, models)
	return err
}

func (db *MongoDB) GetOrderedWithLimit(limit int) ([]DataObject, error) {
	opts := options.Find().SetSort(bson.M{"start_time": -1}).SetLimit(int64(limit))
	cursor, err := db.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}

	var results []DataObject
	err = cursor.All(ctx, &results)
	return results, err
}

func (db *MongoDB) TableSizeInKB() (int, error) {
	var stats bson.M
	command := bson.D{{Key: "collStats", Value: DB_TABLE_NAME}}

	if err := db.conn.Database(DB_NAME).RunCommand(ctx, command).Decode(&stats); err != nil {
		return 0, err
	}

	bytes, ok := stats["totalSize"].(int32)
	if !ok {
		return 0, fmt.Errorf("failed to get totalSize from stats")
	}

	return int(bytes / 1024), nil
}
