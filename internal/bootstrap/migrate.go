// Package bootstrap — DB migration orchestration.
//
// This file relocates AutoMigrate + casbin seeding from the legacy model/init.go.
// It lives in bootstrap (not internal/store) because it must reference every
// domain package's struct types, and every domain package imports internal/store
// — putting AutoMigrate in store would create a store→domain→store import cycle.
package bootstrap

import (
	"fmt"

	"github.com/CloudSilk/usercenter/internal/aicache"
	"github.com/CloudSilk/usercenter/internal/apikeyauth"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/app"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/conversation"
	"github.com/CloudSilk/usercenter/internal/dictionaries"
	"github.com/CloudSilk/usercenter/internal/formcomponent"
	"github.com/CloudSilk/usercenter/internal/gatewaylog"
	"github.com/CloudSilk/usercenter/internal/identity"
	"github.com/CloudSilk/usercenter/internal/language"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/pricing"
	"github.com/CloudSilk/usercenter/internal/project"
	"github.com/CloudSilk/usercenter/internal/prompt"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/systemconfig"
	"github.com/CloudSilk/usercenter/internal/webhook"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/usage"
	"github.com/CloudSilk/usercenter/internal/user"
	"github.com/CloudSilk/usercenter/internal/wechatconfig"
	"github.com/CloudSilk/usercenter/internal/website"
)

// AutoMigrate builds the schema for every domain table in one shot.
// Mirrors the legacy model.AutoMigrate() type list; types now reference their
// canonical home in internal/*.
func AutoMigrate() error {
	db := store.DB()
	if db == nil {
		return fmt.Errorf("AutoMigrate: store DB not initialised (call store.SetDB first)")
	}
	return db.AutoMigrate(
		// permission domain
		&permission.CasbinRule{}, &permission.API{}, &permission.Menu{}, &permission.MenuParameter{},
		&permission.MenuFunc{}, &permission.MenuFuncApi{}, &permission.Role{}, &permission.RoleMenu{},
		&permission.ABACPolicy{},
		// user domain
		&user.User{}, &user.UserRole{}, &user.UserWechatOpenIDMap{},
		// app domain
		&app.APP{}, &app.APPProp{},
		// tenant domain
		&tenant.Tenant{}, &tenant.TenantMenu{}, &tenant.TenantCertificate{},
		// form component domain
		&formcomponent.FormComponent{}, &formcomponent.FormComponentResource{},
		// project domain
		&project.Project{}, &project.ProjectFormComponent{},
		// misc domains
		&dictionaries.Dictionaries{}, &language.Language{}, &systemconfig.SystemConfig{},
		&website.WebSite{}, &wechatconfig.WechatConfig{}, &audit.AuditLog{},

		// REDESIGN 新增域表：AI Key/路由、用量计量、会话、MFA、OAuth、Prompt 模板。
		// 此前这些表不在迁移清单内，全新部署的库访问对应功能会报 Table doesn't exist。
		&apikey.AIProvider{}, &apikey.AIKey{}, &apikey.ModelRoute{},
		&usage.UsageRecord{}, &usage.UsageBudget{},
		&session.Session{},
		&auth.MFAFactor{}, &auth.RefreshToken{}, &auth.OAuthClient{}, &auth.ConsentRecord{},
		&prompt.PromptTemplate{},
		&identity.UserExternalIdentity{},
		&pricing.ModelPrice{},
		// AI 网关增强：对话会话、请求日志、语义缓存。
		&conversation.Session{}, &conversation.Message{},
		&gatewaylog.GatewayLog{},
		&aicache.CacheEntry{},
		// API Key authentication for external services
		&apikeyauth.APIKeyAuth{},
		// Webhook 事件订阅
		&webhook.Subscription{},
	)
}

// RunMigration runs AutoMigrate followed by the casbin rule seeding
// (InitCasbin + the not-check-auth / not-check-login rule refresh).
// Equivalent to the legacy model.initDB(true) path.
func RunMigration() error {
	if err := AutoMigrate(); err != nil {
		return fmt.Errorf("AutoMigrate: %w", err)
	}
	permission.InitCasbin()
	permission.UpdateNotCheckAuthRule()
	permission.UpdateNotCheckLoginRule()
	return nil
}
