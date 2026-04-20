package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chainreactors/fingers/common"
	"github.com/chainreactors/gogo/v2/pkg"
	"github.com/chainreactors/neutron/templates"
	"github.com/chainreactors/parsers"
	sdkfingers "github.com/chainreactors/sdk/fingers"
	gogopkg "github.com/chainreactors/sdk/gogo"
	sdkneutron "github.com/chainreactors/sdk/neutron"
	sdkpkg "github.com/chainreactors/sdk/pkg"
	"github.com/chainreactors/sdk/pkg/cyberhub"
	"github.com/zeromicro/go-zero/core/logx"

	"cscan/pkg/utils"
)

// GogoOptions Gogo扫描选项
type GogoOptions struct {
	Enable       bool   `json:"enable"`
	Ports        string `json:"ports"`
	Threads      int    `json:"threads"`
	Timeout      int    `json:"timeout"`
	VersionLevel int    `json:"versionLevel"`
	Exploit      string `json:"exploit"`
	Mod          string `json:"mod"`
	Delay        int    `json:"delay"`
	HttpsDelay   int    `json:"httpsDelay"`
	Ping         bool   `json:"ping"`
	NoScan       bool   `json:"noScan"`
	Exclude      string `json:"exclude"`
}

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
	validMods := map[string]bool{"": true, "default": true, "s": true, "ss": true, "sc": true, "sb": true}
	if !validMods[o.Mod] {
		return fmt.Errorf("mod must be 'default', 's', 'ss', 'sc' or 'sb', got %s", o.Mod)
	}
	return nil
}

func (o *GogoOptions) SetDefaults() {
	if o.Ports == "" {
		o.Ports = "80,443,8080"
	}
	if o.Threads <= 0 {
		o.Threads = 500
	}
	if o.Timeout <= 0 {
		o.Timeout = 3
	}
	if o.VersionLevel < 0 || o.VersionLevel > 2 {
		o.VersionLevel = 1
	}
	if o.Exploit == "" {
		o.Exploit = "none"
	}
	if o.Delay <= 0 {
		o.Delay = 2
	}
	if o.HttpsDelay <= 0 {
		o.HttpsDelay = 2
	}
	if o.Mod == "" {
		o.Mod = "default"
	}
}

func defaultGogoOptions() *GogoOptions {
	opts := &GogoOptions{}
	opts.SetDefaults()
	return opts
}

func normalizeGogoOptions(opts *GogoOptions) (*GogoOptions, error) {
	if opts == nil {
		return defaultGogoOptions(), nil
	}

	clone := *opts
	if err := clone.Validate(); err != nil {
		return nil, err
	}
	clone.SetDefaults()
	return &clone, nil
}

type rawGogoOptions struct {
	Ports        string `json:"ports"`
	Threads      int    `json:"threads"`
	Timeout      int    `json:"timeout"`
	VersionLevel int    `json:"versionLevel"`
	Exploit      string `json:"exploit"`
	Mod          string `json:"mod"`
	Delay        int    `json:"delay"`
	HttpsDelay   int    `json:"httpsDelay"`
	Ping         bool   `json:"ping"`
	NoScan       bool   `json:"noScan"`
	Exclude      string `json:"exclude"`
}

type rawPortScanConfig struct {
	rawGogoOptions
	Gogo *rawGogoOptions `json:"gogo"`
}

func extractGogoOptions(input interface{}) (*GogoOptions, error) {
	if input == nil {
		return normalizeGogoOptions(nil)
	}
	if opts, ok := input.(*GogoOptions); ok {
		return normalizeGogoOptions(opts)
	}

	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	var raw rawPortScanConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	selected := raw.rawGogoOptions
	if raw.Gogo != nil {
		selected = *raw.Gogo
	}

	return normalizeGogoOptions(&GogoOptions{
		Ports:        selected.Ports,
		Threads:      selected.Threads,
		Timeout:      selected.Timeout,
		VersionLevel: selected.VersionLevel,
		Exploit:      selected.Exploit,
		Mod:          selected.Mod,
		Delay:        selected.Delay,
		HttpsDelay:   selected.HttpsDelay,
		Ping:         selected.Ping,
		NoScan:       selected.NoScan,
		Exclude:      selected.Exclude,
	})
}

type GogoScanner struct {
	BaseScanner
	engine      *gogopkg.GogoEngine
	initMu      sync.Mutex
	cyberhubURL string
	cyberhubKey string
	inited      bool
}

func NewGogoScanner() *GogoScanner {
	return &GogoScanner{BaseScanner: BaseScanner{name: "gogo"}}
}

