package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/gin-gonic/gin"
)

func TestBuildUpstreamRequestWithContextPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sel := &apikey.KeySelection{
		Provider:      &apikey.AIProvider{BaseURL: "https://api.example.com/v1", AuthType: "bearer"},
		APIKey:        "sk-test",
		UpstreamModel: "upstream-model",
	}
	req, err := buildUpstreamRequestWithContext(ctx, sel, []byte(`{"model":"public-model"}`), "/chat/completions")
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(req.Context().Err(), context.Canceled) {
		t.Fatalf("upstream request lost caller cancellation: %v", req.Context().Err())
	}
}

// TestBufferedProxy_ParseUsage 验证整包转发时从 OpenAI usage 字段提取 token 用量。
func TestBufferedProxy_ParseUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"hi"}}],"usage":{"prompt_tokens":42,"completion_tokens":7,"total_tokens":49}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	pt, ct := bufferedProxy(c, resp)

	if pt != 42 || ct != 7 {
		t.Fatalf("expected prompt=42 comp=7, got prompt=%d comp=%d", pt, ct)
	}
	if !strings.Contains(w.Body.String(), `"content":"hi"`) {
		t.Fatalf("response body not proxied: %s", w.Body.String())
	}
}

// TestStreamProxy_PassthroughAndUsage 验证 SSE 流式透传 + 从最后一块捕获 usage。
func TestStreamProxy_PassthroughAndUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	sse := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"Hel"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"lo"}}]}`,
		``,
		`data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sse)),
	}
	pt, ct, content := streamProxy(c, resp)

	if pt != 10 || ct != 2 {
		t.Fatalf("expected prompt=10 comp=2, got prompt=%d comp=%d", pt, ct)
	}
	if !strings.Contains(content, "Hello") {
		t.Fatalf("expected accumulated content to contain 'Hello', got %q", content)
	}
	if !strings.Contains(w.Body.String(), "Hel") || !strings.Contains(w.Body.String(), "[DONE]") {
		t.Fatalf("SSE not fully proxied: %s", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected SSE content-type, got %q", ct)
	}
}

// TestBuildUpstreamRequest_Auth 验证按 AuthType 注入鉴权与 URL 拼装。
func TestBuildUpstreamRequest_Auth(t *testing.T) {
	cases := []struct {
		name      string
		auth      string
		baseURL   string
		storedKey string // 存储的原始 Key（明文）
		wantHdr   string
		wantVal   string // 代码按 authType 拼装后的完整 header 值
		queryKey  string // 非 空 → query 模式：期望出现在 URL ?key= 中的值
		wantURL   string
	}{
		{"bearer", "bearer", "https://api.openai.com/v1", "sk-x", "Authorization", "Bearer sk-x", "", "https://api.openai.com/v1/chat/completions"},
		{"default-empty", "", "https://api.deepseek.com", "sk-y", "Authorization", "Bearer sk-y", "", "https://api.deepseek.com/chat/completions"},
		{"header", "header", "https://api.anthropic.com/v1", "sk-z", "Authorization", "sk-z", "", "https://api.anthropic.com/v1/chat/completions"},
		{"query", "query", "https://api.example.com/v1", "sk-q", "", "", "sk-q", "https://api.example.com/v1/chat/completions?key=sk-q"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel := &apikey.KeySelection{
				Provider:      &apikey.AIProvider{BaseURL: tc.baseURL, AuthType: tc.auth},
				ModelAlias:    "public-model-alias",
				UpstreamModel: "provider-model-name",
				APIKey:        tc.storedKey,
			}
			req, err := buildUpstreamRequest(sel, []byte(`{"model":"public-model-alias"}`), "/chat/completions")
			if err != nil {
				t.Fatal(err)
			}
			if req.URL.String() != tc.wantURL {
				t.Fatalf("url: got %s want %s", req.URL.String(), tc.wantURL)
			}
			requestBody, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if !strings.Contains(string(requestBody), `"model":"provider-model-name"`) {
				t.Fatalf("request model was not rewritten to selected upstream model: %s", requestBody)
			}
			if tc.queryKey != "" {
				// query 模式：密钥在 URL 查询参数中，且不应出现在 Authorization 头
				if got := req.URL.Query().Get("key"); got != tc.queryKey {
					t.Fatalf("query key: got %q want %q", got, tc.queryKey)
				}
				if ah := req.Header.Get("Authorization"); ah != "" {
					t.Fatalf("query mode must not set Authorization header, got %q", ah)
				}
			} else {
				if got := req.Header.Get(tc.wantHdr); got != tc.wantVal {
					t.Fatalf("header %s: got %q want %q", tc.wantHdr, got, tc.wantVal)
				}
			}
		})
	}
}

