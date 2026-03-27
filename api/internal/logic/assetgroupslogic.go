package logic

import (
	"context"
	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/utils"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type AssetGroupsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssetGroupsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssetGroupsLogic {
	return &AssetGroupsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AssetGroups 获取按域名分组的资产统计
func (l *AssetGroupsLogic) AssetGroups(req *types.AssetGroupsReq, workspaceId string) (resp *types.AssetGroupsResp, err error) {
	l.Logger.Infof("AssetGroups查询: workspaceId=%s, page=%d, pageSize=%d", workspaceId, req.Page, req.PageSize)

	// 获取需要查询的工作空间列表
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)
	l.Logger.Infof("AssetGroups查询工作空间列表: %v", wsIds)

	// 用于存储所有分组数据
	domainGroups := make(map[string]*types.AssetGroup)
	// 用于存储每个域名的最近任务执行时长
	domainDuration := make(map[string]time.Duration)

	// 遍历所有工作空间
	for _, wsId := range wsIds {
		// 1. 先从任务中提取目标域名，创建初始分组
		taskModel := l.svcCtx.GetMainTaskModel(wsId)
		tasks, err := taskModel.Find(l.ctx, bson.M{}, 0, 0)
		if err != nil {
			l.Logger.Errorf("查询工作空间 %s 任务失败: %v", wsId, err)
		} else {
			// 用于记录每个域名对应的任务状态
			domainTaskStatus := make(map[string]string)

			for _, task := range tasks {
				l.Logger.Infof("处理任务: taskId=%s, status=%s, target=%s", task.TaskId, task.Status, task.Target)

				// 从任务目标中提取域名
				targets := strings.Split(task.Target, "\n")
				for _, target := range targets {
					target = strings.TrimSpace(target)
					if target == "" {
						continue
					}

					// 提取主域名
					domain := extractMainDomainFromTarget(target)
					if domain == "" {
						continue
					}

					l.Logger.Infof("提取域名: target=%s, domain=%s", target, domain)

					// 记录该域名的任务状态（如果有多个任务，优先显示运行中的状态）
					currentStatus := domainTaskStatus[domain]
					taskStatus := getTaskStatusForGroup(task.Status)

					l.Logger.Infof("任务状态转换: taskStatus=%s -> groupStatus=%s, currentStatus=%s", task.Status, taskStatus, currentStatus)

					// 状态优先级：running > starting > failed > stopped > finished
					if currentStatus == "" ||
						(taskStatus == "running") ||
						(taskStatus == "starting" && currentStatus != "running") ||
						(taskStatus == "failed" && currentStatus != "running" && currentStatus != "starting") ||
						(taskStatus == "stopped" && currentStatus != "running" && currentStatus != "starting" && currentStatus != "failed") {
						domainTaskStatus[domain] = taskStatus
						l.Logger.Infof("更新域名 %s 的状态为: %s", domain, taskStatus)
					}

					// 计算任务的真实执行时长
					if task.StartTime != nil && task.EndTime != nil {
						taskDuration := task.EndTime.Sub(*task.StartTime)
						// 保留最近一次任务的执行时长（更新时间最新的）
						if existingDuration, exists := domainDuration[domain]; !exists || task.UpdateTime.After(task.CreateTime) {
							if !exists || taskDuration > 0 {
								domainDuration[domain] = existingDuration + taskDuration
							}
						}
					} else if task.StartTime != nil && task.Status == "STARTED" {
						// 正在运行的任务，计算从开始到现在的时长
						domainDuration[domain] = time.Since(*task.StartTime)
					}

					// 如果分组不存在，创建新分组
					if _, exists := domainGroups[domain]; !exists {
						domainGroups[domain] = &types.AssetGroup{
							Domain:        domain,
							Source:        "Auto Discovery",
							Status:        domainTaskStatus[domain],
							TotalServices: 0,
							Duration:      "",
							LastUpdated:   "",
							FirstSeen:     task.CreateTime,
							LatestUpdate:  task.UpdateTime,
						}
						l.Logger.Infof("创建新分组: domain=%s, status=%s", domain, domainTaskStatus[domain])
					} else {
						// 更新状态
						domainGroups[domain].Status = domainTaskStatus[domain]
						l.Logger.Infof("更新分组状态: domain=%s, status=%s", domain, domainTaskStatus[domain])
					}
				}
			}
		}

		// 2. 从资产中统计实际数据
		assetModel := l.svcCtx.GetAssetModel(wsId)

		// 查询所有资产
		filter := bson.M{}
		assets, err := assetModel.Find(l.ctx, filter, 0, 0)
		if err != nil {
			l.Logger.Errorf("查询工作空间 %s 资产失败: %v", wsId, err)
			continue
		}

		// 按域名分组统计
		for _, asset := range assets {
			// 提取主域名
			domain := extractMainDomain(asset.Host)
			if domain == "" {
				continue
			}

			// 如果分组不存在，创建新分组
			if _, exists := domainGroups[domain]; !exists {
				domainGroups[domain] = &types.AssetGroup{
					Domain:        domain,
					Source:        "Auto Discovery",
					Status:        "finished", // 如果只有资产没有任务，默认为已完成
					TotalServices: 0,
					Duration:      "",
					LastUpdated:   "",
					FirstSeen:     asset.CreateTime,
					LatestUpdate:  asset.UpdateTime,
				}
			}

			group := domainGroups[domain]
			group.TotalServices++

			// 更新最早和最晚时间
			if asset.CreateTime.Before(group.FirstSeen) {
				group.FirstSeen = asset.CreateTime
			}
			if asset.UpdateTime.After(group.LatestUpdate) {
				group.LatestUpdate = asset.UpdateTime
			}
		}
	}

	// 计算持续时间和格式化时间
	for domain, group := range domainGroups {
		// 使用真实的任务执行时长
		if duration, exists := domainDuration[domain]; exists && duration > 0 {
			group.Duration = formatDuration(duration)
		} else {
			// 如果没有任务记录，显示为 "-"
			group.Duration = "-"
		}

		// 格式化最后更新时间
		now := time.Now()
		diff := now.Sub(group.LatestUpdate)
		if diff < time.Minute {
			group.LastUpdated = "just now"
		} else if diff < time.Hour {
			group.LastUpdated = fmt.Sprintf("%d minutes ago", int(diff.Minutes()))
		} else if diff < 24*time.Hour {
			group.LastUpdated = fmt.Sprintf("%d hours ago", int(diff.Hours()))
		} else {
			days := int(diff.Hours() / 24)
			if days == 1 {
				group.LastUpdated = "1 day ago"
			} else {
				group.LastUpdated = fmt.Sprintf("%d days ago", days)
			}
		}
	}

	// 转换为列表
	list := make([]types.AssetGroup, 0, len(domainGroups))
	for _, group := range domainGroups {
		l.Logger.Infof("分组数据: domain=%s, status=%s, services=%d", group.Domain, group.Status, group.TotalServices)
		list = append(list, *group)
	}

	// 按服务数量排序（降序）
	for i := 0; i < len(list)-1; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].TotalServices < list[j].TotalServices {
				list[i], list[j] = list[j], list[i]
			}
		}
	}

	// 分页
	total := len(list)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		list = []types.AssetGroup{}
	} else {
		if end > total {
			end = total
		}
		list = list[start:end]
	}

	// 记录返回的数据
	for _, group := range list {
		l.Logger.Infof("返回分组: domain=%s, status=%s, services=%d", group.Domain, group.Status, group.TotalServices)
	}

	return &types.AssetGroupsResp{
		Code:  0,
		Msg:   "success",
		Total: total,
		List:  list,
	}, nil
}

