package geolocation

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultDataDir 默认数据目录
	DefaultDataDir = "data"
	// IPv4FileName IPv4 数据文件名
	IPv4FileName = "ip2region_v4.xdb"
	// IPv6FileName IPv6 数据文件名
	IPv6FileName = "ip2region_v6.xdb"
)

// Checker IP 数据库检查器（不包含下载功能，数据库在编译时准备好）
type Checker struct {
	dataDir string
}

// NewChecker 创建检查器
func NewChecker(dataDir string) *Checker {
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	return &Checker{
		dataDir: dataDir,
	}
}

// IsV4DBExists 检查 IPv4 数据库是否存在
func (c *Checker) IsV4DBExists() bool {
	return fileExists(c.getV4DBPath())
}

// IsV6DBExists 检查 IPv6 数据库是否存在
func (c *Checker) IsV6DBExists() bool {
	return fileExists(c.getV6DBPath())
}

// GetV4DBPath 获取 IPv4 数据库路径
func (c *Checker) GetV4DBPath() string {
	return c.getV4DBPath()
}

// GetV6DBPath 获取 IPv6 数据库路径
func (c *Checker) GetV6DBPath() string {
	return c.getV6DBPath()
}

// GetDatabaseDir 获取数据库目录
func (c *Checker) GetDatabaseDir() string {
	return c.dataDir
}

// VerifyDB 验证数据库文件
func (c *Checker) VerifyDB(filePath string) error {
	if !fileExists(filePath) {
		return ErrDatabaseNotFound
	}

	// 验证文件大小（xdb 文件最小约 1MB）
	info, err := os.Stat(filePath)
	if err != nil {
		return ErrDatabaseNotFound
	}

	if info.Size() < 1024*1024 {
		return ErrDatabaseInvalid
	}

	return nil
}

// CalculateSHA256 计算文件 SHA256
func (c *Checker) CalculateSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (c *Checker) getV4DBPath() string {
	return filepath.Join(c.dataDir, IPv4FileName)
}

func (c *Checker) getV6DBPath() string {
	return filepath.Join(c.dataDir, IPv6FileName)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ParseRegion 解析 region 字符串
func ParseRegion(region string) (country, region2, city, isp string) {
	parts := strings.Split(region, "|")
	if len(parts) >= 4 {
		country = parts[0]
		region2 = parts[1]
		city = parts[2]
		isp = parts[3]
	}
	return
}
