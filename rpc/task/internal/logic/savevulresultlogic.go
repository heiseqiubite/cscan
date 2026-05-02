package logic

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cscan/model"
	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// AssetCache 资产缓存，用于批量处理时减少数据库查询
type AssetCache struct {
	assets map[string]*model.Asset
}

func NewAssetCache() *AssetCache {
	return &AssetCache{
		assets: make(map[string]*model.Asset),
	}
}

func (c *AssetCache) getKey(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

func (c *AssetCache) getOrCreate(ctx context.Context, assetModel *model.AssetModel, host string, port int) *model.Asset {
	key := c.getKey(host, port)
	if asset, ok := c.assets[key]; ok {
		return asset
	}

	// 从数据库查询
	asset, _ := assetModel.FindByHostPort(ctx, host, port)
	if asset == nil {
		// 资产不存在，创建一个新的
		asset = &model.Asset{
			Host:       host,
			Port:       port,
			Authority:  fmt.Sprintf("%s:%d", host, port),
			Service:    "http",
			IsHTTP:     true,
			Source:     "poc_scan",
			CreateTime: time.Now(),
			UpdateTime: time.Now(),
		}
		// 设置 HTTPS 端口
		if port == 443 || port == 8443 {
			asset.Service = "https"
		}
		// 插入数据库
		if err := assetModel.Insert(ctx, asset); err != nil {
			logx.Errorf("Failed to create asset for vul: %v", err)
			return nil
		}
		// 查询获取完整对象
		asset, _ = assetModel.FindByHostPort(ctx, host, port)
	}
	if asset != nil {
		c.assets[key] = asset
	}
	return asset
}

type SaveVulResultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveVulResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveVulResultLogic {
	return &SaveVulResultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 保存漏洞结果
// Note: This currently saves to the {workspace}_vul collection using the legacy Vul model.
// The vulnerability scanner produces Vul objects, not ScanResult objects.
// For now, we keep this behavior and add scan_timestamp tracking for history purposes.
func (l *SaveVulResultLogic) SaveVulResult(in *pb.SaveVulResultReq) (*pb.SaveVulResultResp, error) {
	if len(in.Vuls) == 0 {
		return &pb.SaveVulResultResp{
			Success: true,
			Message: "No vulnerabilities to save",
			Total:   0,
		}, nil
	}

	workspaceId := in.WorkspaceId
	if workspaceId == "" {
		workspaceId = "default"
	}

	vulModel := l.svcCtx.GetVulModel(workspaceId)
	assetModel := l.svcCtx.GetAssetModel(workspaceId)

	// 创建资产缓存，减少重复查询
	assetCache := NewAssetCache()

	// 按资产分组计算风险评分
	assetRiskMap := make(map[string]float64) // Key: "host:port", Value: maxScore
	assetVulCount := make(map[string]int)    // Key: "host:port", Value: vul count

	var savedCount int32

	for _, pbVul := range in.Vuls {
		// 解析 URL 获取 host 和 port（如果 Authority 为空）
		host := pbVul.Host
		port := int(pbVul.Port)

		if host == "" && pbVul.Url != "" {
			// 从 URL 解析 host 和 port
			parsedHost, parsedPort := parseHostFromUrl(pbVul.Url)
			if parsedHost != "" {
				host = parsedHost
				if port == 0 {
					port = parsedPort
				}
			}
		}

		// 确保资产存在（自动创建），放入缓存供后续批量更新使用
		assetCache.getOrCreate(l.ctx, assetModel, host, port)

		// 构建漏洞对象
		vul := &model.Vul{
			Authority: pbVul.Authority,
			Host:      host,
			Port:      port,
			Url:       pbVul.Url,
			PocFile:   pbVul.PocFile,
			Source:    pbVul.Source,
			Severity:  pbVul.Severity,
			Extra:     pbVul.Extra,
			Result:    pbVul.Result,
			TaskId:    in.MainTaskId,
		}

		// 漏洞知识库关联字段
		if pbVul.CvssScore != nil {
			vul.CvssScore = *pbVul.CvssScore
		}
		if pbVul.CveId != nil {
			vul.CveId = *pbVul.CveId
		}
		if pbVul.CweId != nil {
			vul.CweId = *pbVul.CweId
		}
		if pbVul.Remediation != nil {
			vul.Remediation = *pbVul.Remediation
		}
		if len(pbVul.References) > 0 {
			vul.References = pbVul.References
		}

		// 证据链字段
		if pbVul.MatcherName != nil {
			vul.MatcherName = *pbVul.MatcherName
		}
		if len(pbVul.ExtractedResults) > 0 {
			vul.ExtractedResults = pbVul.ExtractedResults
		}
		if pbVul.CurlCommand != nil {
			vul.CurlCommand = *pbVul.CurlCommand
		}
		if pbVul.Request != nil {
			vul.Request = *pbVul.Request
		}
		if pbVul.Response != nil {
			vul.Response = *pbVul.Response
		}
		if pbVul.ResponseTruncated != nil {
			vul.ResponseTruncated = *pbVul.ResponseTruncated
		}

		// 漏洞名称和标签
		if pbVul.VulName != nil {
			vul.VulName = *pbVul.VulName
		}
		if len(pbVul.Tags) > 0 {
			vul.Tags = pbVul.Tags
		}
		pbVulName := ""
		if pbVul.VulName != nil {
			pbVulName = *pbVul.VulName
		}
		l.Logger.Infof("[SaveVulResult] poc=%s pbVulName.nil=%v pbVulName=%q pbTags=%v modelVulName=%q modelTags=%v", pbVul.PocFile, pbVul.VulName == nil, pbVulName, pbVul.Tags, vul.VulName, vul.Tags)

		// 使用Upsert避免重复
		// Note: The Upsert method in VulModel already handles scan_count and timestamps
		// which provides basic history tracking through first_seen_time and last_seen_time
		if err := vulModel.Upsert(l.ctx, vul); err != nil {
			l.Logger.Errorf("SaveVulResult: failed to upsert vul: %v", err)
			continue
		}
		savedCount++

		// 记录风险评分用于后续更新资产
		key := fmt.Sprintf("%s:%d", host, port)
		score := vul.CvssScore * 10 // CVSS 10分制转换为 100分制
		if val, ok := assetRiskMap[key]; !ok || score > val {
			assetRiskMap[key] = score
		}
		assetVulCount[key]++
	}

	// 批量更新资产风险评分
	for key, maxScore := range assetRiskMap {
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			continue
		}
		host := parts[0]
		port, _ := strconv.Atoi(parts[1])

		asset := assetCache.getOrCreate(l.ctx, assetModel, host, port)
		if asset == nil {
			continue
		}

		// Determine Risk Level based on CVSS score (already converted to 100 scale)
		riskLevel := "info"
		if maxScore >= 90 {
			riskLevel = "critical"
		} else if maxScore >= 70 {
			riskLevel = "high"
		} else if maxScore >= 40 {
			riskLevel = "medium"
		} else if maxScore > 0 {
			riskLevel = "low"
		}

		// Update asset with risk score
		update := bson.M{
			"last_scan_time": time.Now(),
			"vul_count":      assetVulCount[key],
		}

		// Update if new score is higher
		if maxScore > asset.RiskScore {
			update["risk_score"] = maxScore
			update["risk_level"] = riskLevel
		}

		// Also update if this is a new vulnerability (increase count but don't decrease score)
		if _, exists := assetCache.assets[key]; exists {
			// Asset was already in cache, update the risk if higher
			if maxScore > asset.RiskScore {
				if err := assetModel.Update(l.ctx, asset.Id.Hex(), update); err != nil {
					l.Logger.Errorf("Failed to update asset risk: %v", err)
				}
			}
		} else {
			// New asset, insert it (already done by getOrCreate)
			l.Logger.Infof("[SaveVulResult] Created new asset for vulnerability: %s:%d, risk_score: %.1f", host, port, maxScore)
		}
	}

	l.Logger.Infof("SaveVulResult: saved %d vulnerabilities, updated %d assets", savedCount, len(assetRiskMap))

	// 打印保存成功的漏洞详情
	for _, pbVul := range in.Vuls {
		vulName := ""
		if pbVul.VulName != nil {
			vulName = *pbVul.VulName
		}
		l.Logger.Infof("[SaveVulResult] Saved vul: host=%s, port=%d, url=%s, pocFile=%s, severity=%s, vulName=%s",
			pbVul.Host, pbVul.Port, pbVul.Url, pbVul.PocFile, pbVul.Severity, vulName)
	}

	return &pb.SaveVulResultResp{
		Success: true,
		Message: "Vulnerabilities saved successfully",
		Total:   savedCount,
	}, nil
}

// parseHostFromUrl 从 URL 解析 host 和 port
func parseHostFromUrl(rawUrl string) (string, int) {
	if rawUrl == "" {
		return "", 0
	}

	// 确保 URL 有协议前缀
	if !strings.Contains(rawUrl, "://") {
		rawUrl = "http://" + rawUrl
	}

	u, err := url.Parse(rawUrl)
	if err != nil {
		return "", 0
	}

	host := u.Hostname()
	port := 80 // 默认 HTTP 端口

	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			port = p
		}
	} else if u.Scheme == "https" {
		port = 443
	}

	return host, port
}
