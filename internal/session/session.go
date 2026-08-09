package session

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
	"gorm.io/gorm"
)

// Session 活跃会话记录(用于会话列表 + 单会话吊销)
type Session struct {
	commonmodel.Model
	PrincipalID   string `json:"principalID" gorm:"index;size:36;comment:主体ID"`
	TenantID      string `json:"tenantID" gorm:"index;size:36"`
	TokenSig      string `json:"-" gorm:"index;size:64;comment:token签名段(唯一)"`
	DeviceType    int32  `json:"deviceType" gorm:"comment:0web1android2iphone3ipad"`
	DeviceName    string `json:"deviceName" gorm:"size:200;comment:设备名称"`
	IP            string `json:"ip" gorm:"size:50;comment:登录IP"`
	UserAgent     string `json:"userAgent" gorm:"size:500"`
	Location      string `json:"location" gorm:"size:200;comment:地理位置(粗略)"`
	LastActiveAt  int64  `json:"lastActiveAt" gorm:"index;comment:最后活跃时间"`
	Revoked       bool   `json:"revoked" gorm:"index;default:false;comment:是否已吊销"`
	RevokedReason string `json:"revokedReason" gorm:"size:200"`
}

func (Session) TableName() string { return "user_session" }

// RecordSession 创建会话记录(登录时调用)
func RecordSession(s *Session) {
	if err := CreateSession(s); err != nil {
		log.Errorf(context.Background(), "record session failed: %v", err)
	}
}

// CreateSession persists a login session and reports failures to callers that
// must not return a token unless the corresponding device is manageable.
func CreateSession(s *Session) error {
	if store.DB() == nil {
		return errors.New("session store is not initialized")
	}
	s.LastActiveAt = time.Now().Unix()
	return store.DB().Create(s).Error
}

// ListSessions 列出主体的活跃会话(不含已吊销)
func ListSessions(principalID string) (list []*Session, err error) {
	err = store.DB().Where("principal_id = ? AND revoked = ?", principalID, false).
		Order("last_active_at desc").Find(&list).Error
	return
}

// ListAllSessions 列出主体全部会话(含已吊销)
func ListAllSessions(principalID string) (list []*Session, err error) {
	err = store.DB().Where("principal_id = ?", principalID).
		Order("created_at desc").Find(&list).Error
	return
}

// RevokeSession 吊销单个会话(by session ID)
func RevokeSession(sessionID, reason string) error {
	return store.DB().Model(&Session{}).Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"revoked":        true,
			"revoked_reason": reason,
		}).Error
}

// RevokeSessionForPrincipal revokes one active session only when it belongs to
// the authenticated principal. Self-service account screens must use this
// scoped variant instead of the administrator-level RevokeSession helper.
func RevokeSessionForPrincipal(sessionID, principalID, reason string) (bool, error) {
	result := store.DB().Model(&Session{}).
		Where("id = ? AND principal_id = ? AND revoked = ?", sessionID, principalID, false).
		Updates(map[string]interface{}{
			"revoked":        true,
			"revoked_reason": reason,
		})
	return result.RowsAffected == 1, result.Error
}

// RevokeByTokenSig 按 token 签名吊销(Logout 时调用)
func RevokeByTokenSig(tokenSig, reason string) error {
	return store.DB().Model(&Session{}).Where("token_sig = ?", tokenSig).
		Updates(map[string]interface{}{
			"revoked":        true,
			"revoked_reason": reason,
		}).Error
}

// RotateTokenSignature keeps one manageable device session while replacing
// its access token. Matching the principal and previous signature prevents a
// stale or concurrent refresh from taking over another session.
func RotateTokenSignature(sessionID, principalID, oldSignature, newSignature string) (bool, error) {
	result := store.DB().Model(&Session{}).
		Where("id = ? AND principal_id = ? AND token_sig = ? AND revoked = ?", sessionID, principalID, oldSignature, false).
		Updates(map[string]interface{}{
			"token_sig":      newSignature,
			"last_active_at": time.Now().Unix(),
		})
	return result.RowsAffected == 1, result.Error
}

// RevokeAllByPrincipal 吊销主体全部会话("all devices" 登出)
func RevokeAllByPrincipal(principalID, exceptSessionID, reason string) (count int64, err error) {
	db := store.DB().Model(&Session{}).Where("principal_id = ? AND revoked = ?", principalID, false)
	if exceptSessionID != "" {
		db = db.Where("id != ?", exceptSessionID)
	}
	result := db.Updates(map[string]interface{}{
		"revoked":        true,
		"revoked_reason": reason,
	})
	return result.RowsAffected, result.Error
}

// UpdateActivity 更新会话活跃时间(每次请求调用,或定期批量)
func UpdateActivity(tokenSig string) {
	store.DB().Model(&Session{}).Where("token_sig = ? AND revoked = ?", tokenSig, false).
		Update("last_active_at", time.Now().Unix())
}

// IsRevoked 检查会话是否已吊销(鉴权中间件可调用)
func IsRevoked(tokenSig string) bool {
	var count int64
	store.DB().Model(&Session{}).Where("token_sig = ? AND revoked = ?", tokenSig, true).Count(&count)
	return count > 0
}

// CleanExpiredSessions 清理过期会话(定时任务调用)
func CleanExpiredSessions(maxAgeHours int) (count int64, err error) {
	cutoff := time.Now().Add(-time.Duration(maxAgeHours) * time.Hour)
	result := store.DB().Where("last_active_at < ?", cutoff.Unix()).Delete(&Session{})
	return result.RowsAffected, result.Error
}

// DetectAnomaly 异地登录检测(同主体短时间内不同 IP)
// 返回是否异常 + 上次登录 IP
func DetectAnomaly(principalID, currentIP string, windowMinutes int) (anomaly bool, lastIP string) {
	var sessions []*Session
	cutoff := time.Now().Add(-time.Duration(windowMinutes) * time.Minute).Unix()
	store.DB().Where("principal_id = ? AND revoked = ? AND last_active_at >= ?",
		principalID, false, cutoff).
		Order("last_active_at desc").Limit(1).Find(&sessions)
	if len(sessions) == 0 {
		return false, ""
	}
	if sessions[0].IP != "" && sessions[0].IP != currentIP {
		return true, sessions[0].IP
	}
	return false, sessions[0].IP
}

var _ = json.Marshal
var _ = gorm.ErrRecordNotFound
