package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/alert"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

// TestAlertWebhookConfig 验证告警 Webhook 通道的读取/设置端点。
// 该配置是进程内全局态（alert 包内存变量），故测试前后保存/恢复原值。
func TestAlertWebhookConfig(t *testing.T) {
	prev := alert.WebhookURL()
	t.Cleanup(func() { alert.SetWebhookURL(prev) })
	// 起点为已知干净态。
	alert.SetWebhookURL("")

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", &apipb.CurrentUser{Id: "admin-alert", UserName: "admin-alert"})
		c.Next()
	})
	userhttp.RegisterAdminRouter(r)

	// 1. 初始读取应为空（禁用推送）。
	detail := httptest.NewRecorder()
	r.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/admin/api/alerts/webhook", nil))
	resp := decodeAdminAPIResponse(t, detail.Body.Bytes())
	if resp.Code != int32(apipb.Code_Success) {
		t.Fatalf("get alerts/webhook failed: code=%d body=%s", resp.Code, detail.Body.String())
	}
	var got struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(resp.Data, &got); err != nil {
		t.Fatalf("decode url: %v", err)
	}
	if got.URL != "" {
		t.Fatalf("initial webhook url = %q, want empty", got.URL)
	}

	// 2. 设置一个飞书 webhook URL。
	const feishu = "https://open.feishu.cn/open-apis/bot/v2/hook/abc"
	update := httptest.NewRecorder()
	r.ServeHTTP(update, httptest.NewRequest(
		http.MethodPut, "/admin/api/alerts/webhook",
		strings.NewReader(`{"url":"  `+feishu+`  "}`),
	))
	upd := requireAdminSuccess(t, update.Body.Bytes())
	var set struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(upd.Data, &set); err != nil {
		t.Fatalf("decode set url: %v", err)
	}
	if set.URL != feishu {
		t.Fatalf("set response url = %q, want %q (should be trimmed)", set.URL, feishu)
	}
	if alert.WebhookURL() != feishu {
		t.Fatalf("global webhook url not applied: got %q want %q", alert.WebhookURL(), feishu)
	}

	// 3. 再次读取应回显已设置的 URL。
	recheck := httptest.NewRecorder()
	r.ServeHTTP(recheck, httptest.NewRequest(http.MethodGet, "/admin/api/alerts/webhook", nil))
	re := decodeAdminAPIResponse(t, recheck.Body.Bytes())
	var after struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(re.Data, &after); err != nil {
		t.Fatalf("decode recheck url: %v", err)
	}
	if after.URL != feishu {
		t.Fatalf("recheck url = %q, want %q", after.URL, feishu)
	}

	// 4. 传空 URL 应禁用推送（清空）。
	clear := httptest.NewRecorder()
	r.ServeHTTP(clear, httptest.NewRequest(
		http.MethodPut, "/admin/api/alerts/webhook",
		strings.NewReader(`{"url":""}`),
	))
	requireAdminSuccess(t, clear.Body.Bytes())
	if alert.WebhookURL() != "" {
		t.Fatalf("clear webhook url failed: still %q", alert.WebhookURL())
	}
}
