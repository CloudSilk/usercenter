package model

import "github.com/CloudSilk/usercenter/internal/permission"

// CasbinRule 权限规则(定义已迁至 internal/permission/adapter.go)
type CasbinRule = permission.CasbinRule

// 委托 internal/permission(REDESIGN §4 阶段0)
func SetCasbinRedis(addr, userName, pwd string) { permission.SetCasbinRedis(addr, userName, pwd) }
func InitCasbin()                               { permission.InitCasbin() }
func UpdateCasbin(roleID string, casbinInfos []*CasbinRule) error {
	return permission.UpdateCasbin(roleID, casbinInfos)
}
func UpdateCasbinApi(oldPath, newPath, oldMethod, newMethod string) error {
	return permission.UpdateCasbinApi(oldPath, newPath, oldMethod, newMethod)
}
func GetPolicyPathByRoleID(roleID string) []*CasbinRule {
	return permission.GetPolicyPathByRoleID(roleID)
}
func ClearCasbin(v int, p ...string) (bool, error) { return permission.ClearCasbin(v, p...) }
