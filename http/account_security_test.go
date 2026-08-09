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
	"github.com/CloudSilk/usercenter/internal/tenant"
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
		t.Fatalf("login failed: status=%d code=%v message=%q data_present=%t",
			recorder.Code, login.Code, login.Message, login.Data != "")
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

func TestOwnPhoneChangeRequiresVerificationAndProfileCannotBypass(t *testing.T) {
	const (
		tenantID = "tenant-own-phone-change"
		oldPhone = "13800001111"
		newPhone = "13900002222"
		password = "PhoneChange!123"
	)
	userID := mustCreateUser(t, "own-phone-change-user", tenantID, password)
	if err := store.DB().Model(&user.User{}).Where("id = ?", userID).Update("mobile", oldPhone).Error; err != nil {
		t.Fatalf("set original phone: %v", err)
	}
	router := newTestEngine(&apipb.CurrentUser{Id: userID, TenantID: tenantID, UserName: "own-phone-change-user"})

	profileUpdate := decodeCommonResponse(t, doJSONRequest(t, router, http.MethodPut, "/api/core/auth/user/profile", map[string]any{
		"nickname": "换绑测试", "mobile": newPhone,
	}))
	if profileUpdate.Code != apipb.Code_Success {
		t.Fatalf("ordinary profile update failed: %v (%s)", profileUpdate.Code, profileUpdate.Message)
	}
	var account user.User
	if err := store.DB().Select("id", "mobile").First(&account, "id = ?", userID).Error; err != nil || account.Mobile != oldPhone {
		t.Fatalf("profile endpoint changed security phone: account=%#v err=%v", account, err)
	}

	_ = store.DB().Unscoped().Where("1 = 1").Delete(&user.PhoneVerificationChallenge{}).Error
	sender := &httpPhoneSender{}
	configureHTTPPhoneAuth(t, sender, tenantID)
	issued, err := user.IssuePhoneCode(t.Context(), newPhone, "203.0.113.25", "change-phone-code")
	if err != nil || issued.DebugCode == "" {
		t.Fatalf("issue new phone code: result=%#v err=%v", issued, err)
	}

	withoutProof := decodeCommonResponse(t, doJSONRequest(t, router, http.MethodPost, "/api/core/auth/user/security/phone", map[string]string{
		"phone": newPhone, "code": issued.DebugCode,
	}))
	if withoutProof.Code != apipb.Code_BadRequest {
		t.Fatalf("phone replacement without reauth returned %v", withoutProof.Code)
	}
	proof := requestAccountReauth(t, router, "change_phone:"+newPhone, password, "")
	changed := decodeCommonResponse(t, doJSONRequestWithReauth(
		t, router, http.MethodPost, "/api/core/auth/user/security/phone",
		map[string]string{"phone": newPhone, "code": issued.DebugCode}, proof,
	))
	if changed.Code != apipb.Code_Success {
		t.Fatalf("verified phone replacement failed: %v (%s)", changed.Code, changed.Message)
	}
	if err := store.DB().Select("id", "mobile").First(&account, "id = ?", userID).Error; err != nil || account.Mobile != newPhone {
		t.Fatalf("verified phone was not stored: account=%#v err=%v", account, err)
	}
}

func TestOwnPhoneFirstBindingUsesVerifiedNewNumber(t *testing.T) {
	const (
		tenantID = "tenant-own-phone-first-bind"
		phone    = "13700003333"
	)
	userID := mustCreateUser(t, "own-phone-first-bind-user", tenantID, "FirstBind!123")
	_ = store.DB().Unscoped().Where("1 = 1").Delete(&user.PhoneVerificationChallenge{}).Error
	sender := &httpPhoneSender{}
	configureHTTPPhoneAuth(t, sender, tenantID)
	issued, err := user.IssuePhoneCode(t.Context(), phone, "203.0.113.26", "first-bind-code")
	if err != nil || issued.DebugCode == "" {
		t.Fatalf("issue first-bind code: result=%#v err=%v", issued, err)
	}

	router := newTestEngine(&apipb.CurrentUser{Id: userID, TenantID: tenantID, UserName: "own-phone-first-bind-user"})
	bound := decodeCommonResponse(t, doJSONRequest(t, router, http.MethodPost, "/api/core/auth/user/security/phone", map[string]string{
		"phone": phone, "code": issued.DebugCode,
	}))
	if bound.Code != apipb.Code_Success {
		t.Fatalf("verified first phone binding failed: %v (%s)", bound.Code, bound.Message)
	}
	var account user.User
	if err := store.DB().Select("id", "mobile").First(&account, "id = ?", userID).Error; err != nil || account.Mobile != phone {
		t.Fatalf("first phone binding was not stored: account=%#v err=%v", account, err)
	}
}

