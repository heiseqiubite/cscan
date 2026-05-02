package logic

import (
	"context"
	"encoding/base64"
	"sort"
	"strconv"
	"strings"
	"time"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type IconListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewIconListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IconListLogic {
	return &IconListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IconListLogic) IconList(req *types.IconListReq) (*types.IconListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	workspaceId := middleware.GetWorkspaceId(l.ctx)
	stats, err := l.aggregateIconStats(workspaceId)
	if err != nil {
		return nil, err
	}

	keyword := strings.TrimSpace(req.IconHash)
	list := make([]types.IconItem, 0, len(stats))
	for _, stat := range stats {
		if keyword != "" && !strings.Contains(strings.ToLower(stat.IconHash), strings.ToLower(keyword)) {
			continue
		}

		assets, err := l.findIconAssets(workspaceId, stat.IconHash)
		if err != nil {
			return nil, err
		}

		item, ok := l.pickIconPresentation(stat.IconHash, assets)
		if !ok {
			continue
		}
		list = append(list, item)
	}

	total := int64(len(list))
	start := (req.Page - 1) * req.PageSize
	if start > len(list) {
		start = len(list)
	}
	end := start + req.PageSize
	if end > len(list) {
		end = len(list)
	}

	return &types.IconListResp{Code: 0, Msg: "success", Total: total, List: list[start:end]}, nil
}

func (l *IconListLogic) pickIconPresentation(iconHash string, assets []model.Asset) (types.IconItem, bool) {
	assetNames := make([]string, 0, len(assets))
	iconHashFile := ""
	iconData := ""
	screenshot := ""
	var earliestCreate time.Time
	var latestUpdate time.Time

	for _, asset := range assets {
		assetNames = append(assetNames, asset.Host)
		if iconHashFile == "" && asset.IconHashFile != "" {
			iconHashFile = asset.IconHashFile
		}
		if iconData == "" && len(asset.IconHashBytes) > 0 {
			iconData = base64.StdEncoding.EncodeToString(asset.IconHashBytes)
		}
		if screenshot == "" && asset.Screenshot != "" {
			screenshot = asset.Screenshot
		}
		if !asset.CreateTime.IsZero() && (earliestCreate.IsZero() || asset.CreateTime.Before(earliestCreate)) {
			earliestCreate = asset.CreateTime
		}
		if !asset.UpdateTime.IsZero() && (latestUpdate.IsZero() || asset.UpdateTime.After(latestUpdate)) {
			latestUpdate = asset.UpdateTime
		}
	}

	if iconData == "" {
		return types.IconItem{}, false
	}

	item := types.IconItem{
		Id:           iconHash,
		IconHash:     iconHash,
		IconHashFile: iconHashFile,
		IconData:     iconData,
		Screenshot:   screenshot,
		Assets:       assetNames,
	}
	if !earliestCreate.IsZero() {
		item.CreateTime = earliestCreate.Format("2006-01-02 15:04:05")
	}
	if !latestUpdate.IsZero() {
		item.UpdateTime = latestUpdate.Format("2006-01-02 15:04:05")
	}
	return item, true
}

func (l *IconListLogic) IconStat() (*types.IconStatResp, error) {
	workspaceId := middleware.GetWorkspaceId(l.ctx)
	stats, err := l.aggregateIconStats(workspaceId)
	if err != nil {
		return nil, err
	}

	newCount, err := l.countNewIconAssets(workspaceId)
	if err != nil {
		return nil, err
	}

	return &types.IconStatResp{Code: 0, Msg: "success", Total: len(stats), NewCount: int(newCount)}, nil
}

func (l *IconListLogic) aggregateIconStats(workspaceId string) ([]model.IconHashStatResult, error) {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	merged := make(map[string]model.IconHashStatResult)
	for _, wsId := range wsIds {
		stats, err := l.svcCtx.GetAssetModel(wsId).AggregateIconHash(l.ctx, 1000)
		if err != nil {
			return nil, err
		}
		for _, stat := range stats {
			existing := merged[stat.IconHash]
			existing.IconHash = stat.IconHash
			existing.Count += stat.Count
			if len(existing.IconData) == 0 && len(stat.IconData) > 0 {
				existing.IconData = stat.IconData
			}
			merged[stat.IconHash] = existing
		}
	}

	results := make([]model.IconHashStatResult, 0, len(merged))
	for _, stat := range merged {
		results = append(results, stat)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Count == results[j].Count {
			return results[i].IconHash < results[j].IconHash
		}
		return results[i].Count > results[j].Count
	})
	return results, nil
}

