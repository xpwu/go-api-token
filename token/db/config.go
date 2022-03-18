package db

import (
  "github.com/xpwu/go-config/configs"
  "github.com/xpwu/go-db-redis/rediscache"
)

type redisConfig struct {
  rediscache.Config
}

var confValue = &redisConfig{}

func init() {
  configs.Unmarshal(confValue)
}
