// Package setup 对外暴露 usercenter 内部包的核心启动功能，供嵌入式 consumer（如 banlu-wenshu）使用。
//
// 这是 usercenter 与外部业务模块之间的官方桥接包，封装了 internal/* 包的必要调用入口，
// 避免了 Go 的 `internal` 包可见性限制。
package setup

import (
	"time"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/apikeyauth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/bootstrap"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
)

// InitDB 注入数据库连接并执行 AutoMigrate。
func InitDB(client db.DBClientInterface) {
	store.SetDB(client)
	if err := bootstrap.AutoMigrate(); err != nil {
		panic(err)
	}
}

// RunMigration 执行 usercenter 的 AutoMigrate。
func RunMigration() {
	if err := bootstrap.AutoMigrate(); err != nil {
		panic(err)
	}
}

// RunFullMigration 执行 usercenter 的完整迁移，包括 Casbin 权限策略初始化。
// 嵌入式 consumer 需要在 InitDB 之后调用此函数，以确保鉴权中间件可用。
func RunFullMigration() {
	if err := bootstrap.RunMigration(); err != nil {
		panic(err)
	}
}

// DB 返回当前 *gorm.DB 实例。
func DB() *gorm.DB {
	return store.DB()
}

// InitKeys 初始化 JWT、AI Key 加密、PII 加密、OIDC RSA 四套独立密钥体系。
func InitKeys(tokenKey string, tokenExpiredMinutes int) {
	bootstrap.InitKeys(bootstrap.Keys{
		TokenKey:     tokenKey,
		TokenExpired: tokenExpiredMinutes,
	})
}

// InitConstants 设置平台常量：默认租户、超级管理员角色 ID、默认角色 ID、租户启用、登录锁。
func InitConstants(platformTenantID, superAdminRoleID, defaultRoleID string, enableTenant bool) {
	bootstrap.InitConstants(bootstrap.Constants{
		PlatformTenantID: platformTenantID,
		SuperAdminRoleID: superAdminRoleID,
		DefaultRoleID:    defaultRoleID,
		EnableTenant:     enableTenant,
	})
}

// SeedAdmin 幂等创建初始管理员。返回实际设置的口令。
func SeedAdmin(platformTenantID, superAdminRoleID, pwd string) string {
	seeded, generated, err := bootstrap.SeedBootstrapAdmin(platformTenantID, superAdminRoleID, pwd)
	if err != nil {
		panic(err)
	}
	if !seeded {
		return ""
	}
	if generated != "" {
		return generated
	}
	return pwd
}

// SeedDataIfEmpty 占位：基础字典播种由 usercenter.SeedAdmin 间接完成。
func SeedDataIfEmpty() {
	// no-op: 在当前 usercenter 版本中无需额外播种
}

// WaitForDB 阻塞直到 DB 可用或超时（单位秒）。
func WaitForDB(timeoutSeconds int) {
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	for time.Now().Before(deadline) {
		if store.DB() != nil {
			sqlDB, err := store.DB().DB()
			if err == nil && sqlDB.Ping() == nil {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// SetPasswordConfig 配置用户的密码策略。
func SetPasswordConfig(defaultPwd string, expiredDays int) {
	user.SetDefaultPwd(defaultPwd)
	// 当前 usercenter 版本无 SetPwdExpiredDays；过期天数策略留待后续版本接入。
}

// --- AI 网关桥接 ---

// AIProvider 是 AI 服务商的对外类型别名。
type AIProvider = apikey.AIProvider

// AIKey 是 AI API Key 的对外类型别名。
type AIKey = apikey.AIKey

// ModelRoute 是模型路由的对外类型别名。
type ModelRoute = apikey.ModelRoute

// KeySelection 是模型选 key 结果的对外类型别名。
type KeySelection = apikey.KeySelection

// CreateAIProvider 创建 AI 服务商。
func CreateAIProvider(p *AIProvider) (string, error) {
	return apikey.CreateProvider(p)
}

// CreateAIKey 创建并加密存储 API Key。
func CreateAIKey(k *AIKey, plaintext string) (string, error) {
	return apikey.CreateKey(k, plaintext)
}

// CreateModelRoute 创建模型路由。
func CreateModelRoute(r *ModelRoute) (string, error) {
	return apikey.CreateRoute(r)
}

// SelectAIKey 根据模型别名选择可用 Key。
func SelectAIKey(tenantID, modelAlias string) (*KeySelection, error) {
	return apikey.SelectKey(tenantID, modelAlias)
}

// CountModelRoutes 返回指定租户下某模型路由的数量。
func CountModelRoutes(tenantID, modelAlias string) int64 {
	var count int64
	if db := DB(); db != nil {
		db.Model(&ModelRoute{}).Where("tenant_id = ? AND model_alias = ?", tenantID, modelAlias).Count(&count)
	}
	return count
}

// APIKeyAuth 是对外类型别名。
type APIKeyAuth = apikeyauth.APIKeyAuth

// CreateAPIKeyAuth 创建长期 API Key，返回明文 key（仅一次）。
func CreateAPIKeyAuth(k *APIKeyAuth) (string, error) {
	return apikeyauth.CreateKey(k)
}

// ValidateAPIKeyAuth 校验 API Key 是否有效。
func ValidateAPIKeyAuth(plaintext string) (*APIKeyAuth, error) {
	return apikeyauth.ValidateKey(plaintext)
}

// DecodeToken 解码 JWT access_token,返回 CurrentUser。
// 供嵌入式 consumer 的鉴权中间件使用(无需 Dubbo/Triple RPC)。
// token 无效或过期时返回 error。
func DecodeToken(t string) (*apipb.CurrentUser, error) {
	return token.DecodeToken(t)
}
