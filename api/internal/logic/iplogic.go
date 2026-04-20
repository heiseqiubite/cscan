package logic

import (
	"context"
	"sort"
	"strconv"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"

	"go.mongodb.org/mongo-driver/bson"
)

type IPLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewIPLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IPLogic {
	return &IPLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// IPList IP列表 - 从资产中聚合IP信息
// 显示所有有IP的资产，按IP聚合端口和域名信息
func (l *IPLogic) IPList(req *types.IPListReq, workspaceId string) (*types.IPListResp, error) {
	resp := &types.IPListResp{Code: 0, List: []types.IPAsset{}}

	workspaceIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	if len(workspaceIds) == 0 {
		return resp, nil
	}

	orgMap := common.LoadOrgMap(l.ctx, l.svcCtx)

	// 用于聚合IP信息
	ipMap := make(map[string]*types.IPAsset)

	for _, wsId := range workspaceIds {
		assetModel := model.NewAssetModel(l.svcCtx.MongoDB, wsId)

		// 构建查询条件 - 查询有IP的资产
		// IP来源：host字段是IP 或 ip.ipv4有值
		filter := bson.M{}
		conditions := []bson.M{}

		// 基础条件：有IP的资产
		// 不加基础条件，查询所有资产然后提取IP

		// 最上方筛选 (Query)
		if req.Query != "" {
			conditions = append(conditions, bson.M{
				"$or": []bson.M{
					{"host": bson.M{"$regex": req.Query, "$options": "i"}},
					{"ip.ipv4.ip": bson.M{"$regex": req.Query, "$options": "i"}},
				},
			})
		}

		// IP搜索
		if req.IP != "" {
			conditions = append(conditions, bson.M{
				"$or": []bson.M{
					{"host": bson.M{"$regex": req.IP, "$options": "i"}},
					{"ip.ipv4.ip": bson.M{"$regex": req.IP, "$options": "i"}},
				},
			})
		}
		// 端口搜索
		if req.Port != "" {
			if port, err := strconv.Atoi(req.Port); err == nil {
				conditions = append(conditions, bson.M{"port": port})
			}
		}
		// 服务搜索
		if req.Service != "" {
			conditions = append(conditions, bson.M{"service": bson.M{"$regex": req.Service, "$options": "i"}})
		}
		// 位置搜索
		if req.Location != "" {
			conditions = append(conditions, bson.M{"ip.ipv4.location": bson.M{"$regex": req.Location, "$options": "i"}})
		}
		// 组织
		if req.OrgId != "" {
			conditions = append(conditions, bson.M{"org_id": req.OrgId})
		}

		if len(conditions) > 0 {
			filter["$and"] = conditions
		}

		// 查询所有匹配的资产
		assets, err := assetModel.Find(l.ctx, filter, 0, 0)
		if err != nil {
			continue
		}

		// 聚合IP信息
		for _, asset := range assets {
			// 收集所有IP地址
			var ips []string
			var location string

			// 从ip.ipv4字段获取IP
			for _, ipv4 := range asset.Ip.IpV4 {
				if ipv4.IPName != "" {
					ips = append(ips, ipv4.IPName)
					if location == "" && ipv4.Location != "" {
						location = ipv4.Location
					}
				}
			}

			// 如果host是IP地址，也加入
			if common.IsIPAddress(asset.Host) && asset.Host != "" {
				found := false
				for _, ip := range ips {
					if ip == asset.Host {
						found = true
						break
					}
				}
				if !found {
					ips = append(ips, asset.Host)
				}
			}

			// 如果没有IP，跳过
			if len(ips) == 0 {
				continue
			}

			// 为每个IP创建或更新记录
			for _, ip := range ips {
				if existing, ok := ipMap[ip]; ok {
					// 更新已存在的IP记录
					// 添加端口（去重）
					if asset.Port > 0 {
						portFound := false
						for _, p := range existing.Ports {
							if p.Port == asset.Port {
								portFound = true
								break
							}
						}
						if !portFound {
							existing.Ports = append(existing.Ports, types.PortInfo{
								Port:    asset.Port,
								Service: asset.Service,
							})
						}
					}

					// 添加域名（去重）
					domain := asset.Domain
					if domain == "" && !common.IsIPAddress(asset.Host) {
						domain = asset.Host
					}
					if domain != "" {
						domainFound := false
						for _, d := range existing.Domains {
							if d == domain {
								domainFound = true
								break
							}
						}
						if !domainFound {
							existing.Domains = append(existing.Domains, domain)
							existing.DomainCount = len(existing.Domains)
						}
					}

					// 更新位置信息
					if existing.Location == "" && location != "" {
						existing.Location = location
					}

					// 更新时间取最新
					if assetUpdate := asset.UpdateTime.Local().Format("2006-01-02 15:04:05"); assetUpdate > existing.UpdateTime {
						existing.UpdateTime = assetUpdate
					}
					// 创建时间取最早
					if assetCreate := asset.CreateTime.Local().Format("2006-01-02 15:04:05"); existing.CreateTime == "" || assetCreate < existing.CreateTime {
						existing.CreateTime = assetCreate
					}
				} else {
					// 创建新的IP记录
					ports := []types.PortInfo{}
					if asset.Port > 0 {
						ports = append(ports, types.PortInfo{
							Port:    asset.Port,
							Service: asset.Service,
						})
					}

					domains := []string{}
					domain := asset.Domain
					if domain == "" && !common.IsIPAddress(asset.Host) {
						domain = asset.Host
					}
					if domain != "" {
						domains = append(domains, domain)
					}

					ipMap[ip] = &types.IPAsset{
						Id:          asset.Id.Hex(),
						IP:          ip,
						Location:    location,
						Ports:       ports,
						Domains:     domains,
						DomainCount: len(domains),
						OrgId:       asset.OrgId,
						OrgName:     orgMap[asset.OrgId],
						CreateTime:  asset.CreateTime.Local().Format("2006-01-02 15:04:05"),
						UpdateTime:  asset.UpdateTime.Local().Format("2006-01-02 15:04:05"),
						IsNew:       asset.IsNewAsset,
					}
				}
			}
		}
	}

	// 转换为列表并排序
	allIPs := make([]types.IPAsset, 0, len(ipMap))
	for _, ip := range ipMap {
		allIPs = append(allIPs, *ip)
	}

	// 按端口数量降序排序
	sort.Slice(allIPs, func(i, j int) bool {
		return len(allIPs[i].Ports) > len(allIPs[j].Ports)
	})

	// 分页
	total := len(allIPs)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	resp.Total = total
	if start < total {
		resp.List = allIPs[start:end]
	}
	return resp, nil
}

// IPStat IP统计
func (l *IPLogic) IPStat(workspaceId string) (*types.IPStatResp, error) {
	resp := &types.IPStatResp{Code: 0}

	workspaceIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	if len(workspaceIds) == 0 {
		return resp, nil
	}

	ipSet := make(map[string]bool)
	portSet := make(map[int]bool)
	serviceSet := make(map[string]bool)
	newIPs := make(map[string]bool)

	for _, wsId := range workspaceIds {
		assetModel := model.NewAssetModel(l.svcCtx.MongoDB, wsId)

		// 查询所有资产
		assets, err := assetModel.Find(l.ctx, bson.M{}, 0, 0)
		if err != nil {
			continue
		}

		for _, asset := range assets {
			// 收集IP
			var ips []string
			for _, ipv4 := range asset.Ip.IpV4 {
				if ipv4.IPName != "" {
					ips = append(ips, ipv4.IPName)
				}
			}
			if common.IsIPAddress(asset.Host) && asset.Host != "" {
				found := false
				for _, ip := range ips {
					if ip == asset.Host {
						found = true
						break
					}
				}
				if !found {
					ips = append(ips, asset.Host)
				}
			}

			for _, ip := range ips {
				if !ipSet[ip] {
					ipSet[ip] = true
					if asset.IsNewAsset {
						newIPs[ip] = true
					}
				}
			}

			if asset.Port > 0 {
				portSet[asset.Port] = true
			}

			if asset.Service != "" {
				serviceSet[asset.Service] = true
			}
		}
	}

	resp.Total = len(ipSet)
	resp.PortCount = len(portSet)
	resp.ServiceCount = len(serviceSet)
	resp.NewCount = len(newIPs)

	return resp, nil
}

// IPDelete 删除IP（删除该IP下所有资产）
func (l *IPLogic) IPDelete(req *types.IPDeleteReq, workspaceId string) (*types.BaseResp, error) {
	if req.IP == "" {
		return &types.BaseResp{Code: 400, Msg: "IP不能为空"}, nil
	}

	workspaceIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	var totalDeleted int64

	for _, wsId := range workspaceIds {
		assetModel := model.NewAssetModel(l.svcCtx.MongoDB, wsId)

		// 查询该IP下的所有资产
		filter := bson.M{
			"$or": []bson.M{
				{"host": req.IP},
				{"ip.ipv4.ip": req.IP},
			},
		}
		assets, err := assetModel.Find(l.ctx, filter, 0, 0)
		if err != nil {
			continue
		}

		// 收集资产ID
		ids := make([]string, 0, len(assets))
		for _, asset := range assets {
			ids = append(ids, asset.Id.Hex())
		}

		// 批量删除
		if len(ids) > 0 {
			deleted, _ := assetModel.BatchDelete(l.ctx, ids)
			totalDeleted += deleted
		}
	}

	return &types.BaseResp{Code: 0, Msg: "成功删除 " + strconv.FormatInt(totalDeleted, 10) + " 条资产"}, nil
}

// IPBatchDelete 批量删除IP
func (l *IPLogic) IPBatchDelete(req *types.IPBatchDeleteReq, workspaceId string) (*types.BaseResp, error) {
	if len(req.IPs) == 0 {
		return &types.BaseResp{Code: 400, Msg: "请选择要删除的IP"}, nil
	}

	workspaceIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	var totalDeleted int64

	for _, wsId := range workspaceIds {
		assetModel := model.NewAssetModel(l.svcCtx.MongoDB, wsId)

		// 查询这些IP下的所有资产
		filter := bson.M{
			"$or": []bson.M{
				{"host": bson.M{"$in": req.IPs}},
				{"ip.ipv4.ip": bson.M{"$in": req.IPs}},
			},
		}
		assets, err := assetModel.Find(l.ctx, filter, 0, 0)
		if err != nil {
			continue
		}

		// 收集资产ID
		ids := make([]string, 0, len(assets))
		for _, asset := range assets {
			ids = append(ids, asset.Id.Hex())
		}

		// 批量删除
		if len(ids) > 0 {
			deleted, _ := assetModel.BatchDelete(l.ctx, ids)
			totalDeleted += deleted
		}
	}

	return &types.BaseResp{Code: 0, Msg: "成功删除 " + strconv.FormatInt(totalDeleted, 10) + " 条资产"}, nil
}
