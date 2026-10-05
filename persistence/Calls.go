package persistence

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/sync/errgroup"
)

type UpsertResult string

const (
	Collection                = "calls"
	queryTimeout              = 10 * time.Second
	Inserted     UpsertResult = "inserted"
	Updated      UpsertResult = "updated"
	Unchanged    UpsertResult = "unchanged"
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

func UpsertCall(ctx context.Context, document map[string]any) (UpsertResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	filter := bson.M{"id": document["id"]}
	update := bson.M{"$set": document}
	opts := options.UpdateOne().SetUpsert(true)

	result, err := calls.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return "", err
	}

	switch {
	case result.UpsertedCount == 1:
		return Inserted, nil
	case result.ModifiedCount == 1:
		return Updated, nil
	default:
		return Unchanged, nil
	}
}

// ListCalls returns one page of calls, most recently received first, along
// with the total number of calls. page starts at 1.
func ListCalls(ctx context.Context, page, limit int64) ([]bson.M, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	results := []bson.M{} // encodes as [] rather than null when empty
	var total int64

	// The page and the total are independent queries, so run them at the same
	// time. If either fails, errgroup cancels ctx and the other one stops.
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		// ObjectIDs increase with insertion time, so sorting on _id gives
		// newest-first using the index every collection already has.
		opts := options.Find().
			SetSort(bson.D{{Key: "_id", Value: -1}}).
			SetSkip((page - 1) * limit).
			SetLimit(limit)

		cursor, err := calls.Find(ctx, bson.D{}, opts)
		if err != nil {
			return err
		}
		return cursor.All(ctx, &results)
	})

	g.Go(func() error {
		n, err := calls.CountDocuments(ctx, bson.D{})
		total = n
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

func EnsureIndexes(ctx context.Context) error {
	_, err := calls.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}
