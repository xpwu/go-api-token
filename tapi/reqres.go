package tapi

import "encoding/json"

/**
 Data: 上层接口需要的请求数据或者返回数据
    请求数据：会自动解析了传递给相应api的输入参数，即使没有需要的参数，也必须传递给服务器一个空的json对象 "data:":{}
    返回数据：会自动转为json返回给客户端，即使没有需要返回的参数，也会返回一个空的json对象 "data:":{}

以下例子假定 data 没有数据的情况：
Request：
 {
    "token": "xxxxxxxxxx",
    "data": {
            }
  }

Response：
  {
    "code": 200/401,
    "data": {
            }
  }

LoginRequest：
  {
    "data": {
            }
  }

LoginResponse：
  {
    "uid": "xxxx",
    "token": "xxxx",
    "data": {
            }
  }

*/

type Request struct {
  Token string `json:"token"`

  // 上层接口需要的具体数据，会自动解析了传递给相应api的输入参数
  Data json.RawMessage `json:"data"`
}

type code int

const (
  Success         code = 200
  TokenExpireCode      = 401
)

type Response struct {
  Code code `json:"code"`

  // 上层接口需要返回给客户端的数据
  Data interface{} `json:"data"`
}

type LoginRequest struct {
  // 上层接口需要的具体数据，会自动解析了传递给相应api的输入参数
  Data json.RawMessage `json:"data"`
}

type LoginResponse struct {
  Uid string `json:"uid"`
  // 登录失败，则 token是 "", 可用于判断是否登录成功
  Token string `json:"token"`

  // 上层接口需要返回给客户端的数据
  Data interface{} `json:"data"`
}
