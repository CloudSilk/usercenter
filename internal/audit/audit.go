package audit

import (
	"context"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
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
	AuditActionResetPwd        = "reset_password"
	AuditActionChangePwd       = "change_password"
	AuditActionDeleteUser      = "delete_user"
	AuditActionEnableUser      = "enable_user"
	AuditActionUpdateRole      = "update_role"
	AuditActionUpdateUserRoles = "update_user_roles"
	AuditActionPublishRoleAuth = "publish_role_authorization"
	AuditActionPhoneCode       = "phone_code_request"
	AuditActionPhoneLogin      = "phone_login"
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
	al := &AuditLog{
		UserID:        userID,
		UserName:      userName,
		PrincipalKind: principalKind,
		Action:        action,
		TargetID:      targetID,
		IP:            ip,
		Detail:        detail,
	}
	if err := db.Create(al).Error; err != nil {
		log.Errorf(context.Background(), "record audit failed: %v", err)
		return
	}
	publish(al) // 广播给实时审计订阅者（SSE 大屏）
}

// AuditQuery 审计日志查询条件。各过滤字段为零值时表示不限定。
type AuditQuery struct {
	TenantID      string // 可选:租户隔离过滤
	UserID        string // 可选:按操作人过滤
	Action        string // 可选:按操作类型过滤
	TargetID      string // 可选:按目标对象过滤
	PrincipalKind int32  // 可选:按主体类型过滤(-1 表示不过滤)
	StartTime     int64  // 可选:起始时间(unix 秒,0 不过滤)
	EndTime       int64  // 可选:结束时间(unix 秒,0 不过滤)
	PageIndex     int64  // 从 1 开始
	PageSize      int64  // <=0 时默认 10
}

// QueryAuditLogs 分页查询审计日志。返回列表 + 总数。
// 管理端使用;调用方负责鉴权(仅管理员可见)。
func QueryAuditLogs(q *AuditQuery) (list []*AuditLog, total int64, err error) {
	db := store.DB().Model(&AuditLog{})
	if q.TenantID != "" {
		// 审计日志本身不存 tenant_id,按 user 隔离的意义不大;
		// 这里保留参数以便未来扩展(如加 tenant_id 列)。
	}
	if q.UserID != "" {
		db = db.Where("user_id = ?", q.UserID)
	}
	if q.Action != "" {
		db = db.Where("action = ?", q.Action)
	}
	if q.TargetID != "" {
		db = db.Where("target_id = ?", q.TargetID)
	}
	if q.PrincipalKind >= 0 {
		db = db.Where("principal_kind = ?", q.PrincipalKind)
	}
	if q.StartTime > 0 {
		db = db.Where("created_at >= ?", q.StartTime)
	}
	if q.EndTime > 0 {
		db = db.Where("created_at <= ?", q.EndTime)
	}

	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	pageIndex := q.PageIndex
	if pageIndex <= 0 {
		pageIndex = 1
	}
	err = db.Order("created_at desc").
		Offset(int((pageIndex - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&list).Error
	return list, total, err
}
