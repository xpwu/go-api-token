package db

import (
	"strconv"
)

type Value struct {
	Uid        string
	ClientId   string
	LatestTime uint64   // unix: s
	Session    string
}

func (v *Value) uidKey() string {
	return uidKey(v.Uid)
}

func (v *Value) toMap() map[string]interface{} {
	m := make(map[string]interface{})
	// 都使用string 方便反序列化
	m["uid"] = v.Uid
	m["clientId"] = v.ClientId
	m["session"] = v.Session
	m["latestTime"] = strconv.FormatUint(v.LatestTime, 10)

	return m
}

func fromMap(m map[string]string) *Value {
	v := Value{}
	v.ClientId = m["clientId"]
	v.Uid = m["uid"]
	v.Session = m["session"]

	t, err := strconv.ParseUint(m["latestTime"], 10, 64)
	if err != nil {
		t = 0
	}
	v.LatestTime = t

	return &v
}
