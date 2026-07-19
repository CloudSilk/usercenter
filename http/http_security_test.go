package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/db"
	commonmodel "github.com/CloudSilk/pkg/model"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/bootstrap"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
	glebsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const platformTenant = "platform-test"

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "usercenter_http_test_")
	if err != nil {
		panic(err)
	}
	gin.SetMode(gin.TestMode)
	gdb, err := gorm.Open(glebsqlite.Open(filepath.Join(dir, "test.db")), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	token.InitTokenCache("test-secret-key", "", "", "", 120)
	store.SetDB(db.NewDBClient(gdb, false))
	if err := bootstrap.RunMigration(); err != nil {
		panic(err)
	}
	constants.SetPlatformTenantID(platformTenant)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// newTestEngine 用注入 currentUser 的测试中间件替代全局 AuthRequired，
// 使 handler 内的 middleware.GetUserID/GetTenantID 能读到身份。
func newTestEngine(currentUser *apipb.CurrentUser) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", currentUser)
		c.Next()
	})
	userhttp.RegisterUserRouter(r)
	return r
}

// mustCreateUser 创建一条启用用户。tenantID 同时作为租户 ID 与租户名，
// 以满足 CreateUser 内部的租户存在性/有效期校验。
func mustCreateUser(t *testing.T, userName, tenantID, password string) string {
	t.Helper()
	// 确保租户存在（id = tenantID），忽略“存在相同租户”的重复创建错误
	_ = tenant.CreateTenant(&tenant.Tenant{
		Model:     commonmodel.Model{ID: tenantID},
		Name:      tenantID,
		Enable:    true,
		Expired:   time.Now().Add(24 * time.Hour),
		UserCount: 100,
	})
	u := &user.User{
		TenantModel: commonmodel.TenantModel{TenantID: tenantID},
		UserName:    userName,
		Password:    password,
		Nickname:    userName,
		Enable:      true,
	}
	if err := user.CreateUser(u, false); err != nil {
		t.Fatalf("create user %q: %v", userName, err)
	}
	return u.ID
}

func canLogin(t *testing.T, userName, password string) bool {
	t.Helper()
	resp := &apipb.LoginResponse{Code: commonmodel.Success}
	user.Login(&apipb.LoginRequest{UserName: userName, Password: password}, resp)
	return resp.Code == commonmodel.Success
}

func doResetPwd(t *testing.T, r *gin.Engine, targetID string) *apipb.CommonResponse {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"id": targetID})
	req := httptest.NewRequest(http.MethodPost, "/api/core/auth/user/resetpwd", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	resp := &apipb.CommonResponse{}
	if err := json.Unmarshal(w.Body.Bytes(), resp); err != nil {
		t.Fatalf("decode resetpwd response: %v (body=%s)", err, w.Body.String())
	}
	return resp
}

func doJSONRequest(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode request body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeCommonResponse(t *testing.T, w *httptest.ResponseRecorder) apipb.CommonResponse {
	t.Helper()
	var resp apipb.CommonResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode common response: %v (body=%s)", err, w.Body.String())
	}
	return resp
}

func TestPlatformUserQueryDefaultsToCurrentTenantAndRedactsPassword(t *testing.T) {
	ownID := mustCreateUser(t, "platform-query-own", platformTenant, "Abc12345")
	mustCreateUser(t, "platform-query-other", "tenant-query-other", "Abc12345")
	if err := store.DB().Model(&user.User{}).Where("id = ?", ownID).Updates(map[string]any{
		"title":       "综合处负责人",
		"real_name":   "平台用户",
		"description": "平台租户测试用户",
	}).Error; err != nil {
		t.Fatalf("update management fields: %v", err)
	}

	current := &apipb.CurrentUser{Id: "platform-admin", TenantID: platformTenant, UserName: "platform-admin"}
	w := doJSONRequest(t, newTestEngine(current), http.MethodGet, "/api/core/auth/user/query?pageIndex=1&pageSize=100&keyword=platform-query", nil)
	var resp apipb.QueryUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode query response: %v (body=%s)", err, w.Body.String())
	}
	if resp.Code != commonmodel.Success {
		t.Fatalf("query failed: %v (%s)", resp.Code, resp.Message)
	}
	if len(resp.Data) != 1 || resp.Data[0].Id != ownID {
		t.Fatalf("platform query without tenantID must stay in platform tenant, got %#v", resp.Data)
	}
	if resp.Data[0].Password != "" {
		t.Fatalf("password hash leaked from query response: %q", resp.Data[0].Password)
	}
	if resp.Data[0].Title != "综合处负责人" || resp.Data[0].RealName != "平台用户" || resp.Data[0].CreatedAt == "" {
		t.Fatalf("management fields missing from query response: %#v", resp.Data[0])
	}
}

