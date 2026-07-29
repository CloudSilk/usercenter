// tenant_bridge.go 为嵌入式宿主（如 KinLegacy 多家族单体）提供租户、角色与审计的
// 正式桥接 API。
//
// 背景：宿主业务（KinLegacy）需要在自己的流程里创建租户（一家族=一租户）、
// 创建租户角色模板、写业务审计日志。这些能力此前只能绕过 internal 直接操作共享表，
// 属于脆弱的私有映射。本文件将其纳入 usercenter 官方 setup 契约：
// 宿主不再关心表结构，只调用本文件的函数；usercenter 内部演进时仅需同步此文件。
package setup

import (
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
)

// Tenant 是对外类型别名（usercenter 租户模型）。
type Tenant = tenant.Tenant

// Role 是对外类型别名（usercenter 角色模型）。
type Role = permission.Role

// CreateTenantRecord 为嵌入式宿主创建租户记录。
// 与租户管理端 tenant.CreateTenant 的差异：不做全局名称唯一性校验——
// 宿主的业务唯一键由宿主自行保证（如 KinLegacy 的 family_code），
// 多家族场景下不同家族允许同名。
func CreateTenantRecord(t *Tenant) error {
	return store.DB().Create(t).Error
}

// UpdateTenantEnable 启停租户（家族暂停/恢复时联动）。
func UpdateTenantEnable(tenantID string, enable bool) error {
	return store.DB().Model(&Tenant{}).Where("id = ?", tenantID).Update("enable", enable).Error
}

// GetTenantName 取租户名称；不存在返回空串。
func GetTenantName(tenantID string) string {
	var t Tenant
	if err := store.DB().Select("name").Where("id = ?", tenantID).First(&t).Error; err != nil {
		return ""
	}
	return t.Name
}

// CreateRoleRecord 为嵌入式宿主创建租户角色。
// 角色 ID 由宿主指定（建议含租户前缀，保证语义稳定）；
// 配额校验豁免（宿主自行管理角色模板数量）。
func CreateRoleRecord(r *Role) error {
	return permission.CreateRole(r, func(string) (bool, int32, error) {
		return false, 0, nil
	})
}

// GetRoleByID 按 ID 查询角色。
func GetRoleByID(id string) (*Role, error) {
	return permission.GetRoleByID(id)
}

// RoleBelongsToTenant 校验角色存在、启用且属于指定租户。
// 租户角色授权必须使用同一租户的角色（禁止跨租户授权）。
func RoleBelongsToTenant(roleID, tenantID string) bool {
	var count int64
	store.DB().Model(&Role{}).
		Where("id = ? AND tenant_id = ? AND enable = ?", roleID, tenantID, true).
		Count(&count)
	return count > 0
}

// GetRoleNames 批量取角色名称（ID → 名称）。
func GetRoleNames(roleIDs []string) map[string]string {
	out := map[string]string{}
	if len(roleIDs) == 0 {
		return out
	}
	var roles []Role
	store.DB().Select("id, name").Where("id IN ?", roleIDs).Find(&roles)
	for _, r := range roles {
		out[r.ID] = r.Name
	}
	return out
}

// RecordAudit 记录审计日志（嵌入式宿主的业务写操作统一入口）。
// detail 建议为 JSON 字符串，携带 tenant_id、请求 ID、对象类型、差异摘要与 UA。
func RecordAudit(userID, userName, action, targetID, ip, detail string) {
	audit.RecordAudit(store.DB(), userID, userName, action, targetID, ip, detail)
}