// formatDuration 格式化时间持续时长
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	} else if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%dm%ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	} else {
		days := int(d.Hours() / 24)
		hours := int(d.Hours()) % 24
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
}

// extractMainDomainFromTarget 从任务目标中提取主域名
func extractMainDomainFromTarget(target string) string {
	// 使用 utils 解析目标主域名，自动剥离各种协议和额外字符
	info := utils.ParseTarget(target)
	if info.Host == "" {
		return ""
	}

	// 移除通配符
	host := strings.TrimPrefix(info.Host, "*.")

	// 提取根域名
	rootDomain := utils.GetRootDomain(host)
	if rootDomain != "" {
		return rootDomain
	}

	return host
}

// getTaskStatusForGroup 将任务状态转换为分组状态
func getTaskStatusForGroup(taskStatus string) string {
	switch taskStatus {
	case "CREATED", "PENDING":
		return "starting"
	case "STARTED": // 注意：任务执行中的状态是 STARTED
		return "running"
	case "SUCCESS":
		return "finished"
	case "FAILURE":
		return "failed"
	case "STOPPED", "REVOKED", "PAUSED":
		return "stopped"
	default:
		return "finished"
	}
}

// extractMainDomain 从主机名中提取主域名
func extractMainDomain(host string) string {
	// 如果是IP地址，返回IP
	if isIPAddress(host) {
		return host
	}

	// 使用 utils 的 GetRootDomain 提取
	rootDomain := utils.GetRootDomain(host)
	if rootDomain != "" {
		return rootDomain
	}

	return host
}

// isIPAddress 判断是否为IP地址
func isIPAddress(host string) bool {
	// 简单判断：包含数字和点
	for _, c := range host {
		if (c >= '0' && c <= '9') || c == '.' || c == ':' {
			continue
		}
		return false
	}
	return strings.Contains(host, ".")
}
