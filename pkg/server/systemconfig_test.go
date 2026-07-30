package server

import (
	"testing"

	pkgdb "github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/systemconfig"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSystemConfigValueCompareAndSwap(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open("file:server-system-config-cas?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&systemconfig.SystemConfig{}); err != nil {
		t.Fatalf("migrate system config: %v", err)
	}
	SetDB(pkgdb.NewDBClient(db, false))

	const key = "embedded.product.settings.v1"
	if value, found, err := GetSystemConfigValue(key); err != nil {
		t.Fatalf("read missing value: %v", err)
	} else if found {
		t.Fatalf("missing value unexpectedly found: %#v", value)
	}

	created, err := CompareAndSwapSystemConfigValue(key, nil, `{"revision":1}`)
	if err != nil {
		t.Fatalf("create value: %v", err)
	}
	if !created {
		t.Fatal("first compare-and-swap did not create the value")
	}
	created, err = CompareAndSwapSystemConfigValue(key, nil, `{"revision":1}`)
	if err != nil {
		t.Fatalf("repeat create: %v", err)
	}
	if created {
		t.Fatal("repeat create overwrote an existing value")
	}

	value, found, err := GetSystemConfigValue(key)
	if err != nil {
		t.Fatalf("read created value: %v", err)
	}
	if !found || value.Value != `{"revision":1}` || value.UpdatedAt.IsZero() {
		t.Fatalf("created value = %#v, found=%v", value, found)
	}

	stale := `{"revision":0}`
	updated, err := CompareAndSwapSystemConfigValue(
		key,
		&stale,
		`{"revision":2}`,
	)
	if err != nil {
		t.Fatalf("stale update: %v", err)
	}
	if updated {
		t.Fatal("stale compare-and-swap unexpectedly succeeded")
	}

	expected := value.Value
	updated, err = CompareAndSwapSystemConfigValue(
		key,
		&expected,
		`{"revision":2}`,
	)
	if err != nil {
		t.Fatalf("current update: %v", err)
	}
	if !updated {
		t.Fatal("current compare-and-swap did not update the value")
	}
	value, found, err = GetSystemConfigValue(key)
	if err != nil {
		t.Fatalf("read updated value: %v", err)
	}
	if !found || value.Value != `{"revision":2}` {
		t.Fatalf("updated value = %#v, found=%v", value, found)
	}
}

func TestSystemConfigValueRejectsBlankKey(t *testing.T) {
	if _, _, err := GetSystemConfigValue("  "); err == nil {
		t.Fatal("blank read key was accepted")
	}
	if _, err := CompareAndSwapSystemConfigValue("", nil, "{}"); err == nil {
		t.Fatal("blank compare-and-swap key was accepted")
	}
}
