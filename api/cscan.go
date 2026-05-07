package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cscan/api/internal/config"
	"cscan/api/internal/handler"
	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/model"
	"cscan/scheduler"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"go.mongodb.org/mongo-driver/bson"
)

var configFile = flag.String("f", "etc/cscan.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config

	conf.MustLoad(*configFile, &c)

	// 从环境变量加载 JWT secret（优先级高于配置文件）
	c.LoadSecretFromEnv()
	if c.Auth.AccessSecret == "" {
		logx.Error("JWT secret not configured. Set CSCAN_JWT_SECRET environment variable.")
		c.Auth.AccessSecret = uuid.New().String()
		logx.Error("Using auto-generated JWT secret (NOT suitable for production)")
	}

	logx.MustSetup(c.Log)
	logx.DisableStat()

	fmt.Println(`
   ______ _____  ______          _   _ 
  / ____/ ____|/ __ \ \        / / | \ | |
 | |   | (___ | |  | \ \  /\  / /|  \| |
 | |    \___ \| |  | |\ \/  \/ / | .  |
 | |________) | |__| | \  /\  /  | |\  |
  \_____|_____/ \____/   \/  \/   |_| \_| 
                                         `)
	fmt.Println("---------------------------------------------------------")
	logx.Info("CScan API Service Starting")
	logx.Infof("Config loaded from: %s", *configFile)
	fmt.Println("---------------------------------------------------------")
	// 创建服务上下文
	svcCtx, err := svc.NewServiceContext(c)
	if err != nil {
		logx.Errorf("Failed to initialize service: %v", err)
		return
	}

	// 创建HTTP服务器
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	handler.RegisterHandlers(server, svcCtx)

	// 创建任务调度器服务（复用svcCtx中已有的Scheduler实例，避免创建重复实例）
	schedulerSvc := scheduler.NewSchedulerService(svcCtx.Scheduler, svcCtx.RedisClient, svcCtx.SyncMethods, &cronTaskSourceAdapter{model: svcCtx.CronTaskModel})
	go schedulerSvc.Start()

	// 启动定时任务执行消息订阅
	go startCronExecuteSubscriber(svcCtx, schedulerSvc.GetScheduler())

	// 启动孤儿任务恢复后台任务（每 5 分钟检查一次）
	go startOrphanedTaskRecovery(svcCtx)

	logx.Infof("CScan API is running at: %s:%d", c.Host, c.Port)
	logx.Infof("Environment: %s | LogLevel: %s", c.Mode, c.Log.Level)
	server.Start()
}

// CronExecuteMessage 定时任务执行消息
type CronExecuteMessage struct {
	CronTaskId  string `json:"cronTaskId"`
	WorkspaceId string `json:"workspaceId"`
	MainTaskId  string `json:"mainTaskId"`
	TaskName    string `json:"taskName"`
	Target      string `json:"target"`
	Config      string `json:"config"`
}

// startCronExecuteSubscriber 启动定时任务执行消息订阅（含自动重连）
func startCronExecuteSubscriber(svcCtx *svc.ServiceContext, sched *scheduler.Scheduler) {
	ctx := context.Background()
	retryDelay := 5 * time.Second
	maxRetryDelay := 60 * time.Second

	for {
		pubsub := svcCtx.RedisClient.Subscribe(ctx, "cscan:cron:execute")

		logx.Info("Cron execute subscriber started")

		ch := pubsub.Channel()
		for msg := range ch {
			// 成功接收消息说明连接正常，重置退避延迟
			retryDelay = 5 * time.Second

			var execMsg CronExecuteMessage
			if err := json.Unmarshal([]byte(msg.Payload), &execMsg); err != nil {
				logx.Errorf("Failed to parse cron execute message: %v", err)
				continue
			}

			logx.Infof("Received cron execute message: cronTaskId=%s, taskName=%s", execMsg.CronTaskId, execMsg.TaskName)

			// 创建新的 MainTask 并推送到队列
			if err := createAndPushCronTask(ctx, svcCtx, sched, &execMsg); err != nil {
				logx.Errorf("Failed to create cron task: %v", err)
			}
		}
		pubsub.Close()
		logx.Errorf("[CronExecuteSubscriber] Redis subscription disconnected, reconnecting in %v...", retryDelay)
		time.Sleep(retryDelay)
		// 指数退避，最大60秒
		if retryDelay < maxRetryDelay {
			retryDelay = retryDelay * 2
			if retryDelay > maxRetryDelay {
				retryDelay = maxRetryDelay
			}
		}
	}
}

