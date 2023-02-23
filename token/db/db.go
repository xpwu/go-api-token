package db

import (
  "context"
  "fmt"
  "github.com/go-redis/redis"
  "github.com/xpwu/go-db-redis/rediscache"
  "github.com/xpwu/go-log/log"
  "sort"
  "time"
)

const (
  tokenK = "token:"
  uidK   = "uid:"
)

func uidKey(uid string) string {
  return uidK + uid
}

func tokenKey(token string) string {
  return tokenK + token
}

type DB struct {
  token  string
  value  *Value
  client *redis.Client
  ctx    context.Context
  maxTTL time.Duration
}

/**
 *
 * 存储方式：
 *
 * tokenKey = 'token:' + token
 *
 * tokenKey ---> Value  (0 < TTL <= config.maxTTL)
 *
 * uidKey = 'uid:' + uid
 *
 * uidKey ---> {ClientId_1:token_1, ClientId_2:token_2, ...}
 *
 * 以 tokenKey 作为判断的标准，写的时候后写，删的时候先删
 *
 */

func New(ctx context.Context, suggestedToken string) *DB {
  ctx, logger := log.WithCtx(ctx)
  logger.PushPrefix("token db")

  // adjust
  if confValue.AllowDevices.Min >= confValue.AllowDevices.Max {
    confValue.AllowDevices.Max = 2 * confValue.AllowDevices.Min
  }

  ret := &DB{
    ctx:    ctx,
    token:  suggestedToken,
    client: rediscache.Get(confValue.Redis),
    maxTTL: time.Duration(confValue.MaxTTL) * 24 * time.Hour,
  }
  //if _, err := ret.db.Ping().Result(); err != nil {
  //  panic(err)
  //}

  return ret
}

func (db *DB) RealToken() string {
  return db.token
}

func (db *DB) tokenKey() string {
  return tokenKey(db.token)
}

func must(logger *log.Logger, err error) {
  if err != nil && err != redis.Nil {
    logger.Error(err)
    panic(err)
  }
}

func (db *DB) RefreshTTLto(ttl time.Duration) {
  if ttl < 0 || ttl > db.maxTTL {
    ttl = db.maxTTL
  }

  _, logger := log.WithCtx(db.ctx)
  _, err := db.client.Expire(db.tokenKey(), ttl).Result()
  must(logger, err)
}

func (db *DB) RefreshTTL() {
  db.RefreshTTLto(db.maxTTL)
}

func (db *DB) RefreshTTLAndLastTime(lastTime time.Time) {
  _, logger := log.WithCtx(db.ctx)

  _, err := db.client.Pipelined(func(pipeliner redis.Pipeliner) error {
    tokenKey := db.tokenKey()
    pipeliner.Expire(tokenKey, db.maxTTL)
    pipeliner.HSet(tokenKey, vLatestTime, lastTime)
    return nil
  })
  must(logger, err)
}

func (db *DB) Uid() (uid string, ok bool) {
  _, logger := log.WithCtx(db.ctx)
  uid, err := db.client.HGet(db.tokenKey(), vUid).Result()
  must(logger, err)
  if err == nil {
    return uid, true
  }

  // err == redis.Nil
  logger.Warning(fmt.Sprintf("have no token(%s) or the uid not exist", db.token))
  // 可能是一个没有uid的token，所以做一次清除操作
  db.client.Del(db.tokenKey())
  return "", false
}

func (db *DB) Session() string {
  _, logger := log.WithCtx(db.ctx)
  session, err := db.client.HGet(db.tokenKey(), vSession).Result()
  must(logger, err)

  // not exist, return ZeroValue
  return session
}

func (db *DB) LastTime() time.Time {
  _, logger := log.WithCtx(db.ctx)
  lTime, err := db.client.HGet(db.tokenKey(), vLatestTime).Result()
  must(logger, err)

  // not exist, return ZeroValue
  return decodeLastTime(lTime)
}

func (db *DB) OverWrite(value *Value) {
  _, logger := log.WithCtx(db.ctx)
  db.value = value
  logger.Info(fmt.Sprintf("[uid(%s), clientid(%s)]=>token(%s)", value.Uid,
    value.ClientId, db.token))

  old, err := db.client.HGet(value.uidKey(), value.ClientId).Result()
  must(logger, err)

  pipeliner := db.client.Pipeline()

  // 先删除旧的token
  if err == redis.Nil {
    pipeliner.Del(tokenKey(old))
  }

  // 然后写入新的
  pipeliner.HSet(value.uidKey(), value.ClientId, db.token)
  pipeliner.HMSet(db.tokenKey(), value.toMap())
  // 如果失败了，在使用的时候做补偿
  pipeliner.Expire(db.tokenKey(), db.maxTTL)

  _, err = pipeliner.Exec()
  must(logger, err)
  _ = pipeliner.Close()

  // 最后淘汰
  if eviction(db.ctx, db.client, value.uidKey()) {
    // 重试一次，如果失败，在获取数据等地方时，补偿
    eviction(db.ctx, db.client, value.uidKey())
  }
}