func (s *GogoScanner) Bootstrap(ctx context.Context, config *BootstrapConfig) error {
	if config == nil {
		return fmt.Errorf("bootstrap config is required")
	}

	cache := gogoCachePaths(config.CacheDir)
	if !cache.ready() {
		if config.CyberhubConfig == nil || config.CyberhubConfig.URL == "" || config.CyberhubConfig.Key == "" {
			return fmt.Errorf("gogo bootstrap requires cyberhub config or local cache")
		}
		if err := cache.ensureDir(); err != nil {
			return err
		}
		if err := exportGogoCache(ctx, config.CyberhubConfig, cache); err != nil {
			return err
		}
	}

	return s.initFromLocalCache(cache)
}

type gogoCacheFiles struct {
	root        string
	fingersFile string
	pocsFile    string
}

func gogoCachePaths(cacheDir string) gogoCacheFiles {
	root := filepath.Join(cacheDir, "gogo")
	return gogoCacheFiles{
		root:        root,
		fingersFile: filepath.Join(root, "fingers.yaml"),
		pocsFile:    filepath.Join(root, "pocs.yaml"),
	}
}

func (c gogoCacheFiles) ready() bool {
	if _, err := os.Stat(c.fingersFile); err != nil {
		return false
	}
	if _, err := os.Stat(c.pocsFile); err != nil {
		return false
	}
	return true
}

func (c gogoCacheFiles) ensureDir() error {
	return os.MkdirAll(c.root, 0o755)
}

func exportGogoCache(ctx context.Context, cfg *CyberhubConfig, cache gogoCacheFiles) error {
	client := cyberhub.NewClient(cfg.URL, cfg.Key, 60*time.Second)

	fingersData, _, err := client.ExportFingers(ctx, "")
	if err != nil {
		return fmt.Errorf("export fingers failed: %w", err)
	}
	if err := cyberhub.SaveFingersToFile(cache.fingersFile, fingersData); err != nil {
		return fmt.Errorf("save fingers cache failed: %w", err)
	}

	pocResponses, err := client.ExportPOCs(ctx, nil, nil, "", "")
	if err != nil {
		return fmt.Errorf("export pocs failed: %w", err)
	}
	pocs := make([]*templates.Template, 0, len(pocResponses))
	for _, resp := range pocResponses {
		pocs = append(pocs, resp.GetTemplate())
	}
	if err := cyberhub.SaveTemplatesToFile(cache.pocsFile, pocs); err != nil {
		return fmt.Errorf("save pocs cache failed: %w", err)
	}

	return nil
}

func (s *GogoScanner) initFromLocalCache(cache gogoCacheFiles) error {
	s.initMu.Lock()
	defer s.initMu.Unlock()

	if s.inited {
		return nil
	}

	engine, err := s.createGogoEngineFromLocal(cache)
	if err != nil {
		return fmt.Errorf("failed to create gogo engine: %w", err)
	}

	s.engine = engine
	s.cyberhubURL = ""
	s.cyberhubKey = ""
	s.inited = true
	return nil
}

func (s *GogoScanner) createGogoEngineFromLocal(cache gogoCacheFiles) (*gogopkg.GogoEngine, error) {
	fingersEngine, err := s.createFingersEngineFromLocal(cache.fingersFile)
	if err != nil {
		return nil, fmt.Errorf("fingers engine creation failed: %w", err)
	}
	neutronEngine, err := s.createNeutronEngineFromLocal(cache.pocsFile)
	if err != nil {
		return nil, fmt.Errorf("neutron engine creation failed: %w", err)
	}
	gogoConfig := gogopkg.NewConfig().WithFingersEngine(fingersEngine).WithNeutronEngine(neutronEngine)
	engine := gogopkg.NewEngine(gogoConfig)
	if err := engine.Init(); err != nil {
		return nil, fmt.Errorf("gogo init failed: %w", err)
	}
	return engine, nil
}

func (s *GogoScanner) createFingersEngineFromLocal(file string) (*sdkfingers.Engine, error) {
	cfg := sdkfingers.NewConfig()
	cfg.WithLocalFile(file)
	cfg.SetTimeout(60 * time.Second)
	return sdkfingers.NewEngine(cfg)
}

func (s *GogoScanner) createNeutronEngineFromLocal(path string) (*sdkneutron.Engine, error) {
	cfg := sdkneutron.NewConfig()
	cfg.WithLocalFile(path)
	cfg.SetTimeout(60 * time.Second)
	return sdkneutron.NewEngine(cfg)
}

