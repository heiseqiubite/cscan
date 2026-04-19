package logic

import (
	"context"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type PortListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPortListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PortListLogic {
	return &PortListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PortListLogic) PortList(req *types.PortListReq) (*types.PortListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	workspaceId := middleware.GetWorkspaceId(l.ctx)

	// Prepare match object
	matchObj := bson.M{"port": bson.M{"$gt": 0}}

	// Parsing general text grouping
	if req.Query != "" {
		parseQuerySyntax(req.Query, matchObj)
	}

	if req.Port > 0 {
		matchObj["port"] = req.Port
	}
	if req.Host != "" {
		matchObj["host"] = bson.M{"$regex": req.Host, "$options": "i"}
	}
	if req.OrgId != "" {
		matchObj["org_id"] = req.OrgId
	}

	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	var allPorts []types.PortListItem
	var total int64

	orgMap := common.LoadOrgMap(l.ctx, l.svcCtx)

	skip := (req.Page - 1) * req.PageSize
	limit := req.PageSize

	for _, wsId := range wsIds {
		assetModel := l.svcCtx.GetAssetModel(wsId)
		results, wsTotal, err := assetModel.AggregatePortList(l.ctx, matchObj, skip, limit)
		if err != nil {
			l.Logger.Errorf("查询工作空间 %s 端口聚合失败: %v", wsId, err)
			continue
		}

		total += wsTotal

		for _, r := range results {
			// Find orgname from the first host or fallback mapping if needed. But orgName requires complex lookup if per-group.
			// Currently using global OrgId logic if applied, else taking mapping from req.OrgId
			orgName := ""
			if req.OrgId != "" {
				orgName = orgMap[req.OrgId]
			}

			// Map out empty services cleanly
			services := []string{}
			for _, s := range r.Services {
				if s != "" {
					services = append(services, s)
				}
			}

			item := types.PortListItem{
				Port:       r.Port,
				AssetCount: r.AssetCount,
				Hosts:      r.Hosts,
				Services:   services,
				OrgName:    orgName,
				CreateTime: r.CreateTime.Local().Format("2006-01-02 15:04:05"),
				UpdateTime: r.UpdateTime.Local().Format("2006-01-02 15:04:05"),
			}

			allPorts = append(allPorts, item)
		}
	}

	// Just in case of cross-workspace duplication (if multiple workspaces return the same port page),
	// a higher level distinct loop might be needed. For now, since most requests operate in a single workspace
	// context, this appended list is fine.

	return &types.PortListResp{
		Code:  0,
		Msg:   "success",
		Total: int(total),
		List:  allPorts,
	}, nil
}
