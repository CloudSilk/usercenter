package model

import (
	"testing"
)

var testCarbines = []*CasbinRule{
	{Ptype: "p", RoleID: "SuperAdmin", Path: "/base/login", Method: "POST", CheckAuth: "0"},
}

func TestUpdateCasbin(t *testing.T) {
	err := UpdateCasbin("SuperAdmin", testCarbines)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateCasbinAPI(t *testing.T) {
	UpdateCasbinApi("/base/login", "/api/auth/login", "POST", "POST")
}

func TestGetPolicyPathByRoleID(t *testing.T) {
	t.Log(GetPolicyPathByRoleID("SuperAdmin"))
}

func TestCasbinEnforce(t *testing.T) {
	t.Log(enforcer.Enforce("1", "/api/role/query", "GET"))
	t.Log(enforcer.Enforce("3", "/api/user/profile", "GET"))
}
