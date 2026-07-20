package http_test

import (
	"context"
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

type loginOptionsSender struct{}

func (loginOptionsSender) SendPhoneCode(context.Context, string, string, int) error {
	return nil
}

func TestLoginOptionsArePublicAndSecretFree(t *testing.T) {
	const tenantID = "login-options-tenant"
	configs := []*wechatconfig.WechatConfig{
		{
			AppID: "wx-mini-public", AppName: "login-options-mini",
			DisplayName: "Mini", Secret: "mini-secret-must-not-leak",
			Token: "mini-token-must-not-leak", TenantID: tenantID, AppType: 1,
		},
		{
			AppID: "wx-web-public", AppName: "login-options-web",
			DisplayName: "Web", Secret: "web-secret-must-not-leak",
			EncodingAESKey: "aes-key-must-not-leak", TenantID: tenantID, AppType: 4,
		},
	}
	for _, config := range configs {
		if err := store.DB().Create(config).Error; err != nil {
			t.Fatalf("create wechat config: %v", err)
		}
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Where("tenant_id = ?", tenantID).
			Delete(&wechatconfig.WechatConfig{}).Error
		userhttp.SetSocialLogins(nil)
		_ = userhttp.ConfigurePhoneAuth(userhttp.PhoneAuthConfig{Enabled: false})
	})

	userhttp.SetSocialLogins([]userhttp.SocialLoginConfig{
		{Provider: "google", ClientID: "google-client", ClientSecret: "google-secret"},
		{Provider: "github", ClientID: "github-client", ClientSecret: "github-secret"},
	})
	if err := userhttp.ConfigurePhoneAuth(userhttp.PhoneAuthConfig{
		Enabled: true, TenantID: tenantID, HashKey: "login-options-phone-hash",
		CodeTTLSeconds: 300, CooldownSeconds: 60, MaxAttempts: 5,
		PhoneHourlyLimit: 5, IPHourlyLimit: 20, Sender: loginOptionsSender{},
	}); err != nil {
		t.Fatalf("configure phone auth: %v", err)
	}

	router := gin.New()
	userhttp.RegisterLoginOptionsRouter(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet, "/api/core/auth/login/options", nil,
	))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, secret := range []string{
		"mini-secret-must-not-leak", "mini-token-must-not-leak",
		"web-secret-must-not-leak", "aes-key-must-not-leak",
		"google-secret", "github-secret",
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("secret leaked in login options: %s", secret)
		}
	}

	var response struct {
		Code apipb.Code `json:"code"`
		Data struct {
			Phone struct {
				Enabled bool `json:"enabled"`
			} `json:"phone"`
			SocialProviders []string `json:"socialProviders"`
			Wechat          struct {
				MiniApps []map[string]any `json:"miniApps"`
				WebApps  []map[string]any `json:"webApps"`
			} `json:"wechat"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != apipb.Code_Success || !response.Data.Phone.Enabled {
		t.Fatalf("unexpected response: %#v", response)
	}
	if strings.Join(response.Data.SocialProviders, ",") != "github,google" {
		t.Fatalf("providers are not stable: %#v", response.Data.SocialProviders)
	}
	if len(response.Data.Wechat.MiniApps) != 1 || len(response.Data.Wechat.WebApps) != 1 {
		t.Fatalf("unexpected wechat apps: %#v", response.Data.Wechat)
	}
}
