package tenant

// DB 级租户强制隔离 GORM scope(REDESIGN #2)
// 在 DB 查询层自动追加 tenant_id 条件,而非依赖 handler 层手工过滤

import (
	"context"

	"gorm.io/gorm"
)

// ctxKey 租户上下文 key(不导出)
type ctxKey struct{}

// WithTenantContext 在 context 中注入租户 ID
func WithTenantContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, tenantID)
}

// TenantFromContext 从 context 提取租户 ID
func TenantFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}

// IsPlatformTenant 判断是否为平台租户(跳过隔离)
type PlatformChecker func(tenantID string) bool

// TenantScope GORM 插件:自动为所有查询追加 tenant_id 条件
type TenantScope struct {
	PlatformChecker PlatformChecker
}

// NewTenantScope 创建租户 scope 插件
func NewTenantScope(checker PlatformChecker) *TenantScope {
	return &TenantScope{PlatformChecker: checker}
}

// Apply 为 GORM 查询附加租户条件
// 调用方:db.Scopes(tenantScope.Apply(ctx)).Find(...)
func (ts *TenantScope) Apply(ctx context.Context) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		tenantID := TenantFromContext(ctx)
		// 平台租户或无租户上下文 = 不过滤
		if tenantID == "" || (ts.PlatformChecker != nil && ts.PlatformChecker(tenantID)) {
			return db
		}
		return db.Where("tenant_id = ?", tenantID)
	}
}

// ApplyToCreate 为写入附加租户值
func (ts *TenantScope) ApplyToCreate(ctx context.Context) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		tenantID := TenantFromContext(ctx)
		if tenantID == "" {
			return db
		}
		// 通过 Set 设置 gorm:context_tenant_id,模型 BeforeCreate 钩子可读取
		return db.Set("tenant_scope:id", tenantID)
	}
}
