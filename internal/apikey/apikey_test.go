package apikey

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	SetEncryptionKeyFrom("test-encryption-key")
	m.Run()
}

var testDBSeq atomic.Int64

// isolate 为每个用例提供独立的命名内存库(cache=shared 让连接池内的连接共享同一份数据),
// 结束后关闭并还原全局 store,使用例与执行顺序、-count 重复次数无关。
func isolate(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:apikey-test-%d?mode=memory&cache=shared", testDBSeq.Add(1))
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(&AIProvider{}, &AIKey{}, &ModelRoute{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	prev := store.Client()
	store.SetDB(db.NewDBClient(gdb, false))
	t.Cleanup(func() {
		store.SetDB(prev)
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestCreateAndGetProvider(t *testing.T) {
	isolate(t)
	p := &AIProvider{
		Name:     "test-provider",
		BaseURL:  "https://test.local/v1",
		AuthType: "bearer",
		Healthy:  true,
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
	isolate(t)
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

func TestSelectKeyUsesTenantSpecificRouteProviderKeyAndUpstreamModel(t *testing.T) {
	isolate(t)
	const alias = "tenant-routing-shared-alias"

	providerA := &AIProvider{TenantID: "tenant-route-a", Name: "tenant-route-provider-a", BaseURL: "https://a.example/v1", AuthType: "bearer", Healthy: true}
	providerB := &AIProvider{TenantID: "tenant-route-b", Name: "tenant-route-provider-b", BaseURL: "https://b.example/v1", AuthType: "header", Healthy: true}
	providerAID, err := CreateProvider(providerA)
	if err != nil {
		t.Fatalf("create provider A: %v", err)
	}
	providerBID, err := CreateProvider(providerB)
	if err != nil {
		t.Fatalf("create provider B: %v", err)
	}

	keyA := &AIKey{TenantID: "tenant-route-a", ProviderID: providerAID, Name: "tenant-route-key-a", Priority: 10, Enable: true}
	keyAID, err := CreateKey(keyA, "sk-tenant-route-a")
	if err != nil {
		t.Fatalf("create key A: %v", err)
	}
	// This deliberately attaches a foreign tenant's higher-priority key to
	// provider A. Tenant A must never select it.
	if _, err := CreateKey(&AIKey{TenantID: "tenant-route-b", ProviderID: providerAID, Name: "foreign-key", Priority: -100, Enable: true}, "sk-foreign"); err != nil {
		t.Fatalf("create foreign key: %v", err)
	}
	keyB := &AIKey{TenantID: "tenant-route-b", ProviderID: providerBID, Name: "tenant-route-key-b", Priority: 0, Enable: true}
	keyBID, err := CreateKey(keyB, "sk-tenant-route-b")
	if err != nil {
		t.Fatalf("create key B: %v", err)
	}

	if _, err := CreateRoute(&ModelRoute{TenantID: "tenant-route-a", ModelAlias: alias, UpstreamModel: "provider-a-model", ProviderID: providerAID, Priority: 0, Enable: true}); err != nil {
		t.Fatalf("create route A: %v", err)
	}
	if _, err := CreateRoute(&ModelRoute{TenantID: "tenant-route-b", ModelAlias: alias, UpstreamModel: "provider-b-model", ProviderID: providerBID, Priority: 0, Enable: true}); err != nil {
		t.Fatalf("create route B: %v", err)
	}

	selectionA, err := SelectKey("tenant-route-a", alias)
	if err != nil {
		t.Fatalf("select tenant A: %v", err)
	}
	if selectionA.Key.ID != keyAID || selectionA.Provider.ID != providerAID || selectionA.APIKey != "sk-tenant-route-a" {
		t.Fatalf("tenant A selection crossed boundary: %#v", selectionA)
	}
	if selectionA.Route == nil || selectionA.Route.TenantID != "tenant-route-a" || selectionA.ModelAlias != alias || selectionA.UpstreamModel != "provider-a-model" {
		t.Fatalf("tenant A route metadata mismatch: %#v", selectionA)
	}

	selectionB, err := SelectKey("tenant-route-b", alias)
	if err != nil {
		t.Fatalf("select tenant B: %v", err)
	}
	if selectionB.Key.ID != keyBID || selectionB.Provider.ID != providerBID || selectionB.APIKey != "sk-tenant-route-b" || selectionB.UpstreamModel != "provider-b-model" {
		t.Fatalf("tenant B selection mismatch: %#v", selectionB)
	}
}

func TestSelectKeyPrefersTenantRouteThenFallsBackToGlobalRoute(t *testing.T) {
	isolate(t)
	const alias = "tenant-before-global-alias"

	globalProvider := &AIProvider{Name: "global-route-provider", BaseURL: "https://global.example/v1", AuthType: "bearer", Healthy: true}
	globalProviderID, err := CreateProvider(globalProvider)
	if err != nil {
		t.Fatalf("create global provider: %v", err)
	}
	if _, err := CreateKey(&AIKey{ProviderID: globalProviderID, Name: "global-route-key", Priority: 0, Enable: true}, "sk-global-route"); err != nil {
		t.Fatalf("create global key: %v", err)
	}
	if _, err := CreateRoute(&ModelRoute{ModelAlias: alias, UpstreamModel: "global-model", ProviderID: globalProviderID, Priority: -100, Enable: true}); err != nil {
		t.Fatalf("create global route: %v", err)
	}

	tenantProvider := &AIProvider{TenantID: "tenant-preferred", Name: "tenant-preferred-provider", BaseURL: "https://tenant.example/v1", AuthType: "bearer", Healthy: true}
	tenantProviderID, err := CreateProvider(tenantProvider)
	if err != nil {
		t.Fatalf("create tenant provider: %v", err)
	}
	tenantKey := &AIKey{TenantID: "tenant-preferred", ProviderID: tenantProviderID, Name: "tenant-preferred-key", Priority: 100, Enable: true}
	tenantKeyID, err := CreateKey(tenantKey, "sk-tenant-preferred")
	if err != nil {
		t.Fatalf("create tenant key: %v", err)
	}
	if _, err := CreateRoute(&ModelRoute{TenantID: "tenant-preferred", ModelAlias: alias, UpstreamModel: "tenant-model", ProviderID: tenantProviderID, Priority: 100, Enable: true}); err != nil {
		t.Fatalf("create tenant route: %v", err)
	}

	selection, err := SelectKey("tenant-preferred", alias)
	if err != nil {
		t.Fatalf("select tenant route: %v", err)
	}
	if selection.Provider.ID != tenantProviderID || selection.UpstreamModel != "tenant-model" {
		t.Fatalf("global priority incorrectly outranked tenant route: %#v", selection)
	}

	if err := store.DB().Model(&AIKey{}).Where("id = ?", tenantKeyID).Update("enable", false).Error; err != nil {
		t.Fatalf("disable tenant key: %v", err)
	}
	selection, err = SelectKey("tenant-preferred", alias)
	if err != nil {
		t.Fatalf("fall back to global route: %v", err)
	}
	if selection.Provider.ID != globalProviderID || selection.UpstreamModel != "global-model" || selection.APIKey != "sk-global-route" {
		t.Fatalf("global fallback mismatch: %#v", selection)
	}
}

func TestSelectKeyRejectsForeignTenantKey(t *testing.T) {
	isolate(t)
	const alias = "foreign-key-rejected-alias"
	provider := &AIProvider{TenantID: "tenant-key-owner", Name: "foreign-key-provider", BaseURL: "https://owner.example/v1", AuthType: "bearer", Healthy: true}
	providerID, err := CreateProvider(provider)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if _, err := CreateKey(&AIKey{TenantID: "different-tenant", ProviderID: providerID, Name: "foreign-only-key", Enable: true}, "sk-must-not-leak"); err != nil {
		t.Fatalf("create foreign key: %v", err)
	}
	if _, err := CreateRoute(&ModelRoute{TenantID: "tenant-key-owner", ModelAlias: alias, ProviderID: providerID, Enable: true}); err != nil {
		t.Fatalf("create route: %v", err)
	}

	_, err = SelectKey("tenant-key-owner", alias)
	if err == nil || !strings.Contains(err.Error(), "no available api key") {
		t.Fatalf("expected foreign tenant key to be rejected, got %v", err)
	}
}

func TestSelectKeyExcludingAdvancesWithoutMutatingCooldown(t *testing.T) {
	isolate(t)
	const alias = "request-local-key-failover-alias"
	provider := &AIProvider{Name: "request-local-key-failover-provider", BaseURL: "https://failover.example/v1", AuthType: "bearer", Healthy: true}
	providerID, err := CreateProvider(provider)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	firstID, err := CreateKey(&AIKey{ProviderID: providerID, Name: "request-local-key-first", Priority: 0, Enable: true}, "sk-first")
	if err != nil {
		t.Fatalf("create first key: %v", err)
	}
	secondID, err := CreateKey(&AIKey{ProviderID: providerID, Name: "request-local-key-second", Priority: 1, Enable: true}, "sk-second")
	if err != nil {
		t.Fatalf("create second key: %v", err)
	}
	if _, err := CreateRoute(&ModelRoute{ModelAlias: alias, ProviderID: providerID, Enable: true}); err != nil {
		t.Fatalf("create route: %v", err)
	}

	first, err := SelectKeyExcluding("", alias, nil)
	if err != nil || first.Key.ID != firstID {
		t.Fatalf("select first key: selection=%#v err=%v", first, err)
	}
	second, err := SelectKeyExcluding("", alias, []string{firstID})
	if err != nil || second.Key.ID != secondID {
		t.Fatalf("select distinct second key: selection=%#v err=%v", second, err)
	}
	if _, err := SelectKeyExcluding("", alias, []string{firstID, secondID}); err == nil || !strings.Contains(err.Error(), "no available api key") {
		t.Fatalf("expected exhausted request-local candidates, got %v", err)
	}

	for _, keyID := range []string{firstID, secondID} {
		var key AIKey
		if err := store.DB().First(&key, "id = ?", keyID).Error; err != nil {
			t.Fatalf("reload key %s: %v", keyID, err)
		}
		if key.CooldownEnd != 0 {
			t.Fatalf("request-local exclusion mutated cooldown for %s: %d", keyID, key.CooldownEnd)
		}
	}
}
