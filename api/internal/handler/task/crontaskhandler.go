package task

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/model"
	"cscan/pkg/response"
	"cscan/scheduler"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"go.mongodb.org/mongo-driver/bson"
)

// cronParser 包级别Cron解析器（秒级精度），避免重复创建
var cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// CronTaskListReq 定时任务列表请求
type CronTaskListReq struct {
	Page     int    `json:"page,optional"`
	PageSize int    `json:"pageSize,optional"`
	Keyword  string `json:"keyword,optional"`
}

// CronTaskListResp 定时任务列表响应
type CronTaskListResp struct {
	Code int                   `json:"code"`
	Msg  string                `json:"msg"`
	Data *CronTaskListRespData `json:"data"`
}

type CronTaskListRespData struct {
	List  []*CronTaskItem `json:"list"`
	Total int             `json:"total"`
}

type CronTaskItem struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	ScheduleType string `json:"scheduleType"` // cron/once
	CronSpec     string `json:"cronSpec"`
	ScheduleTime string `json:"scheduleTime"`
	WorkspaceId  string `json:"workspaceId"`
	MainTaskId   string `json:"mainTaskId"`
	TaskName     string `json:"taskName"`
	Target       string `json:"target"`
	TargetShort  string `json:"targetShort"` // 截断后的目标（用于列表显示）
	Config       string `json:"config"`      // 任务配置JSON
	Status       string `json:"status"`
	LastRunTime  string `json:"lastRunTime"`
	NextRunTime  string `json:"nextRunTime"`
	RunCount     int64  `json:"runCount"`
}

// CronTaskSaveReq 保存定时任务请求
type CronTaskSaveReq struct {
	Id           string `json:"id,optional"`
	Name         string `json:"name"`
	ScheduleType string `json:"scheduleType"`          // cron: Cron表达式, once: 指定时间
	CronSpec     string `json:"cronSpec,optional"`     // Cron表达式
	ScheduleTime string `json:"scheduleTime,optional"` // 指定执行时间 (格式: 2006-01-02 15:04:05)
	MainTaskId   string `json:"mainTaskId"`            // 关联的任务ID（用于获取初始配置）
	WorkspaceId  string `json:"workspaceId,optional"`  // 任务所属工作空间ID
	Target       string `json:"target,optional"`       // 扫描目标（可自定义，不填则使用关联任务的目标）
	Config       string `json:"config,optional"`       // 任务配置JSON（可自定义，不填则使用关联任务的配置）
}

// CronTaskToggleReq 开关定时任务请求
type CronTaskToggleReq struct {
	Id     string `json:"id"`
	Status string `json:"status"` // enable/disable
}

// CronTaskDeleteReq 删除定时任务请求
type CronTaskDeleteReq struct {
	Id string `json:"id"`
}

// CronTaskRunNowReq 立即执行定时任务请求
type CronTaskRunNowReq struct {
	Id string `json:"id"`
}

// CronTaskBatchDeleteReq 批量删除定时任务请求
type CronTaskBatchDeleteReq struct {
	Ids []string `json:"ids"`
}

// syncCronTaskToRedis 将MongoDB中的定时任务同步到Redis（供调度器读取）
func syncCronTaskToRedis(ctx context.Context, svcCtx *svc.ServiceContext, cronTask *model.CronTask) {
	cronKey := "cscan:cron:tasks"
	schedTask := scheduler.CronTask{
		Id:           cronTask.CronTaskId,
		Name:         cronTask.Name,
		ScheduleType: cronTask.ScheduleType,
		CronSpec:     cronTask.CronSpec,
		ScheduleTime: cronTask.ScheduleTime,
		WorkspaceId:  cronTask.WorkspaceId,
		MainTaskId:   cronTask.MainTaskId,
		TaskName:     cronTask.TaskName,
		Target:       cronTask.Target,
		Config:       cronTask.Config,
		Status:       cronTask.Status,
		LastRunTime:  cronTask.LastRunTime,
		NextRunTime:  cronTask.NextRunTime,
	}
	data, err := json.Marshal(schedTask)
	if err != nil {
		logx.Errorf("[CronTask] failed to marshal cron task for redis sync: cronTaskId=%s, err=%v", cronTask.CronTaskId, err)
		return
	}
	if err := svcCtx.RedisClient.HSet(ctx, cronKey, cronTask.CronTaskId, data).Err(); err != nil {
		logx.Errorf("[CronTask] sync to redis failed: cronTaskId=%s, err=%v", cronTask.CronTaskId, err)
	}
}

