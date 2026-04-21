package model

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GogoPoc Gogo专用POC
type GogoPoc struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name       string             `bson:"name" json:"name"`               // POC名称
	Source     string             `bson:"source" json:"source"`           // 来源: cyberhub
	Data       []byte             `bson:"data" json:"data"`               // SDK 原始 yaml 数据
	Enabled    bool               `bson:"enabled" json:"enabled"`          // 是否启用
	Severity   string             `bson:"severity" json:"severity"`        // 严重等级
	Tags       []string           `bson:"tags" json:"tags"`               // 标签
	CreateTime time.Time         `bson:"create_time" json:"createTime"`
	UpdateTime time.Time         `bson:"update_time" json:"updateTime"`
}

// GogoPocModel Gogo POC模型
type GogoPocModel struct {
	coll *mongo.Collection
}

func NewGogoPocModel(db *mongo.Database) *GogoPocModel {
	coll := db.Collection("gogo_pocs")
	coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "name", Value: 1}, {Key: "source", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "enabled", Value: 1}}},
		{Keys: bson.D{{Key: "severity", Value: 1}}},
	})
	return &GogoPocModel{coll: coll}
}

func (m *GogoPocModel) Upsert(ctx context.Context, doc *GogoPoc) error {
	if doc.ID.IsZero() {
		doc.ID = primitive.NewObjectID()
	}
	doc.UpdateTime = time.Now()
	if doc.CreateTime.IsZero() {
		doc.CreateTime = doc.UpdateTime
	}
	filter := bson.M{"name": doc.Name, "source": doc.Source}
	update := bson.M{"$set": doc}
	opts := options.Update().SetUpsert(true)
	_, err := m.coll.UpdateOne(ctx, filter, update, opts)
	return err
}

func (m *GogoPocModel) Find(ctx context.Context, filter bson.M, page, pageSize int) ([]GogoPoc, error) {
	opts := options.Find()
	if page > 0 && pageSize > 0 {
		opts.SetSkip(int64((page - 1) * pageSize))
		opts.SetLimit(int64(pageSize))
	}
	opts.SetSort(bson.D{{Key: "create_time", Value: -1}})
	cursor, err := m.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []GogoPoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func (m *GogoPocModel) Count(ctx context.Context, filter bson.M) (int64, error) {
	return m.coll.CountDocuments(ctx, filter)
}

func (m *GogoPocModel) UpdateEnabled(ctx context.Context, id string, enabled bool) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = m.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"enabled": enabled, "update_time": time.Now()}})
	return err
}

func (m *GogoPocModel) GetStats(ctx context.Context) (map[string]int64, error) {
	stats := make(map[string]int64)
	total, _ := m.coll.CountDocuments(ctx, bson.M{})
	stats["total"] = total
	enabled, _ := m.coll.CountDocuments(ctx, bson.M{"enabled": true})
	stats["enabled"] = enabled
	return stats, nil
}

// GetAllData 流式获取所有启用状态的 data，cb 每收到一条调用一次，避免一次性加载到内存
func (m *GogoPocModel) GetAllData(ctx context.Context, cb func(data []byte) error) error {
	opts := options.Find().SetProjection(bson.M{"data": 1})
	cursor, err := m.coll.Find(ctx, bson.M{"enabled": true}, opts)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var result struct {
			Data []byte `bson:"data"`
		}
		if err := cursor.Decode(&result); err != nil {
			return err
		}
		if err := cb(result.Data); err != nil {
			return err
		}
	}
	return cursor.Err()
}
