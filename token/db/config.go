package db

import (
	"github.com/xpwu/go-config/configs"
	"github.com/xpwu/go-db-redis/rediscache"
)

type config struct {
	redis        rediscache.Config
	maxTTL       int64 `conf:"maxTTL, unit:day"`
	allowDevices struct {
		min int64
		max int64
	} `conf:"allowDevices, [min, max)"`
}

var confValue = &config{
	maxTTL: 90,
	allowDevices: struct {
		min int64
		max int64
	}{min: 10, max: 20},
}

func init() {
	configs.Unmarshal(confValue)
}
