package authorization

import (
	"strings"
	"testing"

	"github.com/CloudSilk/pkg/constants"
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

func TestApplyAuthorizationCatalogPreservesPublishedABACPolicy(t *testing.T) {
	gdb := setupAuthorizationCatalogTestDB(t)
	require.NoError(t, gdb.Create(&tenant.Tenant{
		Model: commonmodel.Model{ID: "platform"}, Name: "平台", Enable: true, IsMust: true,
	}).Error)
	catalog := testAuthorizationCatalog()
	_, err := Apply(catalog)
	require.NoError(t, err)

	customCondition := `{"type":"SELF"}`
	require.NoError(t, gdb.Model(&permission.ABACPolicy{}).
		Where("tenant_id = ? AND role_id = ? AND resource = ? AND action = ?", "platform", "2", "user", "read").
		Updates(map[string]any{"data_scope": int32(permission.DataScopeSelf), "condition": customCondition}).Error)

	_, err = Apply(catalog)
	require.NoError(t, err)
	var policy permission.ABACPolicy
	require.NoError(t, gdb.Where("tenant_id = ? AND role_id = ? AND resource = ? AND action = ?", "platform", "2", "user", "read").First(&policy).Error)
	require.Equal(t, int32(permission.DataScopeSelf), policy.DataScope)
	require.Equal(t, customCondition, policy.Condition)
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

func TestApplyAuthorizationCatalogPreservesPublishedTenantMenuBoundary(t *testing.T) {
	gdb := setupAuthorizationCatalogTestDB(t)
	originalPlatformTenantID := constants.PlatformTenantID
	constants.SetPlatformTenantID("catalog-platform")
	t.Cleanup(func() { constants.SetPlatformTenantID(originalPlatformTenantID) })

	const targetTenantID = "catalog-tenant"
	require.NoError(t, gdb.Create([]*tenant.Tenant{
		{Model: commonmodel.Model{ID: constants.PlatformTenantID}, Name: "Platform", Enable: true, IsMust: true},
		{Model: commonmodel.Model{ID: targetTenantID}, Name: "Tenant", Enable: true},
	}).Error)

	catalog := testAuthorizationCatalog()
	catalog.TenantID = targetTenantID
	catalog.DefaultRoleID = ""
	for i := range catalog.TenantGrants {
		catalog.TenantGrants[i].TenantID = targetTenantID
	}
	for i := range catalog.ABACPolicies {
		catalog.ABACPolicies[i].TenantID = targetTenantID
	}
	_, err := Apply(catalog)
	require.NoError(t, err)

	detail, err := permission.GetTenantMenuAuthorization(constants.PlatformTenantID, targetTenantID)
	require.NoError(t, err)
	result, err := permission.PublishTenantMenuAuthorization(
		constants.PlatformTenantID,
		targetTenantID,
		detail.Revision,
		[]permission.TenantMenuAuthorizationSelection{{MenuID: "admin-users", Funcs: []string{"view"}}},
	)
	require.NoError(t, err)
	require.Equal(t, 1, result.Summary.SelectedMenuCount)
	require.Equal(t, 1, result.Summary.SelectedFunctionCount)

	_, err = Apply(catalog)
	require.NoError(t, err)

	after, err := permission.GetTenantMenuAuthorization(constants.PlatformTenantID, targetTenantID)
	require.NoError(t, err)
	require.Equal(t, result.Revision, after.Revision)
	require.Equal(t, []permission.TenantMenuAuthorizationSelection{
		{MenuID: "admin-users", Funcs: []string{"view"}},
	}, after.Selections)

	var roleMenus []permission.RoleMenu
	require.NoError(t, gdb.Order("role_id ASC, menu_id ASC").Find(&roleMenus).Error)
	require.Len(t, roleMenus, 2)
	for _, roleMenu := range roleMenus {
		require.Equal(t, "admin-users", roleMenu.MenuID)
		require.Equal(t, "view", roleMenu.Funcs)
	}

	var policies []permission.CasbinRule
	require.NoError(t, gdb.Where("ptype = ? AND v0 IN ?", "p", []string{"1", "2", "3"}).Order("v0 ASC").Find(&policies).Error)
	require.Len(t, policies, 2)
	for _, policy := range policies {
		require.Equal(t, "/api/core/auth/user/query", policy.Path)
		require.Equal(t, "GET", policy.Method)
	}
}

func TestValidateAuthorizationCatalogRejectsOverlongStableIDs(t *testing.T) {
	overlongID := strings.Repeat("x", authorizationCatalogIDMaxLength+1)
	tests := []struct {
		name       string
		update     func(*AuthorizationCatalog)
		errorLabel string
	}{
		{
			name: "tenant",
			update: func(catalog *AuthorizationCatalog) {
				catalog.TenantID = overlongID
			},
			errorLabel: "authorization catalog tenant ID",
		},
		{
			name: "project",
			update: func(catalog *AuthorizationCatalog) {
				catalog.ProjectID = overlongID
			},
			errorLabel: "authorization catalog project ID",
		},
		{
			name: "role",
			update: func(catalog *AuthorizationCatalog) {
				catalog.Roles[0].ID = overlongID
			},
			errorLabel: "authorization role ID",
		},
		{
			name: "menu",
			update: func(catalog *AuthorizationCatalog) {
				catalog.Menus[0].ID = overlongID
			},
			errorLabel: "authorization menu ID",
		},
		{
			name: "function",
			update: func(catalog *AuthorizationCatalog) {
				catalog.Menus[0].Functions[0].ID = overlongID
			},
			errorLabel: "authorization function ID",
		},
		{
			name: "API",
			update: func(catalog *AuthorizationCatalog) {
				catalog.APIs[0].ID = overlongID
			},
			errorLabel: "authorization API ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := testAuthorizationCatalog()
			tt.update(&catalog)

			err := validateAuthorizationCatalog(normalizeAuthorizationCatalog(catalog))

			require.Error(t, err)
			require.ErrorContains(t, err, tt.errorLabel)
			require.ErrorContains(t, err, "exceeds the 36-character database limit")
		})
	}
}

func TestValidateAuthorizationCatalogAcceptsMaximumStableIDLength(t *testing.T) {
	catalog := testAuthorizationCatalog()
	maximumID := strings.Repeat("x", authorizationCatalogIDMaxLength)
	originalID := catalog.APIs[0].ID
	catalog.APIs[0].ID = maximumID
	for menuIndex := range catalog.Menus {
		for functionIndex := range catalog.Menus[menuIndex].Functions {
			for apiIndex, apiID := range catalog.Menus[menuIndex].Functions[functionIndex].APIIDs {
				if apiID == originalID {
					catalog.Menus[menuIndex].Functions[functionIndex].APIIDs[apiIndex] = maximumID
				}
			}
		}
	}

	require.NoError(
		t,
		validateAuthorizationCatalog(normalizeAuthorizationCatalog(catalog)),
	)
}

func TestValidateAuthorizationCatalogRejectsUnsupportedDataScope(t *testing.T) {
	catalog := testAuthorizationCatalog()
	catalog.ABACPolicies[0].DataScope = 99

	err := validateAuthorizationCatalog(normalizeAuthorizationCatalog(catalog))
	require.ErrorContains(t, err, "unsupported data scope 99")
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
