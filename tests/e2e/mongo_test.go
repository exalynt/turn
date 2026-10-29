//go:build e2e

package e2e

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/exalynt/turn/paginator/cursor"
	"github.com/exalynt/turn/store"
	turnmongo "github.com/exalynt/turn/store/mongo"
)

// Cursors on MongoDB with store.Cursor and turnmongo, under a filter.
func TestStoreCursorMongo(t *testing.T) {
	ctx := context.Background()
	client, err := mongo.Connect(options.Client().ApplyURI(env("TURN_MONGO_URI", "mongodb://turn:turn@localhost:27017/turn?authSource=admin")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("disconnect: %v", err)
		}
	})

	coll := client.Database("turn").Collection(uniqueName(t, "users"))
	t.Cleanup(func() {
		if err := coll.Drop(context.Background()); err != nil {
			t.Errorf("drop %s: %v", coll.Name(), err)
		}
	})
	if _, err := coll.InsertMany(ctx, users); err != nil {
		t.Fatal(err)
	}

	listing := newUserCursor(t, "_id")
	forward, backward := readCursors(t, func(selector cursor.Selector) (cursor.Page[user], error) {
		return listing.List(ctx, selector, func(ctx context.Context, q store.Query) ([]user, error) {
			f := turnmongo.Render(q)
			cur, err := coll.Find(ctx, f.And(bson.D{{Key: "status", Value: "active"}}), f.Options())
			if err != nil {
				return nil, err
			}
			var items []user
			if err := cur.All(ctx, &items); err != nil {
				return nil, err
			}
			return items, nil
		}, "users", "active")
	})

	assertPages(t, "forward", forward, activeForward)
	assertPages(t, "backward", backward, activeBackward)
}
