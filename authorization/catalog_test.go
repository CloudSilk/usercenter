package authorization

import (
	"testing"

	"github.com/CloudSilk/pkg/db"
	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAuthorizationCatalogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(
		&permission.CasbinRule{},
		&permission.API{},
		&permission.Menu{},
		&permission.MenuFunc{},
		&permission.MenuFuncApi{},
		&permission.Role{},
		&permission.RoleMenu{},
		&permission.ABACPolicy{},
		&tenant.Tenant{},
		&tenant.TenantMenu{},
		&user.User{},
		&user.UserRole{},
	))
	store.SetDB(db.NewDBClient(gdb, false))
	return gdb
}

func testAuthorizationCatalog() AuthorizationCatalog {
	return AuthorizationCatalog{
		TenantID:      "platform",
		DefaultRoleID: "3",
		Roles: []AuthorizationRole{
			{ID: "1", Name: "super_admin", DefaultRouter: "admin-dashboard"},
			{ID: "2", Name: "tenant_admin", DefaultRouter: "admin-dashboard"},
			{ID: "3", Name: "normal_user", DefaultRouter: "workspace"},
		},
		APIs: []AuthorizationAPI{
			{
				ID: "api-users-query", Path: "/api/core/auth/user/query", Method: "GET",
				Group: "用户管理", Description: "查询用户", CheckAuth: true, CheckLogin: true,
			},
			{
				ID: "api-user-me", Path: "/api/wenshu/user/me", Method: "GET",
				Group: "工作台", Description: "当前用户", CheckAuth: false, CheckLogin: true,
			},
		},
		Menus: []AuthorizationMenu{
			{
				ID: "workspace", Name: "workspace", Title: "智能工作台", Path: "/",
				Functions: []AuthorizationFunction{{
					ID: "workspace-view", Name: "view", Title: "查看", APIIDs: []string{"api-user-me"},
				}},
			},
			{
				ID: "admin-users", Name: "admin-users", Title: "用户管理", Path: "/admin/users/",
				Functions: []AuthorizationFunction{{
					ID: "admin-users-view", Name: "view", Title: "查看", APIIDs: []string{"api-users-query"},
				}},
			},
		},
		RoleGrants: []AuthorizationRoleGrant{
			{RoleID: "1", MenuID: "workspace", Functions: []string{"view"}, Show: true},
			{RoleID: "1", MenuID: "admin-users", Functions: []string{"view"}, Show: true},
			{RoleID: "2", MenuID: "admin-users", Functions: []string{"view"}, Show: true},
			{RoleID: "3", MenuID: "workspace", Functions: []string{"view"}, Show: true},
		},
		TenantGrants: []AuthorizationTenantGrant{
			{TenantID: "platform", MenuID: "workspace", Functions: []string{"view"}},
			{TenantID: "platform", MenuID: "admin-users", Functions: []string{"view"}},
		},
		ABACPolicies: []AuthorizationABACPolicy{
			{
				TenantID: "platform", RoleID: "2", Resource: "user", Action: "read",
				DataScope: int32(permission.DataScopeTenant), Priority: 100,
			},
			{
				TenantID: "platform", RoleID: "3", Resource: "user", Action: "read",
				DataScope: int32(permission.DataScopeSelf), Priority: 100,
			},
		},
	}
}

