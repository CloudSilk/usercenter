package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	pkgdb "github.com/CloudSilk/pkg/db"
	commonmodel "github.com/CloudSilk/pkg/model"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/usage"
	"github.com/CloudSilk/usercenter/internal/user"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestBackgroundAIGatewayUsesCurrentUserAndTenantAuthorization(t *testing.T) {
	// Full embedding initialization owns process-global stores, Casbin and AI
	// caches. Isolate it from the facade unit tests' partial-schema databases.
	if os.Getenv("UC_BACKGROUND_AI_TEST_CHILD") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command(executable, "-test.run=^TestBackgroundAIGatewayUsesCurrentUserAndTenantAuthorization$", "-test.count=1")
		command.Env = append(os.Environ(), "UC_BACKGROUND_AI_TEST_CHILD=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("background gateway integration: %v\n%s", err, output)
		}
		return
	}
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "background.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	SetDB(pkgdb.NewDBClient(database, false))
	if err := RunMigration(); err != nil {
		t.Fatal(err)
	}
	InitConstants(Constants{PlatformTenantID: "platform", SuperAdminRoleID: "super_admin", EnableTenant: true})
	if _, _, err := SeedBootstrapAdmin("platform", "super_admin", "BackgroundFixture!2026"); err != nil {
		t.Fatal(err)
	}
	var owner user.User
	if err := database.Where("user_name = ?", "admin").First(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&permission.Role{Model: commonmodel.Model{ID: "background-author"}, TenantID: "platform", Name: "author", Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Where("user_id = ?", owner.ID).Delete(&user.UserRole{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&user.UserRole{Model: commonmodel.Model{ID: "background-user-role"}, UserID: owner.ID, RoleID: "background-author"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := permission.UpdateCasbin("background-author", []*permission.CasbinRule{{Path: "/v1/chat/completions", Method: "POST", CheckAuth: "true"}, {Path: "/v1/models", Method: "GET", CheckAuth: "true"}}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer upstream-fixture-key" {
			t.Error("gateway failed to resolve its encrypted provider key")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"fixture-background-response","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`))
	}))
	defer upstream.Close()
	apikey.SetEncryptionKeyFrom("background-encryption-fixture")
	providerID, err := apikey.CreateProvider(&apikey.AIProvider{TenantID: "platform", Name: "background-fixture", BaseURL: upstream.URL, AuthType: "bearer", Healthy: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := apikey.CreateKey(&apikey.AIKey{TenantID: "platform", ProviderID: providerID, Name: "fixture", Enable: true}, "upstream-fixture-key"); err != nil {
		t.Fatal(err)
	}
	if _, err := apikey.CreateRoute(&apikey.ModelRoute{TenantID: "platform", ProviderID: providerID, ModelAlias: "background-fixture", Enable: true}); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	userhttp.RegisterAIGatewayRouter(gin.New())
	transport := NewBackgroundAITransport(func(ctx context.Context) (string, string) { return owner.ID, "platform" })
	call := func(tr http.RoundTripper) int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, "http://usercenter.internal/v1/chat/completions", strings.NewReader(`{"model":"background-fixture","messages":[{"role":"user","content":"hello"}],"cache_bypass":true}`))
		response, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		return response.StatusCode
	}
	if code := call(transport); code != 200 {
		t.Fatalf("active author status=%d", code)
	}
	var records []usage.UsageRecord
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		database.Where("principal_id = ? AND tenant_id = ?", owner.ID, "platform").Find(&records)
		if len(records) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(records) != 1 || records[0].TotalTokens != 5 {
		t.Fatalf("missing tenant/user usage attribution: %+v", records)
	}
	wrongTenant := NewBackgroundAITransport(func(context.Context) (string, string) { return owner.ID, "other" })
	if code := call(wrongTenant); code != 403 {
		t.Fatalf("cross-tenant status=%d", code)
	}
	for _, change := range []struct {
		model            any
		id, column       string
		denied, restored any
	}{
		{&user.User{}, owner.ID, "enable", false, true},
		{&permission.Role{}, "background-author", "enable", false, true},
		{&tenant.Tenant{}, "platform", "enable", false, true},
		{&tenant.Tenant{}, "platform", "expired", time.Now().Add(-time.Hour), time.Now().Add(time.Hour)},
	} {
		if err := database.Model(change.model).Where("id = ?", change.id).Update(change.column, change.denied).Error; err != nil {
			t.Fatal(err)
		}
		if code := call(transport); code != 403 {
			t.Fatalf("%T.%s revocation ignored: %d", change.model, change.column, code)
		}
		if err := database.Model(change.model).Where("id = ?", change.id).Update(change.column, change.restored).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := permission.ClearCasbin(0, "background-author"); err != nil {
		t.Fatal(err)
	}
	if code := call(transport); code != 403 {
		t.Fatalf("permission revocation ignored: %d", code)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://usercenter.internal/admin/api/ai-keys", nil)
	if _, err := transport.RoundTrip(req); err == nil {
		t.Fatal("background transport exposed admin API")
	}
	if calls.Load() != 1 {
		t.Fatalf("denied requests reached provider: calls=%d", calls.Load())
	}
}
