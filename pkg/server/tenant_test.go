package server

import (
	"testing"
	"time"

	pkgdb "github.com/CloudSilk/pkg/db"
	uctenant "github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTenantFacadeLifecycle(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:server-tenant-facade?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&uctenant.Tenant{}, &uctenant.TenantMenu{}, &uctenant.TenantCertificate{}); err != nil {
		t.Fatalf("migrate tenant tables: %v", err)
	}
	SetDB(pkgdb.NewDBClient(database, false))

	expiresAt := time.Now().UTC().AddDate(0, 1, 0).Truncate(time.Second)
	created, err := CreateTenant(CreateTenantRequest{
		ID: "tenant-a", Name: "租户 A", Enabled: true, UserLimit: 50, ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if created.ID != "tenant-a" || !created.Enabled || created.UserLimit != 50 {
		t.Fatalf("unexpected created tenant: %#v", created)
	}

	renewedAt := expiresAt.AddDate(1, 0, 0)
	updated, err := UpdateTenant(UpdateTenantRequest{ID: created.ID, Name: "租户 A 正式版", UserLimit: 100, ExpiresAt: renewedAt})
	if err != nil {
		t.Fatalf("update tenant: %v", err)
	}
	if updated.Name != "租户 A 正式版" || updated.UserLimit != 100 || !updated.ExpiresAt.Equal(renewedAt) {
		t.Fatalf("unexpected updated tenant: %#v", updated)
	}
	if err := SetTenantEnabled(created.ID, false); err != nil {
		t.Fatalf("disable tenant: %v", err)
	}
	stored, err := GetTenant(created.ID)
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if stored.Enabled {
		t.Fatal("tenant remained enabled")
	}
}
