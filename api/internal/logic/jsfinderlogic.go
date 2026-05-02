package logic

import (
	"context"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"cscan/api/internal/logic/common"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/model"
	"cscan/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
)

// JSFinderConfigLogic JSFinder 配置逻辑
type JSFinderConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewJSFinderConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JSFinderConfigLogic {
	return &JSFinderConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Get 获取 JSFinder 配置（不存在则返回内置默认值）
func (l *JSFinderConfigLogic) Get() (*types.JSFinderConfigResp, error) {
	m := model.NewJSFinderConfigModel(l.svcCtx.MongoDB)
	doc, err := m.Get(l.ctx)
	if err != nil {
		l.Errorf("[JSFinderConfig] Get error: %v", err)
		return &types.JSFinderConfigResp{Code: 500, Msg: "获取JSFinder配置失败"}, nil
	}

	updateTime := ""
	if !doc.UpdateTime.IsZero() {
		updateTime = doc.UpdateTime.Format("2006-01-02 15:04:05")
	}

	return &types.JSFinderConfigResp{
		Code: 0,
		Msg:  "success",
		Data: &types.JSFinderConfig{
			HighRiskRoutes:       doc.HighRiskRoutes,
			AuthRequiredKeywords: doc.AuthRequiredKeywords,
			SensitiveKeywords:    doc.SensitiveKeywords,
			DomainBlacklist:      doc.DomainBlacklist,
			UpdateTime:           updateTime,
		},
	}, nil
}

// Save 保存 JSFinder 配置
func (l *JSFinderConfigLogic) Save(req *types.JSFinderConfigSaveReq) (*types.JSFinderConfigResp, error) {
	m := model.NewJSFinderConfigModel(l.svcCtx.MongoDB)

	doc := &model.JSFinderConfig{
		HighRiskRoutes:       sanitizeJSFinderList(req.HighRiskRoutes),
		AuthRequiredKeywords: sanitizeJSFinderList(req.AuthRequiredKeywords),
		SensitiveKeywords:    sanitizeJSFinderList(req.SensitiveKeywords),
		DomainBlacklist:      sanitizeJSFinderList(req.DomainBlacklist),
		UpdateTime:           time.Now(),
	}

	if err := m.Save(l.ctx, doc); err != nil {
		l.Errorf("[JSFinderConfig] Save error: %v", err)
		return &types.JSFinderConfigResp{Code: 500, Msg: "保存JSFinder配置失败"}, nil
	}

	return &types.JSFinderConfigResp{
		Code: 0,
		Msg:  "保存成功",
		Data: &types.JSFinderConfig{
			HighRiskRoutes:       doc.HighRiskRoutes,
			AuthRequiredKeywords: doc.AuthRequiredKeywords,
			SensitiveKeywords:    doc.SensitiveKeywords,
			DomainBlacklist:      doc.DomainBlacklist,
			UpdateTime:           doc.UpdateTime.Format("2006-01-02 15:04:05"),
		},
	}, nil
}

// Reset 重置为内置默认值
func (l *JSFinderConfigLogic) Reset() (*types.JSFinderConfigResp, error) {
	m := model.NewJSFinderConfigModel(l.svcCtx.MongoDB)

	def := model.NewDefaultJSFinderConfig()
	if err := m.Save(l.ctx, def); err != nil {
		l.Errorf("[JSFinderConfig] Reset error: %v", err)
		return &types.JSFinderConfigResp{Code: 500, Msg: "重置JSFinder配置失败"}, nil
	}

	return &types.JSFinderConfigResp{
		Code: 0,
		Msg:  "重置成功",
		Data: &types.JSFinderConfig{
			HighRiskRoutes:       def.HighRiskRoutes,
			AuthRequiredKeywords: def.AuthRequiredKeywords,
			SensitiveKeywords:    def.SensitiveKeywords,
			DomainBlacklist:      def.DomainBlacklist,
			UpdateTime:           def.UpdateTime.Format("2006-01-02 15:04:05"),
		},
	}, nil
}

// sanitizeJSFinderList 去除空字符串与首尾空格，保留顺序与重复（用户自管去重）
func sanitizeJSFinderList(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

type JSFinderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewJSFinderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JSFinderLogic {
	return &JSFinderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SaveJSFinderResult 保存 JSFinder 扫描结果
func (l *JSFinderLogic) SaveJSFinderResult(req *types.SaveJSFinderResultReq) error {
	if req.WorkspaceId == "" {
		return xerr.NewParamError("workspaceId cannot be empty")
	}

	if len(req.Results) == 0 {
		return nil
	}

	modelResults := make([]*model.JSFinderResult, 0, len(req.Results))
	for _, r := range req.Results {
		modelResults = append(modelResults, &model.JSFinderResult{
			WorkspaceId:      req.WorkspaceId,
			MainTaskId:       req.MainTaskId,
			Authority:        r.Authority,
			Host:             r.Host,
			Port:             r.Port,
			URL:              r.URL,
			Severity:         r.Severity,
			VulName:          r.VulName,
			Result:           r.Result,
			Tags:             r.Tags,
			MatcherName:      r.MatcherName,
			ExtractedResults: r.ExtractedResults,
			CurlCommand:      r.CurlCommand,
			Request:          r.Request,
			Response:         r.Response,
		})
	}

	m := l.svcCtx.GetJSFinderResultModel(req.WorkspaceId)
	// 确保索引存在
	_ = m.EnsureIndexes(l.ctx)

	if err := m.InsertMany(l.ctx, modelResults); err != nil {
		l.Logger.Errorf("SaveJSFinderResult Error: %v", err)
		// InsertMany可能会因为唯一索引冲突而报错，在这里忽略 Duplicate Key Error，保证其余正常插入
		// 这里只是打出错误日志，由于 MongoDB 的 InsertMany Ordered: false，出错条目会被跳过
	}

	return nil
}

// GetJSFinderList 获取 JSFinder 结果列表
func (l *JSFinderLogic) GetJSFinderList(req *types.JSFinderListReq) (*types.JSFinderListResp, error) {
	workspaceId := req.WorkspaceId

	filter := bson.M{}

	if req.Query != "" {
		filter["$or"] = []bson.M{
			{"url": primitive.Regex{Pattern: req.Query, Options: "i"}},
			{"vul_name": primitive.Regex{Pattern: req.Query, Options: "i"}},
			{"host": primitive.Regex{Pattern: req.Query, Options: "i"}},
		}
	}

	if req.Severity != "" {
		filter["severity"] = req.Severity
	}

	if req.Tags != "" {
		filter["tags"] = req.Tags
	}

	if req.MatcherName != "" {
		filter["matcher_name"] = req.MatcherName
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 10
	}

	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	var total int64
	var allResults []*model.JSFinderResult

	// 支持多工作空间查询
	if len(wsIds) > 1 || workspaceId == "" || workspaceId == "all" {
		for _, wsId := range wsIds {
			m := l.svcCtx.GetJSFinderResultModel(wsId)
			wsTotal, _ := m.Count(l.ctx, filter)
			total += wsTotal

			wsResults, _ := m.Find(l.ctx, filter, nil)
			allResults = append(allResults, wsResults...)
		}

		// 按创建时间排序
		sort.Slice(allResults, func(i, j int) bool {
			return allResults[i].CreateTime.After(allResults[j].CreateTime)
		})

		// 内存分页
		start := (req.Page - 1) * req.PageSize
		end := start + req.PageSize
		if start > len(allResults) {
			start = len(allResults)
		}
		if end > len(allResults) {
			end = len(allResults)
		}
		allResults = allResults[start:end]
	} else {
		m := l.svcCtx.GetJSFinderResultModel(workspaceId)

		var err error
		total, err = m.Count(l.ctx, filter)
		if err != nil {
			return nil, xerr.NewServerError("Count JSFinderResult Error: " + err.Error())
		}

		opt := options.Find().
			SetSkip(int64((req.Page - 1) * req.PageSize)).
			SetLimit(int64(req.PageSize)).
			SetSort(bson.D{{Key: "create_time", Value: -1}})

		allResults, err = m.Find(l.ctx, filter, opt)
		if err != nil {
			return nil, xerr.NewServerError("Find JSFinderResult Error: " + err.Error())
		}
	}

	respList := make([]*types.JSFinderResult, 0, len(allResults))
	for _, r := range allResults {
		respList = append(respList, &types.JSFinderResult{
			Id:               r.Id.Hex(),
			WorkspaceId:      r.WorkspaceId,
			MainTaskId:       r.MainTaskId,
			TaskName:         r.TaskName,
			Authority:        r.Authority,
			Host:             r.Host,
			Port:             r.Port,
			URL:              r.URL,
			Severity:         r.Severity,
			VulName:          r.VulName,
			Result:           r.Result,
			Tags:             r.Tags,
			MatcherName:      r.MatcherName,
			ExtractedResults: r.ExtractedResults,
			CurlCommand:      r.CurlCommand,
			Request:          r.Request,
			Response:         r.Response,
			CreateTime:       r.CreateTime.Format("2006-01-02 15:04:05"),
			UpdateTime:       r.UpdateTime.Format("2006-01-02 15:04:05"),
		})
	}

	return &types.JSFinderListResp{
		Code:  0,
		Msg:   "success",
		Total: total,
		List:  respList,
	}, nil
}

// ClearJSFinderResults 清空 JSFinder 结果
func (l *JSFinderLogic) ClearJSFinderResults(workspaceId string) error {
	wsIds := common.GetWorkspaceIds(l.ctx, l.svcCtx, workspaceId)

	for _, wsId := range wsIds {
		m := l.svcCtx.GetJSFinderResultModel(wsId)
		_, err := m.DeleteMany(l.ctx, bson.M{})
		if err != nil {
			l.Logger.Errorf("ClearJSFinderResults Error for workspace %s: %v", wsId, err)
			return xerr.NewServerError("清空JSFinder结果失败: " + err.Error())
		}
	}

	return nil
}
