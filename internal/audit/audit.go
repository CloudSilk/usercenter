package audit

import (
	"context"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"gorm.io/gorm"
)

// AuditLog 用户行为审计日志,记录敏感操作便于追溯。
//
// 从 model/audit.go 迁入(REDESIGN §4 阶段0)。后续将扩展为结构化全量审计
// (REDESIGN Batch 1):actor 统一为 Principal(含类型),机器行为与人类行为
// 同表可过滤;落 WORM 存储。
type AuditLog struct {
	commonmodel.Model
	UserID       string `json:"userID" gorm:"index;size:36;comment:操作人ID"`
	UserName     string `json:"userName" gorm:"size:50;comment:操作人名"`
	PrincipalKind int32 `json:"principalKind" gorm:"index;comment:主体类型0人1Agent2Service"`
	Action       string `json:"action" gorm:"index;size:50;comment:操作类型"`
	TargetID     string `json:"targetID" gorm:"index;size:36;comment:目标对象ID"`
	IP           string `json:"ip" gorm:"size:50;comment:操作IP"`
	Detail       string `json:"detail" gorm:"size:1000;comment:操作详情"`
}

// 审计操作类型常量
const (
	AuditActionResetPwd   = "reset_password"
	AuditActionChangePwd  = "change_password"
	AuditActionDeleteUser = "delete_user"
	AuditActionEnableUser = "enable_user"
	AuditActionUpdateRole = "update_role"
)

// RecordAudit 记录一条审计日志。db 由调用方注入(阶段0 去全局依赖)。
// 失败仅记录错误日志,不影响主业务流程。
func RecordAudit(db *gorm.DB, userID, userName, action, targetID, ip, detail string) {
	if db == nil {
		return
	}
	if err := db.Create(&AuditLog{
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

// RecordAuditWithKind 记录带主体类型的审计日志(阶段2:Principal-aware 审计)。
// principalKind: 0=人类, 1=Agent, 2=Service (对应 principal.Kind)
func RecordAuditWithKind(db *gorm.DB, userID, userName string, principalKind int32, action, targetID, ip, detail string) {
	if db == nil {
		return
	}
	if err := db.Create(&AuditLog{
		UserID:        userID,
		UserName:      userName,
		PrincipalKind: principalKind,
		Action:        action,
		TargetID:      targetID,
		IP:            ip,
		Detail:        detail,
	}).Error; err != nil {
		log.Errorf(context.Background(), "record audit failed: %v", err)
	}
}