// createAndPushCronTask 创建定时任务的 MainTask 并推送到队列
func createAndPushCronTask(ctx context.Context, svcCtx *svc.ServiceContext, sched *scheduler.Scheduler, msg *CronExecuteMessage) error {
	workspaceId := msg.WorkspaceId
	if workspaceId == "" {
		workspaceId = "default"
	}

	// 解析任务配置
	var taskConfig map[string]interface{}
	if err := json.Unmarshal([]byte(msg.Config), &taskConfig); err != nil {
		return fmt.Errorf("failed to parse task config: %v", err)
	}

	// 生成新的任务ID
	newTaskId := uuid.New().String()

	// 创建新的 MainTask
	taskModel := svcCtx.GetMainTaskModel(workspaceId)
	newTask := &model.MainTask{
		TaskId:   newTaskId,
		Name:     fmt.Sprintf("%s (定时)", msg.TaskName),
		Target:   msg.Target,
		Config:   msg.Config,
		Status:   model.TaskStatusCreated,
		IsCron:   true,
		CronRule: msg.CronTaskId,
	}

	if err := taskModel.Insert(ctx, newTask); err != nil {
		return fmt.Errorf("failed to insert main task: %v", err)
	}

	logx.Infof("Created cron main task: taskId=%s, name=%s", newTaskId, newTask.Name)

	// 计算子任务数量（基于目标数量和启用的模块数）
	targets := strings.Split(msg.Target, "\n")
	var validTargets []string
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t != "" {
			validTargets = append(validTargets, t)
		}
	}

	// 计算启用的模块数（与 worker/worker.go 中的 enabledPhases 计算保持一致）
	// 关键规则：portscan 默认启用（enable != false），其他模块需要 enable == true
	enabledModules := 0

	// DomainScan
	if ds, ok := taskConfig["domainscan"].(map[string]interface{}); ok {
		if enable, _ := ds["enable"].(bool); enable {
			enabledModules++
		}
	}

	// PortScan (default enabled if missing or nil, same as task_builder.go logic)
	if ps, ok := taskConfig["portscan"].(map[string]interface{}); ok {
		if enable, ok := ps["enable"].(bool); ok && enable {
			enabledModules++
		}
		// Note: if portscan exists but enable is not true (or is false), don't count
		// This matches task_builder.go logic
	} else {
		// portscan is nil or doesn't exist, count as enabled (default behavior)
		enabledModules++
	}

	// Fingerprint
	if fp, ok := taskConfig["fingerprint"].(map[string]interface{}); ok {
		if enable, _ := fp["enable"].(bool); enable {
			enabledModules++
		}
	}
	// PocScan
	if poc, ok := taskConfig["pocscan"].(map[string]interface{}); ok {
		if enable, _ := poc["enable"].(bool); enable {
			enabledModules++
		}
	}
	// DirScan
	if dir, ok := taskConfig["dirscan"].(map[string]interface{}); ok {
		if enable, _ := dir["enable"].(bool); enable {
			enabledModules++
		}
	}
	// PortIdentify (missing from original code, but should be included)
	if pi, ok := taskConfig["portidentify"].(map[string]interface{}); ok {
		if enable, _ := pi["enable"].(bool); enable {
			enabledModules++
		}
	}
	// BruteScan
	if bs, ok := taskConfig["brutescan"].(map[string]interface{}); ok {
		if enable, _ := bs["enable"].(bool); enable {
			enabledModules++
		}
	}
	// JSFinder
	if js, ok := taskConfig["jsfinder"].(map[string]interface{}); ok {
		if enable, _ := js["enable"].(bool); enable {
			enabledModules++
		}
	}

	if enabledModules == 0 {
		enabledModules = 1
	}

	// 自动计算最佳批次大小
	// 子任务总数控制在 10~30 范围内，避免碎片化或过度聚合
	const (
		minSubTasks = 10
		maxSubTasks = 30
		minBatch    = 20
		maxBatch    = 200
	)
	targetCount := len(validTargets)
	optimalBatches := (minSubTasks + maxSubTasks) / 2 / enabledModules
	if optimalBatches < 1 {
		optimalBatches = 1
	}
	batchSize := targetCount / optimalBatches
	if batchSize < 1 {
		batchSize = 1
	}
	if batchSize < minBatch {
		batchSize = minBatch
	}
	if batchSize > maxBatch {
		batchSize = maxBatch
	}
	if targetCount <= minBatch {
		batchSize = targetCount
	}
	// 如果用户显式设置了 batchSize > 0，优先使用
	if bs, ok := taskConfig["batchSize"].(float64); ok && bs > 0 {
		batchSize = int(bs)
	}
	logx.Infof("Cron task %s: auto-calculated batchSize=%d (targets=%d, modules=%d)", newTaskId, batchSize, targetCount, enabledModules)

	var batches []string
	for i := 0; i < len(validTargets); i += batchSize {
		end := i + batchSize
		if end > len(validTargets) {
			end = len(validTargets)
		}
		batches = append(batches, strings.Join(validTargets[i:end], "\n"))
	}
	if len(batches) == 0 {
		batches = []string{msg.Target}
	}

	// 子任务计数器在 worker 端每完成一个子任务（一个 batch 的全部阶段）才递增一次，
	// 因此 subTaskCount 必须等于 batches 数量，否则任务永远卡在 progress<100% 而不会被标记为 SUCCESS。
	subTaskCount := len(batches)

	// 更新任务状态为 STARTED
	now := time.Now()
	taskModel.Update(ctx, newTask.Id.Hex(), map[string]interface{}{
		"status":         model.TaskStatusPending,
		"sub_task_count": subTaskCount,
		"sub_task_done":  0,
		"start_time":     now,
	})

	// 保存主任务信息到 Redis
	taskInfoKey := "cscan:task:info:" + newTaskId
	taskInfoData, err := json.Marshal(map[string]interface{}{
		"workspaceId":    workspaceId,
		"mainTaskId":     newTask.Id.Hex(),
		"subTaskCount":   subTaskCount,
		"batchCount":     len(batches),
		"enabledModules": enabledModules,
	})
	if err != nil {
		logx.Errorf("Failed to marshal task info for redis: %v", err)
	} else {
		svcCtx.RedisClient.Set(ctx, taskInfoKey, taskInfoData, 24*time.Hour)
	}

	// 从配置中获取指定的 Worker 列表
	var workers []string
	if w, ok := taskConfig["workers"].([]interface{}); ok {
		for _, v := range w {
			if s, ok := v.(string); ok {
				workers = append(workers, s)
			}
		}
	}

	// 为每个批次创建子任务并推送到队列
	for i, batch := range batches {
		// 复制配置并替换目标
		subConfig := make(map[string]interface{})
		for k, v := range taskConfig {
			subConfig[k] = v
		}
		subConfig["target"] = batch
		subConfig["subTaskIndex"] = i
		subConfig["subTaskTotal"] = len(batches)
		subConfigBytes, err := json.Marshal(subConfig)
		if err != nil {
			logx.Errorf("Failed to marshal sub config: %v", err)
			continue
		}

		// 生成子任务ID
		subTaskId := newTaskId
		if len(batches) > 1 {
			subTaskId = newTaskId + "-" + strconv.Itoa(i)
		}

		schedTask := &scheduler.TaskInfo{
			TaskId:      subTaskId,
			MainTaskId:  newTask.Id.Hex(),
			WorkspaceId: workspaceId,
			TaskName:    newTask.Name,
			Config:      string(subConfigBytes),
			Priority:    0,
			Workers:     workers,
		}

		logx.Infof("Pushing cron sub-task %d/%d: taskId=%s, targets=%d", i+1, len(batches), subTaskId, len(strings.Split(batch, "\n")))

		if err := sched.PushTask(ctx, schedTask); err != nil {
			logx.Errorf("Failed to push cron task to queue: %v", err)
			continue
		}

		// 保存子任务信息到 Redis（多批次时）
		if len(batches) > 1 {
			subTaskInfoKey := "cscan:task:info:" + subTaskId
			subTaskInfoData, err := json.Marshal(map[string]interface{}{
				"workspaceId":  workspaceId,
				"mainTaskId":   newTask.Id.Hex(),
				"subTaskCount": subTaskCount,
			})
			if err != nil {
				logx.Errorf("Failed to marshal sub task info for redis: %v", err)
			} else {
				svcCtx.RedisClient.Set(ctx, subTaskInfoKey, subTaskInfoData, 24*time.Hour)
			}
		}
	}

	logx.Infof("Cron task created and pushed: taskId=%s, batches=%d, subTaskCount=%d", newTaskId, len(batches), subTaskCount)
	return nil
}

