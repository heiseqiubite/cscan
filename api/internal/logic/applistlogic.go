package logic

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type AppListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppListLogic {
	return &AppListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AppListLogic) AppList(req *types.AppListReq) (*types.AppListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	workspaceId := middleware.GetWorkspaceId(l.ctx)
	stats, err := l.aggregateAppStats(workspaceId)
	if err != nil {
		return nil, err
	}

	orgMap := common.LoadOrgMap(l.ctx, l.svcCtx)
	filtered := make([]model.StatResult, 0, len(stats))
	keyword := strings.TrimSpace(req.App)
	for _, stat := range stats {
		if keyword == "" || strings.Contains(strings.ToLower(stat.Field), strings.ToLower(keyword)) {
			filtered = append(filtered, stat)
		}
	}

	total := int64(len(filtered))
	start := (req.Page - 1) * req.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + req.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	pageItems := filtered[start:end]
	list := make([]types.AppItem, 0, len(pageItems))
	for _, stat := range pageItems {
		assets, err := l.findAppAssets(workspaceId, stat.Field)
		if err != nil {
			return nil, err
		}

		assetNames := make([]string, 0, len(assets))
		var createTime, updateTime string
		orgName := ""
		for _, asset := range assets {
			assetNames = append(assetNames, asset.Host)
			if assetCreate := asset.CreateTime.Local().Format("2006-01-02 15:04:05"); createTime == "" || assetCreate < createTime {
				createTime = assetCreate
			}
			if assetUpdate := asset.UpdateTime.Local().Format("2006-01-02 15:04:05"); assetUpdate > updateTime {
				updateTime = assetUpdate
			}
			if orgName == "" {
				orgName = orgMap[asset.OrgId]
			}
		}

		list = append(list, types.AppItem{
			Id:         stat.Field,
			App:        stat.Field,
			Category:   "-",
			Assets:     assetNames,
			OrgName:    orgName,
			CreateTime: createTime,
			UpdateTime: updateTime,
		})
	}

	return &types.AppListResp{Code: 0, Msg: "success", Total: total, List: list}, nil
}

func (l *AppListLogic) AppStat() (*types.AppStatResp, error) {
	workspaceId := middleware.GetWorkspaceId(l.ctx)
	stats, err := l.aggregateAppStats(workspaceId)
	if err != nil {
		return nil, err
	}

	newCount, err := l.countNewAppAssets(workspaceId)
	if err != nil {
		return nil, err
	}

	return &types.AppStatResp{Code: 0, Msg: "success", Total: len(stats), NewCount: int(newCount)}, nil
}

func (l *AppListLogic) aggregateAppStats(workspaceId string) ([]model.StatResult, error) {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	merged := make(map[string]int)
	for _, wsId := range wsIds {
		stats, err := l.svcCtx.GetAssetModel(wsId).AggregateApp(l.ctx, 1000)
		if err != nil {
			return nil, err
		}
		for _, stat := range stats {
			merged[stat.Field] += stat.Count
		}
	}

	results := make([]model.StatResult, 0, len(merged))
	for field, count := range merged {
		results = append(results, model.StatResult{Field: field, Count: count})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Count == results[j].Count {
			return results[i].Field < results[j].Field
		}
		return results[i].Count > results[j].Count
	})
	return results, nil
}

func (l *AppListLogic) findAppAssets(workspaceId, app string) ([]model.Asset, error) {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	allAssets := make([]model.Asset, 0)
	for _, wsId := range wsIds {
		assets, err := l.svcCtx.GetAssetModel(wsId).FindFull(l.ctx, bson.M{
			"app": bson.M{"$in": []string{app}},
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

func (l *AppListLogic) countNewAppAssets(workspaceId string) (int64, error) {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	var total int64
	for _, wsId := range wsIds {
		count, err := l.svcCtx.GetAssetModel(wsId).Count(l.ctx, bson.M{"app": bson.M{"$exists": true, "$ne": bson.A{}}, "new": true})
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (l *AppListLogic) AppDelete(req *types.AppDeleteReq) (*types.BaseResp, error) {
	if req.Id == "" {
		return &types.BaseResp{Code: 400, Msg: "应用不能为空"}, nil
	}

	deleted, err := l.deleteAppAssets(middleware.GetWorkspaceId(l.ctx), bson.M{"app": bson.M{"$in": []string{req.Id}}})
	if err != nil {
		return nil, err
	}
	if deleted == 0 {
		return &types.BaseResp{Code: 500, Msg: "删除失败"}, nil
	}
	return &types.BaseResp{Code: 0, Msg: "成功删除 " + strconv.FormatInt(deleted, 10) + " 条资产"}, nil
}

func (l *AppListLogic) AppBatchDelete(req *types.AppBatchDeleteReq) (*types.BaseResp, error) {
	if len(req.Ids) == 0 {
		return &types.BaseResp{Code: 400, Msg: "请选择要删除的应用"}, nil
	}

	deleted, err := l.deleteAppAssets(middleware.GetWorkspaceId(l.ctx), bson.M{"app": bson.M{"$in": req.Ids}})
	if err != nil {
		return nil, err
	}
	if deleted == 0 {
		return &types.BaseResp{Code: 500, Msg: "删除失败"}, nil
	}
	return &types.BaseResp{Code: 0, Msg: "成功删除 " + strconv.FormatInt(deleted, 10) + " 条资产"}, nil
}

func (l *AppListLogic) AppClear() (*types.BaseResp, error) {
	deleted, err := l.deleteAppAssets(middleware.GetWorkspaceId(l.ctx), bson.M{"app": bson.M{"$exists": true, "$ne": bson.A{}}})
	if err != nil {
		return nil, err
	}
	return &types.BaseResp{Code: 0, Msg: "成功清空 " + strconv.FormatInt(deleted, 10) + " 条资产"}, nil
}

func (l *AppListLogic) deleteAppAssets(workspaceId string, filter bson.M) (int64, error) {
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
