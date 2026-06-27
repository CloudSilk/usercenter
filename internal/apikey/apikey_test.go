package apikey

import (
	"testing"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	_ = gdb.AutoMigrate(&AIProvider{}, &AIKey{}, &ModelRoute{})
	store.SetDB(db.NewDBClient(gdb, false))
	SetEncryptionKeyFrom("test-encryption-key")
	m.Run()
}

func TestCreateAndGetProvider(t *testing.T) {
	p := &AIProvider{
		Name:    "test-provider",
		BaseURL: "https://test.local/v1",
		AuthType: "bearer",
		Healthy: true,
	}
	pid, err := CreateProvider(p)
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	if pid == "" {
		t.Fatal("expected non-empty id")
	}

	got, err := GetProviderByID(pid)
	if err != nil {
		t.Fatalf("GetProviderByID: %v", err)
	}
	if got.Name != "test-provider" {
		t.Fatalf("expected Name 'test-provider', got %q", got.Name)
	}
}

func TestCreateAndSelectKey(t *testing.T) {
	p := &AIProvider{Name: "p1", BaseURL: "https://x/v1", AuthType: "bearer", Healthy: true}
	pid, _ := CreateProvider(p)

	k := &AIKey{
		TenantID:   "t1",
		ProviderID: pid,
		Name:       "my-key",
		Priority:   0,
		Enable:     true,
	}
	kid, err := CreateKey(k, "sk-plaintext-secret")
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	if kid == "" {
		t.Fatal("expected non-empty key id")
	}
}