type intStringSortMap struct {
  key   []string
  value []time.Time
}

func (m *intStringSortMap) Len() int {
  return len(m.value)
}

func (m *intStringSortMap) Less(i, j int) bool {
  return m.value[i].Before(m.value[j])
}

func (m *intStringSortMap) Swap(i, j int) {
  m.key[i], m.key[j] = m.key[j], m.key[i]
  m.value[i], m.value[j] = m.value[j], m.value[i]
}

// todo 使用Lua脚本实现，优化效率
func eviction(ctx context.Context, redisC *redis.Client, uidKey string) (needRetry bool) {
  _, logger := log.WithCtx(ctx)
  l, err := redisC.HLen(uidKey).Result()
  must(logger, err)

  // 大于 confValue.allowDevices.max 时，做一次token扫描，看是否有无效token, 并强制淘汰早期的token，
  // 剩余不超过 confValue.allowDevices.max 个

  if l < confValue.AllowDevices.Max {
    return
  }

  err = redisC.Watch(func(tx *redis.Tx) error {
    clients, err := redisC.HGetAll(uidKey).Result()
    must(logger, err)
    // 再次判读
    l = int64(len(clients))
    if l < confValue.AllowDevices.Max {
      return nil
    }

    sortMap := intStringSortMap{
      key:   make([]string, len(clients)),
      value: make([]time.Time, len(clients)),
    }

    index := 0
    pipeliner := redisC.Pipeline()
    for client, token := range clients {
      sortMap.key[index] = client
      pipeliner.HGet(tokenKey(token), vLatestTime)
      index++
    }
    rets, err := pipeliner.Exec()
    must(logger, err)
    _ = pipeliner.Close()
    for i, ret := range rets {
      must(logger, ret.Err())
      // 不存在
      if ret.Err() == redis.Nil {
        sortMap.value[i] = time.Time{}
        continue
      }
      sortMap.value[i] = decodeLastTime(ret.(*redis.StringCmd).Val())
    }

    sort.Sort(&sortMap)

    // transaction
    pipeliner = tx.Pipeline()
    for _, client := range sortMap.key {
      pipeliner.Del(clients[client])
      pipeliner.HDel(uidKey, client)
      l--
      if l <= confValue.AllowDevices.Min {
        break
      }
    }
    _, err = pipeliner.Exec()

    if err == redis.TxFailedErr {
      needRetry = true
      return nil
    }
    must(logger, err)
    _ = pipeliner.Close()
    return nil
  }, uidKey)
  must(logger, err)

  return
}

func (db *DB) SetOrUseOld(value *Value) {
  _, logger := log.WithCtx(db.ctx)
  ownerKey := value.uidKey()
  // 先淘汰
  if eviction(db.ctx, db.client, value.uidKey()) {
    // 重试一次，如果失败，在获取数据等地方补偿
    eviction(db.ctx, db.client, value.uidKey())
  }

  // 必须使用nx 保证并发安全
  newSet, err := db.client.HSetNX(ownerKey, value.ClientId, db.token).Result()
  must(logger, err)

  pipeliner := db.client.Pipeline()
  if !newSet {
    // 有旧值，使用旧值
    oldToken, err := db.client.HGet(ownerKey, value.ClientId).Result()
    must(logger, err)
    db.token = oldToken
    pipeliner.HSet(tokenKey(oldToken), vLatestTime, value.encodeLastTime())
  } else {
    pipeliner.HMSet(tokenKey(db.token), value.toMap())
  }
  pipeliner.Expire(tokenKey(db.token), db.maxTTL)
  _, err = pipeliner.Exec()
  must(logger, err)
  _ = pipeliner.Close()

  db.value = value
  logger.Info(fmt.Sprintf("[uid(%s), clientid(%s)]=>token(%s)", value.Uid,
    value.ClientId, db.token))
}

func (db *DB) IsValidToken() bool {
  _, logger := log.WithCtx(db.ctx)
  ret, err := db.client.Exists(db.tokenKey()).Result()
  must(logger, err)

  return ret == 1
}

