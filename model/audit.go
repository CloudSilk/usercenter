package model

import (
	"context"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
)

// AuditLog 用户行为审计日志，记录敏感操作便于追溯。
type AuditLog struct {
	commonmodel.Model
	UserID   string `json:"userID" gorm:"index;size:36;comment:操作人ID"`
	UserName string `json:"userName" gorm:"size:50;comment:操作人名"`
	Action   string `json:"action" gorm:"index;size:50;comment:操作类型"`
	TargetID string `json:"targetID" gorm:"index;size:36;comment:目标对象ID"`
	IP       string `json:"ip" gorm:"size:50;comment:操作IP"`
	Detail   string `json:"detail" gorm:"size:1000;comment:操作详情"`
}

// 审计操作类型常量
const (
	AuditActionResetPwd   = "reset_password"
	AuditActionChangePwd  = "change_password"
	AuditActionDeleteUser = "delete_user"
	AuditActionEnableUser = "enable_user"
	AuditActionUpdateRole = "update_role"
)

// RecordAudit 记录一条审计日志。失败仅记录错误日志，不影响主业务流程。
func RecordAudit(userID, userName, action, targetID, ip, detail string) {
	if dbClient == nil || dbClient.DB() == nil {
		return
	}
	if err := dbClient.DB().Create(&AuditLog{
		UserID:   userID,
		UserName: userName,
		Action:   action,
		TargetID: targetID,
		IP:       ip,
		Detail:   detail,
	}).Error; err != nil {
		log.Errorf(context.Background(), "record audit failed: %v", err)
	}
}
