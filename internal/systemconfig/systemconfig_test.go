package systemconfig

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
	if err := gdb.AutoMigrate(&SystemConfig{}); err != nil {
		panic(err)
	}
	store.SetDB(db.NewDBClient(gdb, false))
	m.Run()
}

func TestUpsertAndGetByKey(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("`key` LIKE ?", "sc-cov-%").Delete(&SystemConfig{}).Error
	})

	// 创建
	if err := UpsertSystemConfigByKey("sc-cov-a", "v1"); err != nil {
		t.Fatalf("upsert create: %v", err)
	}
	got, err := GetSystemConfigByKey("sc-cov-a")
	if err != nil || got.Value != "v1" {
		t.Fatalf("get: %+v err=%v", got, err)
	}

	// 空键是无操作
	if err := UpsertSystemConfigByKey("", "x"); err != nil {
		t.Fatalf("empty key upsert should be no-op: %v", err)
	}

	// 更新
	if err := UpsertSystemConfigByKey("sc-cov-a", "v2"); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	got, _ = GetSystemConfigByKey("sc-cov-a")
	if got.Value != "v2" {
		t.Fatalf("expected v2, got %q", got.Value)
	}
}

func TestGetSystemConfigByKeyVariants(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("`key` LIKE ?", "sc-cov-b%").Delete(&SystemConfig{}).Error
	})
	for _, kv := range [][2]string{{"sc-cov-b1", "1"}, {"sc-cov-b2", "2"}} {
		if err := UpsertSystemConfigByKey(kv[0], kv[1]); err != nil {
			t.Fatalf("upsert %s: %v", kv[0], err)
		}
	}

	// 批量取
	m, err := GetSystemConfigMapByKeys([]string{"sc-cov-b1", "sc-cov-b2", "sc-cov-missing"})
	if err != nil {
		t.Fatalf("map by keys: %v", err)
	}
	if m["sc-cov-b1"] != "1" || m["sc-cov-b2"] != "2" {
		t.Fatalf("unexpected map: %v", m)
	}
	if _, ok := m["sc-cov-missing"]; ok {
		t.Fatal("missing key should not appear")
	}

	// 前缀查询
	list, err := GetSystemConfigsByKeyPrefix("sc-cov-b")
	if err != nil {
		t.Fatalf("prefix: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 by prefix, got %d", len(list))
	}

	// 不存在的键
	if _, err := GetSystemConfigByKey("sc-cov-none"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestCompareAndSwapSystemConfigByKey(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("`key` LIKE ?", "sc-cas-%").Delete(&SystemConfig{}).Error
	})

	// nil expected:仅当键不存在时创建;已存在则返回 false
	created, err := CompareAndSwapSystemConfigByKey("sc-cas-k", nil, "init")
	if err != nil || !created {
		t.Fatalf("cas create: created=%v err=%v", created, err)
	}
	created, err = CompareAndSwapSystemConfigByKey("sc-cas-k", nil, "again")
	if err != nil || created {
		t.Fatalf("cas create on existing key should not swap: created=%v err=%v", created, err)
	}

	// expected 匹配才更新
	swapped, err := CompareAndSwapSystemConfigByKey("sc-cas-k", strPtr("init"), "updated")
	if err != nil || !swapped {
		t.Fatalf("cas swap: swapped=%v err=%v", swapped, err)
	}
	got, _ := GetSystemConfigByKey("sc-cas-k")
	if got.Value != "updated" {
		t.Fatalf("expected updated, got %q", got.Value)
	}
	// expected 不匹配则不更新
	swapped, err = CompareAndSwapSystemConfigByKey("sc-cas-k", strPtr("stale"), "hacked")
	if err != nil || swapped {
		t.Fatalf("stale cas should not swap: swapped=%v err=%v", swapped, err)
	}
	got, _ = GetSystemConfigByKey("sc-cas-k")
	if got.Value != "updated" {
		t.Fatalf("value should remain after stale swap: %q", got.Value)
	}

	// 空键拒绝
	if _, err := CompareAndSwapSystemConfigByKey("", strPtr("x"), "y"); err == nil {
		t.Fatal("empty key should be rejected")
	}
}

func TestCRUDAndPB(t *testing.T) {
	id, err := CreateSystemConfig(&SystemConfig{Key: "sc-crud-k", Value: "v", TenantID: "t1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&SystemConfig{}, "id = ?", id).Error
	})

	got, err := GetSystemConfigByID(id)
	if err != nil || got.Value != "v" {
		t.Fatalf("get by id: %+v err=%v", got, err)
	}
	got.Value = "v2"
	if err := UpdateSystemConfig(got); err != nil {
		t.Fatalf("update: %v", err)
	}
	if byIDs, err := GetSystemConfigByIDs([]string{id}); err != nil || len(byIDs) != 1 {
		t.Fatalf("get by ids: %v", err)
	}
	if all, err := GetAllSystemConfigs(); err != nil || len(all) == 0 {
		t.Fatalf("get all: %v", err)
	}

	if err := DeleteSystemConfig(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := GetSystemConfigByID(id); err == nil {
		t.Fatal("deleted config should be gone")
	}
}

func strPtr(s string) *string { return &s }