func TestApplyAuthorizationCatalogSeedsCompleteNativeGraphIdempotently(t *testing.T) {
	gdb := setupAuthorizationCatalogTestDB(t)
	require.NoError(t, gdb.Create(&tenant.Tenant{
		Model: commonmodel.Model{ID: "platform"}, Name: "平台", Enable: true, IsMust: true,
	}).Error)
	require.NoError(t, gdb.Create(&user.User{
		TenantModel: commonmodel.TenantModel{Model: commonmodel.Model{ID: "admin"}, TenantID: "platform"},
		UserName:    "admin", Nickname: "管理员", Enable: true,
	}).Error)
	require.NoError(t, gdb.Create(&user.User{
		TenantModel: commonmodel.TenantModel{Model: commonmodel.Model{ID: "writer"}, TenantID: "platform"},
		UserName:    "writer", Nickname: "撰稿人", Enable: true,
	}).Error)
	require.NoError(t, gdb.Create(&user.UserRole{UserID: "admin", RoleID: "1"}).Error)

	first, err := Apply(testAuthorizationCatalog())
	require.NoError(t, err)
	require.Equal(t, 3, first.RoleCount)
	require.Equal(t, 2, first.MenuCount)
	require.Equal(t, 2, first.FunctionCount)
	require.Equal(t, 2, first.APICount)
	require.Equal(t, 1, first.DefaultRoleUsersAdded)
	require.Equal(t, 4, first.CasbinRuleCount)

	second, err := Apply(testAuthorizationCatalog())
	require.NoError(t, err)
	require.Equal(t, 0, second.DefaultRoleUsersAdded)

	assertCatalogCount := func(model any, expected int64) {
		t.Helper()
		var count int64
		require.NoError(t, gdb.Model(model).Count(&count).Error)
		require.Equal(t, expected, count)
	}
	assertCatalogCount(&permission.Role{}, 3)
	assertCatalogCount(&permission.Menu{}, 2)
	assertCatalogCount(&permission.MenuFunc{}, 2)
	assertCatalogCount(&permission.MenuFuncApi{}, 2)
	assertCatalogCount(&permission.API{}, 2)
	assertCatalogCount(&permission.RoleMenu{}, 4)
	assertCatalogCount(&tenant.TenantMenu{}, 2)
	assertCatalogCount(&permission.ABACPolicy{}, 2)

	var writerRoles []user.UserRole
	require.NoError(t, gdb.Where("user_id = ?", "writer").Find(&writerRoles).Error)
	require.Len(t, writerRoles, 1)
	require.Equal(t, "3", writerRoles[0].RoleID)

	var policies []permission.CasbinRule
	require.NoError(t, gdb.Where("v0 IN ?", []string{"1", "2", "3"}).Find(&policies).Error)
	require.Len(t, policies, 4)
}

func TestApplyAuthorizationCatalogRestoresSystemRowsAndPreservesCustomData(t *testing.T) {
	gdb := setupAuthorizationCatalogTestDB(t)
	require.NoError(t, gdb.Create(&tenant.Tenant{
		Model: commonmodel.Model{ID: "platform"}, Name: "平台", Enable: true, IsMust: true,
	}).Error)
	custom := &permission.Role{
		Model: commonmodel.Model{ID: "custom-reviewer"}, TenantID: "platform",
		Name: "custom-reviewer", Enable: true, CanDel: true,
	}
	require.NoError(t, gdb.Create(custom).Error)

	catalog := testAuthorizationCatalog()
	_, err := Apply(catalog)
	require.NoError(t, err)
	require.NoError(t, gdb.Delete(&permission.Menu{}, "id = ?", "admin-users").Error)

	_, err = Apply(catalog)
	require.NoError(t, err)

	var restored permission.Menu
	require.NoError(t, gdb.First(&restored, "id = ?", "admin-users").Error)
	require.True(t, restored.IsMust)

	var customCount int64
	require.NoError(t, gdb.Model(&permission.Role{}).Where("id = ?", custom.ID).Count(&customCount).Error)
	require.Equal(t, int64(1), customCount)
}

func TestEnsureUserRoleRejectsCrossTenantRole(t *testing.T) {
	gdb := setupAuthorizationCatalogTestDB(t)
	require.NoError(t, gdb.Create(&user.User{
		TenantModel: commonmodel.TenantModel{Model: commonmodel.Model{ID: "user-a"}, TenantID: "tenant-a"},
		UserName:    "user-a", Nickname: "A", Enable: true,
	}).Error)
	require.NoError(t, gdb.Create(&permission.Role{
		Model: commonmodel.Model{ID: "role-b"}, TenantID: "tenant-b", Name: "role-b", Enable: true,
	}).Error)

	require.Error(t, EnsureUserRole("user-a", "role-b"))
}
