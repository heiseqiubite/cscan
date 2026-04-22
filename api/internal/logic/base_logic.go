package logic

import (
	"context"
	"fmt"
	"sync"

	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BaseLogic 基础逻辑层，提供通用功能
type BaseLogic struct {
	logx.Logger
	Ctx    context.Context
	SvcCtx *svc.ServiceContext
}

// NewBaseLogic 创建基础逻辑
func NewBaseLogic(ctx context.Context, svcCtx *svc.ServiceContext) BaseLogic {
	return BaseLogic{
		Logger: logx.WithContext(ctx),
		Ctx:    ctx,
		SvcCtx: svcCtx,
	}
}

// PageRequest 通用分页请求
type PageRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Sort     string `json:"sort,omitempty"`
	Order    string `json:"order,omitempty"` // asc or desc
}

// PageResponse 通用分页响应
type PageResponse struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	Pages    int         `json:"pages"`
	Data     interface{} `json:"data"`
}

// BuildFindOptions 构建通用查询选项
func (l *BaseLogic) BuildFindOptions(req PageRequest) *options.FindOptions {
	opts := options.Find()

	// 分页
	if req.Page > 0 && req.PageSize > 0 {
		opts.SetSkip(int64((req.Page - 1) * req.PageSize))
		opts.SetLimit(int64(req.PageSize))
	}

	// 排序
	if req.Sort != "" {
		order := -1
		if req.Order == "asc" {
			order = 1
		}
		opts.SetSort(bson.D{{Key: req.Sort, Value: order}})
	}

	return opts
}

// BuildPageResponse 构建分页响应
func (l *BaseLogic) BuildPageResponse(req PageRequest, total int64, data interface{}) *PageResponse {
	pages := int(total) / req.PageSize
	if int(total)%req.PageSize > 0 {
		pages++
	}

	return &PageResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Pages:    pages,
		Data:     data,
	}
}

// GetUserId 获取当前用户ID
func (l *BaseLogic) GetUserId() string {
	return middleware.GetUserId(l.Ctx)
}

// GetUsername 获取当前用户名
func (l *BaseLogic) GetUsername() string {
	return middleware.GetUsername(l.Ctx)
}

// GetWorkspaceId 获取当前工作空间ID
func (l *BaseLogic) GetWorkspaceId() string {
	return middleware.GetWorkspaceId(l.Ctx)
}

// GetWorkspaceIds 获取要查询的工作空间列表
func (l *BaseLogic) GetWorkspaceIds(workspaceId string) []string {
	if workspaceId == "" || workspaceId == "all" {
		// 获取用户有权限的所有工作空间
		return l.getUserWorkspaces()
	}
	return []string{workspaceId}
}

func (l *BaseLogic) getUserWorkspaces() []string {
	userId := l.GetUserId()
	if userId == "" {
		return []string{"default"}
	}

	// 从缓存获取
	cacheKey := fmt.Sprintf("user_workspaces:%s", userId)
	if l.SvcCtx.RedisClient != nil {
		if cached, err := l.SvcCtx.RedisClient.SMembers(l.Ctx, cacheKey).Result(); err == nil && len(cached) > 0 {
			return cached
		}
	}

	// 查询所有工作空间（简化实现，实际应根据用户权限过滤）
	workspaces, err := l.SvcCtx.WorkspaceModel.Find(l.Ctx, bson.M{"status": "enable"}, 0, 0)
	if err != nil {
		l.Logger.Errorf("Failed to get user workspaces: %v", err)
		return []string{"default"}
	}

	wsIds := make([]string, 0, len(workspaces))
	for _, ws := range workspaces {
		wsIds = append(wsIds, ws.Id.Hex())
	}

	if len(wsIds) == 0 {
		return []string{"default"}
	}

	// 缓存到Redis（5分钟过期）
	if l.SvcCtx.RedisClient != nil && len(wsIds) > 0 {
		l.SvcCtx.RedisClient.SAdd(l.Ctx, cacheKey, wsIds)
		l.SvcCtx.RedisClient.Expire(l.Ctx, cacheKey, 5*60)
	}

	return wsIds
}

// AggregateMultiWorkspace 跨工作空间聚合查询
func (l *BaseLogic) AggregateMultiWorkspace(
	workspaceId string,
	queryFn func(wsId string) (interface{}, error),
) ([]interface{}, error) {
	wsIds := l.GetWorkspaceIds(workspaceId)

	var (
		results = make([]interface{}, 0)
		mu      sync.Mutex
		wg      sync.WaitGroup
		errCh   = make(chan error, len(wsIds))
	)

	for _, wsId := range wsIds {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			result, err := queryFn(id)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(wsId)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// CountMultiWorkspace 跨工作空间统计
func (l *BaseLogic) CountMultiWorkspace(
	workspaceId string,
	countFn func(wsId string) (int64, error),
) (int64, error) {
	wsIds := l.GetWorkspaceIds(workspaceId)

	var (
		total int64
		mu    sync.Mutex
		wg    sync.WaitGroup
		errCh = make(chan error, len(wsIds))
	)

	for _, wsId := range wsIds {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			count, err := countFn(id)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			total += count
			mu.Unlock()
		}(wsId)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return 0, err
		}
	}

	return total, nil
}

// ValidatePagination 验证分页参数
func (l *BaseLogic) ValidatePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// LogOperation 记录操作日志
func (l *BaseLogic) LogOperation(operation string, details map[string]interface{}) {
	l.Logger.Infow(operation,
		logx.Field("userId", l.GetUserId()),
		logx.Field("username", l.GetUsername()),
		logx.Field("workspaceId", l.GetWorkspaceId()),
		logx.Field("details", details),
	)
}
