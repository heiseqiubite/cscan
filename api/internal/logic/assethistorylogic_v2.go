package logic

import (
	"context"
	"fmt"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"
)

type AssetHistoryV2Logic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetHistoryV2Logic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetHistoryV2Logic {
	return &AssetHistoryV2Logic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetHistoryV2 retrieves historical scan versions for a specific asset
func (l *AssetHistoryV2Logic) AssetHistoryV2(req *types.AssetScanHistoryReq, workspaceId string) (*types.AssetScanHistoryResp, error) {
	// Validate asset ID
	if req.AssetId == "" {
		return nil, fmt.Errorf("asset_id is required")
	}

	// Fetch asset - 当 workspaceId 为 "all" 时，需要遍历所有工作空间查找资产
	var asset *model.Asset
	var actualWorkspaceId string

	workspaceIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	for _, wsId := range workspaceIds {
		assetModel := model.NewAssetModel(l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName), wsId)
		found, err := assetModel.FindById(l.ctx, req.AssetId)
		if err == nil && found != nil {
			asset = found
			actualWorkspaceId = wsId
			break
		}
	}

	if asset == nil {
		return nil, fmt.Errorf("asset not found: assetId=%s, searched workspaces=%v", req.AssetId, workspaceIds)
	}

	// Parse time range if provided
	var startTime, endTime time.Time
	var err error
	if req.StartTime != "" {
		startTime, err = time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return nil, fmt.Errorf("invalid start_time format: %w", err)
		}
	}
	if req.EndTime != "" {
		endTime, err = time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return nil, fmt.Errorf("invalid end_time format: %w", err)
		}
	}

	// Create history service and fetch historical versions
	historyService := svc.NewHistoryService(l.svcCtx.MongoClient.Database(l.svcCtx.Config.Mongo.DbName))

	historyReq := &svc.GetResultHistoryReq{
		WorkspaceId: actualWorkspaceId,
		Authority:   asset.Authority,
		Host:        asset.Host,
		Port:        asset.Port,
		StartTime:   startTime,
		EndTime:     endTime,
	}

	historyResp, err := historyService.GetResultHistory(l.ctx, historyReq)
	if err != nil {
		return nil, err
	}

	// Convert to response format
	versions := make([]types.HistoricalVersion, len(historyResp.Versions))
	for i, version := range historyResp.Versions {
		versions[i] = types.HistoricalVersion{
			VersionId:      version.VersionId,
			ScanTimestamp:  version.ScanTimestamp.Format(time.RFC3339),
			DirScanCount:   version.DirScanCount,
			VulnScanCount:  version.VulnScanCount,
			ChangesSummary: version.ChangesSummary,
		}
	}

	return &types.AssetScanHistoryResp{
		Code:     0,
		Msg:      "success",
		Versions: versions,
	}, nil
}