// TestForwardWithRetrySameKeyBackoffAbsorbsUpstream529 验证唯一 Key 场景下，
// 上游连续 529 时 forwardWithRetry 通过退避轮复用同 Key，吸收瞬时过载后成功。
func TestForwardWithRetrySameKeyBackoffAbsorbsUpstream529(t *testing.T) {
	var calls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.WriteHeader(529)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","choices":[{"message":{"role":"assistant","content":"pong"}}]}`))
	}))
	defer upstream.Close()

	old := forwardBackoffBase
	forwardBackoffBase = time.Millisecond
	defer func() { forwardBackoffBase = old }()

	const alias = "mock-model-samekey-backoff"
	const tenant = "platform-test"
	apikey.SetEncryptionKeyFrom("test-encryption-key")
	p := &apikey.AIProvider{TenantID: tenant, Name: "mock-provider-backoff", BaseURL: upstream.URL, AuthType: "bearer", Healthy: true}
	pid, err := apikey.CreateProvider(p)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if _, err := apikey.CreateKey(&apikey.AIKey{TenantID: tenant, ProviderID: pid, Name: "mock-key-backoff", Priority: 0, Enable: true}, "sk-mock-backoff"); err != nil {
		t.Fatalf("create key: %v", err)
	}
	if _, err := apikey.CreateRoute(&apikey.ModelRoute{TenantID: tenant, ModelAlias: alias, ProviderID: pid, Priority: 0, Enable: true}); err != nil {
		t.Fatalf("create route: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"`+alias+`"}`))

	resp, sel, err := forwardWithRetry(c, tenant, alias, []byte(`{"model":"`+alias+`"}`), false, "/chat/completions")
	if err != nil {
		t.Fatalf("expected backoff rounds to absorb 529 storm, got err: %v", err)
	}
	defer resp.Body.Close()
	if sel == nil {
		t.Fatal("expected key selection")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 upstream calls (529,529,200), got %d", got)
	}
}

// TestForwardWithRetrySameKeyBackoffExhausted 验证退避轮耗尽后仍返回最后的上游错误。
func TestForwardWithRetrySameKeyBackoffExhausted(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(529)
	}))
	defer upstream.Close()

	old := forwardBackoffBase
	forwardBackoffBase = time.Millisecond
	defer func() { forwardBackoffBase = old }()

	const alias = "mock-model-samekey-exhausted"
	const tenant = "platform-test"
	apikey.SetEncryptionKeyFrom("test-encryption-key")
	p := &apikey.AIProvider{TenantID: tenant, Name: "mock-provider-exhausted", BaseURL: upstream.URL, AuthType: "bearer", Healthy: true}
	pid, err := apikey.CreateProvider(p)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if _, err := apikey.CreateKey(&apikey.AIKey{TenantID: tenant, ProviderID: pid, Name: "mock-key-exhausted", Priority: 0, Enable: true}, "sk-mock-exhausted"); err != nil {
		t.Fatalf("create key: %v", err)
	}
	if _, err := apikey.CreateRoute(&apikey.ModelRoute{TenantID: tenant, ModelAlias: alias, ProviderID: pid, Priority: 0, Enable: true}); err != nil {
		t.Fatalf("create route: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"`+alias+`"}`))

	_, _, err = forwardWithRetry(c, tenant, alias, []byte(`{"model":"`+alias+`"}`), false, "/chat/completions")
	if err == nil || !strings.Contains(err.Error(), "529") {
		t.Fatalf("expected final 上游 529 error after backoff exhaustion, got %v", err)
	}
}