// removeCronTaskFromRedis 从Redis中删除定时任务缓存
func removeCronTaskFromRedis(ctx context.Context, svcCtx *svc.ServiceContext, cronTaskId string) {
	cronKey := "cscan:cron:tasks"
	svcCtx.RedisClient.HDel(ctx, cronKey, cronTaskId)
	// 删除运行次数记录
	runCountKey := fmt.Sprintf("cscan:cron:runcount:%s", cronTaskId)
	svcCtx.RedisClient.Del(ctx, runCountKey)
}

// CronTaskListHandler 定时任务列表（从MongoDB读取）
func CronTaskListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskListReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		workspaceId := middleware.GetWorkspaceId(r.Context())
		ctx := r.Context()

		// 从MongoDB读取定时任务（关键字过滤在MongoDB层完成）
		tasks, total, err := svcCtx.CronTaskModel.FindByWorkspaceId(ctx, workspaceId, req.Keyword, req.Page, req.PageSize)
		if err != nil {
			response.Error(w, fmt.Errorf("获取定时任务失败: %v", err))
			return
		}

		var list []*CronTaskItem
		for _, task := range tasks {
			// 从Redis获取运行次数（仅运行次数保留在Redis，属于临时计数器）
			runCountKey := fmt.Sprintf("cscan:cron:runcount:%s", task.CronTaskId)
			runCount, _ := svcCtx.RedisClient.Get(ctx, runCountKey).Int64()

			// 截取目标显示（用于列表）
			targetShort := task.Target
			if len(targetShort) > 100 {
				targetShort = targetShort[:100] + "..."
			}

			list = append(list, &CronTaskItem{
				Id:           task.CronTaskId,
				Name:         task.Name,
				ScheduleType: task.ScheduleType,
				CronSpec:     task.CronSpec,
				ScheduleTime: task.ScheduleTime,
				WorkspaceId:  task.WorkspaceId,
				MainTaskId:   task.MainTaskId,
				TaskName:     task.TaskName,
				Target:       task.Target,
				TargetShort:  targetShort,
				Config:       task.Config,
				Status:       task.Status,
				LastRunTime:  task.LastRunTime,
				NextRunTime:  task.NextRunTime,
				RunCount:     runCount,
			})
		}

		if list == nil {
			list = []*CronTaskItem{}
		}

		httpx.OkJson(w, &CronTaskListResp{
			Code: 0,
			Msg:  "success",
			Data: &CronTaskListRespData{
				List:  list,
				Total: int(total),
			},
		})
	}
}

