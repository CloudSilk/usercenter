package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/conversation"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

// TestConversationSummarizeEndpoint 验证 POST /admin/api/conversations/:id/summarize
// 端点可达、返回更新后的会话，且对不存在的会话报错。
//
// 注意：标题生成依赖已配置的 AI Key；测试环境通常无可用的上游模型，summarizeSession
// 会静默 no-op，标题保持不变——此处只断言端点契约，不断言 LLM 生成的具体标题。
func TestConversationSummarizeEndpoint(t *testing.T) {
	const tenant = "conv-summarize-tenant"
	sessionID, err := conversation.CreateSession(&conversation.Session{
		TenantID: tenant, PrincipalID: "u-1", ModelAlias: "gpt-x", Title: "原始标题",
	})
	if err != nil || sessionID == "" {
		t.Fatalf("seed session: %v", err)
	}
	t.Cleanup(func() {
		conversation.DeleteSession(sessionID)
	})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", &apipb.CurrentUser{Id: "admin-conv", UserName: "admin-conv"})
		c.Next()
	})
	userhttp.RegisterAdminRouter(r)

	// 1. 已存在的会话：端点应成功并回显会话（title 字段存在）。
	ok := httptest.NewRecorder()
	r.ServeHTTP(ok, httptest.NewRequest(
		http.MethodPost, "/admin/api/conversations/"+sessionID+"/summarize", nil))
	resp := requireAdminSuccess(t, ok.Body.Bytes())
	var got conversation.Session
	if err := json.Unmarshal(resp.Data, &got); err != nil {
		t.Fatalf("decode summarized session: %v (data=%s)", err, string(resp.Data))
	}
	if got.ID != sessionID {
		t.Fatalf("returned session id = %q, want %q", got.ID, sessionID)
	}

	// 2. 不存在的会话：应返回 BadRequest。
	missing := httptest.NewRecorder()
	r.ServeHTTP(missing, httptest.NewRequest(
		http.MethodPost, "/admin/api/conversations/does-not-exist/summarize", nil))
	mr := decodeAdminAPIResponse(t, missing.Body.Bytes())
	if mr.Code != int32(apipb.Code_BadRequest) {
		t.Fatalf("missing session: code=%d, want BadRequest(%d) body=%s",
			mr.Code, int32(apipb.Code_BadRequest), missing.Body.String())
	}
}
