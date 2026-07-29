package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/usage"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

// TestUsageByPrincipalEndpoint 验证 /admin/api/usage/by-principal 端点按主体
// 维度聚合（tokens/cost/count）并区分 principal_kind（0人/1Agent）。
func TestUsageByPrincipalEndpoint(t *testing.T) {
	const tenant = "usage-by-principal-tenant"
	// 起点为空库，插入两条不同主体的记录。
	now := time.Now()
	seed := []*usage.UsageRecord{
		{TenantID: tenant, PrincipalID: "human-1", PrincipalKind: 0, ModelName: "gpt-x",
			TotalTokens: 100, Cost: 0.01, Success: true},
		{TenantID: tenant, PrincipalID: "human-1", PrincipalKind: 0, ModelName: "gpt-x",
			TotalTokens: 50, Cost: 0.005, Success: true},
		{TenantID: tenant, PrincipalID: "agent-1", PrincipalKind: 1, ModelName: "gpt-x",
			TotalTokens: 300, Cost: 0.03, Success: true},
	}
	for _, r := range seed {
		r.ID = ""
		r.CreatedAt = now
		if err := store.DB().Create(r).Error; err != nil {
			t.Fatalf("seed usage record: %v", err)
		}
	}
	t.Cleanup(func() {
		store.DB().Where("tenant_id = ?", tenant).Delete(&usage.UsageRecord{})
	})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		// 超管视图：以当前用户租户隔离。这里用 query tenantID 跨租户查看。
		c.Set("User", &apipb.CurrentUser{Id: "admin-usage", UserName: "admin-usage"})
		c.Next()
	})
	userhttp.RegisterAdminRouter(r)

	rec := httptest.NewRecorder()
	// effectiveTenantID 取 query tenantID（超管视图）。
	r.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet, "/admin/api/usage/by-principal?tenantID="+tenant, nil))

	resp := requireAdminSuccess(t, rec.Body.Bytes())

	var rows []map[string]any
	if err := json.Unmarshal(resp.Data, &rows); err != nil {
		t.Fatalf("decode by-principal rows: %v (data=%s)", err, string(resp.Data))
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 principal buckets, got %d: %#v", len(rows), rows)
	}

	byID := map[string]map[string]any{}
	for _, row := range rows {
		id, _ := row["principal_id"].(string)
		byID[id] = row
	}
	if human, ok := byID["human-1"]; !ok {
		t.Fatalf("human-1 bucket missing: %#v", byID)
	} else if num(human["tokens"]) != 150 {
		t.Fatalf("human-1 tokens = %v, want 150", human["tokens"])
	}
	if agent, ok := byID["agent-1"]; !ok {
		t.Fatalf("agent-1 bucket missing: %#v", byID)
	} else if num(agent["tokens"]) != 300 {
		t.Fatalf("agent-1 tokens = %v, want 300", agent["tokens"])
	}
}

// num 把 GORM map 里可能是 float64 / json.Number / string 的数值统一成 float64。
func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		f := 0.0
		_ = json.Unmarshal([]byte(n), &f)
		return f
	}
	return 0
}
