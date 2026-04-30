package config

import (
	"os"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	Mongo struct {
		Uri    string
		DbName string
	}
	Redis   redis.RedisConf
	TaskRpc zrpc.RpcClientConf
	Console ConsoleConfig `json:",optional"`
}

// LoadSecretFromEnv 从环境变量加载 JWT secret，优先级高于配置文件
func (c *Config) LoadSecretFromEnv() {
	if env := os.Getenv("CSCAN_JWT_SECRET"); env != "" {
		c.Auth.AccessSecret = env
	}
}