func (l *IconListLogic) findIconAssets(workspaceId, iconHash string) ([]model.Asset, error) {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	allAssets := make([]model.Asset, 0)
	for _, wsId := range wsIds {
		assets, err := l.svcCtx.GetAssetModel(wsId).FindFull(l.ctx, bson.M{
			"icon_hash": iconHash,
		}, 1, 20)
		if err != nil {
			return nil, err
		}
		allAssets = append(allAssets, assets...)
	}
	sort.Slice(allAssets, func(i, j int) bool {
		return allAssets[i].UpdateTime.After(allAssets[j].UpdateTime)
	})
	if len(allAssets) > 20 {
		allAssets = allAssets[:20]
	}
	return allAssets, nil
}

func (l *IconListLogic) countNewIconAssets(workspaceId string) (int64, error) {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	var total int64
	for _, wsId := range wsIds {
		count, err := l.svcCtx.GetAssetModel(wsId).Count(l.ctx, bson.M{"icon_hash": bson.M{"$exists": true, "$ne": ""}, "new": true})
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (l *IconListLogic) IconDelete(req *types.IconDeleteReq) (*types.BaseResp, error) {
	if req.Id == "" {
		return &types.BaseResp{Code: 400, Msg: "Icon不能为空"}, nil
	}

	deleted, err := l.deleteIconAssets(middleware.GetWorkspaceId(l.ctx), bson.M{"icon_hash": req.Id})
	if err != nil {
		return nil, err
	}
	if deleted == 0 {
		return &types.BaseResp{Code: 500, Msg: "删除失败"}, nil
	}
	return &types.BaseResp{Code: 0, Msg: "成功删除 " + strconv.FormatInt(deleted, 10) + " 条资产"}, nil
}

func (l *IconListLogic) IconBatchDelete(req *types.IconBatchDeleteReq) (*types.BaseResp, error) {
	if len(req.Ids) == 0 {
		return &types.BaseResp{Code: 400, Msg: "请选择要删除的Icon"}, nil
	}

	deleted, err := l.deleteIconAssets(middleware.GetWorkspaceId(l.ctx), bson.M{"icon_hash": bson.M{"$in": req.Ids}})
	if err != nil {
		return nil, err
	}
	if deleted == 0 {
		return &types.BaseResp{Code: 500, Msg: "删除失败"}, nil
	}
	return &types.BaseResp{Code: 0, Msg: "成功删除 " + strconv.FormatInt(deleted, 10) + " 条资产"}, nil
}

func (l *IconListLogic) IconClear() (*types.BaseResp, error) {
	deleted, err := l.deleteIconAssets(middleware.GetWorkspaceId(l.ctx), bson.M{"icon_hash": bson.M{"$exists": true, "$ne": ""}})
	if err != nil {
		return nil, err
	}
	return &types.BaseResp{Code: 0, Msg: "成功清空 " + strconv.FormatInt(deleted, 10) + " 条资产"}, nil
}

func (l *IconListLogic) deleteIconAssets(workspaceId string, filter bson.M) (int64, error) {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	var total int64
	for _, wsId := range wsIds {
		deleted, err := l.svcCtx.GetAssetModel(wsId).DeleteByFilter(l.ctx, filter)
		if err != nil {
			return 0, err
		}
		total += deleted
	}
	return total, nil
}
