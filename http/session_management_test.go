package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/auth"
	authtoken "github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

func TestSessionManagementRecordsLoginAnomalyAndForceLogout(t *testing.T) {
	const (
		tenantID = "tenant-session-management"
		userName = "session-management-user"
		password = "SessionManagement!123"
	)
	userID := mustCreateUser(t, userName, tenantID, password)
	auth.SetPIIKeyFrom("session-management-login-record-key")
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate TOTP secret: %v", err)
	}
	encryptedSecret, err := auth.EncryptPII(secret)
	if err != nil {
		t.Fatalf("encrypt TOTP secret: %v", err)
	}
	if err := store.DB().Create(&auth.MFAFactor{
		ID:          "factor-session-management",
		PrincipalID: userID,
		Type:        "totp",
		SecretEnc:   encryptedSecret,
		Name:        "operations console",
		Enable:      true,
		CreatedAt:   time.Now().Unix(),
	}).Error; err != nil {
		t.Fatalf("create MFA factor: %v", err)
	}

	router := gin.New()
	userhttp.RegisterUserRouter(router)
	userhttp.RegisterAdminRouter(router)

	first := completeMFALogin(t, router, userName, password, secret, "203.0.113.10")
	second := completeMFALogin(t, router, userName, password, secret, "203.0.113.11")
	failed := passwordLoginRequest(t, router, userName, "wrong-password", "203.0.113.12")
	if failed.Code != apipb.Code_UserNameOrPasswordIsWrong {
		t.Fatalf("wrong-password result = %#v", failed)
	}

	sessionsResponse := doJSONRequest(t, router, http.MethodGet,
		"/admin/api/sessions?active=1&keyword="+userName+"&pageIndex=1&pageSize=20", nil)
	var sessions struct {
		Code  int64           `json:"code"`
		Data  []*session.View `json:"data"`
		Total int64           `json:"total"`
	}
	if err := json.Unmarshal(sessionsResponse.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("decode sessions: %v body=%s", err, sessionsResponse.Body.String())
	}
	if sessions.Code != int64(apipb.Code_Success) || sessions.Total != 2 || len(sessions.Data) != 2 {
		t.Fatalf("unexpected sessions response: %#v", sessions)
	}
	for _, item := range sessions.Data {
		if item.PrincipalID != userID || item.UserName != userName || item.TenantName != tenantID || item.Status != session.SessionStatusActive {
			t.Fatalf("unexpected session projection: %#v", item)
		}
	}

	recordsResponse := doJSONRequest(t, router, http.MethodGet,
		"/admin/api/login-records?keyword="+userName+"&pageIndex=1&pageSize=20", nil)
	var records struct {
		Code  int64                      `json:"code"`
		Data  []*session.LoginRecordView `json:"data"`
		Total int64                      `json:"total"`
	}
	if err := json.Unmarshal(recordsResponse.Body.Bytes(), &records); err != nil {
		t.Fatalf("decode login records: %v body=%s", err, recordsResponse.Body.String())
	}
	if records.Code != int64(apipb.Code_Success) || records.Total != 5 || len(records.Data) != 5 {
		t.Fatalf("unexpected login records response: %#v", records)
	}
	var abnormalSuccess, failedPassword, mfaChallenge bool
	for _, item := range records.Data {
		switch {
		case item.Result == session.LoginResultSuccess && item.SessionID == second.SessionID:
			abnormalSuccess = item.Abnormal && item.PreviousIP == "203.0.113.10" && item.MFAUsed
		case item.Result == session.LoginResultFailed && item.ResultCode == int32(apipb.Code_UserNameOrPasswordIsWrong):
			failedPassword = !item.MFAUsed && item.IP == "203.0.113.12"
		case item.Result == session.LoginResultChallenge:
			mfaChallenge = item.MFAUsed
		}
	}
	if !abnormalSuccess || !failedPassword || !mfaChallenge {
		t.Fatalf("missing login lifecycle evidence: abnormal=%t failed=%t challenge=%t records=%#v",
			abnormalSuccess, failedPassword, mfaChallenge, records.Data)
	}

	revoke := doJSONRequest(t, router, http.MethodDelete, "/admin/api/sessions/"+first.SessionID, nil)
	var revokeBody struct {
		Code int64 `json:"code"`
	}
	if err := json.Unmarshal(revoke.Body.Bytes(), &revokeBody); err != nil || revokeBody.Code != int64(apipb.Code_Success) {
		t.Fatalf("revoke session response = %s err=%v", revoke.Body.String(), err)
	}
	var revoked session.Session
	if err := store.DB().First(&revoked, "id = ?", first.SessionID).Error; err != nil || !revoked.Revoked {
		t.Fatalf("first session was not revoked: %#v err=%v", revoked, err)
	}

	revokeAll := doJSONRequest(t, router, http.MethodPost, "/admin/api/sessions/revoke-all", map[string]string{
		"principalID": userID,
		"reason":      "security review",
	})
	var revokeAllBody struct {
		Code int64 `json:"code"`
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(revokeAll.Body.Bytes(), &revokeAllBody); err != nil ||
		revokeAllBody.Code != int64(apipb.Code_Success) || revokeAllBody.Data.Count != 1 {
		t.Fatalf("revoke-all response = %s err=%v", revokeAll.Body.String(), err)
	}
}

func completeMFALogin(
	t *testing.T,
	router *gin.Engine,
	userName string,
	password string,
	secret string,
	ip string,
) *apipb.CurrentUser {
	t.Helper()
	challenge := passwordLoginRequest(t, router, userName, password, ip)
	if challenge.Code != 41008 || challenge.Data == "" {
		t.Fatalf("password MFA challenge = %#v", challenge)
	}
	verification := loginRequest(t, router, "/api/core/auth/user/mfa/verify", map[string]interface{}{
		"mfaToken":   challenge.Data,
		"code":       auth.GenerateTOTPCode(secret),
		"deviceType": 0,
		"deviceName": "MaintNexus Operations",
	}, ip)
	if verification.Code != apipb.Code_Success || verification.Data == "" {
		t.Fatalf("MFA verification = %#v", verification)
	}
	current, err := authtoken.DecodeToken(verification.Data)
	if err != nil {
		t.Fatalf("decode MFA token: %v", err)
	}
	return current
}

func passwordLoginRequest(t *testing.T, router *gin.Engine, userName, password, ip string) *apipb.LoginResponse {
	t.Helper()
	return loginRequest(t, router, "/api/core/auth/user/login", map[string]interface{}{
		"userName":   userName,
		"password":   password,
		"deviceType": 0,
		"deviceName": "MaintNexus Operations",
	}, ip)
}

func loginRequest(t *testing.T, router *gin.Engine, path string, body map[string]interface{}, ip string) *apipb.LoginResponse {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode login request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("transid", "login-record-"+ip)
	request.RemoteAddr = ip + ":443"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	result := &apipb.LoginResponse{}
	if err := json.Unmarshal(response.Body.Bytes(), result); err != nil {
		t.Fatalf("decode login response: %v body=%s", err, response.Body.String())
	}
	return result
}