func (s *GogoScanner) Init(url, key string) error {
	s.initMu.Lock()
	defer s.initMu.Unlock()

	if s.inited {
		return nil
	}

	s.cyberhubURL = url
	s.cyberhubKey = key

	engine, err := s.createGogoEngine(url, key)
	if err != nil {
		return fmt.Errorf("failed to create gogo engine: %w", err)
	}

	s.engine = engine
	s.inited = true
	return nil
}

func (s *GogoScanner) createGogoEngine(url, key string) (*gogopkg.GogoEngine, error) {
	fingersEngine, err := s.createFingersEngine(url, key)
	if err != nil {
		return nil, fmt.Errorf("fingers engine creation failed: %w", err)
	}
	neutronEngine, err := s.createNeutronEngine(url, key)
	if err != nil {
		return nil, fmt.Errorf("neutron engine creation failed: %w", err)
	}
	gogoConfig := gogopkg.NewConfig().WithFingersEngine(fingersEngine).WithNeutronEngine(neutronEngine)
	engine := gogopkg.NewEngine(gogoConfig)
	if err := engine.Init(); err != nil {
		return nil, fmt.Errorf("gogo init failed: %w", err)
	}
	return engine, nil
}

func (s *GogoScanner) createFingersEngine(url, key string) (*sdkfingers.Engine, error) {
	fingersConfig := sdkfingers.NewConfig()
	fingersConfig.WithCyberhub(url, key)
	fingersConfig.SetTimeout(60 * time.Second)
	return sdkfingers.NewEngine(fingersConfig)
}

func (s *GogoScanner) createNeutronEngine(url, key string) (*sdkneutron.Engine, error) {
	neutronConfig := sdkneutron.NewConfig()
	neutronConfig.WithCyberhub(url, key)
	neutronConfig.SetTimeout(60 * time.Second)
	return sdkneutron.NewEngine(neutronConfig)
}

func (s *GogoScanner) IsInited() bool {
	s.initMu.Lock()
	defer s.initMu.Unlock()
	return s.inited
}

func (s *GogoScanner) ensureInitialized() error {
	if !s.IsInited() {
		return fmt.Errorf("gogo scanner not initialized, please call Init() first")
	}
	return nil
}

func (s *GogoScanner) buildExecutionContext(ctx context.Context, opts *GogoOptions) *gogopkg.Context {
	gogoCtx := gogopkg.NewContext().WithContext(ctx)
	gogoCtx.SetThreads(opts.Threads)
	gogoCtx.SetVersionLevel(opts.VersionLevel)
	gogoCtx.SetExploit(opts.Exploit)
	gogoCtx.SetDelay(opts.Delay)
	return gogoCtx
}

func useWorkflowTask(opts *GogoOptions) bool {
	return (opts.Mod != "" && opts.Mod != "default") || opts.Ping || opts.NoScan
}

func (s *GogoScanner) executeTargetScan(ctx context.Context, target string, opts *GogoOptions) (<-chan sdkpkg.Result, error) {
	gogoCtx := s.buildExecutionContext(ctx, opts)
	if useWorkflowTask(opts) {
		workflow := &pkg.Workflow{
			IP:      target,
			Ports:   opts.Ports,
			Mod:     opts.Mod,
			Ping:    opts.Ping,
			NoScan:  opts.NoScan,
			Exploit: opts.Exploit,
			Verbose: opts.VersionLevel,
		}
		return s.engine.Execute(gogoCtx, gogopkg.NewWorkflowTask(workflow))
	}

	return s.engine.Execute(gogoCtx, gogopkg.NewScanTask(target, opts.Ports))
}

func (s *GogoScanner) Scan(ctx context.Context, config *ScanConfig) (*ScanResult, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, err
	}

	opts, err := normalizeGogoOptions(nil)
	if err != nil {
		return nil, err
	}

	opts, err = extractGogoOptions(config.Options)
	if err != nil {
		return nil, fmt.Errorf("invalid gogo options: %w", err)
	}

	targets := parseTargets(config.Target)
	if len(config.Targets) > 0 {
		targets = append(targets, config.Targets...)
	}
	if len(targets) == 0 {
		return &ScanResult{WorkspaceId: config.WorkspaceId, MainTaskId: config.MainTaskId, Assets: []*Asset{}}, nil
	}

	logInfo := func(format string, args ...interface{}) {
		if config.TaskLogger != nil {
			config.TaskLogger("INFO", format, args...)
		}
		logx.Infof(format, args...)
	}

	assets, vulns, aliveHosts := s.runGogoScan(ctx, targets, opts, logInfo, config.OnProgress)
	return &ScanResult{
		WorkspaceId:     config.WorkspaceId,
		MainTaskId:      config.MainTaskId,
		Assets:          assets,
		Vulnerabilities: vulns,
		AliveHosts:      aliveHosts,
	}, nil
}

