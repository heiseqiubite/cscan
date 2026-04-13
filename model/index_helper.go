package model

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/mongo"
)

var indexOnce sync.Map

func ensureIndexes(coll *mongo.Collection, indexes []mongo.IndexModel) {
	key := coll.Name()
	if _, loaded := indexOnce.LoadOrStore(key, true); !loaded {
		coll.Indexes().CreateMany(context.Background(), indexes)
	}
}
