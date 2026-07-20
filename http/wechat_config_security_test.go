package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/wechatconfig"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

func TestWechatConfigReadIsRedactedAndBlankUpdatePreservesSecrets(t *testing.T) {
	config := &wechatconfig.WechatConfig{
		AppID: "wx-security", AppName: "wechat-security-test",
		DisplayName: "Before", TenantID: "wechat-security-tenant", AppType: 4,
		Secret: "secret-value", Token: "token-value", EncodingAESKey: "aes-value",
	}
	if err := store.DB().Create(config).Error; err != nil {
		t.Fatalf("create config: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&wechatconfig.WechatConfig{}, "id = ?", config.ID).Error
	})

	router := gin.New()
	userhttp.RegisterWechatConfigRouter(router)
	detail := httptest.NewRecorder()
	router.ServeHTTP(detail, httptest.NewRequest(
		http.MethodGet, "/api/core/wechat/config/detail?id="+config.ID, nil,
	))
	for _, secret := range []string{"secret-value", "token-value", "aes-value"} {
		if strings.Contains(detail.Body.String(), secret) {
			t.Fatalf("detail leaked %s: %s", secret, detail.Body.String())
		}
	}

	update := &apipb.WechatConfigInfo{
		Id: config.ID, AppID: config.AppID, AppName: config.AppName,
		DisplayName: "After", TenantID: config.TenantID, AppType: config.AppType,
	}
	recorder := performJSON(router, http.MethodPut, "/api/core/wechat/config/update", update)
	var response apipb.CommonResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil ||
		response.Code != apipb.Code_Success {
		t.Fatalf("update response=%#v err=%v body=%s", response, err, recorder.Body.String())
	}

	var stored wechatconfig.WechatConfig
	if err := store.DB().First(&stored, "id = ?", config.ID).Error; err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if stored.DisplayName != "After" || stored.Secret != "secret-value" ||
		stored.Token != "token-value" || stored.EncodingAESKey != "aes-value" {
		t.Fatalf("unexpected stored config: %#v", stored)
	}
}
