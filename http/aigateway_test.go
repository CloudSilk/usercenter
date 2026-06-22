package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/gin-gonic/gin"
)

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
	pt, ct := streamProxy(c, resp)

	if pt != 10 || ct != 2 {
		t.Fatalf("expected prompt=10 comp=2, got prompt=%d comp=%d", pt, ct)
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
		wantURL   string
	}{
		{"bearer", "bearer", "https://api.openai.com/v1", "sk-x", "Authorization", "Bearer sk-x", "https://api.openai.com/v1/chat/completions"},
		{"default-empty", "", "https://api.deepseek.com", "sk-y", "Authorization", "Bearer sk-y", "https://api.deepseek.com/chat/completions"},
		{"header", "header", "https://api.anthropic.com/v1", "sk-z", "Authorization", "sk-z", "https://api.anthropic.com/v1/chat/completions"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel := &apikey.KeySelection{
				Provider: &apikey.AIProvider{BaseURL: tc.baseURL, AuthType: tc.auth},
				APIKey:   tc.storedKey,
			}
			req, err := buildUpstreamRequest(sel, []byte(`{"model":"x"}`))
			if err != nil {
				t.Fatal(err)
			}
			if req.URL.String() != tc.wantURL {
				t.Fatalf("url: got %s want %s", req.URL.String(), tc.wantURL)
			}
			if got := req.Header.Get(tc.wantHdr); got != tc.wantVal {
				t.Fatalf("header %s: got %q want %q", tc.wantHdr, got, tc.wantVal)
			}
		})
	}
}
