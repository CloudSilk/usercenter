// Package server 是 usercenter 对外的公开入口包。
//
// 背景：usercenter 的启动逻辑（store 注入、bootstrap 迁移与初始化、密钥/常量/种子）
// 都位于 internal/* 下。Go 的 internal 规则禁止其他模块 import internal 包。
// 当 usercenter 作为库被另一个独立 Go 模块（如 CapsiFarm 单体二进制）内嵌复用时，
// 宿主模块无法直接调用 internal/bootstrap。
//
// 本包作为薄转发层（thin facade），把宿主需要的启动能力重新导出，转发给
// internal/bootstrap 与 internal/store。包内不做业务逻辑，只做参数透传与类型映射，
// 以便 usercenter 内部演进时仅此一处需同步调整。
//
// 典型用法（宿主 main.go）：
//
//	dbClient := sqlite.NewSqlite("./app.db?_pragma=journal_mode(WAL)", true)
//	server.SetDB(dbClient)                       // 1. 注入 GORM 客户端
//	if err := server.RunMigration(); err != nil { // 2. usercenter 全部表迁移
//	    log.Fatal(err)
//	}
//	server.InitKeys(server.Keys{TokenKey: "...", TokenExpired: 1440})
//	server.InitConstants(server.Constants{PlatformTenantID: "platform", SuperAdminRoleID: "1", EnableTenant: true, LoginLockMaxErr: 5, LoginLockMinutes: 15})
//	server.SeedAdmin("platform", "1", "")        // 3. 首次部署播种超管（幂等）
//	// 4. 宿主再做自己业务表的 AutoMigrate、挂业务路由、ListenAndServe
//
// 路由注册、鉴权中间件本身在 public 包（usercenter/http、usercenter/utils/middleware），
// 宿主可直接 import，无需本包转发。
package server

import (
	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/bootstrap"
	"github.com/CloudSilk/usercenter/internal/store"
	"gorm.io/gorm"
)

// SetDB 注入全局 GORM 客户端，供 usercenter 所有 internal 域包使用。
// 必须在 RunMigration / InitKeys 之前调用。
func SetDB(client db.DBClientInterface) {
	store.SetDB(client)
}

// DB 返回当前 GORM DB 句柄。宿主可据此对自己业务表再做 AutoMigrate。
func DB() *gorm.DB {
	return store.DB()
}

// Keys 转发 bootstrap.Keys：三套独立密钥（JWT/AI Key 加密/PII 加密）+ OIDC RSA 密钥。
// 留空的密钥槽会从 TokenKey 派生（SHA-256），OIDCSigningKey 留空则自动生成 RSA-2048。
type Keys = bootstrap.Keys

// InitKeys 转发 bootstrap.InitKeys：初始化 JWT 缓存、AI Key 加密、PII 加密、OIDC RSA。
// 进程内须在 DB 就绪后调用一次。
func InitKeys(k Keys) {
	bootstrap.InitKeys(k)
}

// Constants 转发 bootstrap.Constants：平台租户、超管角色、默认角色、租户开关、
// 登录锁定策略、默认口令等全局常量。
type Constants = bootstrap.Constants

// InitConstants 转发 bootstrap.InitConstants：设置全局平台常量。
func InitConstants(c Constants) {
	bootstrap.InitConstants(c)
}

// RunMigration 转发 bootstrap.RunMigration：对所有 usercenter 域表执行 AutoMigrate，
// 并初始化 Casbin + 刷新免鉴权规则。幂等，可重复执行。
func RunMigration() error {
	return bootstrap.RunMigration()
}

// AutoMigrate 仅迁移 usercenter 域表（不初始化 Casbin）。一般用 RunMigration 即可。
func AutoMigrate() error {
	return bootstrap.AutoMigrate()
}

// SeedAdmin 转发 bootstrap.SeedAdmin：首次部署幂等播种平台租户、超管角色与初始管理员。
// defaultPwd 为空则随机生成强口令并打印到 stdout，首登强制改密。
func SeedAdmin(platformTenantID, superAdminRoleID, defaultPwd string) {
	bootstrap.SeedAdmin(platformTenantID, superAdminRoleID, defaultPwd)
}

// SeedBootstrapAdmin 转发 bootstrap.SeedBootstrapAdmin：SeedAdmin 的底层实现，
// 返回是否实际播种 (seeded) 与生成的随机口令 (generatedPwd)。
// 已有用户时 seeded=false，为 no-op。
func SeedBootstrapAdmin(tenantID, superAdminRoleID, pwd string) (seeded bool, generatedPwd string, err error) {
	return bootstrap.SeedBootstrapAdmin(tenantID, superAdminRoleID, pwd)
}

// CheckKeyIsolation 转发 bootstrap.CheckKeyIsolation：检测三套密钥是否被误配为相同，
// 任意两者相同则 panic。生产部署建议在 InitKeys 后调用。
func CheckKeyIsolation(tokenKey, apiKeyEncKey, piiEncKey string) {
	bootstrap.CheckKeyIsolation(tokenKey, apiKeyEncKey, piiEncKey)
}

// SocialLoginConfig 转发 bootstrap.SocialLoginConfig：社交登录提供方配置。
type SocialLoginConfig = bootstrap.SocialLoginConfig

// SetSocialLogins 转发 bootstrap.SetSocialLogins：注册社交登录提供方（GitHub/Google 等）。
func SetSocialLogins(cfgs []SocialLoginConfig) {
	bootstrap.SetSocialLogins(cfgs)
}

// SetAlertWebhook 转发 bootstrap.SetAlertWebhook：配置告警 Webhook（飞书/Slack 等）。
func SetAlertWebhook(url string) {
	bootstrap.SetAlertWebhook(url)
}

// ---------- 密码哈希（供宿主种子播种用户时使用）----------

// EncryptPassword 转发 auth.EncryptedPassword：用 scrypt 对密码哈希。
// 宿主在播种初始用户时，须先调用本函数得到哈希再写入 users.password。
func EncryptPassword(password string) (string, error) {
	return auth.EncryptedPassword(password)
}

// ValidPasswordStrength 转发 auth.ValidPasswdStrength：校验密码强度
// （>=8 位 + 数字 + 小写 + 大写）。首登改密时可用。
func ValidPasswordStrength(str string) bool {
	return auth.ValidPasswdStrength(str)
}
