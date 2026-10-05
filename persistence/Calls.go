package persistence

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	Collection   = "calls"
	queryTimeout = 10 * time.Second
)

// One client is shared by the whole app; it manages its own connection pool.
var (
	client *mongo.Client
	calls  *mongo.Collection
)

// Connect configures the shared client. The driver connects lazily, so this
// does not fail if MongoDB isn't up yet — Ping (used by /readyz) reports that.
func Connect(uri, database string) error {
	c, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	client = c
	calls = c.Database(database).Collection(Collection)
	return nil
}

func Disconnect(ctx context.Context) error {
	return client.Disconnect(ctx)
}

func Ping(ctx context.Context) error {
	return client.Ping(ctx, nil)
}

func InsertCall(ctx context.Context, document map[string]any) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	result, err := calls.InsertOne(ctx, document)
	if err != nil {
		return nil, err
	}
	return result.InsertedID, nil
}

func UpsertCall(ctx context.Context, document map[string]any) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	filter := bson.M{"id": document["id"]}
	update := bson.M{"$set": document}
	opts := options.UpdateOne().SetUpsert(true)

	result, err := calls.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return false, err
	}

	return result.UpsertedCount == 1, nil
}

func ListCalls(ctx context.Context) ([]bson.M, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	cursor, err := calls.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}

	results := []bson.M{} // encodes as [] rather than null when empty
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func EnsureIndexes(ctx context.Context) error {
	_, err := calls.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}