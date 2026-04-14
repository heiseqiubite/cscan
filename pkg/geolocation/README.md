# IP 地理位置服务

基于 [ip2region](https://github.com/lionsoul2014/ip2region) 实现的离线 IP 地址定位库。

## 功能特性

- 支持 IPv4 和 IPv6 地址查询
- 离线查询，无需外部 API
- 高性能，十微秒级查询速度
- 数据库在编译时准备好，无运行时下载
- 支持批量查询

## 数据库文件

IP 数据库文件存储在 `data/` 目录：

| 文件 | 说明 | 大小 |
|------|------|------|
| `data/ip2region_v4.xdb` | IPv4 数据库 | ~10MB |
| `data/ip2region_v6.xdb` | IPv6 数据库 | ~2MB |

**数据库来源**: https://github.com/lionsoul2014/ip2region/tree/master/data

## 安装/初始化

### Docker 构建（推荐）

Dockerfile 会在编译时自动下载 IP 数据库：

```bash
# 构建 API 服务
docker build -f docker/Dockerfile.api -t cscan-api .

# 构建 RPC 服务
docker build -f docker/Dockerfile.rpc -t cscan-rpc .

# 构建 Worker
docker build -f docker/Dockerfile.worker -t cscan-worker .
```

### 本地开发

下载数据库文件：

**Windows:**
```powershell
.\scripts\download-ip2region.bat
```

**Linux/macOS:**
```bash
chmod +x ./scripts/download-ip2region.sh
./scripts/download-ip2region.sh
```

或者手动下载：
```bash
mkdir -p data
curl -L https://github.com/lionsoul2014/ip2region/raw/master/data/ip2region_v4.xdb -o data/ip2region_v4.xdb
curl -L https://github.com/lionsoul2014/ip2region/raw/master/data/ip2region_v6.xdb -o data/ip2region_v6.xdb
```

## 使用方法

### 1. 初始化服务

```go
import "cscan/pkg/geolocation"

// 方式一：使用默认配置
geolocation.GetManager().Init("")

// 方式二：使用自定义配置
config := geolocation.Config{
    Enabled: true,
    DataDir: "data",
}
geolocation.GetManager().InitWithConfig(config)

// 方式三：启动时必须初始化（失败会 panic）
geolocation.GetManager().MustInit()
```

### 2. 查询单个 IP

```go
// 查询 IP 地址
location, err := geolocation.GetManager().Search("8.8.8.8")
if err != nil {
    logx.Errorf("查询失败: %v", err)
}
fmt.Println(location) // 输出: 中国|0|广东省|广州市|谷歌

// 查询详细地理位置
detail, err := geolocation.GetManager().SearchWithDetail("8.8.8.8")
if err != nil {
    logx.Errorf("查询失败: %v", err)
}
fmt.Printf("国家: %s, 省份: %s, 城市: %s, ISP: %s\n", 
    detail.Country, detail.Region, detail.City, detail.ISP)
```

### 3. 批量查询

```go
locator := geolocation.NewIPLocator()

// 批量查询
ips := []string{"8.8.8.8", "1.1.1.1", "114.114.114.114"}
locations := locator.LocateBatch(ips)
for ip, loc := range locations {
    fmt.Printf("%s -> %s\n", ip, geolocation.NormalizeLocation(loc))
}

// 批量查询详细地理位置
details := locator.LocateBatchDetail(ips)
for ip, detail := range details {
    fmt.Printf("%s -> %s-%s-%s\n", ip, detail.Country, detail.Region, detail.City)
}
```

### 4. 标准化地理位置格式

```go
// 原始格式: "中国|0|广东省|深圳市|电信"
// 标准化后: "广东省深圳市-电信"
normalized := geolocation.NormalizeLocation("中国|0|广东省|深圳市|电信")
fmt.Println(normalized) // 输出: 广东省深圳市-电信
```

### 5. 在扫描器中使用

IP 地理位置功能已集成到 Naabu 端口扫描器中：

```go
// 扫描时会自动填充 IP 地理位置
result, err := scanner.Scan(ctx, config)
for _, asset := range result.Assets {
    for _, ipv4 := range asset.IPV4 {
        fmt.Printf("IP: %s, 位置: %s\n", ipv4.IP, ipv4.Location)
    }
}
```

## 配置文件

可以通过 JSON 配置文件配置服务：

```json
{
    "enabled": true,
    "dataDir": "data",
    "autoDownload": false
}
```

## IP 数据格式

返回的地理位置信息格式为：`国家|区域|省份|城市|ISP`

示例：
- `中国|0|广东省|深圳市|电信`
- `美国|0|0|0|谷歌`
- `日本|0|东京都|东京|0`

## 注意事项

1. **数据库必须在编译时准备好** - 数据库文件应提前下载到 `data/` 目录或通过 Dockerfile 自动下载
2. 私有 IP 地址（如 `192.168.x.x`, `10.x.x.x` 等）不会进行查询，返回空字符串
3. 数据库文件约 10-12MB，请确保有足够的磁盘空间

## API 参考

### ProviderManager

| 方法 | 说明 |
|------|------|
| `GetManager()` | 获取全局管理器单例 |
| `Init(configPath)` | 初始化服务 |
| `MustInit()` | 初始化服务，失败会 panic |
| `Search(ip)` | 查询 IP 地理位置 |
| `SearchWithDetail(ip)` | 查询详细地理位置 |
| `IsEnabled()` | 检查服务是否启用 |
| `Close()` | 关闭服务 |

### IPLocator

| 方法 | 说明 |
|------|------|
| `NewIPLocator()` | 创建 IP 定位器 |
| `Locate(ip)` | 查询单个 IP |
| `LocateDetail(ip)` | 查询详细地理位置 |
| `LocateBatch(ips)` | 批量查询 |
| `LocateBatchDetail(ips)` | 批量查询详细地理位置 |
