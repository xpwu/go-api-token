package db

import (
  "fmt"
  "reflect"
  "testing"
)

func TestInterface(t *testing.T) {
  type c struct {
    c1 string
    c2 int
  }

  cc := &c{
    c1: "c1",
    c2: -1,
  }

  var ci interface{} = cc

  ca := &c{}
  var cai interface{} = ca

  reflect.ValueOf(cai).Elem().Set(reflect.ValueOf(ci).Elem())

  fmt.Sprintln(ca)
}
