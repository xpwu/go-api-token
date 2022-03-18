package db

import (
  "context"
  "errors"
  "fmt"
  "github.com/go-redis/redis"
  "github.com/xpwu/go-db-redis/rediscache"
  "github.com/xpwu/go-log/log"
  "sort"
  "time"
)

const (
  token_ = "tk_"
  uid_   = "uid_"
  cid_   = "cid_"
)

func uidKey(uid string) string {
  return uid_ + uid
}

func cidField(cid string) string {
  return cid_ + cid
}

func tokenKey(tid string) string {
  return token_ + tid
}

func keyToToken(tokenKey string) string {
  return tokenKey[len(token_):]
}

type DB struct {
  tid    string
  value  *Value
  db     *redis.Client
  ctx    context.Context
  logger *log.Logger
}

/**
 *
 * 存储方式：
 *
 * tkKey = 'tk_' + tokenId
 *
 * tkKey ---> TokenValue (encode string)  (0< TTL <= 30*24*3600)
 *
 * uidKey = 'uid_' + uid
 * cidKey = 'cid_' + clientId
 *
 * uidKey ---> {cidKey1:tkKey1, cidKey2:tkKey2, ...}
 *
 */

func New(ctx context.Context, tid string) *DB {
  ctx, logger := log.WithCtx(ctx)
  logger.PushPrefix("token db")

  ret := &DB{
    ctx:    ctx,
    logger: logger,
    tid:    tid,
    db:     rediscache.Get(confValue.Config),
  }
  //if _, err := ret.db.Ping().Result(); err != nil {
  //  panic(err)
  //}

  return ret
}

func (c *DB) Tid() string {
  return c.tid
}

func (c *DB) tokenKey() string {
  return tokenKey(c.tid)
}

func (c *DB) IsValidToken() bool {
  return c.exist(c.tokenKey())
}

func (c *DB) RefreshTTL(ttl time.Duration) {
  if ttl < 0 || ttl > 30*24*time.Hour {
    ttl = 30 * 24 * time.Hour
  }

  _, err := c.db.Expire(c.tokenKey(), ttl).Result()
  if err != nil {
    panic(err)
  }
}

func (c *DB) OverWrite(value *Value) {
  if value.TTL < 0 || value.TTL > 30*24*time.Hour {
    value.TTL = 30 * 24 * time.Hour
  }

  c.value = value
  c.logger.Info(fmt.Sprintf("[uid(%s), clientid(%s)]=>token(%s)", value.Uid,
    value.ClientId, c.Tid()))

  old, err := c.db.HGet(value.uidKey(), value.cidField()).Result()

  if err != nil && err != redis.Nil {
    panic(err)
  }

  // 先删除旧的
  if err != redis.Nil {
    c.db.Del(old)
    c.db.HDel(value.uidKey(), value.cidField())
  }

  // 再淘汰
  c.eviction(value.uidKey())

  // 最后写入
  if _, err = c.db.HSet(value.uidKey(), value.cidField(), c.tokenKey()).Result(); err != nil {
    panic(err)
  }
  if _, err = c.db.HMSet(c.tokenKey(), value.toMap()).Result(); err != nil {
    panic(err)
  }
  if _, err = c.db.Expire(c.tokenKey(), value.TTL).Result(); err != nil {
    panic(err)
  }
}

type intStringSortMap struct {
  key   []string
  value []time.Duration
}

func (m *intStringSortMap) Len() int {
  return len(m.value)
}

func (m *intStringSortMap) Less(i, j int) bool {
  return m.value[i] < m.value[j]
}

func (m *intStringSortMap) Swap(i, j int) {
  m.key[i], m.key[j] = m.key[j], m.key[i]
  m.value[i], m.value[j] = m.value[j], m.value[i]
}

func (c *DB) eviction(uidKey string) {
  l, err := c.db.HLen(uidKey).Result()
  if err != nil {
    panic(err)
  }

  // 大于20个时，做一次token扫描，看是否有无效token, 并强制淘汰最近过期的，剩余不超过10个
  // 对于永久不过期的  一样做淘汰
  if l < 20 {
    return
  }

  nets, err := c.db.HGetAll(uidKey).Result()
  if err != nil {
    panic(err)
  }

  sortMap := intStringSortMap{key: make([]string, len(nets), len(nets)),
    value: make([]time.Duration, len(nets), len(nets))}

  index := 0
  for idenField, net := range nets {
    sortMap.key[index] = idenField
    d, err := c.db.TTL(net).Result()
    if err != nil {
      panic(err)
    }
    sortMap.value[index] = d
  }

  sort.Sort(&sortMap)

  for _, field := range sortMap.key {
    net, err := c.db.HGet(uidKey, field).Result()
    if err != nil && err != redis.Nil {
      panic(err)
    }

    c.db.Del(net)
    c.db.HDel(uidKey, field)
    l--
    if l <= 10 {
      break
    }
  }
}

