package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chainreactors/fingers/common"
	"github.com/chainreactors/parsers"
	"github.com/chainreactors/sdk/fingers"
	"github.com/chainreactors/sdk/gogo"
	"github.com/chainreactors/sdk/neutron"
	"github.com/zeromicro/go-zero/core/logx"

	"cscan/pkg/utils"
)

// GogoOptions Gogo扫描选项
type GogoOptions struct {
	Enable       bool   `json:"enable"`        // 是否启用
	Ports        string `json:"ports"`         // 端口列表 "80,443,8080" 或 "top100"
	Threads      int    `json:"threads"`       // 并发线程数
	Timeout      int    `json:"timeout"`       // HTTP超时(秒)
	VersionLevel int    `json:"versionLevel"`  // 指纹识别深度 0-2
	Exploit      string `json:"exploit"`       // 漏洞模式: "none", "auto"
}

// Validate 验证 GogoOptions 配置是否有效
func (o *GogoOptions) Validate() error {
	if o.Threads < 0 {
		return fmt.Errorf("threads must be non-negative, got %d", o.Threads)
	}
	if o.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %d", o.Timeout)
	}
	if o.VersionLevel < 0 || o.VersionLevel > 2 {
		return fmt.Errorf("versionLevel must be 0-2, got %d", o.VersionLevel)
	}
	if o.Exploit != "" && o.Exploit != "none" && o.Exploit != "auto" {
		return fmt.Errorf("exploit must be 'none' or 'auto', got %s", o.Exploit)
	}
	return nil
}

// GogoScanner Gogo扫描器
type GogoScanner struct {
	BaseScanner
	engine    *gogo.GogoEngine
	initMu    sync.Mutex
	cyberhubURL string
	cyberhubKey string
}

// NewGogoScanner 创建Gogo扫描器
func NewGogoScanner() *GogoScanner {
	return &GogoScanner{
		BaseScanner: BaseScanner{name: "gogo"},
	}
}

// SetCyberhubConfig 设置 Cyberhub 配置
func (s *GogoScanner) SetCyberhubConfig(url, key string) {
	s.initMu.Lock()
	s.cyberhubURL = url
	s.cyberhubKey = key
	s.initMu.Unlock()
}

// ensureInit 初始化 engine（如果尚未初始化）
func (s *GogoScanner) ensureInit() error {
	s.initMu.Lock()
	defer s.initMu.Unlock()

	// 已经初始化
	if s.engine != nil {
		return nil
	}

	// Cyberhub 配置（从 API 获取或通过配置文件设置）
	cyberhubURL := s.cyberhubURL
	cyberhubKey := s.cyberhubKey

	// 加载指纹库
	fingersConfig := fingers.NewConfig()
	fingersConfig.WithCyberhub(cyberhubURL, cyberhubKey)
	fingersConfig.SetTimeout(60 * time.Second)
	fingersEngine, err := fingers.NewEngine(fingersConfig)
	if err != nil {
		return fmt.Errorf("fingers engine init failed: %w", err)
	}

	// 加载 POC
	neutronConfig := neutron.NewConfig()
	neutronConfig.WithCyberhub(cyberhubURL, cyberhubKey)
	neutronConfig.SetTimeout(60 * time.Second)
	neutronEngine, err := neutron.NewEngine(neutronConfig)
	if err != nil {
		return fmt.Errorf("neutron engine init failed: %w", err)
	}

	// 创建集成扫描器
	gogoConfig := gogo.NewConfig().
		WithFingersEngine(fingersEngine).
		WithNeutronEngine(neutronEngine)
	engine := gogo.NewEngine(gogoConfig)
	if err := engine.Init(); err != nil {
		return fmt.Errorf("gogo init failed: %w", err)
	}

	s.engine = engine
	return nil
}

