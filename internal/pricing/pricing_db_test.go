package pricing

import (
	"testing"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	glebsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	gdb, err := gorm.Open(glebsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := gdb.AutoMigrate(&ModelPrice{}); err != nil {
		panic(err)
	}
	store.SetDB(db.NewDBClient(gdb, false))
	m.Run()
}

func TestCalculateCost_NilPrice(t *testing.T) {
	if cost := CalculateCost(nil, 1000, 500); cost != 0 {
		t.Fatalf("nil price should yield 0, got %f", cost)
	}
}

// TestGetPrice_TenantOverrideOverGlobal 租户级计价优先于全局计价。
func TestGetPrice_TenantOverrideOverGlobal(t *testing.T) {
	global := &ModelPrice{TenantID: "", ModelName: "cov-model", InputPer1M: 1, OutputPer1M: 2, Enable: true}
	tenant := &ModelPrice{TenantID: "t-cov", ModelName: "cov-model", InputPer1M: 10, OutputPer1M: 20, Enable: true}
	if _, err := CreatePrice(global); err != nil {
		t.Fatalf("create global: %v", err)
	}
	if _, err := CreatePrice(tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&ModelPrice{}, "model_name = ?", "cov-model").Error
	})

	got, err := GetPrice("t-cov", "cov-model")
	if err != nil || got == nil {
		t.Fatalf("GetPrice: %v", err)
	}
	if got.InputPer1M != 10 {
		t.Fatalf("expected tenant override (10), got %v", got.InputPer1M)
	}
	// 其他租户回退到全局
	other, err := GetPrice("t-other", "cov-model")
	if err != nil || other == nil || other.InputPer1M != 1 {
		t.Fatalf("expected global fallback, got %+v err=%v", other, err)
	}
}

// TestGetPrice_DisabledAndMissing 禁用计价与缺失计价都不计费。
func TestGetPrice_DisabledAndMissing(t *testing.T) {
	disabled := &ModelPrice{TenantID: "t-cov2", ModelName: "cov-off", InputPer1M: 9, Enable: false}
	if _, err := CreatePrice(disabled); err != nil {
		t.Fatalf("create disabled: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&ModelPrice{}, "model_name = ?", "cov-off").Error
	})
	got, err := GetPrice("t-cov2", "cov-off")
	if err != nil {
		t.Fatalf("GetPrice: %v", err)
	}
	if got != nil {
		t.Fatalf("disabled price should not be returned, got %+v", got)
	}
}

// TestPriceCRUD_ListPrices 覆盖增改删与列表的租户/全局范围语义。
func TestPriceCRUD_ListPrices(t *testing.T) {
	id, err := CreatePrice(&ModelPrice{TenantID: "t-cov3", ModelName: "crud-model", InputPer1M: 3, OutputPer1M: 4, Enable: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := CreatePrice(&ModelPrice{TenantID: "", ModelName: "crud-global", InputPer1M: 1, Enable: true}); err != nil {
		t.Fatalf("create global: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&ModelPrice{}, "model_name IN ?", []string{"crud-model", "crud-global"}).Error
	})

	// 租户列表包含全局计价
	list, err := ListPrices("t-cov3")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	names := map[string]bool{}
	for _, p := range list {
		names[p.ModelName] = true
	}
	if !names["crud-model"] || !names["crud-global"] {
		t.Fatalf("tenant list should include global, got %v", names)
	}
	// 空租户只列全局
	globals, err := ListPrices("")
	if err != nil {
		t.Fatalf("global list: %v", err)
	}
	for _, p := range globals {
		if p.TenantID != "" {
			t.Fatalf("global list should exclude tenant rows: %+v", p)
		}
	}

	// 更新后生效
	var stored ModelPrice
	if err := store.DB().First(&stored, "id = ?", id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	stored.InputPer1M = 99
	if err := UpdatePrice(&stored); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := DeletePrice(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var count int64
	_ = store.DB().Model(&ModelPrice{}).Where("id = ?", id).Count(&count).Error
	if count != 0 {
		t.Fatal("expected deleted price to be gone")
	}
}
