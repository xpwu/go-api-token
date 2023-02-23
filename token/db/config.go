package db

import (
	"github.com/xpwu/go-config/configs"
	"github.com/xpwu/go-db-redis/rediscache"
)

type config struct {
	Redis        rediscache.Config
	MaxTTL       int64 `conf:"maxTTL, unit:day"`
	AllowDevices struct {
		Min int64
		Max int64
	} `conf:"allowDevices, allow Device count, [min, max)"`
}

var confValue = &config{
	MaxTTL: 90,
	AllowDevices: struct {
		Min int64
		Max int64
	}{Min: 10, Max: 20},
}

func init() {
	configs.Unmarshal(confValue)
}
