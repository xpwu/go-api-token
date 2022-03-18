package token

import (
  "context"
  "crypto/hmac"
  "crypto/sha1"
  "encoding/hex"
  "github.com/xpwu/go-api-token/token/db"
  "github.com/xpwu/go-reqid/reqid"
)

type Token struct {
  DB *db.DB
}

// 返回token的值，常用于传递给客户端
func (t *Token) Id() string {
  return t.DB.Tid()
}

func (t *Token) Uid() string {
  return t.DB.Value().Uid
}

func (t *Token) Value() db.Value {
  return *t.DB.Value()
}

func (t *Token) Del() {
  t.DB.Del()
}

func (t *Token)IsValid() bool {
  if t.DB == nil {
    return false
  }

  return t.DB.IsValidToken()
}

func New(ctx context.Context, value db.Value) *Token {

  if value.Uid == "" {
    panic("uid is empty")
  }

  if value.ClientId == "" {
    panic("ClientId is empty")
  }

  id := NewId(value.Uid, value.ClientId)

  d := db.New(ctx, id)
  d.OverWrite(&value)

  return &Token{DB: d}
}

func NewOrUseOld(ctx context.Context, value db.Value) *Token {
  if value.Uid == "" {
    panic("uid is empty")
  }

  if value.ClientId == "" {
    panic("ClientId is empty")
  }

  id := NewId(value.Uid, value.ClientId)

  d := db.New(ctx, id)
  d.SetOrUseOld(&value)

  return &Token{DB: d}
}

func ResumeFrom(ctx context.Context, token string) *Token {
  return &Token{DB: db.New(ctx, token)}
}

func ResumeFromUidClientId(ctx context.Context, uid, clientId string) *Token {
  d, ok := db.Find(ctx, uid, clientId)
  if !ok {
    return &Token{}
  }

  return &Token{DB: d}
}

func NewId(uid, clientId string) string {
  hash := hmac.New(sha1.New, []byte(uid))
  hash.Write([]byte(clientId))
  hash.Write([]byte(reqid.RandomID()))
  return hex.EncodeToString(hash.Sum([]byte{}))
}
