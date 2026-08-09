package bootstrap

import (
	"fmt"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	"gorm.io/gorm"
)

const selfServiceProjectID = "usercenter-self"

var selfServiceAPIs = []permission.API{
	{
		Model: commonmodel.Model{ID: "uc-self-profile-get"},
		Path:  "/api/core/auth/user/profile", Method: "GET",
		Group: "个人中心", Description: "读取当前用户资料",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-profile-put"},
		Path:  "/api/core/auth/user/profile", Method: "PUT",
		Group: "个人中心", Description: "更新当前用户资料",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-password-post"},
		Path:  "/api/core/auth/user/changepwd", Method: "POST",
		Group: "个人中心", Description: "修改当前用户密码",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-logout-post"},
		Path:  "/api/core/auth/user/logout", Method: "POST",
		Group: "个人中心", Description: "退出当前登录",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-token-refresh-post"},
		Path:  "/api/core/auth/user/token/refresh", Method: "POST",
		Group: "个人中心", Description: "续期当前登录",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-mfa-enroll"},
		Path:  "/api/core/auth/user/mfa/totp/enroll", Method: "POST",
		Group: "个人中心", Description: "开始绑定 TOTP 验证器",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-mfa-confirm"},
		Path:  "/api/core/auth/user/mfa/totp/confirm", Method: "POST",
		Group: "个人中心", Description: "确认绑定 TOTP 验证器",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-mfa-list"},
		Path:  "/api/core/auth/user/mfa/factors", Method: "GET",
		Group: "个人中心", Description: "读取当前用户 MFA 因子",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-mfa-delete"},
		Path:  "/api/core/auth/user/mfa/:id", Method: "DELETE",
		Group: "个人中心", Description: "解绑当前用户 MFA 因子",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-security-summary"},
		Path:  "/api/core/auth/user/security/summary", Method: "GET",
		Group: "个人中心", Description: "读取当前用户安全摘要",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-security-reverify"},
		Path:  "/api/core/auth/user/security/reverify", Method: "POST",
		Group: "个人中心", Description: "为高风险账户操作重新验证身份",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-security-session-all"},
		Path:  "/api/core/auth/user/security/sessions/revoke-all", Method: "POST",
		Group: "个人中心", Description: "退出当前用户的全部设备",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-security-session-delete"},
		Path:  "/api/core/auth/user/security/sessions/:id", Method: "DELETE",
		Group: "个人中心", Description: "停用当前用户自己的设备会话",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-security-tenant-switch"},
		Path:  "/api/core/auth/user/security/tenant/switch", Method: "POST",
		Group: "个人中心", Description: "切换当前用户的活动租户",
	},
	{
		Model: commonmodel.Model{ID: "uc-self-security-account-delete"},
		Path:  "/api/core/auth/user/security/account", Method: "DELETE",
		Group: "个人中心", Description: "重新验证后注销当前用户账号",
	},
}

// ensureSelfServiceAuthorization reconciles the authentication-only routes that
// every embedded UserCenter host needs. These APIs deliberately require a
// valid login but never a product role, so an ordinary customer can manage
// only their own account without being granted an administration menu.
func ensureSelfServiceAuthorization() error {
	db := store.DB()
	if db == nil {
		return fmt.Errorf("self-service authorization: store DB not initialised")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for index := range selfServiceAPIs {
			definition := selfServiceAPIs[index]
			definition.ProjectID = selfServiceProjectID
			definition.Enable = true
			definition.CheckAuth = false
			definition.CheckLogin = true
			definition.IsMust = true

			var current permission.API
			result := tx.Unscoped().
				Where("path = ? AND method = ?", definition.Path, definition.Method).
				Order("is_must DESC").
				Limit(1).
				Find(&current)
			switch {
			case result.Error != nil:
				return fmt.Errorf("query self-service API %s %s: %w", definition.Method, definition.Path, result.Error)
			case result.RowsAffected == 1:
				if err := tx.Unscoped().Model(&current).Updates(map[string]any{
					"deleted_at":  nil,
					"group":       definition.Group,
					"description": definition.Description,
					"enable":      true,
					"check_auth":  false,
					"check_login": true,
					"is_must":     true,
				}).Error; err != nil {
					return fmt.Errorf("update self-service API %s %s: %w", definition.Method, definition.Path, err)
				}
			case result.RowsAffected == 0:
				if err := tx.Create(&definition).Error; err != nil {
					return fmt.Errorf("create self-service API %s %s: %w", definition.Method, definition.Path, err)
				}
			}
		}
		return nil
	})
}