func (s *GogoScanner) runGogoScan(ctx context.Context, targets []string, opts *GogoOptions, logInfo func(string, ...interface{}), onProgress func(int, string)) ([]*Asset, []*Vulnerability, []string) {
	var allAssets []*Asset
	var allVulns []*Vulnerability
	var aliveHosts []string
	var mu sync.Mutex

	totalTargets := len(targets)
	isPingMode := opts.Ping && opts.NoScan

	if isPingMode {
		logInfo("Gogo: ping sweep mode, targets=%d, threads=%d", totalTargets, opts.Threads)
	} else {
		logInfo("Gogo: scanning %d targets, ports=%s, threads=%d, versionLevel=%d, exploit=%s, mod=%s, delay=%d",
			totalTargets, opts.Ports, opts.Threads, opts.VersionLevel, opts.Exploit, opts.Mod, opts.Delay)
	}

	for i, target := range targets {
		select {
		case <-ctx.Done():
			logInfo("Gogo: cancelled at %d/%d targets", i, totalTargets)
			return allAssets, allVulns, aliveHosts
		default:
		}

		if onProgress != nil {
			progress := (i * 100) / totalTargets
			onProgress(progress, fmt.Sprintf("Gogo scan: %d/%d", i, totalTargets))
		}

		resultCh, err := s.executeTargetScan(ctx, target, opts)
		if err != nil {
			logInfo("Gogo: scan error for %s: %v", target, err)
			continue
		}

		for result := range resultCh {
			if result == nil || !result.Success() {
				continue
			}

			gogoResult, ok := result.Data().(*parsers.GOGOResult)
			if !ok || gogoResult == nil {
				continue
			}

			if isPingMode {
				mu.Lock()
				aliveHosts = append(aliveHosts, gogoResult.Ip)
				mu.Unlock()
				continue
			}

			asset := gogoResultToAsset(gogoResult)
			if asset != nil {
				mu.Lock()
				allAssets = append(allAssets, asset)
				mu.Unlock()
			}

			if len(gogoResult.Vulns) > 0 {
				vulns := gogoVulnsToVulnerabilities(gogoResult.Ip, gogoResult.Port, gogoResult.Vulns)
				mu.Lock()
				allVulns = append(allVulns, vulns...)
				mu.Unlock()
			}
		}
	}

	if onProgress != nil {
		if isPingMode {
			onProgress(100, fmt.Sprintf("Gogo ping sweep completed: %d alive hosts", len(aliveHosts)))
		} else {
			onProgress(100, fmt.Sprintf("Gogo scan completed: %d assets, %d vulns", len(allAssets), len(allVulns)))
		}
	}

	if isPingMode {
		logInfo("Gogo: ping sweep completed, found %d alive hosts", len(aliveHosts))
	} else {
		logInfo("Gogo: completed, found %d assets, %d vulns", len(allAssets), len(allVulns))
	}

	return allAssets, allVulns, aliveHosts
}

func cleanHost(host string) string {
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	if idx := strings.Index(host, ":"); idx > 0 {
		rest := host[idx+1:]
		for _, c := range rest {
			if c < '0' || c > '9' {
				return host
			}
		}
	}
	return host
}

func gogoResultToAsset(result *parsers.GOGOResult) *Asset {
	if result == nil {
		return nil
	}

	port, _ := strconv.Atoi(result.Port)
	if port == 0 {
		port = 80
	}

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
	asset.Authority = utils.BuildTargetWithPort(host, port)

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

func gogoVulnsToVulnerabilities(ip, port string, vulns common.Vulns) []*Vulnerability {
	if len(vulns) == 0 {
		return nil
	}

	vulnsList := make([]*Vulnerability, 0, len(vulns))
	intPort, _ := strconv.Atoi(port)
	if intPort == 0 {
		intPort = 80
	}
	cleanIP := cleanHost(ip)

	for name, vuln := range vulns {
		severity := convertSeverity(vuln.SeverityLevel)
		v := &Vulnerability{
			Authority: utils.BuildTargetWithPort(cleanIP, intPort),
			Host:      cleanIP,
			Port:      intPort,
			Url:       utils.BuildTargetWithPort(cleanIP, intPort),
			VulName:   name,
			Severity:  severity,
			Tags:      vuln.Tags,
			Source:    "gogo",
			Extra:     vuln.GetDetail(),
		}
		vulnsList = append(vulnsList, v)
	}

	return vulnsList
}

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

