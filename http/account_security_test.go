package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/authn"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

func TestPasswordLoginCreatesManageableSession(t *testing.T) {
	const (
		tenantID = "tenant-login-session"
		userName = "login-session-user"
		password = "LoginSession!123"
		clientIP = "203.0.113.7"
	)
	userID := mustCreateUser(t, userName, tenantID, password)

	router := gin.New()
	userhttp.RegisterUserRouter(router)
	payload, err := json.Marshal(map[string]string{
		"userName": userName,
		"password": password,
	})
	if err != nil {
		t.Fatalf("encode login payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/core/auth/user/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0 Safari/537.36")
	req.RemoteAddr = clientIP + ":45678"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	var login apipb.LoginResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &login); err != nil {
		t.Fatalf("decode login response: %v (body=%s)", err, recorder.Body.String())
	}
	if recorder.Code != http.StatusOK || login.Code != apipb.Code_Success || login.Data == "" {
		t.Fatalf("login failed: status=%d response=%#v", recorder.Code, login)
	}

	current, err := token.DecodeToken(login.Data)
	if err != nil {
		t.Fatalf("decode login token: %v", err)
	}
	if current.SessionID == "" || current.Id != userID || current.TenantID != tenantID || current.DeviceType != 0 || current.ClientIP != clientIP {
		t.Fatalf("unexpected login token session claims: %#v", current)
	}

	var persisted session.Session
	if err := store.DB().First(&persisted, "id = ?", current.SessionID).Error; err != nil {
		t.Fatalf("load persisted login session: %v", err)
	}
	if persisted.PrincipalID != userID || persisted.TenantID != tenantID || persisted.TokenSig != token.GetTokenSignature(login.Data) ||
		persisted.DeviceType != 0 || persisted.DeviceName != "Chrome · Windows" || persisted.IP != clientIP || persisted.Revoked {
		t.Fatalf("unexpected persisted login session: %#v", persisted)
	}

	if _, authenticated, code, err := authn.AuthenticatePrincipal(login.Data, http.MethodGet, "/api/core/auth/user/profile", false); err != nil || code != commonmodel.Success || authenticated == nil {
		t.Fatalf("fresh login token was not accepted: user=%#v code=%d err=%v", authenticated, code, err)
	}
	if revoked, err := session.RevokeSessionForPrincipal(current.SessionID, userID, "test revoke"); err != nil || !revoked {
		t.Fatalf("revoke login session: revoked=%v err=%v", revoked, err)
	}
	if _, _, code, err := authn.AuthenticatePrincipal(login.Data, http.MethodGet, "/api/core/auth/user/profile", false); err == nil || code != commonmodel.TokenInvalid {
		t.Fatalf("revoked login token remained valid: code=%d err=%v", code, err)
	}
}