// CronTaskSaveHandler 保存定时任务（MongoDB主存储 + Redis调度缓存同步）
func CronTaskSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if req.Name == "" {
			response.ParamError(w, "任务名称不能为空")
			return
		}
		if req.MainTaskId == "" {
			response.ParamError(w, "请选择关联的扫描任务")
			return
		}
		if req.ScheduleType == "" {
			req.ScheduleType = "cron"
		}

		var nextRunTime string

		// 验证调度配置
		if req.ScheduleType == "cron" {
			if req.CronSpec == "" {
				response.ParamError(w, "Cron表达式不能为空")
				return
			}
			schedule, err := cronParser.Parse(req.CronSpec)
			if err != nil {
				response.ParamError(w, fmt.Sprintf("无效的Cron表达式: %v", err))
				return
			}
			nextRunTime = schedule.Next(time.Now()).Local().Format("2006-01-02 15:04:05")
		} else if req.ScheduleType == "once" {
			if req.ScheduleTime == "" {
				response.ParamError(w, "请选择执行时间")
				return
			}
			t, err := time.ParseInLocation("2006-01-02 15:04:05", req.ScheduleTime, time.Local)
			if err != nil {
				response.ParamError(w, "时间格式无效，请使用 YYYY-MM-DD HH:mm:ss 格式")
				return
			}
			if t.Before(time.Now()) {
				response.ParamError(w, "执行时间不能早于当前时间")
				return
			}
			nextRunTime = req.ScheduleTime
		} else {
			response.ParamError(w, "无效的调度类型")
			return
		}

		workspaceId := req.WorkspaceId
		if workspaceId == "" || workspaceId == "all" {
			workspaceId = middleware.GetWorkspaceId(r.Context())
		}
		ctx := r.Context()

		// 获取关联任务的信息
		var mainTask *model.MainTask
		var foundWorkspaceId string

		if workspaceId == "" || workspaceId == "all" {
			workspaces, wsErr := svcCtx.WorkspaceModel.FindAll(ctx)
			if wsErr != nil {
				logx.Errorf("[CronTaskSave] failed to list workspaces: %v", wsErr)
			}
			workspaceIds := []string{"default"}
			for _, ws := range workspaces {
				workspaceIds = append(workspaceIds, ws.Id.Hex())
			}
			for _, wsId := range workspaceIds {
				taskModel := svcCtx.GetMainTaskModel(wsId)
				task, err := taskModel.FindByTaskId(ctx, req.MainTaskId)
				if err == nil && task != nil {
					mainTask = task
					foundWorkspaceId = wsId
					break
				}
			}
		} else {
			taskModel := svcCtx.GetMainTaskModel(workspaceId)
			var findErr error
			mainTask, findErr = taskModel.FindByTaskId(ctx, req.MainTaskId)
			if findErr != nil {
				logx.Errorf("[CronTaskSave] failed to find task by taskId=%s in workspace=%s: %v", req.MainTaskId, workspaceId, findErr)
			}
			foundWorkspaceId = workspaceId
		}

		if mainTask == nil {
			response.ParamError(w, "关联的任务不存在")
			return
		}
		workspaceId = foundWorkspaceId

		// 确定使用的目标和配置
		target := req.Target
		if target == "" {
			target = mainTask.Target
		}
		config := req.Config
		if config == "" {
			logx.Infof("[CronTaskSave] config is empty for cron task '%s', falling back to mainTask.Config (taskId=%s)", req.Name, req.MainTaskId)
			config = mainTask.Config
		}

		isNew := req.Id == ""

		if isNew {
			// 新建 - 写入MongoDB
			cronTaskId := uuid.New().String()
			cronTask := &model.CronTask{
				CronTaskId:   cronTaskId,
				Name:         req.Name,
				ScheduleType: req.ScheduleType,
				CronSpec:     req.CronSpec,
				ScheduleTime: req.ScheduleTime,
				WorkspaceId:  workspaceId,
				MainTaskId:   req.MainTaskId,
				TaskName:     mainTask.Name,
				Target:       target,
				Config:       config,
				Status:       "enable",
				NextRunTime:  nextRunTime,
			}
			if err := svcCtx.CronTaskModel.Insert(ctx, cronTask); err != nil {
				response.Error(w, fmt.Errorf("保存定时任务失败: %v", err))
				return
			}
			// 同步到Redis调度缓存
			syncCronTaskToRedis(ctx, svcCtx, cronTask)
			// 通知调度器重新加载
			svcCtx.RedisClient.Publish(ctx, "cscan:cron:reload", cronTaskId)
		} else {
			// 更新 - 从MongoDB获取并更新
			existingTask, err := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
			if err != nil || existingTask == nil {
				response.ParamError(w, "定时任务不存在")
				return
			}

			update := bson.M{
				"name":          req.Name,
				"schedule_type": req.ScheduleType,
				"cron_spec":     req.CronSpec,
				"schedule_time": req.ScheduleTime,
				"workspace_id":  workspaceId,
				"main_task_id":  req.MainTaskId,
				"task_name":     mainTask.Name,
				"target":        target,
				"config":        config,
				"next_run_time": nextRunTime,
				"status":        "enable",
			}
			if err := svcCtx.CronTaskModel.UpdateByCronTaskId(ctx, req.Id, update); err != nil {
				response.Error(w, fmt.Errorf("更新定时任务失败: %v", err))
				return
			}

			// 读取更新后的完整数据同步到Redis
			updatedTask, _ := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
			if updatedTask != nil {
				syncCronTaskToRedis(ctx, svcCtx, updatedTask)
			}

			// 通知调度器重新加载（无论启用/禁用状态，确保内存与MongoDB一致）
			svcCtx.RedisClient.Publish(ctx, "cscan:cron:reload", req.Id)
		}

		response.SuccessWithMsg(w, "保存成功")
	}
}