func TestPlatformUserQueryCanExplicitlySelectAnotherTenant(t *testing.T) {
	targetID := mustCreateUser(t, "platform-explicit-target", "tenant-query-explicit", "Abc12345")
	current := &apipb.CurrentUser{Id: "platform-admin", TenantID: platformTenant, UserName: "platform-admin"}
	path := "/api/core/auth/user/query?pageIndex=1&pageSize=100&tenantID=tenant-query-explicit&keyword=platform-explicit"
	w := doJSONRequest(t, newTestEngine(current), http.MethodGet, path, nil)
	var resp apipb.QueryUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode query response: %v (body=%s)", err, w.Body.String())
	}
	if resp.Code != commonmodel.Success || len(resp.Data) != 1 || resp.Data[0].Id != targetID {
		t.Fatalf("explicit tenant query failed: code=%v data=%#v", resp.Code, resp.Data)
	}
}

func TestUserQueryKeywordAndStatusFilters(t *testing.T) {
	enabledID := mustCreateUser(t, "filter-enabled", platformTenant, "Abc12345")
	disabledID := mustCreateUser(t, "filter-disabled", platformTenant, "Abc12345")
	if err := store.DB().Model(&user.User{}).Where("id = ?", enabledID).Update("mobile", "13900001234").Error; err != nil {
		t.Fatalf("update enabled mobile: %v", err)
	}
	if err := store.DB().Model(&user.User{}).Where("id = ?", disabledID).Updates(map[string]any{
		"email":  "needle-user@example.com",
		"enable": false,
	}).Error; err != nil {
		t.Fatalf("update disabled user: %v", err)
	}

	current := &apipb.CurrentUser{Id: "platform-admin", TenantID: platformTenant, UserName: "platform-admin"}
	path := "/api/core/auth/user/query?pageIndex=1&pageSize=100&keyword=needle-user&enable=false"
	w := doJSONRequest(t, newTestEngine(current), http.MethodGet, path, nil)
	var resp apipb.QueryUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode query response: %v (body=%s)", err, w.Body.String())
	}
	if resp.Code != commonmodel.Success || len(resp.Data) != 1 || resp.Data[0].Id != disabledID {
		t.Fatalf("keyword/status filters failed: code=%v data=%#v", resp.Code, resp.Data)
	}

	invalid := doJSONRequest(t, newTestEngine(current), http.MethodGet, "/api/core/auth/user/query?pageIndex=1&pageSize=10&enable=invalid", nil)
	if err := json.Unmarshal(invalid.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode invalid-filter response: %v", err)
	}
	if resp.Code != apipb.Code_BadRequest {
		t.Fatalf("invalid enable filter should be rejected, got %v", resp.Code)
	}
}

func TestUserManagementRejectsCrossTenantTargets(t *testing.T) {
	targetID := mustCreateUser(t, "cross-tenant-managed", "tenant-managed-B", "Abc12345")
	current := &apipb.CurrentUser{Id: "tenant-admin-a", TenantID: "tenant-managed-A", UserName: "tenant-admin-a"}
	r := newTestEngine(current)

	detail := doJSONRequest(t, r, http.MethodGet, "/api/core/auth/user/detail?id="+targetID, nil)
	var detailResp apipb.GetUserDetailResponse
	if err := json.Unmarshal(detail.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if detailResp.Code != apipb.Code_NoPermission {
		t.Fatalf("cross-tenant detail should be denied, got %v", detailResp.Code)
	}

	requests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{
			name: "enable", method: http.MethodPost, path: "/api/core/auth/user/enable",
			body: map[string]any{"id": targetID, "enable": false},
		},
		{
			name: "update", method: http.MethodPut, path: "/api/core/auth/user/update",
			body: map[string]any{
				"id": targetID, "tenantID": "tenant-managed-A", "userName": "cross-tenant-managed",
				"nickname": "越权修改", "mobile": "13800000001", "enable": true,
			},
		},
		{
			name: "delete", method: http.MethodDelete, path: "/api/core/auth/user/delete",
			body: map[string]any{"id": targetID},
		},
	}
	for _, tc := range requests {
		t.Run(tc.name, func(t *testing.T) {
			resp := decodeCommonResponse(t, doJSONRequest(t, r, tc.method, tc.path, tc.body))
			if resp.Code != apipb.Code_NoPermission {
				t.Fatalf("cross-tenant %s should be denied, got %v (%s)", tc.name, resp.Code, resp.Message)
			}
		})
	}

	var target user.User
	if err := store.DB().First(&target, "id = ?", targetID).Error; err != nil {
		t.Fatalf("target user must still exist: %v", err)
	}
	if !target.Enable || target.Nickname == "越权修改" {
		t.Fatalf("cross-tenant mutations changed target: %#v", target)
	}
}

