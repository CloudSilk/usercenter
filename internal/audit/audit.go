package audit

import (
	"context"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"gorm.io/gorm"
)

type AuditLog struct {
	commonmodel.Model
	UserID        string `json:"userID" gorm:"index;size:36;comment:操作人ID"`
	UserName      string `json:"userName" gorm:"size:50;comment:操作人名"`
	PrincipalKind int32  `json:"principalKind" gorm:"index;comment:主体类型0人1Agent2Service"`
	Action        string `json:"action" gorm:"index;size:50;comment:操作类型"`
	TargetID      string `json:"targetID" gorm:"index;size:36;comment:目标对象ID"`
	IP            string `json:"ip" gorm:"size:50;comment:操作IP"`
	Detail        string `json:"detail" gorm:"size:1000;comment:操作详情"`
}

const (
	AuditActionResetPwd   = "reset_password"
	AuditActionChangePwd  = "change_password"
	AuditActionDeleteUser = "delete_user"
	AuditActionEnableUser = "enable_user"
	AuditActionUpdateRole = "update_role"
)

// RecordAudit 记录审计日志(principalKind=0 表示人类,向后兼容)。
func RecordAudit(db *gorm.DB, userID, userName, action, targetID, ip, detail string) {
	RecordAuditWithKind(db, userID, userName, 0, action, targetID, ip, detail)
}

// RecordAuditWithKind 记录带主体类型的审计日志。
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
