package geolocation

import (
	"net"

	"github.com/zeromicro/go-zero/core/logx"
)

// IPLocator IP 定位器，用于批量查询 IP 地理位置
type IPLocator struct {
	manager   *ProviderManager
	batchSize int
}

// IPInfo IP 信息
type IPInfo struct {
	IP       string `json:"ip"`
	Location string `json:"location"`
}

// NewIPLocator 创建 IP 定位器
func NewIPLocator() *IPLocator {
	return &IPLocator{
		manager:   GetManager(),
		batchSize: 100,
	}
}

// NewIPLocatorWithManager 使用指定的管理器创建定位器
func NewIPLocatorWithManager(manager *ProviderManager) *IPLocator {
	return &IPLocator{
		manager:   manager,
		batchSize: 100,
	}
}

// SetBatchSize 设置批量查询大小
func (l *IPLocator) SetBatchSize(size int) {
	if size > 0 {
		l.batchSize = size
	}
}

// Locate 查询单个 IP 的地理位置
func (l *IPLocator) Locate(ip string) (string, error) {
	if ip == "" {
		return "", ErrInvalidIPAddress
	}

	// 验证 IP 格式
	if net.ParseIP(ip) == nil {
		return "", ErrInvalidIPAddress
	}

	// 检查是否是私有 IP
	if isPrivateIP(ip) {
		return "", nil
	}

	return l.manager.Search(ip)
}

// LocateDetail 查询单个 IP 的详细地理位置
func (l *IPLocator) LocateDetail(ip string) (*Location, error) {
	if ip == "" {
		return nil, ErrInvalidIPAddress
	}

	if net.ParseIP(ip) == nil {
		return nil, ErrInvalidIPAddress
	}

	if isPrivateIP(ip) {
		return &Location{}, nil
	}

	return l.manager.SearchWithDetail(ip)
}

// LocateBatch 批量查询 IP 的地理位置
func (l *IPLocator) LocateBatch(ips []string) map[string]string {
	results := make(map[string]string)
	if len(ips) == 0 {
		return results
	}

	// 去重
	seen := make(map[string]bool)
	uniqueIPs := make([]string, 0, len(ips))
	for _, ip := range ips {
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true

		// 过滤无效和私有 IP
		if net.ParseIP(ip) == nil || isPrivateIP(ip) {
			continue
		}
		uniqueIPs = append(uniqueIPs, ip)
	}

	if len(uniqueIPs) == 0 {
		return results
	}

	// 使用批量查询器
	batch := NewBatchSearch(l.manager.GetProvider())
	return batch.SearchBatch(uniqueIPs)
}

// LocateBatchDetail 批量查询 IP 的详细地理位置
func (l *IPLocator) LocateBatchDetail(ips []string) map[string]*Location {
	results := make(map[string]*Location)
	if len(ips) == 0 {
		return results
	}

	// 去重
	seen := make(map[string]bool)
	uniqueIPs := make([]string, 0, len(ips))
	for _, ip := range ips {
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true

		// 过滤无效和私有 IP
		if net.ParseIP(ip) == nil || isPrivateIP(ip) {
			continue
		}
		uniqueIPs = append(uniqueIPs, ip)
	}

	if len(uniqueIPs) == 0 {
		return results
	}

	// 使用批量查询器
	batch := NewBatchSearch(l.manager.GetProvider())
	return batch.SearchBatchWithDetail(uniqueIPs)
}

// FillIPLocations 为 IP 列表填充地理位置
// 返回新的 IPInfo 列表，包含地理位置信息
func (l *IPLocator) FillIPLocations(ips []IPInfo) []IPInfo {
	if len(ips) == 0 {
		return ips
	}

	// 收集所有 IP
	ipList := make([]string, len(ips))
	for i, ipInfo := range ips {
		ipList[i] = ipInfo.IP
	}

	// 批量查询
	locations := l.LocateBatch(ipList)

	// 填充结果
	for i := range ips {
		if loc, ok := locations[ips[i].IP]; ok {
			ips[i].Location = NormalizeLocation(loc)
		}
	}

	return ips
}

// isPrivateIP 检查是否是私有 IP
func isPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// 检查私有 IPv4 范围
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, block := range privateBlocks {
		_, cidr, err := net.ParseCIDR(block)
		if err != nil {
			continue
		}
		if cidr.Contains(parsedIP) {
			return true
		}
	}

	// 检查 IPv6 私有地址 (fc00::/7)
	if parsedIP.To4() == nil {
		return parsedIP.IsLoopback() || parsedIP.IsUnspecified() || isIPv6Private(parsedIP)
	}

	return false
}

// isIPv6Private 检查 IPv6 是否是私有地址
func isIPv6Private(ip net.IP) bool {
	if len(ip) == 16 {
		return ip[0] == 0xfc || ip[0] == 0xfd
	}
	return false
}

// GetAllIPsFromIPInfoList 从 IPInfo 列表中提取所有 IP 地址
func GetAllIPsFromIPInfoList(ipInfos []IPInfo) []string {
	ipSet := make(map[string]bool)
	for _, ipInfo := range ipInfos {
		if ipInfo.IP != "" && !ipSet[ipInfo.IP] {
			ipSet[ipInfo.IP] = true
		}
	}

	ips := make([]string, 0, len(ipSet))
	for ip := range ipSet {
		ips = append(ips, ip)
	}

	return ips
}

// FillLocationsForIPInfo 为一组 IPInfo 填充地理位置
// 这是一个便捷函数，用于在扫描流程中快速填充地理位置
func FillLocationsForIPInfo(ipv4 []IPInfo, ipv6 []IPInfo) (newIPv4 []IPInfo, newIPv6 []IPInfo) {
	locator := NewIPLocator()
	if !locator.manager.IsEnabled() {
		logx.Slow("geolocation service not initialized, skipping IP location fill")
		return ipv4, ipv6
	}

	// 处理 IPv4
	if len(ipv4) > 0 {
		ipv4 = locator.FillIPLocations(ipv4)
	}

	// 处理 IPv6
	if len(ipv6) > 0 {
		ipv6 = locator.FillIPLocations(ipv6)
	}

	return ipv4, ipv6
}
