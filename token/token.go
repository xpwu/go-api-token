package token

import (
  "context"
  "crypto/hmac"
  "crypto/sha1"
  "encoding/hex"
  "fmt"
  "github.com/xpwu/go-api-token/token/db"
  "github.com/xpwu/go-reqid/reqid"
)

type Token struct {
  DB  *db.DB
  uid func()string
}

// Id 返回token的值，常用于传递给客户端
func (t *Token) Id() string {
  return t.DB.RealToken()
}

// UidOrInvalid
// ok true: token is valid, false: invalid
func (t *Token) UidOrInvalid() (uid string, ok bool) {
  uid, ok = t.DB.Uid()
  if ok {
    t.uid = func() string {
      return uid
    }
  }

  return
}

func (t *Token) mustUid() string {
  uid, ok := t.DB.Uid()
  if !ok {
    panic(fmt.Sprintf("token(%s) does not have uid", t.DB.RealToken()))
  }
  return uid
}

func (t *Token) Uid() string {
  return t.uid()
}

// Del 退出登录时，应该调用此接口删除token数据，可重复多次调用
func (t *Token) Del() {
  t.DB.Del()
}

func New(ctx context.Context, value db.Value) *Token {

  if value.Uid == "" {
    panic("uid is empty")
  }

  if value.ClientId == "" {
    panic("ClientId is empty")
  }

  newToken := NewId(value.Uid, value.ClientId)

  d := db.New(ctx, newToken)
  d.OverWrite(&value)

  return &Token{DB: d, uid: func() string {
    return value.Uid
  }}
}

func NewOrUseOld(ctx context.Context, value db.Value) *Token {
  if value.Uid == "" {
    panic("uid is empty")
  }

  if value.ClientId == "" {
    panic("ClientId is empty")
  }

  newToken := NewId(value.Uid, value.ClientId)

  d := db.New(ctx, newToken)
  d.SetOrUseOld(&value)

  return &Token{DB: d, uid: func() string {
    return value.Uid
  }}
}

func Resume(ctx context.Context, token string) *Token {
  ret := &Token{DB: db.New(ctx, token)}
  ret.uid = ret.mustUid
  return ret
}

func ResumeFromUidClientId(ctx context.Context, uid, clientId string) (token *Token, ok bool) {
  d, ok := db.Find(ctx, uid, clientId)
  if !ok {
    return nil, false
  }

  ret := &Token{DB: d}
  ret.uid = func() string {
    return uid
  }
  return ret, true
}

func NewId(uid, clientId string) string {
  hash := hmac.New(sha1.New, []byte(uid))
  hash.Write([]byte(clientId))
  hash.Write([]byte(reqid.RandomID()))
  return hex.EncodeToString(hash.Sum([]byte{}))
}
