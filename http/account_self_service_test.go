package http_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
)

func TestSelfServiceAuthorizationIsSeededForProductionAuth(t *testing.T) {
	definitions := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/core/auth/user/profile"},
		{http.MethodPut, "/api/core/auth/user/profile"},
		{http.MethodPost, "/api/core/auth/user/changepwd"},
		{http.MethodPost, "/api/core/auth/user/logout"},
		{http.MethodPost, "/api/core/auth/user/token/refresh"},
		{http.MethodPost, "/api/core/auth/user/mfa/totp/enroll"},
		{http.MethodPost, "/api/core/auth/user/mfa/totp/confirm"},
		{http.MethodGet, "/api/core/auth/user/mfa/factors"},
		{http.MethodDelete, "/api/core/auth/user/mfa/:id"},
		{http.MethodGet, "/api/core/auth/user/security/summary"},
		{http.MethodPost, "/api/core/auth/user/security/reverify"},
		{http.MethodPost, "/api/core/auth/user/security/sessions/revoke-all"},
		{http.MethodDelete, "/api/core/auth/user/security/sessions/:id"},
		{http.MethodPost, "/api/core/auth/user/security/tenant/switch"},
		{http.MethodDelete, "/api/core/auth/user/security/account"},
	}
	for _, definition := range definitions {
		var api permission.API
		if err := store.DB().
			Where("path = ? AND method = ?", definition.path, definition.method).
			First(&api).Error; err != nil {
			t.Fatalf("query self-service API %s %s: %v", definition.method, definition.path, err)
		}
		if !api.Enable || api.CheckAuth || !api.CheckLogin || !api.IsMust {
			t.Fatalf("unexpected self-service API policy for %s %s: %#v", definition.method, definition.path, api)
		}
		allowed, err := permission.EnforceCached("0", definition.path, definition.method)
		if err != nil {
			t.Fatalf("enforce self-service API %s %s: %v", definition.method, definition.path, err)
		}
		if !allowed {
			t.Fatalf("authenticated users cannot access %s %s", definition.method, definition.path)
		}
	}
}

func TestDeleteOwnAccountRequiresBoundReauthAndDeletesOnlyCurrentUser(t *testing.T) {
	currentID := mustCreateUser(t, "delete-self", "delete-tenant", "Delete12345")
	foreignID := mustCreateUser(t, "delete-foreign", "delete-tenant", "Foreign12345")
	current := &apipb.CurrentUser{Id: currentID, TenantID: "delete-tenant", UserName: "delete-self"}
	router := newTestEngine(current)

	withoutProof := decodeCommonResponse(t, doJSONRequest(
		t, router, http.MethodDelete, "/api/core/auth/user/security/account", nil,
	))
	if withoutProof.Code != apipb.Code_BadRequest {
		t.Fatalf("delete without reauth returned %v", withoutProof.Code)
	}

	proof := requestAccountReauth(t, router, "delete_account", "Delete12345", "")
	deleted := decodeCommonResponse(t, doJSONRequestWithReauth(
		t, router, http.MethodDelete, "/api/core/auth/user/security/account", nil, proof,
	))
	if deleted.Code != apipb.Code_Success {
		t.Fatalf("delete own account failed: %v (%s)", deleted.Code, deleted.Message)
	}
	var currentCount, foreignCount int64
	store.DB().Model(&user.User{}).Where("id = ?", currentID).Count(&currentCount)
	store.DB().Model(&user.User{}).Where("id = ?", foreignID).Count(&foreignCount)
	if currentCount != 0 || foreignCount != 1 {
		t.Fatalf("unexpected deletion boundary: current=%d foreign=%d", currentCount, foreignCount)
	}
}