// Scan 执行Gogo扫描
func (s *GogoScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	// 默认配置
	opts := &GogoOptions{
		Ports:        "80,443,8080",
		Threads:      500,
		Timeout:      3,
		VersionLevel: 1,
		Exploit:      "none",
	}

	// 从配置中提取选项
	if config.Options != nil {
		switch v := config.Options.(type) {
		case *GogoOptions:
			opts = v
		default:
			// 尝试通过JSON转换，支持 PortScanConfig 或直接的 GogoOptions
			if data, err := json.Marshal(config.Options); err == nil {
				// 定义能匹配 PortScanConfig 的结构（包含嵌套的 gogo 字段）
				var portScanConfig struct {
					Ports        string `json:"ports"`
					Threads      int    `json:"threads"`
					Timeout      int    `json:"timeout"`
					Rate         int    `json:"rate"`
					VersionLevel int    `json:"versionLevel"`
					Exploit      string `json:"exploit"`
					Tool         string `json:"tool"`
					Gogo         *struct {
						Enable       bool   `json:"enable"`
						Ports        string `json:"ports"`
						Threads      int    `json:"threads"`
						VersionLevel int    `json:"versionLevel"`
						Exploit      string `json:"exploit"`
					} `json:"gogo"`
				}
				if err := json.Unmarshal(data, &portScanConfig); err == nil {
					// 优先使用嵌套的 gogo 配置
					if portScanConfig.Gogo != nil {
						if portScanConfig.Gogo.Ports != "" {
							opts.Ports = portScanConfig.Gogo.Ports
						}
						if portScanConfig.Gogo.Threads > 0 {
							opts.Threads = portScanConfig.Gogo.Threads
						}
						if portScanConfig.Gogo.VersionLevel >= 0 {
							opts.VersionLevel = portScanConfig.Gogo.VersionLevel
						}
						if portScanConfig.Gogo.Exploit != "" {
							opts.Exploit = portScanConfig.Gogo.Exploit
						}
					} else {
						// 顶层字段作为备选
						if portScanConfig.Ports != "" {
							opts.Ports = portScanConfig.Ports
						}
						if portScanConfig.Threads > 0 {
							opts.Threads = portScanConfig.Threads
						}
						if portScanConfig.Timeout > 0 {
							opts.Timeout = portScanConfig.Timeout
						}
						if portScanConfig.VersionLevel >= 0 {
							opts.VersionLevel = portScanConfig.VersionLevel
						}
						if portScanConfig.Exploit != "" {
							opts.Exploit = portScanConfig.Exploit
						}
					}
				}
			}
		}
	}

	// 解析目标
	targets := parseTargets(config.Target)
	if len(config.Targets) > 0 {
		targets = append(targets, config.Targets...)
	}

	if len(targets) == 0 {
		return &ScanResult{
			WorkspaceId: config.WorkspaceId,
			MainTaskId:  config.MainTaskId,
			Assets:      []*Asset{},
		}, nil
	}

	// 如果提供了 CyberhubConfig，更新配置
	if config.CyberhubConfig != nil && config.CyberhubConfig.URL != "" {
		s.SetCyberhubConfig(config.CyberhubConfig.URL, config.CyberhubConfig.Key)
	}

	// 初始化 engine
	if err := s.ensureInit(); err != nil {
		return nil, err
	}

	// 日志函数
	logInfo := func(format string, args ...interface{}) {
		if config.TaskLogger != nil {
			config.TaskLogger("INFO", format, args...)
		}
		logx.Infof(format, args...)
	}

	// 进度回调
	onProgress := config.OnProgress

	// 执行扫描
	assets, vulns := s.runGogoScan(ctx, targets, opts, logInfo, onProgress)

	return &ScanResult{
		WorkspaceId:  config.WorkspaceId,
		MainTaskId:   config.MainTaskId,
		Assets:       assets,
		Vulnerabilities: vulns,
	}, nil
}

// runGogoScan 运行Gogo扫描
func (s *GogoScanner) runGogoScan(ctx context.Context, targets []string, opts *GogoOptions, logInfo func(string, ...interface{}), onProgress func(int, string)) ([]*Asset, []*Vulnerability) {
	var allAssets []*Asset
	var allVulns []*Vulnerability
	var mu sync.Mutex

	totalTargets := len(targets)
	logInfo("Gogo: scanning %d targets, ports=%s, threads=%d, versionLevel=%d, exploit=%s",
		totalTargets, opts.Ports, opts.Threads, opts.VersionLevel, opts.Exploit)

	// 按单个目标串行执行
	for i, target := range targets {
		// 检查 context 是否取消
		select {
		case <-ctx.Done():
			logInfo("Gogo: cancelled at %d/%d targets", i, totalTargets)
			return allAssets, allVulns
		default:
		}

		// 报告进度 (0-100)
		if onProgress != nil {
			progress := (i * 100) / totalTargets
			onProgress(progress, fmt.Sprintf("Gogo scan: %d/%d", i, totalTargets))
		}

		// 创建 gogo Context 并设置参数
		gogoCtx := gogo.NewContext().WithContext(ctx)
		gogoCtx.SetThreads(opts.Threads)
		gogoCtx.SetVersionLevel(opts.VersionLevel)
		gogoCtx.SetExploit(opts.Exploit)

		// // 创建工作流
		// workflow := &pkg.Workflow{
		// 	Name:        "gogo-scan",
		// 	Description: "Gogo unified scan",
		// 	IP:          target,
		// 	Ports:       opts.Ports,
		// 	Verbose:     opts.VersionLevel,
		// 	Exploit:     opts.Exploit,
		// }

		// // 执行工作流
		// resultCh, err := s.engine.WorkflowStream(gogoCtx, workflow)
		// if err != nil {
		// 	logInfo("Gogo: workflow error for %s: %v", target, err)
		// 	continue
		// }

		task := gogo.NewScanTask(target, opts.Ports)
		resultCh, err := s.engine.Execute(gogoCtx, task)
        if err != nil {
  			logInfo("Gogo: workflow error for %s: %v", target, err)
  			continue
		}

		// 收集结果
		for result := range resultCh {
			if result == nil || !result.Success() {
				continue
			}

			// 从 Result 接口获取 GOGOResult
			gogoResult, ok := result.Data().(*parsers.GOGOResult)
			if !ok || gogoResult == nil {
				continue
			}

			// 转换结果
			asset := gogoResultToAsset(gogoResult)
			if asset != nil {
				mu.Lock()
				allAssets = append(allAssets, asset)
				mu.Unlock()
			}

			// 转换漏洞
			if len(gogoResult.Vulns) > 0 {
				vulns := gogoVulnsToVulnerabilities(gogoResult.Ip, gogoResult.Port, gogoResult.Vulns)
				mu.Lock()
				allVulns = append(allVulns, vulns...)
				mu.Unlock()
			}
		}
	}

	// 进度到100%
	if onProgress != nil {
		onProgress(100, fmt.Sprintf("Gogo scan completed: %d assets, %d vulns", len(allAssets), len(allVulns)))
	}

	logInfo("Gogo: completed, found %d assets, %d vulns", len(allAssets), len(allVulns))
	return allAssets, allVulns
}

