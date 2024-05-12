package model

import "testing"

func TestCreateAPI(t *testing.T) {
	t.Log(CreateAPI(&API{
		Path: "/base/login", Description: "用户登录", Group: "base", Method: "POST", Enable: true, CheckAuth: false,
	}))
}
