package model

import (
	"github.com/CloudSilk/usercenter/internal/audit"
)

// AuditLog 审计日志(定义已迁至 internal/audit,此处为兼容别名)。
// model 包的历史调用方(http/user.go 的 RecordAudit、init.go 的 AutoMigrate)
// 继续使用 model.AuditLog,无需改动 —— 这是 REDESIGN §4 阶段0 的"签名零变更"。
type AuditLog = audit.AuditLog

// 审计操作类型常量(委托 internal/audit)
const (
	AuditActionResetPwd   = audit.AuditActionResetPwd
	AuditActionChangePwd  = audit.AuditActionChangePwd
	AuditActionDeleteUser = audit.AuditActionDeleteUser
	AuditActionEnableUser = audit.AuditActionEnableUser
	AuditActionUpdateRole = audit.AuditActionUpdateRole
)

// RecordAudit 记录审计日志。委托 internal/audit.RecordAudit(注入 dbClient),
// 向后兼容现有 model.RecordAudit(...) 调用。
func RecordAudit(userID, userName, action, targetID, ip, detail string) {
	if dbClient == nil {
		return
	}
	audit.RecordAudit(dbClient.DB(), userID, userName, action, targetID, ip, detail)
}