func TestPhoneRegisteredAccountCanReverifyWithBoundPhoneCode(t *testing.T) {
	const phone = "13800138888"
	currentID := mustCreateUser(t, "phone-reauth-self", "phone-reauth-tenant", "Internal12345")
	if err := store.DB().Model(&user.User{}).Where("id = ?", currentID).Update("mobile", phone).Error; err != nil {
		t.Fatalf("bind test phone: %v", err)
	}
	_ = store.DB().Unscoped().Where("1 = 1").Delete(&user.PhoneVerificationChallenge{}).Error
	sender := &httpPhoneSender{}
	configureHTTPPhoneAuth(t, sender, "phone-reauth-tenant")
	if _, err := user.IssuePhoneCode(t.Context(), phone, "127.0.0.1", "phone-reauth-code"); err != nil {
		t.Fatalf("issue reauth code: %v", err)
	}
	router := newTestEngine(&apipb.CurrentUser{
		Id: currentID, TenantID: "phone-reauth-tenant", UserName: "phone-reauth-self",
	})
	response := doJSONRequest(t, router, http.MethodPost, "/api/core/auth/user/security/reverify", map[string]string{
		"action": "delete_account", "phoneCode": sender.code,
	})
	var result struct {
		Code int32 `json:"code"`
		Data struct {
			Proof string `json:"proof"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil ||
		result.Code != int32(apipb.Code_Success) || result.Data.Proof == "" {
		t.Fatalf("phone reauth failed: %#v err=%v body=%s", result, err, response.Body.String())
	}
}

func TestUpdateProfileOnlyChangesCurrentUserAndRecordsAudit(t *testing.T) {
	currentID := mustCreateUser(t, "profile-current", "profile-tenant", "Old12345")
	foreignID := mustCreateUser(t, "profile-foreign", "profile-tenant", "Bee12345")
	current := &apipb.CurrentUser{
		Id: currentID, TenantID: "profile-tenant", UserName: "profile-current",
	}

	resp := decodeCommonResponse(t, doJSONRequest(
		t,
		newTestEngine(current),
		http.MethodPut,
		"/api/core/auth/user/profile",
		map[string]any{
			"id":       foreignID,
			"tenantID": "other-tenant",
			"nickname": "当前用户新昵称",
			"mobile":   "13800000001",
			"email":    "current@example.com",
			"realName": "当前用户",
		},
	))
	if resp.Code != apipb.Code_Success {
		t.Fatalf("update profile failed: %v (%s)", resp.Code, resp.Message)
	}

	var currentUser user.User
	if err := store.DB().First(&currentUser, "id = ?", currentID).Error; err != nil {
		t.Fatalf("query current user: %v", err)
	}
	if currentUser.Nickname != "当前用户新昵称" ||
		currentUser.Email != "current@example.com" ||
		currentUser.TenantID != "profile-tenant" {
		t.Fatalf("current user profile not updated safely: %#v", currentUser)
	}
	var foreignUser user.User
	if err := store.DB().First(&foreignUser, "id = ?", foreignID).Error; err != nil {
		t.Fatalf("query foreign user: %v", err)
	}
	if foreignUser.Nickname != "profile-foreign" || foreignUser.Email != "" {
		t.Fatalf("foreign user was modified: %#v", foreignUser)
	}

	var auditCount int64
	if err := store.DB().Model(&audit.AuditLog{}).
		Where("action = ? AND user_id = ? AND target_id = ?",
			audit.AuditActionUpdateProfile, currentID, currentID).
		Count(&auditCount).Error; err != nil {
		t.Fatalf("count profile audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("profile audit count = %d, want 1", auditCount)
	}
}

func TestChangePasswordClearsForceChangeFlagAndRecordsAudit(t *testing.T) {
	currentID := mustCreateUser(t, "self-password", "password-tenant", "Old12345")
	if err := store.DB().Model(&user.User{}).
		Where("id = ?", currentID).
		Update("force_change_pwd", true).Error; err != nil {
		t.Fatalf("mark force-change password: %v", err)
	}
	current := &apipb.CurrentUser{
		Id: currentID, TenantID: "password-tenant", UserName: "self-password",
	}

	resp := decodeCommonResponse(t, doJSONRequest(
		t,
		newTestEngine(current),
		http.MethodPost,
		"/api/core/auth/user/changepwd",
		map[string]any{
			"oldPwd":        "Old12345",
			"newPwd":        "New12345",
			"newConfirmPwd": "New12345",
		},
	))
	if resp.Code != apipb.Code_Success {
		t.Fatalf("change password failed: %v (%s)", resp.Code, resp.Message)
	}
	var changed user.User
	if err := store.DB().First(&changed, "id = ?", currentID).Error; err != nil {
		t.Fatalf("query changed user: %v", err)
	}
	if changed.ForceChangePwd {
		t.Fatal("self-service password change must clear force_change_pwd")
	}
	if !canLogin(t, "self-password", "New12345") {
		t.Fatal("new password is not usable")
	}

	var auditCount int64
	if err := store.DB().Model(&audit.AuditLog{}).
		Where("action = ? AND user_id = ? AND target_id = ?",
			audit.AuditActionChangePwd, currentID, currentID).
		Count(&auditCount).Error; err != nil {
		t.Fatalf("count password audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("password audit count = %d, want 1", auditCount)
	}
}

func TestMFAUserRoutesConfirmListAndEnforceOwnerBoundary(t *testing.T) {
	auth.SetPIIKeyFrom("account-self-service-test-key")
	currentID := mustCreateUser(t, "mfa-self", "mfa-tenant", "Old12345")
	foreignID := mustCreateUser(t, "mfa-foreign", "mfa-tenant", "Bee12345")
	current := &apipb.CurrentUser{
		Id: currentID, TenantID: "mfa-tenant", UserName: "mfa-self",
	}
	router := newTestEngine(current)

	enroll := doJSONRequest(
		t,
		router,
		http.MethodPost,
		"/api/core/auth/user/mfa/totp/enroll",
		nil,
	)
	var enrollResp struct {
		Code int32 `json:"code"`
		Data struct {
			Secret  string `json:"secret"`
			URI     string `json:"uri"`
			Account string `json:"account"`
		} `json:"data"`
	}
	if err := json.Unmarshal(enroll.Body.Bytes(), &enrollResp); err != nil {
		t.Fatalf("decode MFA enrollment: %v (body=%s)", err, enroll.Body.String())
	}
	if enrollResp.Code != int32(apipb.Code_Success) ||
		enrollResp.Data.Secret == "" ||
		enrollResp.Data.URI == "" ||
		enrollResp.Data.Account != "mfa-self" {
		t.Fatalf("unexpected MFA enrollment: %#v", enrollResp)
	}

	wrongCode := "000000"
	if wrongCode == auth.GenerateTOTPCode(enrollResp.Data.Secret) {
		wrongCode = "999999"
	}
	wrong := decodeCommonResponse(t, doJSONRequest(
		t,
		router,
		http.MethodPost,
		"/api/core/auth/user/mfa/totp/confirm",
		map[string]any{
			"secret": enrollResp.Data.Secret,
			"code":   wrongCode,
			"name":   "工作手机",
		},
	))
	if wrong.Code != apipb.Code_BadRequest {
		t.Fatalf("wrong MFA code returned %v, want bad request", wrong.Code)
	}

	confirm := doJSONRequest(
		t,
		router,
		http.MethodPost,
		"/api/core/auth/user/mfa/totp/confirm",
		map[string]any{
			"secret": enrollResp.Data.Secret,
			"code":   auth.GenerateTOTPCode(enrollResp.Data.Secret),
			"name":   "工作手机",
		},
	)
	var confirmResp struct {
		Code int32 `json:"code"`
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(confirm.Body.Bytes(), &confirmResp); err != nil {
		t.Fatalf("decode MFA confirmation: %v (body=%s)", err, confirm.Body.String())
	}
	if confirmResp.Code != int32(apipb.Code_Success) || confirmResp.Data.ID == "" {
		t.Fatalf("unexpected MFA confirmation: %#v", confirmResp)
	}

	foreignFactor := &auth.MFAFactor{
		ID: "foreign-mfa-factor", PrincipalID: foreignID, Type: "totp",
		SecretEnc: "encrypted", Name: "其他用户", Enable: true, CreatedAt: time.Now().Unix(),
	}
	if err := store.DB().Create(foreignFactor).Error; err != nil {
		t.Fatalf("create foreign factor: %v", err)
	}
	foreignDelete := decodeCommonResponse(t, doJSONRequest(
		t,
		router,
		http.MethodDelete,
		"/api/core/auth/user/mfa/"+foreignFactor.ID,
		nil,
	))
	if foreignDelete.Code != apipb.Code_BadRequest {
		t.Fatalf("foreign MFA delete returned %v, want bad request", foreignDelete.Code)
	}
	var foreignCount int64
	if err := store.DB().Model(&auth.MFAFactor{}).
		Where("id = ? AND principal_id = ?", foreignFactor.ID, foreignID).
		Count(&foreignCount).Error; err != nil {
		t.Fatalf("count foreign factor: %v", err)
	}
	if foreignCount != 1 {
		t.Fatal("foreign MFA factor was deleted")
	}

	list := doJSONRequest(
		t,
		router,
		http.MethodGet,
		"/api/core/auth/user/mfa/factors",
		nil,
	)
	var listResp struct {
		Code int32             `json:"code"`
		Data []*auth.MFAFactor `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode MFA list: %v (body=%s)", err, list.Body.String())
	}
	if listResp.Code != int32(apipb.Code_Success) ||
		len(listResp.Data) != 1 ||
		listResp.Data[0].ID != confirmResp.Data.ID {
		t.Fatalf("unexpected MFA list: %#v", listResp)
	}

	proof := requestAccountReauth(
		t,
		router,
		"disable_mfa:"+confirmResp.Data.ID,
		"Old12345",
		auth.GenerateTOTPCode(enrollResp.Data.Secret),
	)
	deleted := decodeCommonResponse(t, doJSONRequestWithReauth(
		t,
		router,
		http.MethodDelete,
		"/api/core/auth/user/mfa/"+confirmResp.Data.ID,
		nil,
		proof,
	))
	if deleted.Code != apipb.Code_Success {
		t.Fatalf("delete own MFA factor failed: %v (%s)", deleted.Code, deleted.Message)
	}
}
