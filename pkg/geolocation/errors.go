package geolocation

import (
	"errors"
)

// 错误定义
var (
	// ErrDatabaseNotFound 数据库文件未找到
	ErrDatabaseNotFound = errors.New("ip2region database file not found")

	// ErrDatabaseInvalid 数据库文件无效
	ErrDatabaseInvalid = errors.New("ip2region database file is invalid")

	// ErrInvalidIPAddress 无效的 IP 地址
	ErrInvalidIPAddress = errors.New("invalid IP address")

	// ErrSearchFailed 查询失败
	ErrSearchFailed = errors.New("IP geolocation search failed")

	// ErrServiceNotInitialized 服务未初始化
	ErrServiceNotInitialized = errors.New("geolocation service not initialized")

	// ErrDownloadFailed 下载失败
	ErrDownloadFailed = errors.New("database download failed")

	// ErrFileNotWritable 文件不可写
	ErrFileNotWritable = errors.New("database file is not writable")
)

// GeolocationError 地理位置服务错误
type GeolocationError struct {
	Op  string // 操作
	Err error  // 原始错误
}

func (e *GeolocationError) Error() string {
	return e.Op + ": " + e.Err.Error()
}

func (e *GeolocationError) Unwrap() error {
	return e.Err
}

// NewGeolocationError 创建新的地理位置错误
func NewGeolocationError(op string, err error) *GeolocationError {
	return &GeolocationError{
		Op:  op,
		Err: err,
	}
}