const (
	orphanedTaskCheckInterval = 5 * time.Minute
	orphanedTaskThreshold     = 30 * time.Minute
)

// startOrphanedTaskRecovery 启动孤儿任务恢复后台任务
// 定期检查并恢复卡住的任务（状态为 STARTED 但长时间没有更新的任务）
func startOrphanedTaskRecovery(svcCtx *svc.ServiceContext) {
	logx.Info("Orphaned task recovery background job started")

	ticker := time.NewTicker(orphanedTaskCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		logic.RecoverOrphanedTasks(context.Background(), svcCtx, orphanedTaskThreshold)
		logic.CleanupStaleProcessingTasks(context.Background(), svcCtx, "")
	}
}

// cronTaskSourceAdapter 适配器：将model.CronTaskModel适配为scheduler.CronTaskSource接口
type cronTaskSourceAdapter struct {
	model *model.CronTaskModel
}

func (a *cronTaskSourceAdapter) FindAllCronTasks(ctx context.Context) ([]scheduler.CronTaskData, error) {
	tasks, err := a.model.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]scheduler.CronTaskData, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, scheduler.CronTaskData{
			CronTaskId:   t.CronTaskId,
			Name:         t.Name,
			ScheduleType: t.ScheduleType,
			CronSpec:     t.CronSpec,
			ScheduleTime: t.ScheduleTime,
			WorkspaceId:  t.WorkspaceId,
			MainTaskId:   t.MainTaskId,
			TaskName:     t.TaskName,
			Target:       t.Target,
			Config:       t.Config,
			Status:       t.Status,
			LastRunTime:  t.LastRunTime,
			NextRunTime:  t.NextRunTime,
		})
	}
	return result, nil
}

func (a *cronTaskSourceAdapter) FindCronTaskByCronTaskId(ctx context.Context, cronTaskId string) (*scheduler.CronTaskData, error) {
	t, err := a.model.FindByCronTaskId(ctx, cronTaskId)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, nil
	}
	return &scheduler.CronTaskData{
		CronTaskId:   t.CronTaskId,
		Name:         t.Name,
		ScheduleType: t.ScheduleType,
		CronSpec:     t.CronSpec,
		ScheduleTime: t.ScheduleTime,
		WorkspaceId:  t.WorkspaceId,
		MainTaskId:   t.MainTaskId,
		TaskName:     t.TaskName,
		Target:       t.Target,
		Config:       t.Config,
		Status:       t.Status,
		LastRunTime:  t.LastRunTime,
		NextRunTime:  t.NextRunTime,
	}, nil
}

func (a *cronTaskSourceAdapter) UpdateCronTaskRunInfo(ctx context.Context, cronTaskId string, lastRunTime, nextRunTime, status string) error {
	update := bson.M{
		"last_run_time": lastRunTime,
		"next_run_time": nextRunTime,
		"status":        status,
	}
	return a.model.UpdateByCronTaskId(ctx, cronTaskId, update)
}
