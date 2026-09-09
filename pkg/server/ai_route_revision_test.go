package server

import (
	"context"
	"path/filepath"
	"testing"

	pkgdb "github.com/CloudSilk/pkg/db"
	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAIModelRouteRevisionTracksTenantAndFallbackWithoutKeys(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "routes.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	SetDB(pkgdb.NewDBClient(db, false))
	if err := db.AutoMigrate(&apikey.AIProvider{}, &apikey.ModelRoute{}); err != nil {
		t.Fatal(err)
	}
	for _, tenant := range []string{"", "tenant-a", "tenant-b"} {
		provider := apikey.AIProvider{Model: commonmodel.Model{ID: "provider-" + tenant}, TenantID: tenant, BaseURL: "https://models.example.test/" + tenant, Healthy: true}
		route := apikey.ModelRoute{Model: commonmodel.Model{ID: "route-" + tenant}, TenantID: tenant, ProviderID: provider.ID, ModelAlias: "embed", UpstreamModel: "embedding-v1", Enable: true}
		if err := db.Create(&provider).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&route).Error; err != nil {
			t.Fatal(err)
		}
	}
	read := func() AIModelRouteRevision {
		t.Helper()
		v, err := GetAIModelRouteRevision(t.Context(), "tenant-a", "embed")
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	original := read()
	if len(original.Digest) != 64 || len(original.UpstreamModels) != 1 || original.UpstreamModels[0] != "embedding-v1" {
		t.Fatalf("invalid revision: %#v", original)
	}
	if err := db.Model(&apikey.AIProvider{}).Where("id = ?", "provider-tenant-a").Update("healthy", false).Error; err != nil {
		t.Fatal(err)
	}
	if read().Digest != original.Digest {
		t.Fatal("transient provider health changed model binding")
	}
	if err := db.Model(&apikey.ModelRoute{}).Where("id = ?", "route-tenant-b").Update("upstream_model", "unrelated-v2").Error; err != nil {
		t.Fatal(err)
	}
	if read().Digest != original.Digest {
		t.Fatal("another tenant changed the revision")
	}
	if err := db.Model(&apikey.ModelRoute{}).Where("id = ?", "route-").Update("upstream_model", "embedding-v2").Error; err != nil {
		t.Fatal(err)
	}
	changed := read()
	if changed.Digest == original.Digest || len(changed.UpstreamModels) != 2 {
		t.Fatal("global failover change was ignored")
	}
	if err := db.Model(&apikey.AIProvider{}).Where("id = ?", "provider-tenant-a").Update("base_url", "https://other.example.test/v1").Error; err != nil {
		t.Fatal(err)
	}
	if read().Digest == changed.Digest {
		t.Fatal("endpoint change was ignored")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := GetAIModelRouteRevision(ctx, "tenant-a", "embed"); err == nil {
		t.Fatal("canceled revision read succeeded")
	}
}
