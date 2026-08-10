package server

import (
	"testing"
	"time"

	pkgdb "github.com/CloudSilk/pkg/db"
	ucpermission "github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/session"
	uctenant "github.com/CloudSilk/usercenter/internal/tenant"
	ucuser "github.com/CloudSilk/usercenter/internal/user"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTenantAdministratorFacadeLifecycle(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:server-tenant-admin-facade?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&uctenant.Tenant{}, &uctenant.TenantMenu{}, &uctenant.TenantCertificate{}, &ucpermission.Role{}, &ucpermission.RoleMenu{}, &ucuser.User{}, &ucuser.UserRole{}, &session.Session{}); err != nil {
		t.Fatalf("migrate administrator tables: %v", err)
	}
	SetDB(pkgdb.NewDBClient(database, false))
	if _, err := CreateTenant(CreateTenantRequest{ID: "tenant-admin-a", Name: "租户管理员测试", Enabled: true, UserLimit: 5, ExpiresAt: time.Now().AddDate(1, 0, 0)}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	administrator, err := CreateTenantAdministrator(CreateTenantAdministratorRequest{
		TenantID: "tenant-admin-a", ProjectID: "manunexus", UserName: "tenant.owner",
		Nickname: "租户负责人", Email: "owner@example.com", Password: "TenantAdmin!123",
	})
	if err != nil {
		t.Fatalf("create administrator: %v", err)
	}
	if administrator.RoleID == "" || !administrator.Enabled {
		t.Fatalf("unexpected administrator: %#v", administrator)
	}
	administrators, err := ListTenantAdministrators("tenant-admin-a", "manunexus")
	if err != nil || len(administrators) != 1 || administrators[0].ID != administrator.ID {
		t.Fatalf("unexpected administrator list: %#v, %v", administrators, err)
	}
	if err := SetTenantAdministratorEnabled("tenant-admin-a", "manunexus", administrator.ID, false); err != nil {
		t.Fatalf("disable administrator: %v", err)
	}
	if err := ResetTenantAdministratorPassword("tenant-admin-a", "manunexus", administrator.ID, "ResetAdmin!456"); err != nil {
		t.Fatalf("reset administrator password: %v", err)
	}

	var stored ucuser.User
	if err := database.Where("id = ?", administrator.ID).First(&stored).Error; err != nil {
		t.Fatalf("load administrator: %v", err)
	}
	if stored.Enable || !stored.ForceChangePwd {
		t.Fatalf("administrator state not updated: %#v", stored)
	}
	if err := SetTenantAdministratorEnabled("another-tenant", "manunexus", administrator.ID, true); err == nil {
		t.Fatal("cross-tenant administrator update was accepted")
	}
}