// CronTaskToggleHandler 开关定时任务
func CronTaskToggleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskToggleReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if req.Id == "" {
			response.ParamError(w, "任务ID不能为空")
			return
		}
		if req.Status != "enable" && req.Status != "disable" {
			response.ParamError(w, "状态值无效")
			return
		}

		ctx := r.Context()

		// 从MongoDB获取现有任务
		task, err := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
		if err != nil || task == nil {
			response.ParamError(w, "任务不存在")
			return
		}

		update := bson.M{"status": req.Status}

		// 如果启用，更新下次运行时间
		if req.Status == "enable" {
			if task.ScheduleType == "cron" {
				if task.CronSpec == "" {
					response.ParamError(w, "Cron表达式为空，请先编辑任务设置Cron表达式")
					return
				}
				schedule, parseErr := cronParser.Parse(task.CronSpec)
				if parseErr != nil {
					response.ParamError(w, fmt.Sprintf("Cron表达式无效，请先编辑任务修正: %v", parseErr))
					return
				}
				nextRun := schedule.Next(time.Now()).Local().Format("2006-01-02 15:04:05")
				update["next_run_time"] = nextRun
			} else if task.ScheduleType == "once" {
				if task.ScheduleTime == "" {
					response.ParamError(w, "执行时间未设置，请先编辑任务设置执行时间")
					return
				}
				t, parseErr := time.ParseInLocation("2006-01-02 15:04:05", task.ScheduleTime, time.Local)
				if parseErr != nil {
					response.ParamError(w, "执行时间格式无效，请先编辑任务重新设置")
					return
				}
				if t.Before(time.Now()) {
					response.ParamError(w, "指定的执行时间已过，请先编辑任务修改执行时间")
					return
				}
				update["next_run_time"] = task.ScheduleTime
			}
		}

		// 更新MongoDB
		if err := svcCtx.CronTaskModel.UpdateByCronTaskId(ctx, req.Id, update); err != nil {
			response.Error(w, fmt.Errorf("更新定时任务失败: %v", err))
			return
		}

		// 读取更新后的完整数据同步到Redis
		updatedTask, _ := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
		if updatedTask != nil {
			syncCronTaskToRedis(ctx, svcCtx, updatedTask)
		}

		// 通知调度器
		svcCtx.RedisClient.Publish(ctx, "cscan:cron:reload", req.Id)

		msg := "已启用"
		if req.Status == "disable" {
			msg = "已禁用"
		}
		response.SuccessWithMsg(w, msg)
	}
}

// CronTaskDeleteHandler 删除定时任务
func CronTaskDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if req.Id == "" {
			response.ParamError(w, "任务ID不能为空")
			return
		}

		ctx := r.Context()

		// 从MongoDB删除
		if err := svcCtx.CronTaskModel.DeleteByCronTaskId(ctx, req.Id); err != nil {
			response.Error(w, fmt.Errorf("删除定时任务失败: %v", err))
			return
		}

		// 从Redis删除调度缓存
		removeCronTaskFromRedis(ctx, svcCtx, req.Id)

		// 通知调度器移除任务
		svcCtx.RedisClient.Publish(ctx, "cscan:cron:remove", req.Id)

		response.SuccessWithMsg(w, "删除成功")
	}
}

// CronTaskBatchDeleteHandler 批量删除定时任务
func CronTaskBatchDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskBatchDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if len(req.Ids) == 0 {
			response.ParamError(w, "请选择要删除的任务")
			return
		}

		ctx := r.Context()

		// 从MongoDB批量删除
		deletedCount, err := svcCtx.CronTaskModel.BatchDeleteByCronTaskIds(ctx, req.Ids)
		if err != nil {
			response.Error(w, fmt.Errorf("批量删除定时任务失败: %v", err))
			return
		}

		// 从Redis删除调度缓存并通知调度器
		for _, id := range req.Ids {
			removeCronTaskFromRedis(ctx, svcCtx, id)
			svcCtx.RedisClient.Publish(ctx, "cscan:cron:remove", id)
		}

		response.SuccessWithMsg(w, fmt.Sprintf("成功删除 %d 个定时任务", deletedCount))
	}
}

// CronTaskRunNowHandler 立即执行定时任务
func CronTaskRunNowHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CronTaskRunNowReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		if req.Id == "" {
			response.ParamError(w, "任务ID不能为空")
			return
		}

		ctx := r.Context()

		// 先确保Redis缓存是最新的（从MongoDB同步）
		cronTask, err := svcCtx.CronTaskModel.FindByCronTaskId(ctx, req.Id)
		if err != nil || cronTask == nil {
			response.Error(w, fmt.Errorf("定时任务不存在"))
			return
		}
		syncCronTaskToRedis(ctx, svcCtx, cronTask)

		// 通知调度器立即执行
		svcCtx.RedisClient.Publish(ctx, "cscan:cron:runnow", req.Id)

		response.SuccessWithMsg(w, "已触发执行")
	}
}

// ValidateCronSpecHandler 验证Cron表达式
func ValidateCronSpecHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CronSpec string `json:"cronSpec"`
		}
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		schedule, err := cronParser.Parse(req.CronSpec)
		if err != nil {
			httpx.OkJson(w, map[string]any{
				"code": 1,
				"msg":  fmt.Sprintf("无效的Cron表达式: %v", err),
				"data": nil,
			})
			return
		}

		var nextTimes []string
		t := time.Now()
		for i := 0; i < 5; i++ {
			t = schedule.Next(t)
			nextTimes = append(nextTimes, t.Local().Format("2006-01-02 15:04:05"))
		}

		httpx.OkJson(w, map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{
				"valid":     true,
				"nextTimes": nextTimes,
			},
		})
	}
}