func (db *DB) Value() (value *Value, ok bool) {
  _, logger := log.WithCtx(db.ctx)

  if db.value != nil {
    return db.value, true
  }

  m, err := db.client.HGetAll(db.tokenKey()).Result()
  must(logger, err)
  if err == redis.Nil || len(m) == 0 {
    return nil, false
  }

  db.value = fromMap(m)
  return db.value, true
}

func (db *DB) TTL() (ttl time.Duration) {
  _, logger := log.WithCtx(db.ctx)
  ttl, err := db.client.TTL(db.tokenKey()).Result()
  must(logger, err)

  return
}

// Del 可重复多次调用
func (db *DB) Del() {
  _, logger := log.WithCtx(db.ctx)

  value, ok := db.Value()
  if !ok {
    return
  }

  log.Info(fmt.Sprintf("del the token(%s) of uid(%s) for clientid(%s)",
    db.token, value.Uid, value.ClientId))

  db.value = nil
  _, err := db.client.Pipelined(func(pipeliner redis.Pipeliner) error {
    pipeliner.Del(db.tokenKey())
    pipeliner.HDel(value.uidKey(), value.ClientId)
    return nil
  })
  must(logger, err)
}

func DelClientIdForUid(ctx context.Context, uid string, clientId string) {
  _, logger := log.WithCtx(ctx)
  logger.PushPrefix("token db")

  db := rediscache.Get(confValue.Redis)
  token, err := db.HGet(uidKey(uid), clientId).Result()
  must(logger, err)
  if err == redis.Nil {
    log.Info(fmt.Sprintf("DelClientIdForUid: uid(%s) donot have token for clientid(%s)", uid, clientId))
    return
  }

  log.Info(fmt.Sprintf("del the token(%s) of uid(%s) for clientid(%s)", token, uid, clientId))

  _, err = db.Pipelined(func(pipeliner redis.Pipeliner) error {
    pipeliner.Del(tokenKey(token))
    pipeliner.HDel(uidKey(uid), clientId)
    return nil
  })
  must(logger, err)
}

func DelAllForUid(ctx context.Context, uid string) {
  _, logger := log.WithCtx(ctx)
  logger.PushPrefix("token db")

  db := rediscache.Get(confValue.Redis)
  clients, err := db.HGetAll(uidKey(uid)).Result()
  must(logger, err)

  tokenKeys := make([]string, 0, len(clients))
  for _, token := range clients {
    tokenKeys = append(tokenKeys, tokenKey(token))
  }

  log.Info(fmt.Sprintf("del all tokens of uid(%s)", uid))

  _, err = db.Pipelined(func(pipeliner redis.Pipeliner) error {
    pipeliner.Del(tokenKeys...)
    pipeliner.Del(uidKey(uid))
    return nil
  })
  must(logger, err)
}

func Find(ctx context.Context, uid string, clientId string) (db *DB, ok bool) {
  ctx, logger := log.WithCtx(ctx)
  logger.PushPrefix("token db")

  rdb := rediscache.Get(confValue.Redis)

  // 先淘汰，重试两次都失败，直接返回
  if eviction(ctx, rdb, uidKey(uid)) && eviction(ctx, rdb, uidKey(uid)) {
    logger.Error("Find: eviction twice error")
    return nil, false
  }

  token, err := rdb.HGet(uidKey(uid), clientId).Result()
  must(logger, err)

  if err == redis.Nil {
    logger.Warning(fmt.Sprintf("not find token of uid(%s) for clientId(%s)", uid, clientId))
    return nil, false
  }

  return New(ctx, token), true
}

func FindAll(ctx context.Context, uid string) []*DB {
  ctx, logger := log.WithCtx(ctx)
  logger.PushPrefix("token db")

  rdb := rediscache.Get(confValue.Redis)

  // 先淘汰，重试两次都失败，直接返回
  if eviction(ctx, rdb, uidKey(uid)) && eviction(ctx, rdb, uidKey(uid)) {
    logger.Error("FindAll: eviction twice error")
    return []*DB{}
  }

  clients, err := rdb.HGetAll(uidKey(uid)).Result()
  must(logger, err)

  ret := make([]*DB, 0, len(clients))
  if len(clients) == 0 {
    logger.Warning(fmt.Sprintf("not find any token of uid(%s)", uid))
    return ret
  }

  for _, token := range clients {
    ret = append(ret, New(ctx, token))
  }

  return ret
}
