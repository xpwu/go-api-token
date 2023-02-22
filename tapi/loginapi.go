package tapi

import (
  "context"
  "encoding/json"
  "github.com/xpwu/go-api-token/token"
  "github.com/xpwu/go-api-token/token/db"
  "github.com/xpwu/go-log/log"
  "github.com/xpwu/go-tinyserver/api"
)

/**
  业务在写登录接口时，应该在suit中嵌入 PostJsonLoginAPI
  如果业务接口判断用户登录成功，需要调用 SucceedXXX 中的某一个接口，具体调用哪一个由业务实际情况而定
*/

type PostJsonLoginAPI struct {
  success bool
  Token   *token.Token
  Request *api.Request
  value   *db.Value
}

func (l *PostJsonLoginAPI) SetUp(ctx context.Context, r *api.Request, apiReq interface{}) bool {

  _, logger := log.WithCtx(ctx)

  rData := &LoginRequest{}
  err := json.Unmarshal(r.RawData, rData)
  if err != nil {
    logger.Error(err)
    r.Terminate(err)
  }

  err = json.Unmarshal(rData.Data, apiReq)
  if err != nil {
    logger.Error(err)
    r.Terminate(err)
  }

  l.Request = r
  return true
}

func (l *PostJsonLoginAPI) Succeed(token *token.Token) {
  l.success = true
  l.Token = token
  l.value, _ = token.DB.Value()
}

func (l *PostJsonLoginAPI) SucceedAndOverWrite(ctx context.Context, value db.Value) {
  l.success = true
  l.Token = token.New(ctx, value)
  l.value = &value
}

func (l *PostJsonLoginAPI) SucceedAndSetOrUseOld(ctx context.Context, value db.Value) {
  l.success = true
  l.Token = token.NewOrUseOld(ctx, value)
  l.value = &value
}

func (l *PostJsonLoginAPI) TearDown(ctx context.Context, apiRes interface{}, res *api.Response) {
  ctx, logger := log.WithCtx(ctx)

  rData := &LoginResponse{
    Uid:   "",
    Token: "",
    Data:  apiRes,
  }
  if l.success {
    rData.Uid = l.value.Uid
    rData.Token = l.Token.Id()
  }

  var err error
  res.RawData, err = json.Marshal(rData)
  if err != nil {
    logger.Error(err)
    res.Request().Terminate(err)
  }
}
