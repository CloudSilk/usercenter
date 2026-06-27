package usercenterclient

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// recordedRequest 捕获 mock 服务收到的请求，供断言。
type recordedRequest struct {
	method      string
	path        string
	authHeader  string
	contentType string
	body        string
}

// newMockServer 启动一个模拟 usercenter 的测试服务器，返回 (server, 最后收到的请求)。
// 每个端点返回固定的 OpenAI 风格响应。
func newMockServer(t *testing.T) (*httptest.Server, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/core/auth/user/login", func(w http.ResponseWriter, r *http.Request) {
		rec.method, rec.path, rec.authHeader = r.Method, r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":20000,"message":"ok","data":"jwt-token-xyz"}`))
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		capture(rec, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"role":"assistant","content":"hi there"}}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`))
	})

	mux.HandleFunc("/v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
		capture(rec, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2],"index":0}],"usage":{"prompt_tokens":4}}`))
	})

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		capture(rec, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-4","object":"model"}]}`))
	})

	mux.HandleFunc("/v1/audio/transcriptions", func(w http.ResponseWriter, r *http.Request) {
		capture(rec, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hello world"}`))
	})

	mux.HandleFunc("/v1/moderations", func(w http.ResponseWriter, r *http.Request) {
		capture(rec, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"flagged":false}]}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, rec
}

func capture(rec *recordedRequest, r *http.Request) {
	rec.method = r.Method
	rec.path = r.URL.Path
	rec.authHeader = r.Header.Get("Authorization")
	rec.contentType = r.Header.Get("Content-Type")
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		rec.body = string(b)
		_ = r.Body.Close()
	}
}

func TestClient_LoginSetsToken(t *testing.T) {
	srv, rec := newMockServer(t)
	c := NewClient(srv.URL)

	resp, err := c.Login("admin", "Admin@123456")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if resp.Data != "jwt-token-xyz" {
		t.Fatalf("expected token jwt-token-xyz, got %q", resp.Data)
	}
	if c.Token != "jwt-token-xyz" {
		t.Fatalf("client did not store token, got %q", c.Token)
	}
	if rec.method != "POST" || rec.path != "/api/core/auth/user/login" {
		t.Fatalf("unexpected request: %s %s", rec.method, rec.path)
	}
}

func TestClient_ChatCompletion(t *testing.T) {
	srv, rec := newMockServer(t)
	c := NewClient(srv.URL)
	c.SetToken("tok-1")

	body, err := c.ChatCompletion("gpt-4", []map[string]string{
		{"role": "user", "content": "hello"},
	}, map[string]any{"session_id": "sess-1", "stream": false})
	if err != nil {
		t.Fatalf("chat completion failed: %v", err)
	}

	// 鉴权头
	if rec.authHeader != "Bearer tok-1" {
		t.Fatalf("expected Bearer tok-1, got %q", rec.authHeader)
	}
	// body 含 model/messages 与增强字段
	var got map[string]any
	if err := json.Unmarshal([]byte(rec.body), &got); err != nil {
		t.Fatalf("body not JSON: %v (body=%s)", err, rec.body)
	}
	if got["model"] != "gpt-4" {
		t.Fatalf("expected model gpt-4, got %v", got["model"])
	}
	if got["session_id"] != "sess-1" {
		t.Fatalf("expected session_id sess-1, got %v", got["session_id"])
	}
	// 原始响应体回传
	if !strings.Contains(string(body), "hi there") {
		t.Fatalf("unexpected response body: %s", string(body))
	}
}

func TestClient_Embeddings(t *testing.T) {
	srv, rec := newMockServer(t)
	c := NewClient(srv.URL)

	body, err := c.Embeddings("text-embedding-3-small", "hello")
	if err != nil {
		t.Fatalf("embeddings failed: %v", err)
	}
	if rec.path != "/v1/embeddings" || rec.method != "POST" {
		t.Fatalf("unexpected request: %s %s", rec.method, rec.path)
	}
	if !strings.Contains(string(body), "embedding") {
		t.Fatalf("unexpected embeddings body: %s", string(body))
	}
}

func TestClient_ListModels(t *testing.T) {
	srv, rec := newMockServer(t)
	c := NewClient(srv.URL)

	body, err := c.ListModels()
	if err != nil {
		t.Fatalf("list models failed: %v", err)
	}
	if rec.method != "GET" || rec.path != "/v1/models" {
		t.Fatalf("unexpected request: %s %s", rec.method, rec.path)
	}
	if !strings.Contains(string(body), "gpt-4") {
		t.Fatalf("unexpected models body: %s", string(body))
	}
}

func TestClient_AudioTranscriptionMultipart(t *testing.T) {
	srv, rec := newMockServer(t)
	c := NewClient(srv.URL)

	// 写一个临时音频文件
	tmp := t.TempDir() + "/audio.mp3"
	if err := os.WriteFile(tmp, []byte("fake-audio-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	body, err := c.AudioTranscription("whisper-1", tmp)
	if err != nil {
		t.Fatalf("audio transcription failed: %v", err)
	}
	// multipart：Content-Type 必须含 boundary
	if !strings.HasPrefix(rec.contentType, "multipart/form-data; boundary=") {
		t.Fatalf("expected multipart content-type, got %q", rec.contentType)
	}
	// body 含文件字段与 model 字段
	if !strings.Contains(rec.body, "name=\"file\"") {
		t.Fatalf("multipart body missing file field: %s", rec.body)
	}
	if !strings.Contains(rec.body, "whisper-1") {
		t.Fatalf("multipart body missing model: %s", rec.body)
	}
	if !strings.Contains(string(body), "hello world") {
		t.Fatalf("unexpected transcription body: %s", string(body))
	}
}

func TestClient_Moderation(t *testing.T) {
	srv, rec := newMockServer(t)
	c := NewClient(srv.URL)

	body, err := c.Moderation("text-moderation-latest", "some text")
	if err != nil {
		t.Fatalf("moderation failed: %v", err)
	}
	if rec.path != "/v1/moderations" {
		t.Fatalf("unexpected path: %s", rec.path)
	}
	if !strings.Contains(string(body), "flagged") {
		t.Fatalf("unexpected moderation body: %s", string(body))
	}
}
