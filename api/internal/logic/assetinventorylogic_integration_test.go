package logic

import (
	"context"
	"testing"
	"time"

	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestAssetInventoryRequireRecognitionOrShotFiltersEmptyAssets(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	db := client.Database("cscan_test")
	workspaceId := primitive.NewObjectID().Hex()
	cleanupCollections(t, db, workspaceId)
	defer cleanupCollections(t, db, workspaceId)

	insertWorkspaces(t, db, []string{workspaceId})

	now := time.Now()
	assetModel := model.NewAssetModel(db, workspaceId)

	err = assetModel.Insert(ctx, &model.Asset{
		Id:         primitive.NewObjectID(),
		Authority:  "shot.example.com:443",
		Host:       "shot.example.com",
		Port:       443,
		Screenshot: "https://example.com/shot.png",
		CreateTime: now,
		UpdateTime: now,
	})
	require.NoError(t, err)

	err = assetModel.Insert(ctx, &model.Asset{
		Id:         primitive.NewObjectID(),
		Authority:  "tech.example.com:80",
		Host:       "tech.example.com",
		Port:       80,
		App:        []string{"nginx[httpx]"},
		CreateTime: now.Add(time.Second),
		UpdateTime: now.Add(time.Second),
	})
	require.NoError(t, err)

	err = assetModel.Insert(ctx, &model.Asset{
		Id:         primitive.NewObjectID(),
		Authority:  "empty.example.com:8080",
		Host:       "empty.example.com",
		Port:       8080,
		CreateTime: now.Add(2 * time.Second),
		UpdateTime: now.Add(2 * time.Second),
	})
	require.NoError(t, err)

	logicCtx := context.WithValue(context.Background(), middleware.WorkspaceIdKey, workspaceId)
	svcCtx := &svc.ServiceContext{MongoDB: db, WorkspaceModel: model.NewWorkspaceModel(db)}

	resp, err := NewAssetInventoryLogic(logicCtx, svcCtx).AssetInventory(&types.AssetInventoryReq{
		Page:                     1,
		PageSize:                 10,
		SortBy:                   "time",
		RequireRecognitionOrShot: true,
	}, workspaceId)
	require.NoError(t, err)
	require.Equal(t, 2, resp.Total)
	require.Len(t, resp.List, 2)

	hosts := []string{resp.List[0].Host, resp.List[1].Host}
	require.ElementsMatch(t, []string{"shot.example.com", "tech.example.com"}, hosts)
}

func TestAssetInventoryRequireRecognitionOrShotComposesWithQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	db := client.Database("cscan_test")
	workspaceId := primitive.NewObjectID().Hex()
	cleanupCollections(t, db, workspaceId)
	defer cleanupCollections(t, db, workspaceId)

	insertWorkspaces(t, db, []string{workspaceId})

	now := time.Now()
	assetModel := model.NewAssetModel(db, workspaceId)

	err = assetModel.Insert(ctx, &model.Asset{
		Id:         primitive.NewObjectID(),
		Authority:  "match-shot.example.com:443",
		Host:       "match-shot.example.com",
		Port:       443,
		Screenshot: "https://example.com/match.png",
		CreateTime: now,
		UpdateTime: now,
	})
	require.NoError(t, err)

	err = assetModel.Insert(ctx, &model.Asset{
		Id:         primitive.NewObjectID(),
		Authority:  "match-empty.example.com:80",
		Host:       "match-empty.example.com",
		Port:       80,
		CreateTime: now.Add(time.Second),
		UpdateTime: now.Add(time.Second),
	})
	require.NoError(t, err)

	err = assetModel.Insert(ctx, &model.Asset{
		Id:         primitive.NewObjectID(),
		Authority:  "other-tech.example.com:443",
		Host:       "other-tech.example.com",
		Port:       443,
		App:        []string{"gozero"},
		CreateTime: now.Add(2 * time.Second),
		UpdateTime: now.Add(2 * time.Second),
	})
	require.NoError(t, err)

	logicCtx := context.WithValue(context.Background(), middleware.WorkspaceIdKey, workspaceId)
	svcCtx := &svc.ServiceContext{MongoDB: db, WorkspaceModel: model.NewWorkspaceModel(db)}

	resp, err := NewAssetInventoryLogic(logicCtx, svcCtx).AssetInventory(&types.AssetInventoryReq{
		Page:                     1,
		PageSize:                 10,
		Query:                    "match",
		SortBy:                   "time",
		RequireRecognitionOrShot: true,
	}, workspaceId)
	require.NoError(t, err)
	require.Equal(t, 1, resp.Total)
	require.Len(t, resp.List, 1)
	require.Equal(t, "match-shot.example.com", resp.List[0].Host)
}
