package config

import (
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
	Cyberhub CyberhubConfig `json:",optional"`
}

// CyberhubConfig Cyberhub 配置
type CyberhubConfig struct {
	URL string `json:",optional"`
	Key string `json:",optional"`
}
