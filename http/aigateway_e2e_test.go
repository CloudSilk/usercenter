package http_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/gatewaylog"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/usage"
	userhttp "github.com/CloudSilk/usercenter/http"
)

// seedAIProvider 在测试 DB 中播种一个指向 baseURL 的服务商 + Key + 路由，
// 使 ChatCompletions 的 forwardWithRetry → apikey.SelectKey 能选到它。
func seedAIProvider(t *testing.T, baseURL, modelAlias string) {
	t.Helper()
	apikey.SetEncryptionKeyFrom("test-encryption-key") // 派生 AES-GCM 密钥，CreateKey 加密明文

	p := &apikey.AIProvider{
		TenantID: platformTenant,
		Name:     "mock-provider",
		BaseURL:  baseURL,
		AuthType: "bearer",
		Healthy:  true,
	}
	pid, err := apikey.CreateProvider(p)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	k := &apikey.AIKey{
		TenantID:   platformTenant,
		ProviderID: pid,
		Name:       "mock-key",
		Priority:   0,
		Enable:     true,
	}
	if _, err := apikey.CreateKey(k, "sk-mock-plaintext"); err != nil {
		t.Fatalf("create key: %v", err)
	}
	rt := &apikey.ModelRoute{
		TenantID:   platformTenant,
		ModelAlias: modelAlias,
		ProviderID: pid,
		Priority:   0,
		Enable:     true,
	}
	if _, err := apikey.CreateRoute(rt); err != nil {
		t.Fatalf("create route: %v", err)
	}
}

// newAIGatewayEngine 构造一个注入 Human Principal 的 gin 引擎并注册 AI 网关路由。
func newAIGatewayEngine(userID, tenantID string) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("Principal", principal.NewHuman(userID, tenantID, nil))
		c.Next()
	})
	userhttp.RegisterAIGatewayRouter(r)
	return r
}

// TestChatCompletions_E2E 端到端验证整条链路：
// 请求 → 限流 → 解析 → 配额 → 路由选 Key(解密) → 转发 mock 上游 → 计量/日志落库。
func TestChatCompletions_E2E(t *testing.T) {
	// mock 上游：记录收到的鉴权头，返回固定 OpenAI 响应
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","choices":[{"message":{"role":"assistant","content":"pong"}}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`))
	}))
	defer upstream.Close()

	seedAIProvider(t, upstream.URL, "mock-model")
	r := newAIGatewayEngine("u1", platformTenant)

	body := []byte(`{"model":"mock-model","messages":[{"role":"user","content":"ping"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("pong")) {
		t.Fatalf("expected response to contain 'pong', got %s", w.Body.String())
	}
	// 上游应收到解密后的 Bearer 明文 key（验证加密存储→解密转发链路）
	if gotAuth != "Bearer sk-mock-plaintext" {
		t.Fatalf("expected upstream Authorization 'Bearer sk-mock-plaintext', got %q", gotAuth)
	}
	// 用量记录应落库
	var usageCount int64
	store.DB().Model(&usage.UsageRecord{}).Where("model_name = ?", "mock-model").Count(&usageCount)
	if usageCount != 1 {
		t.Fatalf("expected 1 usage record for mock-model, got %d", usageCount)
	}
	// 网关日志应落库
	var logCount int64
	store.DB().Model(&gatewaylog.GatewayLog{}).Where("model_alias = ?", "mock-model").Count(&logCount)
	if logCount != 1 {
		t.Fatalf("expected 1 gateway log for mock-model, got %d", logCount)
	}
}

// TestChatCompletions_E2E_Streaming 端到端验证流式链路：
// stream:true → streamProxy 逐行透传 SSE + 累积 delta.content + 从最后一块提取 usage → 用量落库。
// 这是此前 P0 双写 bug 所在路径，必须有覆盖。
func TestChatCompletions_E2E_Streaming(t *testing.T) {
	sse := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":2}}\n\n" +
		"data: [DONE]\n\n")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write(sse)
	}))
	defer upstream.Close()

	seedAIProvider(t, upstream.URL, "mock-stream-model")
	r := newAIGatewayEngine("us", platformTenant)

	body := []byte(`{"model":"mock-stream-model","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	// SSE 内容应完整透传到客户端
	respBody := w.Body.String()
	if !strings.Contains(respBody, "Hel") || !strings.Contains(respBody, "[DONE]") {
		t.Fatalf("expected SSE stream to contain 'Hel' and '[DONE]', got %s", respBody)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected SSE content-type, got %q", ct)
	}
	// 流式 usage 应从最后一块提取并落库（completion_tokens=2）
	var rec usage.UsageRecord
	if err := store.DB().Where("model_name = ?", "mock-stream-model").First(&rec).Error; err != nil {
		t.Fatalf("expected usage record for stream model, got %v", err)
	}
	if rec.CompTokens != 2 || rec.PromptTokens != 8 {
		t.Fatalf("expected prompt=8 comp=2 from stream usage chunk, got prompt=%d comp=%d", rec.PromptTokens, rec.CompTokens)
	}
}

// TestChatCompletions_E2E_NoRoute 验证无可用路由时返回 502（forwardWithRetry 失败链路）。
func TestChatCompletions_E2E_NoRoute(t *testing.T) {
	r := newAIGatewayEngine("u2", platformTenant)
	// 不播种任何 provider/route → SelectKey 失败 → 502

	body := []byte(`{"model":"nonexistent-model","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for missing route, got %d (body=%s)", w.Code, w.Body.String())
	}
}

// TestChatCompletions_E2E_MissingModel 验证缺 model 字段返回 400。
func TestChatCompletions_E2E_MissingModel(t *testing.T) {
	r := newAIGatewayEngine("u3", platformTenant)

	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing model, got %d (body=%s)", w.Code, w.Body.String())
	}
}

// TestCheckBudget_EnforcesModelBudget 回归测试：验证按模型配额能真正拦截。
// 修复前 CheckBudget 查询列名写成 model（实际为 model_name），导致 SQL 报错被当作
// "无预算"放行 —— 配额被静默绕过。此用例在修复前会失败（allowed=true），修复后通过。
func TestCheckBudget_EnforcesModelBudget(t *testing.T) {
	// 种子：平台租户 + 模型日 token 上限 10
	budget := &usage.UsageBudget{
		TenantID:        platformTenant,
		ModelName:       "mock-budget-model",
		DailyTokenLimit: 10,
		Enable:          true,
	}
	if err := store.DB().Create(budget).Error; err != nil {
		t.Fatalf("create budget: %v", err)
	}
	// 种子：一条当日用量 total_tokens=20（超额）
	usage.RecordUsage(&usage.UsageRecord{
		TenantID:      platformTenant,
		ModelName:     "mock-budget-model",
		PromptTokens:  20,
		PrincipalKind: 0,
	})

	allowed, _, _, err := usage.CheckBudget(platformTenant, "", "mock-budget-model")
	if err != nil {
		t.Fatalf("check budget: %v", err)
	}
	if allowed {
		t.Fatal("expected budget to BLOCK (daily limit 10, used 20), got allowed=true")
	}
}
