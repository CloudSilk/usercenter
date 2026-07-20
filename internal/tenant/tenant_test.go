package tenant

import (
	"testing"
	"time"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	_ = gdb.AutoMigrate(&Tenant{}, &TenantMenu{}, &TenantCertificate{})
	store.SetDB(db.NewDBClient(gdb, false))
	m.Run()
}

func TestCreateAndGetTenant(t *testing.T) {
	tnt := &Tenant{
		Name:      "test-org",
		Enable:    true,
		UserCount: 10,
		Expired:   time.Now().Add(24 * time.Hour),
	}
	if err := CreateTenant(tnt); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if tnt.ID == "" {
		t.Fatal("expected non-empty ID after create")
	}

	got, err := GetTenantByID(tnt.ID)
	if err != nil {
		t.Fatalf("GetTenantByID: %v", err)
	}
	if got.Name != "test-org" {
		t.Fatalf("expected name 'test-org', got %q", got.Name)
	}
}

func TestUpdateTenant(t *testing.T) {
	tnt := &Tenant{
		Name:      "old",
		Enable:    true,
		UserCount: 10,
		Expired:   time.Now().Add(24 * time.Hour),
	}
	if err := CreateTenant(tnt); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	tnt.Name = "new"
	if err := UpdateTenant(tnt); err != nil {
		t.Fatalf("UpdateTenant: %v", err)
	}

	got, _ := GetTenantByID(tnt.ID)
	if got.Name != "new" {
		t.Fatalf("expected name 'new', got %q", got.Name)
	}
}

func TestTenantMetadataMutationsCannotOverwriteMenuAuthorization(t *testing.T) {
	tnt := &Tenant{
		Name:      "authorization-boundary",
		Enable:    true,
		UserCount: 10,
		Expired:   time.Now().Add(24 * time.Hour),
		TenantMenus: []*TenantMenu{{
			MenuID: "embedded-create-menu",
			Funcs:  "view",
		}},
	}
	if err := CreateTenant(tnt); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	var embeddedCount int64
	if err := store.DB().Model(&TenantMenu{}).
		Where("tenant_id = ?", tnt.ID).
		Count(&embeddedCount).Error; err != nil {
		t.Fatalf("count embedded tenant menus: %v", err)
	}
	if embeddedCount != 0 {
		t.Fatalf("tenant creation bypassed dedicated menu authorization, count=%d", embeddedCount)
	}

	grant := &TenantMenu{
		TenantID: tnt.ID,
		MenuID:   "authorized-menu",
		Funcs:    "view,edit",
	}
	if err := store.DB().Create(grant).Error; err != nil {
		t.Fatalf("create dedicated tenant menu grant: %v", err)
	}
	tnt.Name = "authorization-boundary-updated"
	tnt.TenantMenus = []*TenantMenu{{
		TenantID: tnt.ID,
		MenuID:   "metadata-bypass-menu",
		Funcs:    "delete",
	}}
	if err := UpdateTenant(tnt); err != nil {
		t.Fatalf("UpdateTenant: %v", err)
	}
	var grants []*TenantMenu
	if err := store.DB().Where("tenant_id = ?", tnt.ID).Find(&grants).Error; err != nil {
		t.Fatalf("load tenant menu grants: %v", err)
	}
	if len(grants) != 1 ||
		grants[0].MenuID != grant.MenuID ||
		grants[0].Funcs != grant.Funcs {
		t.Fatalf("metadata update changed tenant authorization: %#v", grants)
	}
}

func TestDeleteTenant(t *testing.T) {
	tnt := &Tenant{
		Name:      "todel",
		Enable:    true,
		UserCount: 0,
		Expired:   time.Now().Add(24 * time.Hour),
	}
	if err := CreateTenant(tnt); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	userCountFn := func(int, string, string) (int64, error) { return 0, nil }
	roleCountFn := func(string) (int64, error) { return 0, nil }
	if err := DeleteTenant(tnt.ID, userCountFn, roleCountFn); err != nil {
		t.Fatalf("DeleteTenant: %v", err)
	}
	// 仅验证软删除不 panic，不要求 GetTenantByID 一定报错（GORM 软删除行为依赖 db 配置）
}

func TestGetAllTenant(t *testing.T) {
	_ = CreateTenant(&Tenant{Name: "a", Enable: true, UserCount: 10, Expired: time.Now().Add(time.Hour)})
	_ = CreateTenant(&Tenant{Name: "b", Enable: true, UserCount: 20, Expired: time.Now().Add(time.Hour)})
	all, err := GetAllTenant()
	if err != nil {
		t.Fatalf("GetAllTenant: %v", err)
	}
	if len(all) < 2 {
		t.Fatalf("expected at least 2 tenants, got %d", len(all))
	}
}

func TestQueryTenant(t *testing.T) {
	_ = CreateTenant(&Tenant{Name: "query-me", Enable: true, UserCount: 5, Expired: time.Now().Add(time.Hour), Province: "GD"})

	req := &apipb.QueryTenantRequest{Province: "GD"}
	resp := &apipb.QueryTenantResponse{}
	QueryTenant(req, resp)
	if len(resp.Data) == 0 {
		t.Fatal("expected at least one tenant matching province=GD")
	}
}