func (c *DB) SetOrUseOld(value *Value) {
  ownerKey := value.uidKey()
  // 先淘汰
  c.eviction(ownerKey)

  identifierField := value.cidField()
  netKey := c.tokenKey()
  // 必须使用nx 保证并发安全
  if err := c.db.HSetNX(ownerKey, identifierField, netKey).Err(); err != nil {
    panic(err)
  }

  oldNetKey, err := c.db.HGet(ownerKey, identifierField).Result()
  if err != nil && err != redis.Nil {
    panic(err)
  }
  if err != redis.Nil && c.exist(oldNetKey) && oldNetKey != netKey {
    // 有old 就使用old
    c.tid = keyToToken(oldNetKey)
    return
  }

  if value.TTL < 0 || value.TTL > 30*24*time.Hour {
    value.TTL = 30 * 24 * time.Hour
  }

  c.value = value
  c.logger.Info(fmt.Sprintf("[uid(%s), clientid(%s)]=>token(%s)", value.Uid,
    value.ClientId, c.Tid()))

  if _, err = c.db.HMSet(c.tokenKey(), value.toMap()).Result(); err != nil {
    panic(err)
  }
  if _, err = c.db.Expire(c.tokenKey(), value.TTL).Result(); err != nil {
    panic(err)
  }
}

func (c *DB) exist(key string) bool {
  ret, err := c.db.Exists(key).Result()
  if err != nil {
    panic(err)
  }

  return ret == 1
}

func (c *DB) read() {
  if c.value != nil {
    return
  }

  if !c.IsValidToken() {
    panic(errors.New(fmt.Sprintf("token (%s) is not valid", c.tid)))
  }

  m, err := c.db.HGetAll(c.tokenKey()).Result()
  if err != nil {
    panic(err)
  }

  c.value = fromMap(m)
}

func (c *DB) Value() *Value {
  c.read()
  return c.value
}

/**
  返回真实的剩余时间

  通过Value返回的是一开始设置的时间
 */
func (c *DB) ReadLeftTTL() (ttl time.Duration) {
  ttl, err := c.db.TTL(c.tokenKey()).Result()

  if err != nil {
    panic(err)
  }

  return
}

func (c *DB) Del() {
  defer func() {
    if r := recover(); r != nil {
      c.logger.Error(r)
    }
  }()

  if !c.exist(c.tokenKey()) {
    return
  }

  c.read()
  c.db.Del(c.tokenKey())
  c.db.HDel(c.value.uidKey(), c.value.cidField())
  c.value = nil

}

func DelClientIdForUid(ctx context.Context, uid string, clientId string) {
  ctx, logger := log.WithCtx(ctx)
  logger.PushPrefix("token db")

  defer func() {
    if r := recover(); r != nil {
      logger.Error(r)
    }
  }()

  db := rediscache.Get(confValue.Config)
  token, err := db.HGet(uidKey(uid), cidField(clientId)).Result()
  if err != nil {
    panic(err)
  }

  db.Del(token)
  db.HDel(uidKey(uid), cidField(clientId))
}

func DelAllForUid(ctx context.Context, uid string) {
  ctx, logger := log.WithCtx(ctx)
  logger.PushPrefix("token db")

  defer func() {
    if r := recover(); r != nil {
      logger.Error(r)
    }
  }()

  db := rediscache.Get(confValue.Config)
  tokens, err := db.HGetAll(uidKey(uid)).Result()
  if err != nil {
    panic(err)
  }

  for _, t := range tokens {
    db.Del(t)
  }

  db.Del(uidKey(uid))
}

func Find(ctx context.Context, uid string, clientId string) (db *DB, ok bool) {
  ctx, logger := log.WithCtx(ctx)
  logger.PushPrefix("token db")

  defer func() {
    if r := recover(); r != nil {
      logger.Error(r)
      ok = false
    }
  }()

  rdb := rediscache.Get(confValue.Config)

  tKey, err := rdb.HGet(uidKey(uid), cidField(clientId)).Result()
  if err != nil && err != redis.Nil {
    panic(err)
  }

  if err == redis.Nil {
    logger.Warning(fmt.Sprintf("Not Find tokenKey for %s(%s)", uid, clientId))
    return nil, false
  }

  return New(ctx, keyToToken(tKey)), true

}
