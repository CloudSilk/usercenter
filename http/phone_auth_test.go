package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	commonmodel "github.com/CloudSilk/pkg/model"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

type httpPhoneSender struct {
	code string
}

func (s *httpPhoneSender) SendPhoneCode(_ context.Context, _ string, code string, _ int) error {
	s.code = code
	return nil
}

func configureHTTPPhoneAuth(t *testing.T, sender *httpPhoneSender, tenantID string) {
	t.Helper()
	if err := userhttp.ConfigurePhoneAuth(userhttp.PhoneAuthConfig{
		Enabled:          true,
		TenantID:         tenantID,
		HashKey:          "http-phone-test-key",
		CodeTTLSeconds:   300,
		CooldownSeconds:  60,
		MaxAttempts:      5,
		PhoneHourlyLimit: 5,
		IPHourlyLimit:    20,
		ExposeDebugCode:  true,
		Sender:           sender,
	}); err != nil {
		t.Fatalf("ConfigurePhoneAuth: %v", err)
	}
	t.Cleanup(func() {
		_ = userhttp.ConfigurePhoneAuth(userhttp.PhoneAuthConfig{Enabled: false})
	})
}

func performJSON(r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "phone-http-test")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPublicPhoneAuthRoutesIssueCodeAndNativeToken(t *testing.T) {
	const phone = "13400134000"
	const tenantID = "phone-http-tenant"
	_ = store.DB().Unscoped().Where("mobile = ?", phone).Delete(&user.User{}).Error
	_ = store.DB().Unscoped().Where("1 = 1").Delete(&user.PhoneVerificationChallenge{}).Error
	u := &user.User{
		TenantModel: commonmodel.TenantModel{TenantID: tenantID},
		UserName:    "http-phone-user",
		Nickname:    "手机用户",
		Mobile:      phone,
		Enable:      true,
	}
	if err := store.DB().Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	sender := &httpPhoneSender{}
	configureHTTPPhoneAuth(t, sender, tenantID)
	r := gin.New()
	userhttp.RegisterPublicPhoneAuthRouter(r)

	status := httptest.NewRecorder()
	r.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/core/auth/user/phone/status", nil))
	var statusResp struct {
		Code apipb.Code `json:"code"`
		Data struct {
			Enabled bool `json:"enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(status.Body.Bytes(), &statusResp); err != nil ||
		statusResp.Code != apipb.Code_Success || !statusResp.Data.Enabled {
		t.Fatalf("status response=%#v err=%v body=%s", statusResp, err, status.Body.String())
	}

	codeResponse := performJSON(r, http.MethodPost, "/api/core/auth/user/phone/code", gin.H{"phone": phone})
	var codeResp struct {
		Code apipb.Code `json:"code"`
		Data struct {
			DebugCode   string `json:"debug_code"`
			PhoneMasked string `json:"phone_masked"`
		} `json:"data"`
	}
	if err := json.Unmarshal(codeResponse.Body.Bytes(), &codeResp); err != nil ||
		codeResp.Code != apipb.Code_Success || codeResp.Data.DebugCode == "" ||
		codeResp.Data.PhoneMasked != "134****4000" || sender.code != codeResp.Data.DebugCode {
		t.Fatalf("code response=%#v err=%v body=%s", codeResp, err, codeResponse.Body.String())
	}

	loginResponse := performJSON(r, http.MethodPost, "/api/core/auth/user/phone/login", gin.H{
		"phone": phone,
		"code":  codeResp.Data.DebugCode,
	})
	var loginResp struct {
		Code apipb.Code `json:"code"`
		Data struct {
			Token    string `json:"token"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &loginResp); err != nil ||
		loginResp.Code != apipb.Code_Success || loginResp.Data.Token == "" ||
		loginResp.Data.Username != "http-phone-user" {
		t.Fatalf("login response=%#v err=%v body=%s", loginResp, err, loginResponse.Body.String())
	}
	current, err := token.DecodeToken(loginResp.Data.Token)
	if err != nil || current.Id != u.ID || current.TenantID != tenantID || current.SessionID == "" {
		t.Fatalf("native token current=%#v err=%v", current, err)
	}
	var managed session.Session
	if err := store.DB().Where("id = ? AND principal_id = ?", current.SessionID, u.ID).First(&managed).Error; err != nil {
		t.Fatalf("phone login session was not persisted: %v", err)
	}
}

func TestPublicPhoneAuthRouteRejectsMalformedInput(t *testing.T) {
	sender := &httpPhoneSender{}
	configureHTTPPhoneAuth(t, sender, "phone-http-tenant")
	r := gin.New()
	userhttp.RegisterPublicPhoneAuthRouter(r)
	response := performJSON(r, http.MethodPost, "/api/core/auth/user/phone/code", gin.H{"phone": "123"})
	var resp struct {
		Code    apipb.Code `json:"code"`
		Message string     `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &resp); err != nil ||
		resp.Code != apipb.Code_BadRequest || resp.Message == "" {
		t.Fatalf("response=%#v err=%v body=%s", resp, err, response.Body.String())
	}
}
