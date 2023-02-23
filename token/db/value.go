package db

import (
	"strconv"
	"time"
)

type Value struct {
	Uid        string
	ClientId   string
	// 具体意义由使用方决定传入什么，比如最后通信、最后登录等
	LatestTime time.Time
	Session    string
}

func (v *Value) uidKey() string {
	return uidKey(v.Uid)
}

const (
	vUid = "uid"
	vClientId = "clientId"
	vSession = "session"
	vLatestTime = "latestTime"
)

func encodeLastTime(lastTime time.Time) string {
	return strconv.FormatInt(lastTime.Unix(), 10)
}

func (v *Value) toMap() map[string]interface{} {
	m := make(map[string]interface{})
	// 都使用string 方便反序列化
	m[vUid] = v.Uid
	m[vClientId] = v.ClientId
	m[vSession] = v.Session
	m[vLatestTime] = encodeLastTime(v.LatestTime)

	return m
}

func decodeLastTime(str string) time.Time {
	t, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		t = 0
	}

	return time.Unix(t, 0)
}

func fromMap(m map[string]string) *Value {
	v := &Value{}
	v.ClientId = m[vClientId]
	v.Uid = m[vUid]
	v.Session = m[vSession]
	v.LatestTime = decodeLastTime(m[vLatestTime])

	return v
}