func TestResetPwdAcceptsExplicitStrongPassword(t *testing.T) {
	targetID := mustCreateUser(t, "explicit-reset", platformTenant, "Abc12345")
	current := &apipb.CurrentUser{Id: "platform-admin", TenantID: platformTenant, UserName: "platform-admin"}
	r := newTestEngine(current)

	resp := decodeCommonResponse(t, doJSONRequest(t, r, http.MethodPost, "/api/core/auth/user/resetpwd", map[string]any{
		"id": targetID, "password": "Reset9876A",
	}))
	if resp.Code != commonmodel.Success {
		t.Fatalf("explicit reset failed: %v (%s)", resp.Code, resp.Message)
	}
	if !canLogin(t, "explicit-reset", "Reset9876A") {
		t.Fatal("explicit reset password should be usable")
	}

	weak := decodeCommonResponse(t, doJSONRequest(t, r, http.MethodPost, "/api/core/auth/user/resetpwd", map[string]any{
		"id": targetID, "password": "12345678",
	}))
	if weak.Code != apipb.Code_BadRequest {
		t.Fatalf("weak reset password should be rejected, got %v", weak.Code)
	}
}

// A2: 非平台租户调用方重置其他租户用户密码 → 应被拒绝，且目标密码不变
func TestResetPwdRejectsCrossTenant(t *testing.T) {
	target := mustCreateUser(t, "crosstenant", "tenant-B", "Abc12345")
	current := &apipb.CurrentUser{Id: "admin-a", TenantID: "tenant-A", UserName: "admin-a"}

	resp := doResetPwd(t, newTestEngine(current), target)
	if resp.Code != commonmodel.NoPermission {
		t.Fatalf("expected NoPermission for cross-tenant reset, got %v (%s)", resp.Code, resp.Message)
	}
	if !canLogin(t, "crosstenant", "Abc12345") {
		t.Fatal("target password must remain unchanged after rejected reset")
	}
}

// A2: 平台租户可重置任意租户用户密码
func TestResetPwdAllowsPlatformTenant(t *testing.T) {
	target := mustCreateUser(t, "platformtarget", "tenant-B", "Abc12345")
	current := &apipb.CurrentUser{Id: "platform-admin", TenantID: platformTenant, UserName: "platform-admin"}

	resp := doResetPwd(t, newTestEngine(current), target)
	if resp.Code != commonmodel.Success {
		t.Fatalf("expected success for platform tenant reset, got %v (%s)", resp.Code, resp.Message)
	}
}

// A2: 非平台租户调用方重置本租户用户密码 → 允许
func TestResetPwdAllowsSameTenant(t *testing.T) {
	target := mustCreateUser(t, "sametenant", "tenant-A", "Abc12345")
	current := &apipb.CurrentUser{Id: "admin-a", TenantID: "tenant-A", UserName: "admin-a"}

	resp := doResetPwd(t, newTestEngine(current), target)
	if resp.Code != commonmodel.Success {
		t.Fatalf("expected success for same-tenant reset, got %v (%s)", resp.Code, resp.Message)
	}
}

// A3: ChangePwd 必须忽略请求体中的 id，只改当前登录用户自己的密码
func TestChangePwdIgnoresForeignID(t *testing.T) {
	userA := mustCreateUser(t, "pwda", "tenant-A", "Old12345")
	userB := mustCreateUser(t, "pwdb", "tenant-A", "Bee12345")
	current := &apipb.CurrentUser{Id: userA, TenantID: "tenant-A", UserName: "pwda"}
	r := newTestEngine(current)

	// 请求体故意传入 userB 的 id，期望仍只修改 userA（current user）
	body, _ := json.Marshal(map[string]string{
		"id":            userB,
		"oldPwd":        "Old12345",
		"newPwd":        "New12345",
		"newConfirmPwd": "New12345",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/core/auth/user/changepwd", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp apipb.CommonResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode changepwd response: %v (body=%s)", err, w.Body.String())
	}
	if resp.Code != commonmodel.Success {
		t.Fatalf("expected change-pwd success, got %v (%s)", resp.Code, resp.Message)
	}

	// userA：新密码可用、旧密码失效
	if !canLogin(t, "pwda", "New12345") {
		t.Error("userA should log in with the new password")
	}
	if canLogin(t, "pwda", "Old12345") {
		t.Error("userA old password should no longer work")
	}
	// userB：密码未受影响
	if !canLogin(t, "pwdb", "Bee12345") {
		t.Error("userB password should remain unchanged")
	}
}
