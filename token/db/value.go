package db

import (
  "strconv"
  "time"
)

type Value struct {
  Uid      string
  ClientId string
  TTL      time.Duration
  Session  string
}

func (v *Value) uidKey() string {
  return uidKey(v.Uid)
}

func (v *Value) cidField() string {
  return cidField(v.ClientId)
}

func (v *Value) toMap() map[string]interface{} {
  m := make(map[string]interface{})
  // 都使用string 方便反序列化
  m["uid"] = v.Uid
  m["clientId"] = v.ClientId
  m["session"] = v.Session
  m["ttl"] = strconv.Itoa(int(v.TTL.Seconds())) + "s"

  return m
}

func fromMap(m map[string]string) *Value {
  v := Value{}
  v.ClientId = m["clientId"]
  v.Uid = m["uid"]
  v.Session = m["session"]

  ttl := m["ttl"]
  ttl = ttl[:len(ttl)-1]
  t, err := strconv.Atoi(ttl)
  if err != nil {
    panic(err)
  }
  v.TTL = time.Duration(t) * time.Second

  return &v
}

