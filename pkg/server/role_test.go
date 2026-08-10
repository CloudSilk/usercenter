package server

import (
	"testing"

	pkgdb "github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/permission"
	ucuser "github.com/CloudSilk/usercenter/internal/user"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func roleFacadeTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&permission.Role{}, &permission.RoleMenu{}, &permission.CasbinRule{}, &permission.ABACPolicy{}, &ucuser.UserRole{}); err != nil {
		t.Fatalf("migrate role tables: %v", err)
	}
	SetDB(pkgdb.NewDBClient(database, false))
	return database
}

func TestTenantProjectRoleFacadeScopesLifecycle(t *testing.T) {
	database := roleFacadeTestDB(t, "server-role-facade")
	created, err := CreateTenantProjectRole(CreateTenantProjectRoleRequest{
		TenantID: "tenant-a", ProjectID: "kinlegacy", Code: "CUSTOM_REVIEWER",
		DisplayName: "支系审核员", DefaultRouter: "/pages/home/index",
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if created.ID == "" || created.TenantID != "tenant-a" || created.ProjectID != "kinlegacy" || !created.Deletable {
		t.Fatalf("unexpected created role: %#v", created)
	}
	if !RoleBelongsToTenantProject(created.ID, "tenant-a", "kinlegacy") {
		t.Fatal("created role was not found in its tenant/project boundary")
	}
	if RoleBelongsToTenantProject(created.ID, "tenant-a", "another-project") ||
		RoleBelongsToTenantProject(created.ID, "tenant-b", "kinlegacy") {
		t.Fatal("role escaped its tenant/project boundary")
	}
	if _, err := CreateTenantProjectRole(CreateTenantProjectRoleRequest{
		TenantID: "tenant-a", ProjectID: "kinlegacy", Code: "CUSTOM_REVIEWER", DisplayName: "重复",
	}); err == nil {
		t.Fatal("duplicate tenant/project role code was accepted")
	}
	if _, err := RenameTenantProjectRole("tenant-b", "kinlegacy", created.ID, "越权名称"); err == nil {
		t.Fatal("cross-tenant rename was accepted")
	}
	renamed, err := RenameTenantProjectRole("tenant-a", "kinlegacy", created.ID, "二房审核员")
	if err != nil || renamed.DisplayName != "二房审核员" || renamed.Code != "CUSTOM_REVIEWER" {
		t.Fatalf("rename role: role=%#v err=%v", renamed, err)
	}
	roles, err := ListTenantProjectRoles("tenant-a", "kinlegacy")
	if err != nil || len(roles) != 1 || roles[0].ID != created.ID {
		t.Fatalf("list roles: roles=%#v err=%v", roles, err)
	}
	if err := database.Create(&permission.ABACPolicy{TenantID: "tenant-a", RoleID: created.ID, Resource: "person", Action: "access", Enable: true}).Error; err != nil {
		t.Fatalf("create role ABAC policy: %v", err)
	}
	if err := DeleteTenantProjectRole("tenant-b", "kinlegacy", created.ID); err == nil {
		t.Fatal("cross-tenant delete was accepted")
	}
	if err := DeleteTenantProjectRole("tenant-a", "kinlegacy", created.ID); err != nil {
		t.Fatalf("delete role: %v", err)
	}
	var abacCount int64
	if err := database.Model(&permission.ABACPolicy{}).Where("role_id = ?", created.ID).Count(&abacCount).Error; err != nil || abacCount != 0 {
		t.Fatalf("role ABAC policies remained: count=%d err=%v", abacCount, err)
	}
}

func TestPublishRoleAuthorizationWithABACRollsBackAllPolicies(t *testing.T) {
	database := roleFacadeTestDB(t, "server-role-authorization-atomic")
	if err := database.Exec(`CREATE TABLE tenant_menus (
		id TEXT PRIMARY KEY,
		tenant_id TEXT,
		menu_id TEXT,
		funcs TEXT,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create tenant menu table: %v", err)
	}
	created, err := CreateTenantProjectRole(CreateTenantProjectRoleRequest{
		TenantID: "tenant-a", ProjectID: "kinlegacy", Code: "CUSTOM_EDITOR",
		DisplayName: "支系编辑员", DefaultRouter: "/pages/home/index",
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := database.Create(&permission.RoleMenu{
		RoleID: created.ID, MenuID: "legacy-menu", Funcs: "read", Show: true,
	}).Error; err != nil {
		t.Fatalf("seed role menu: %v", err)
	}
	if err := database.Create(&permission.CasbinRule{
		Ptype: "p", RoleID: created.ID, Path: "/legacy", Method: "GET", CheckAuth: "true",
	}).Error; err != nil {
		t.Fatalf("seed Casbin rule: %v", err)
	}
	if err := database.Create(&permission.ABACPolicy{
		TenantID: "tenant-a", RoleID: created.ID, Resource: "kinlegacy.person", Action: "access",
		DataScope: 1, Condition: `{"type":"ALL"}`, Enable: true, Priority: 100,
	}).Error; err != nil {
		t.Fatalf("seed ABAC policy: %v", err)
	}
	if err := database.Create(&ucuser.UserRole{UserID: "user-a", RoleID: created.ID}).Error; err != nil {
		t.Fatalf("seed assigned user: %v", err)
	}
	detail, err := GetRoleAuthorization(created.ID)
	if err != nil {
		t.Fatalf("get current authorization: %v", err)
	}

	_, err = PublishRoleAuthorizationWithABAC(created.ID, detail.Revision, nil, RoleAuthorizationABACPolicy{
		TenantID: "tenant-a", Resource: "kinlegacy.person", Action: "access",
		DataScope: 2, Condition: `{"type":"SELF"}`, Priority: 50,
	})
	if err == nil {
		t.Fatal("publish unexpectedly succeeded without user_session table")
	}

	var menuCount, casbinCount int64
	if err := database.Model(&permission.RoleMenu{}).Where("role_id = ? AND menu_id = ?", created.ID, "legacy-menu").Count(&menuCount).Error; err != nil || menuCount != 1 {
		t.Fatalf("role menu was not rolled back: count=%d err=%v", menuCount, err)
	}
	if err := database.Model(&permission.CasbinRule{}).Where("ptype = ? AND v0 = ? AND v1 = ?", "p", created.ID, "/legacy").Count(&casbinCount).Error; err != nil || casbinCount != 1 {
		t.Fatalf("Casbin policy was not rolled back: count=%d err=%v", casbinCount, err)
	}
	var policy permission.ABACPolicy
	if err := database.Where("tenant_id = ? AND role_id = ? AND resource = ? AND action = ?",
		"tenant-a", created.ID, "kinlegacy.person", "access").First(&policy).Error; err != nil {
		t.Fatalf("load rolled-back ABAC policy: %v", err)
	}
	if policy.DataScope != 1 || policy.Condition != `{"type":"ALL"}` || policy.Priority != 100 {
		t.Fatalf("ABAC policy was not rolled back: %#v", policy)
	}
}
