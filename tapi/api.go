package tapi

import (
  "context"
  "encoding/json"
  "fmt"
  "github.com/xpwu/go-api-token/token"
  "github.com/xpwu/go-log/log"
  "github.com/xpwu/go-tinyserver/api"
  "net/http"
)

/**
 在需要用到token的suit中，应该嵌入 PostJsonAPI ，其登录接口的suit需要嵌入 PostJsonLoginAPI
 */

type PostJsonAPI struct {
  Token         *token.Token
  Request       *api.Request
  errorResponse *api.Response
  UidContext    context.Context
}

func (a *PostJsonAPI) TearDown(ctx context.Context, apiRes interface{}, res *api.Response) {
  if a.errorResponse != nil {
    *res = *a.errorResponse
    return
  }

  ctx, logger := log.WithCtx(a.UidContext)

  rData := &Response{
    Code: Success,
    Data: apiRes,
  }

  var err error
  res.RawData, err = json.Marshal(rData)
  if err != nil {
    logger.Error(err)
    res.Request().Terminate(err)
  }
}

func (a *PostJsonAPI) SetUp(ctx context.Context, r *api.Request, apiReq interface{}) bool {
  ctx, logger := log.WithCtx(ctx)

  rData := &Request{}
  if err := json.Unmarshal(r.RawData, rData); err != nil {
    logger.Error(err)
    r.Terminate(err)
  }

  tk := rData.Token

  if tk == "" {
    logger.Error("request has no 'token'")
    goto _401
  }

  a.Token = token.Resume(ctx, tk)
  if !a.Token.IsValid() {
    logger.Error(fmt.Sprintf("token(%s) error or expire", tk))
    goto _401
  }

  logger.PushPrefix("uid=" + a.Token.Uid())
  a.UidContext = ctx
  a.Request = r

  if err := json.Unmarshal(rData.Data, apiReq); err != nil {
    logger.Error(err)
    r.Terminate(err)
  }

  return true

  // 401
_401:
  resp := Response{
    Code: TokenExpireCode,
    Data: struct {
    }{},
  }

  resData, err := json.Marshal(resp)
  if err != nil {
    logger.Error(err)
    r.Terminate(err)
  }

  a.errorResponse = api.NewResponse(r)
  // token 的错误，并不影响底层http的状态码
  a.errorResponse.HttpStatus = http.StatusOK
  a.errorResponse.RawData = resData

  return false
}

func (a *PostJsonAPI) Logout() {
  _, logger := log.WithCtx(a.UidContext)
  logger.PushPrefix("logout token")
  a.Token.Del()
}
