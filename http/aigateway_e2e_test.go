package http_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/aicache"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/conversation"
	"github.com/CloudSilk/usercenter/internal/gatewaylog"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/prompt"
	"github.com/CloudSilk/usercenter/internal/ratelimit"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/usage"
	"github.com/gin-gonic/gin"
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
func TestChatCompletions_E2E_CacheBypassSkipsReadAndWrite(t *testing.T) {
	aicache.Init(aicache.CacheConfig{Enabled: true})
	aicache.Clear()
	defer aicache.Clear()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cache-bypass","choices":[{"message":{"role":"assistant","content":"fresh"}}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`))
	}))
	defer upstream.Close()

	seedAIProvider(t, upstream.URL, "mock-cache-bypass-model")
	r := newAIGatewayEngine("cache-bypass-user", platformTenant)
	body := []byte(`{"model":"mock-cache-bypass-model","cache_bypass":true,"messages":[{"role":"user","content":"same prompt"}]}`)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d (body=%s)", i+1, w.Code, w.Body.String())
		}
		if got := w.Header().Get("X-Cache"); got != "" {
			t.Fatalf("request %d unexpectedly used cache: X-Cache=%q", i+1, got)
		}
	}
	if upstreamCalls != 2 {
		t.Fatalf("cache bypass forwarded %d upstream calls, want 2", upstreamCalls)
	}
	if stats := aicache.Stats(); stats.Entries != 0 {
		t.Fatalf("cache bypass persisted %d entries, want 0", stats.Entries)
	}
}

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

// TestChatCompletions_E2E_SessionPersistence 验证带 session_id 时 user + assistant 消息落库。
func TestChatCompletions_E2E_SessionPersistence(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"session-reply"}}],"usage":{"prompt_tokens":3,"completion_tokens":1}}`))
	}))
	defer upstream.Close()
	seedAIProvider(t, upstream.URL, "mock-sess-model")

	// 预创建会话（网关只 AppendMessage，不自动建 session）
	sess := &conversation.Session{
		TenantID:    platformTenant,
		PrincipalID: "us",
		ModelAlias:  "mock-sess-model",
		Title:       "test-session",
	}
	sid, err := conversation.CreateSession(sess)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	r := newAIGatewayEngine("us", platformTenant)
	body := []byte(`{"model":"mock-sess-model","session_id":"` + sid + `","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	// 应持久化 user + assistant 两条消息
	var msgCount int64
	store.DB().Model(&conversation.Message{}).Where("session_id = ?", sid).Count(&msgCount)
	if msgCount != 2 {
		t.Fatalf("expected 2 persisted messages (user+assistant), got %d", msgCount)
	}
	var assistantMsg conversation.Message
	store.DB().Where("session_id = ? AND role = ?", sid, "assistant").First(&assistantMsg)
	if assistantMsg.Content != "session-reply" {
		t.Fatalf("expected assistant message 'session-reply', got %q", assistantMsg.Content)
	}
}

// TestChatCompletions_E2E_RateLimited 验证超出令牌桶 burst 时返回 429（限流先于路由/配额）。
func TestChatCompletions_E2E_RateLimited(t *testing.T) {
	// 为专用 principal 设低 burst=2，使第 3 个请求即被限流
	ratelimit.Default.SetConfig("rateuser", 1, 2)
	r := newAIGatewayEngine("rateuser", platformTenant)

	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)
	got429 := false
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			got429 = true
		}
	}
	if !got429 {
		t.Fatal("expected at least one 429 when exceeding burst=2")
	}
}

// TestChatCompletions_E2E_PromptTemplateInjection 验证 prompt_template_id 渲染注入 system message，
// 且增强字段从转发给上游的 body 中剥离。
func TestChatCompletions_E2E_PromptTemplateInjection(t *testing.T) {
	tpl := &prompt.PromptTemplate{
		TenantID:  platformTenant,
		Name:      "test-tpl",
		Content:   "You are a {{role}} assistant.",
		Variables: "role",
		Enable:    true,
		Version:   1,
	}
	tid, err := prompt.Create(tpl)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	var forwarded string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		forwarded = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer upstream.Close()
	seedAIProvider(t, upstream.URL, "mock-tpl-model")

	r := newAIGatewayEngine("ut", platformTenant)
	body := []byte(`{"model":"mock-tpl-model","prompt_template_id":"` + tid + `","prompt_vars":{"role":"support"},"messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	// 转发 body 应含渲染后的模板（变量已替换）
	if !strings.Contains(forwarded, "You are a support assistant.") {
		t.Fatalf("expected rendered template in forwarded body, got %s", forwarded)
	}
	// 增强字段应从转发 body 剥离（上游不认识）
	if strings.Contains(forwarded, "prompt_template_id") || strings.Contains(forwarded, "prompt_vars") {
		t.Fatalf("enhancement fields must be stripped before forwarding, got %s", forwarded)
	}
}