func TestAccountSecuritySummaryAndOwnSessionRevoke(t *testing.T) {
	const (
		tenantID = "tenant-account-security"
		userName = "account-security-user"
		password = "AccountSecurity!123"
	)
	userID := mustCreateUser(t, userName, tenantID, password)
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
	auth.SetPIIKeyFrom("account-security-summary-test-key")
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate MFA secret: %v", err)
	}
	encryptedSecret, err := auth.EncryptPII(secret)
	if err != nil {
		t.Fatalf("encrypt MFA secret: %v", err)
	}
	if err := store.DB().Create(&auth.MFAFactor{
		ID: "factor-account-security", PrincipalID: userID, Type: "totp",
		SecretEnc: encryptedSecret, Name: "生产主管手机", Enable: true, CreatedAt: time.Now().Add(-time.Hour).Unix(),
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
			LoginVerified    bool   `json:"loginIdentifierVerified"`
			MFAEnabled       bool   `json:"mfaEnabled"`
			MFARecovery      string `json:"mfaRecovery"`
			CurrentSessionID string `json:"currentSessionID"`
			DataScope        struct {
				Code string `json:"code"`
			} `json:"dataScope"`
			AvailableTenants []struct {
				ID      string `json:"id"`
				Current bool   `json:"current"`
			} `json:"availableTenants"`
			Roles []struct {
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
	if !summary.Data.LoginVerified || summary.Data.MFARecovery == "" || summary.Data.DataScope.Code == "" ||
		len(summary.Data.AvailableTenants) != 1 || !summary.Data.AvailableTenants[0].Current {
		t.Fatalf("unexpected account context summary: %#v", summary.Data)
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

	withoutProof := decodeCommonResponse(t, doJSONRequest(t, router, http.MethodDelete, "/api/core/auth/user/security/sessions/"+other.ID, nil))
	if withoutProof.Code != apipb.Code_BadRequest {
		t.Fatalf("other-session revoke without reauth returned %v", withoutProof.Code)
	}
	proof := requestAccountReauth(t, router, "revoke_session:"+other.ID, password, auth.GenerateTOTPCode(secret))
	revoke := doJSONRequestWithReauth(t, router, http.MethodDelete, "/api/core/auth/user/security/sessions/"+other.ID, nil, proof)
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

func TestAccountSecuritySwitchTenantAndRevokeAllUseBoundReauth(t *testing.T) {
	const (
		originTenant = "tenant-switch-origin"
		targetTenant = "tenant-switch-target"
		password     = "TenantSwitch!123"
	)
	userID := mustCreateUser(t, "tenant-switch-user", originTenant, password)
	_ = tenant.CreateTenant(&tenant.Tenant{
		Model: commonmodel.Model{ID: targetTenant}, Name: "目标标签工厂", Enable: true,
		Expired: time.Now().Add(24 * time.Hour), UserCount: 100,
	})
	targetRole := permission.Role{
		Model: commonmodel.Model{ID: "role-tenant-switch-target"}, TenantID: targetTenant,
		Name: "目标租户审核员", Enable: true,
	}
	if err := store.DB().Create(&targetRole).Error; err != nil {
		t.Fatalf("create target role: %v", err)
	}
	if err := store.DB().Create(&user.UserRole{
		Model: commonmodel.Model{ID: "assignment-tenant-switch-target"}, UserID: userID, RoleID: targetRole.ID,
	}).Error; err != nil {
		t.Fatalf("assign target role: %v", err)
	}
	currentSession := session.Session{
		Model: commonmodel.Model{ID: "session-before-tenant-switch"}, PrincipalID: userID, TenantID: originTenant,
		TokenSig: "switch-old-token", DeviceType: 0, DeviceName: "Chrome · Windows", IP: "192.0.2.8",
		Location: "上海", LastActiveAt: time.Now().Unix(),
	}
	if err := store.DB().Create(&currentSession).Error; err != nil {
		t.Fatalf("create current session: %v", err)
	}
	router := newTestEngine(&apipb.CurrentUser{
		Id: userID, TenantID: originTenant, UserName: "tenant-switch-user",
		SessionID: currentSession.ID, DeviceType: currentSession.DeviceType, ClientIP: currentSession.IP,
	})

	wrongActionProof := requestAccountReauth(t, router, "switch_tenant:"+targetTenant, password, "")
	wrongAction := decodeCommonResponse(t, doJSONRequestWithReauth(
		t, router, http.MethodPost, "/api/core/auth/user/security/sessions/revoke-all", nil, wrongActionProof,
	))
	if wrongAction.Code != apipb.Code_BadRequest {
		t.Fatalf("action-bound proof was accepted by another operation: %v", wrongAction.Code)
	}

	switchProof := requestAccountReauth(t, router, "switch_tenant:"+targetTenant, password, "")
	switchResponse := doJSONRequestWithReauth(
		t, router, http.MethodPost, "/api/core/auth/user/security/tenant/switch",
		map[string]string{"tenantID": targetTenant}, switchProof,
	)
	var switched struct {
		Code int32 `json:"code"`
		Data struct {
			Token  string `json:"token"`
			Tenant struct {
				ID string `json:"id"`
			} `json:"tenant"`
		} `json:"data"`
	}
	if err := json.Unmarshal(switchResponse.Body.Bytes(), &switched); err != nil || switched.Code != int32(apipb.Code_Success) {
		t.Fatalf("switch tenant failed: body=%s err=%v", switchResponse.Body.String(), err)
	}
	current, err := token.DecodeToken(switched.Data.Token)
	if err != nil || current.TenantID != targetTenant || len(current.RoleIDs) != 1 || current.RoleIDs[0] != targetRole.ID {
		t.Fatalf("unexpected switched token: current=%#v err=%v", current, err)
	}
	var oldSession session.Session
	if err := store.DB().First(&oldSession, "id = ?", currentSession.ID).Error; err != nil || !oldSession.Revoked {
		t.Fatalf("old tenant session was not revoked: %#v err=%v", oldSession, err)
	}
	var newSession session.Session
	if err := store.DB().First(&newSession, "id = ?", current.SessionID).Error; err != nil || newSession.Revoked || newSession.TenantID != targetTenant {
		t.Fatalf("replacement session was not persisted: %#v err=%v", newSession, err)
	}
	reusedProof := decodeCommonResponse(t, doJSONRequestWithReauth(
		t, router, http.MethodPost, "/api/core/auth/user/security/tenant/switch",
		map[string]string{"tenantID": targetTenant}, switchProof,
	))
	if reusedProof.Code == apipb.Code_Success {
		t.Fatal("single-use reauthentication proof was accepted twice")
	}

	revokeAllProof := requestAccountReauth(t, router, "revoke_all_sessions", password, "")
	revokeAll := decodeCommonResponse(t, doJSONRequestWithReauth(
		t, router, http.MethodPost, "/api/core/auth/user/security/sessions/revoke-all", nil, revokeAllProof,
	))
	if revokeAll.Code != apipb.Code_Success {
		t.Fatalf("revoke all sessions failed: %v (%s)", revokeAll.Code, revokeAll.Message)
	}
	if err := store.DB().First(&newSession, "id = ?", newSession.ID).Error; err != nil || !newSession.Revoked {
		t.Fatalf("replacement session remained active after revoke all: %#v err=%v", newSession, err)
	}
}

func requestAccountReauth(t *testing.T, router *gin.Engine, action, password, mfaCode string) string {
	t.Helper()
	response := doJSONRequest(t, router, http.MethodPost, "/api/core/auth/user/security/reverify", map[string]string{
		"action": action, "password": password, "mfaCode": mfaCode,
	})
	var result struct {
		Code int32 `json:"code"`
		Data struct {
			Proof string `json:"proof"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Code != int32(apipb.Code_Success) || result.Data.Proof == "" {
		t.Fatalf("account reauth failed: body=%s err=%v", response.Body.String(), err)
	}
	return result.Data.Proof
}

func doJSONRequestWithReauth(t *testing.T, router *gin.Engine, method, path string, body any, proof string) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-UserCenter-Reauth", proof)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}