// cleanHost 清理 Host 中的 scheme 前缀
func cleanHost(host string) string {
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	// 移除端口前的任何剩余内容
	if idx := strings.Index(host, ":"); idx > 0 {
		// 检查是否是端口号（数字）
		rest := host[idx+1:]
		for _, c := range rest {
			if c < '0' || c > '9' {
				return host // 不是端口，保留原样
			}
		}
	}
	return host
}

// gogoResultToAsset 将 GOGOResult 转换为 Asset
func gogoResultToAsset(result *parsers.GOGOResult) *Asset {
	if result == nil {
		return nil
	}

	port, _ := strconv.Atoi(result.Port)
	if port == 0 {
		port = 80
	}

	// 清理 Host 中的 scheme 前缀
	host := cleanHost(result.Ip)

	asset := &Asset{
		Host:       host,
		Port:       port,
		Service:    result.Protocol,
		Server:     result.Midware,
		Title:      result.Title,
		HttpStatus: result.Status,
		IsHTTP:     result.IsHttp(),
	}

	// 设置 Authority（统一格式：host:port，无协议前缀，与其他扫描器一致）
	asset.Authority = utils.BuildTargetWithPort(host, port)

	// 转换指纹信息
	if len(result.Frameworks) > 0 {
		asset.App = make([]string, 0, len(result.Frameworks))
		for name, frame := range result.Frameworks {
			appName := name
			if frame.Version != "" {
				appName = fmt.Sprintf("%s %s", name, frame.Version)
			}
			asset.App = append(asset.App, appName)
		}
	}

	return asset
}

// gogoVulnsToVulnerabilities 将 GOGOResult.Vulns 转换为 Vulnerability 列表
func gogoVulnsToVulnerabilities(ip, port string, vulns common.Vulns) []*Vulnerability {
	if len(vulns) == 0 {
		return nil
	}

	vulnsList := make([]*Vulnerability, 0, len(vulns))
	intPort, _ := strconv.Atoi(port)
	if intPort == 0 {
		intPort = 80
	}

	// 清理 IP 中的 scheme 前缀
	cleanIP := cleanHost(ip)

	for name, vuln := range vulns {
		severity := convertSeverity(vuln.SeverityLevel)
		v := &Vulnerability{
			Authority: utils.BuildTargetWithPort(cleanIP, intPort),
			Host:     cleanIP,
			Port:     intPort,
			Url:      utils.BuildTargetWithPort(cleanIP, intPort),
			VulName:  name,
			Severity: severity,
			Tags:     vuln.Tags,
			Source:   "gogo",
			Extra:    vuln.GetDetail(),
		}
		vulnsList = append(vulnsList, v)
	}

	return vulnsList
}

// convertSeverity 将 gogo 严重级别转换为 cscan 格式
func convertSeverity(level int) string {
	switch level {
	case 0:
		return "info"
	case 1:
		return "low"
	case 2:
		return "medium"
	case 3:
		return "high"
	case 4:
		return "critical"
	default:
		return "unknown"
	}
}
