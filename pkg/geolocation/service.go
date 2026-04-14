package geolocation

import (
	"fmt"
	"sync"

	"github.com/lionsoul2014/ip2region/binding/golang/service"
	"github.com/zeromicro/go-zero/core/logx"
)

// Provider IP 地理位置查询服务提供者
type Provider interface {
	// Search 查询 IP 地址的地理位置
	Search(ip string) (string, error)
	// SearchWithDetail 查询并返回详细地理位置信息
	SearchWithDetail(ip string) (*Location, error)
	// Close 关闭服务
	Close() error
}

// Location 地理位置信息
type Location struct {
	Country string `json:"country"` // 国家
	Region  string `json:"region"`  // 区域/省份
	City    string `json:"city"`    // 城市
	ISP     string `json:"isp"`     // 运营商
	Raw     string `json:"raw"`     // 原始字符串
}

// Ip2RegionProvider ip2region 服务提供者
type Ip2RegionProvider struct {
	ip2region *service.Ip2Region
	v4Path    string
	v6Path    string
	mu        sync.RWMutex
}

// NewIp2RegionProvider 创建 ip2region 服务提供者
// 使用 NewIp2RegionWithPath 简化初始化，自动检测数据库文件
func NewIp2RegionProvider(v4Path, v6Path string) (*Ip2RegionProvider, error) {
	// 使用简化方式创建服务
	ip2region, err := service.NewIp2RegionWithPath(v4Path, v6Path)
	if err != nil {
		return nil, fmt.Errorf("create ip2region service failed: %w", err)
	}

	return &Ip2RegionProvider{
		ip2region: ip2region,
		v4Path:    v4Path,
		v6Path:    v6Path,
	}, nil
}

// Search 查询 IP 地址的地理位置
func (p *Ip2RegionProvider) Search(ip string) (string, error) {
	if ip == "" {
		return "", ErrInvalidIPAddress
	}

	region, err := p.ip2region.Search(ip)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	return region, nil
}

// SearchWithDetail 查询并返回详细地理位置信息
func (p *Ip2RegionProvider) SearchWithDetail(ip string) (*Location, error) {
	region, err := p.Search(ip)
	if err != nil {
		return nil, err
	}

	loc := &Location{
		Raw: region,
	}
	loc.Country, loc.Region, loc.City, loc.ISP = ParseRegion(region)

	return loc, nil
}

// Close 关闭服务
func (p *Ip2RegionProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ip2region != nil {
		p.ip2region.Close()
	}
	return nil
}

// SimpleProvider 简化版 IP 查询服务（不使用 ip2region）
// 用于不支持 ip2region 的环境
type SimpleProvider struct{}

// NewSimpleProvider 创建简化版查询服务
func NewSimpleProvider() *SimpleProvider {
	return &SimpleProvider{}
}

// Search 简化查询（返回空）
func (p *SimpleProvider) Search(ip string) (string, error) {
	if ip == "" {
		return "", ErrInvalidIPAddress
	}
	return "", nil
}

// SearchWithDetail 简化详细查询
func (p *SimpleProvider) SearchWithDetail(ip string) (*Location, error) {
	return &Location{}, nil
}

// Close 关闭服务
func (p *SimpleProvider) Close() error {
	return nil
}

// BatchSearch 批量查询
type BatchSearch struct {
	provider Provider
	mu       sync.Mutex
}

// NewBatchSearch 创建批量查询器
func NewBatchSearch(provider Provider) *BatchSearch {
	return &BatchSearch{
		provider: provider,
	}
}

// SearchBatch 批量查询 IP 地址的地理位置
func (b *BatchSearch) SearchBatch(ips []string) map[string]string {
	results := make(map[string]string)
	if len(ips) == 0 {
		return results
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// 限制并发数
	concurrency := 50
	sem := make(chan struct{}, concurrency)

	for _, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}

		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()

			region, err := b.provider.Search(ip)
			if err != nil {
				logx.Slowf("search IP %s failed: %v", ip, err)
				return
			}

			mu.Lock()
			results[ip] = region
			mu.Unlock()
		}(ip)
	}

	wg.Wait()
	return results
}

// SearchBatchWithDetail 批量查询详细地理位置
func (b *BatchSearch) SearchBatchWithDetail(ips []string) map[string]*Location {
	results := make(map[string]*Location)
	if len(ips) == 0 {
		return results
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	concurrency := 50
	sem := make(chan struct{}, concurrency)

	for _, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}

		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()

			loc, err := b.provider.SearchWithDetail(ip)
			if err != nil {
				logx.Slowf("search IP %s failed: %v", ip, err)
				return
			}

			mu.Lock()
			results[ip] = loc
			mu.Unlock()
		}(ip)
	}

	wg.Wait()
	return results
}