func TestAccountSecuritySummaryAndOwnSessionRevoke(t *testing.T) {
	const (
		tenantID = "tenant-account-security"
		userName = "account-security-user"
	)
	userID := mustCreateUser(t, userName, tenantID, "AccountSecurity!123")
	if err := store.DB().Model(&user.User{}).Where("id = ?", userID).Updates(map[string]any{
		"real_name": "李文", "email": "li.wen@example.com", "mobile": "13800000000",
	}).Error; err != nil {
		t.Fatalf("update account profile: %v", err)
	}

	role := permission.Role{
		Model: commonmodel.Model{ID: "role-account-security"}, TenantID: tenantID,
		Name: "生产主管", Enable: true,
	}
	if err := store.DB().Create(&role).Error; err != nil {
		t.Fatalf("create account role: %v", err)
	}
	if err := store.DB().Create(&user.UserRole{
		Model: commonmodel.Model{ID: "assignment-account-security"}, UserID: userID, RoleID: role.ID,
	}).Error; err != nil {
		t.Fatalf("assign account role: %v", err)
	}
	if err := store.DB().Create(&auth.MFAFactor{
		ID: "factor-account-security", PrincipalID: userID, Type: "totp",
		SecretEnc: "must-never-leak", Name: "生产主管手机", Enable: true, CreatedAt: time.Now().Add(-time.Hour).Unix(),
	}).Error; err != nil {
		t.Fatalf("create account MFA factor: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	current := session.Session{
		Model:       commonmodel.Model{ID: "session-account-current", CreatedAt: now.Add(-2 * time.Hour)},
		PrincipalID: userID, TenantID: tenantID, TokenSig: "secret-current-token",
		DeviceType: 1, DeviceName: "Android A13", IP: "127.0.0.1", Location: "上海", LastActiveAt: now.Unix(),
	}
	other := session.Session{
		Model:       commonmodel.Model{ID: "session-account-other", CreatedAt: now.Add(-24 * time.Hour)},
		PrincipalID: userID, TenantID: tenantID, TokenSig: "secret-other-token",
		DeviceType: 0, DeviceName: "Chrome on Windows", IP: "10.0.0.8", Location: "杭州", LastActiveAt: now.Add(-time.Hour).Unix(),
	}
	foreign := session.Session{
		Model:       commonmodel.Model{ID: "session-account-foreign", CreatedAt: now.Add(-time.Hour)},
		PrincipalID: "another-user", TenantID: tenantID, TokenSig: "secret-foreign-token",
		DeviceType: 2, DeviceName: "Foreign iPhone", LastActiveAt: now.Unix(),
	}
	if err := store.DB().Create(&[]session.Session{current, other, foreign}).Error; err != nil {
		t.Fatalf("create account sessions: %v", err)
	}

	router := newTestEngine(&apipb.CurrentUser{
		Id: userID, TenantID: tenantID, UserName: userName, SessionID: current.ID,
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/core/auth/user/security/summary", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("summary HTTP status = %d", recorder.Code)
	}
	if body := recorder.Body.String(); strings.Contains(body, "must-never-leak") || strings.Contains(body, "secret-current-token") || strings.Contains(body, foreign.DeviceName) {
		t.Fatalf("security summary leaked private state: %s", body)
	}
	var summary struct {
		Code int `json:"code"`
		Data struct {
			UserID           string `json:"userID"`
			TenantName       string `json:"tenantName"`
			DisplayName      string `json:"displayName"`
			MFAEnabled       bool   `json:"mfaEnabled"`
			CurrentSessionID string `json:"currentSessionID"`
			Roles            []struct {
				Name string `json:"name"`
			} `json:"roles"`
			Sessions []struct {
				ID      string `json:"id"`
				Current bool   `json:"current"`
			} `json:"sessions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode security summary: %v", err)
	}
	if summary.Code != 20000 || summary.Data.UserID != userID || summary.Data.DisplayName != "李文" || summary.Data.TenantName != tenantID {
		t.Fatalf("unexpected security summary identity: %#v", summary)
	}
	if !summary.Data.MFAEnabled || len(summary.Data.Roles) != 1 || summary.Data.Roles[0].Name != role.Name {
		t.Fatalf("unexpected security summary roles/MFA: %#v", summary.Data)
	}
	if len(summary.Data.Sessions) != 2 || summary.Data.CurrentSessionID != current.ID {
		t.Fatalf("unexpected self sessions: %#v", summary.Data.Sessions)
	}
	currentMarked := false
	for _, item := range summary.Data.Sessions {
		if item.ID == current.ID && item.Current {
			currentMarked = true
		}
	}
	if !currentMarked {
		t.Fatalf("current session was not marked: %#v", summary.Data.Sessions)
	}

	revoke := httptest.NewRecorder()
	router.ServeHTTP(revoke, httptest.NewRequest(http.MethodDelete, "/api/core/auth/user/security/sessions/"+other.ID, nil))
	var revokeResponse struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(revoke.Body.Bytes(), &revokeResponse); err != nil || revokeResponse.Code != 20000 {
		t.Fatalf("revoke own session response = %s err=%v", revoke.Body.String(), err)
	}
	var revoked session.Session
	if err := store.DB().First(&revoked, "id = ?", other.ID).Error; err != nil || !revoked.Revoked {
		t.Fatalf("own session was not revoked: %#v err=%v", revoked, err)
	}

	deny := httptest.NewRecorder()
	router.ServeHTTP(deny, httptest.NewRequest(http.MethodDelete, "/api/core/auth/user/security/sessions/"+foreign.ID, nil))
	var denyResponse struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(deny.Body.Bytes(), &denyResponse); err != nil || denyResponse.Code != 40000 {
		t.Fatalf("foreign session revoke response = %s err=%v", deny.Body.String(), err)
	}
	var untouched session.Session
	if err := store.DB().First(&untouched, "id = ?", foreign.ID).Error; err != nil || untouched.Revoked {
		t.Fatalf("foreign session was modified: %#v err=%v", untouched, err)
	}
}
