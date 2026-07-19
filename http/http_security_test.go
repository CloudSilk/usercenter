package http_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
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
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/bootstrap"
	"github.com/CloudSilk/usercenter/internal/permission"
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

func newRoleTestEngine(currentUser *apipb.CurrentUser) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", currentUser)
		c.Next()
	})
	userhttp.RegisterRoleRouter(r)
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

func doMultipartJSONRequest(t *testing.T, r *gin.Engine, path, fieldName, fileName string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if err := json.NewEncoder(part).Encode(payload); err != nil {
		t.Fatalf("encode multipart payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
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

func TestRoleManagementEnforcesTenantScopeAndPreservesAuthorization(t *testing.T) {
	if err := tenant.CreateTenant(&tenant.Tenant{
		Model:     commonmodel.Model{ID: "managed-role-tenant-a"},
		Name:      "managed-role-tenant-a",
		Enable:    true,
		Expired:   time.Now().Add(24 * time.Hour),
		UserCount: 100,
	}); err != nil {
		t.Fatalf("create managed role tenant: %v", err)
	}
	ownedRole := &permission.Role{
		Model:       commonmodel.Model{ID: "managed-role-owned"},
		TenantID:    "managed-role-tenant-a",
		Name:        "本租户角色",
		Description: "原说明",
		CanDel:      true,
		Enable:      true,
	}
	foreignRole := &permission.Role{
		Model:    commonmodel.Model{ID: "managed-role-foreign"},
		TenantID: "managed-role-tenant-b",
		Name:     "其他租户角色",
		CanDel:   true,
		Enable:   true,
	}
	if err := store.DB().Create([]*permission.Role{ownedRole, foreignRole}).Error; err != nil {
		t.Fatalf("create roles: %v", err)
	}
	roleMenu := &permission.RoleMenu{
		Model:  commonmodel.Model{ID: "managed-role-menu"},
		RoleID: ownedRole.ID,
		MenuID: "managed-role-menu-resource",
		Funcs:  "view",
		Show:   true,
	}
	if err := store.DB().Create(roleMenu).Error; err != nil {
		t.Fatalf("create role menu: %v", err)
	}

	current := &apipb.CurrentUser{
		Id: "managed-role-admin-a", TenantID: ownedRole.TenantID, UserName: "managed-role-admin-a",
	}
	r := newRoleTestEngine(current)

	detail := doJSONRequest(t, r, http.MethodGet, "/api/core/auth/role/detail?id="+foreignRole.ID, nil)
	var detailResp apipb.GetRoleDetailResponse
	if err := json.Unmarshal(detail.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("decode foreign role detail: %v", err)
	}
	if detailResp.Code != apipb.Code_NoPermission {
		t.Fatalf("foreign role detail should be denied, got %v", detailResp.Code)
	}

	crossTenantRequests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{
			name: "update", method: http.MethodPut, path: "/api/core/auth/role/update",
			body: map[string]any{"id": foreignRole.ID, "name": "越权修改", "tenantID": ownedRole.TenantID},
		},
		{
			name: "enable", method: http.MethodPost, path: "/api/core/auth/role/enable",
			body: map[string]any{"id": foreignRole.ID, "enable": false},
		},
		{
			name: "delete", method: http.MethodDelete, path: "/api/core/auth/role/delete",
			body: map[string]any{"id": foreignRole.ID},
		},
	}
	for _, tc := range crossTenantRequests {
		t.Run(tc.name, func(t *testing.T) {
			resp := decodeCommonResponse(t, doJSONRequest(t, r, tc.method, tc.path, tc.body))
			if resp.Code != apipb.Code_NoPermission {
				t.Fatalf("cross-tenant role %s should be denied, got %v (%s)", tc.name, resp.Code, resp.Message)
			}
		})
	}

	update := decodeCommonResponse(t, doJSONRequest(t, r, http.MethodPut, "/api/core/auth/role/update", map[string]any{
		"id": ownedRole.ID, "tenantID": foreignRole.TenantID, "name": "本租户角色已更新",
		"description": "新说明", "public": true, "defaultRouter": "dashboard",
	}))
	if update.Code != apipb.Code_Success {
		t.Fatalf("same-tenant role update failed: %v (%s)", update.Code, update.Message)
	}
	var stored permission.Role
	if err := store.DB().First(&stored, "id = ?", ownedRole.ID).Error; err != nil {
		t.Fatalf("reload owned role: %v", err)
	}
	if stored.TenantID != ownedRole.TenantID || stored.Public || stored.Name != "本租户角色已更新" {
		t.Fatalf("tenant-scoped update escaped its boundary: %#v", stored)
	}
	var roleMenuCount int64
	if err := store.DB().Model(&permission.RoleMenu{}).
		Where("role_id = ?", ownedRole.ID).
		Count(&roleMenuCount).Error; err != nil {
		t.Fatalf("count preserved role menus: %v", err)
	}
	if roleMenuCount != 1 {
		t.Fatalf("metadata-only role update removed authorization links, count=%d", roleMenuCount)
	}

	childRole := &permission.Role{
		Model:    commonmodel.Model{ID: "managed-role-child"},
		TenantID: ownedRole.TenantID,
		Name:     "下级角色",
		ParentID: ownedRole.ID,
		CanDel:   true,
		Enable:   true,
	}
	if err := store.DB().Create(childRole).Error; err != nil {
		t.Fatalf("create child role: %v", err)
	}
	cycle := decodeCommonResponse(t, doJSONRequest(t, r, http.MethodPut, "/api/core/auth/role/update", map[string]any{
		"id": ownedRole.ID, "tenantID": ownedRole.TenantID, "name": stored.Name,
		"parentID": childRole.ID,
	}))
	if cycle.Code == apipb.Code_Success {
		t.Fatal("role update must reject an indirect parent cycle")
	}
	if err := store.DB().First(&stored, "id = ?", ownedRole.ID).Error; err != nil {
		t.Fatalf("reload owned role after cycle rejection: %v", err)
	}
	if stored.ParentID != "" {
		t.Fatalf("cycle rejection changed the role parent: %#v", stored)
	}

	query := doJSONRequest(t, r, http.MethodGet, "/api/core/auth/role/query?pageIndex=1&pageSize=100", nil)
	var queryResp apipb.QueryRoleResponse
	if err := json.Unmarshal(query.Body.Bytes(), &queryResp); err != nil {
		t.Fatalf("decode role query: %v", err)
	}
	for _, role := range queryResp.Data {
		if role.Id == foreignRole.ID {
			t.Fatalf("foreign role leaked into tenant query: %#v", role)
		}
	}

	importedRoleID := "managed-role-import-created"
	importResp := doMultipartJSONRequest(
		t,
		r,
		"/api/core/auth/role/import",
		"files",
		"roles.json",
		[]map[string]any{
			{
				"id": foreignRole.ID, "tenantID": ownedRole.TenantID,
				"name": "越权导入修改", "public": false,
			},
			{
				"id": importedRoleID, "tenantID": foreignRole.TenantID,
				"name": "本租户导入角色", "public": true,
			},
		},
	)
	var imported apipb.QueryRoleResponse
	if err := json.Unmarshal(importResp.Body.Bytes(), &imported); err != nil {
		t.Fatalf("decode role import: %v (body=%s)", err, importResp.Body.String())
	}
	if imported.Code != apipb.Code_Success || imported.Message != "导入成功数量:1,导入失败数量:1" {
		t.Fatalf("unexpected tenant-scoped import result: %#v", imported)
	}
	if err := store.DB().First(foreignRole, "id = ?", foreignRole.ID).Error; err != nil {
		t.Fatalf("reload foreign role after import: %v", err)
	}
	if foreignRole.Name == "越权导入修改" {
		t.Fatal("tenant import modified a foreign role")
	}
	var importedRole permission.Role
	if err := store.DB().First(&importedRole, "id = ?", importedRoleID).Error; err != nil {
		t.Fatalf("load imported role: %v", err)
	}
	if importedRole.TenantID != ownedRole.TenantID || importedRole.Public {
		t.Fatalf("tenant import escaped its scope: %#v", importedRole)
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

func TestUpdateUserWithoutRoleFieldsPreservesExistingRoles(t *testing.T) {
	targetID := mustCreateUser(t, "role-preserving-update", platformTenant, "Abc12345")
	roleLink := &user.UserRole{UserID: targetID, RoleID: "role-preserved"}
	if err := store.DB().Create(roleLink).Error; err != nil {
		t.Fatalf("create user role: %v", err)
	}
	current := &apipb.CurrentUser{Id: "platform-admin", TenantID: platformTenant, UserName: "platform-admin"}

	resp := decodeCommonResponse(t, doJSONRequest(t, newTestEngine(current), http.MethodPut, "/api/core/auth/user/update", map[string]any{
		"id": targetID, "tenantID": platformTenant, "userName": "role-preserving-update",
		"nickname": "资料已更新", "mobile": "13800000009", "enable": true,
	}))
	if resp.Code != commonmodel.Success {
		t.Fatalf("update failed: %v (%s)", resp.Code, resp.Message)
	}
	var count int64
	if err := store.DB().Model(&user.UserRole{}).
		Where("user_id = ? AND role_id = ?", targetID, "role-preserved").
		Count(&count).Error; err != nil {
		t.Fatalf("count user roles: %v", err)
	}
	if count != 1 {
		t.Fatalf("profile-only update removed roles, count=%d", count)
	}
}

func TestUpdateUserRolesUsesDedicatedTenantScopedEndpointAndAudit(t *testing.T) {
	targetID := mustCreateUser(t, "dedicated-role-target", platformTenant, "Abc12345")
	roles := []*permission.Role{
		{Model: commonmodel.Model{ID: "dedicated-role-1"}, TenantID: platformTenant, Name: "起草人员"},
		{Model: commonmodel.Model{ID: "dedicated-role-2"}, TenantID: platformTenant, Name: "审校人员"},
	}
	if err := store.DB().Create(&roles).Error; err != nil {
		t.Fatalf("create roles: %v", err)
	}
	current := &apipb.CurrentUser{Id: "platform-admin", TenantID: platformTenant, UserName: "platform-admin"}
	w := doJSONRequest(t, newTestEngine(current), http.MethodPut, "/api/core/auth/user/roles", map[string]any{
		"id": targetID, "roleIDs": []string{"dedicated-role-2", "dedicated-role-1"},
	})
	var resp struct {
		Code apipb.Code `json:"code"`
		Data struct {
			UserID                string   `json:"userID"`
			RoleIDs               []string `json:"roleIDs"`
			SessionsRevoked       int64    `json:"sessionsRevoked"`
			CurrentSessionRevoked bool     `json:"currentSessionRevoked"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode roles response: %v (body=%s)", err, w.Body.String())
	}
	if resp.Code != apipb.Code_Success || resp.Data.UserID != targetID {
		t.Fatalf("role assignment failed: %#v", resp)
	}
	if len(resp.Data.RoleIDs) != 2 || resp.Data.RoleIDs[0] != "dedicated-role-1" || resp.Data.RoleIDs[1] != "dedicated-role-2" {
		t.Fatalf("unexpected assigned roles: %#v", resp.Data.RoleIDs)
	}
	if resp.Data.CurrentSessionRevoked {
		t.Fatal("assigning another user's roles must not report the administrator session revoked")
	}

	detail := doJSONRequest(t, newTestEngine(current), http.MethodGet, "/api/core/auth/user/detail?id="+targetID, nil)
	var detailResp apipb.GetUserDetailResponse
	if err := json.Unmarshal(detail.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detailResp.Code != apipb.Code_Success || len(detailResp.Data.RoleIDs) != 2 {
		t.Fatalf("assigned roles missing from user detail: %#v", detailResp.Data)
	}

	var auditCount int64
	if err := store.DB().Model(&audit.AuditLog{}).
		Where("action = ? AND target_id = ?", audit.AuditActionUpdateUserRoles, targetID).
		Count(&auditCount).Error; err != nil {
		t.Fatalf("count role assignment audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("role assignment audit count = %d, want 1", auditCount)
	}
}

func TestUpdateUserRolesRejectsCrossTenantManagerAndForeignRole(t *testing.T) {
	targetID := mustCreateUser(t, "role-boundary-target", "role-boundary-a", "Abc12345")
	foreignRole := &permission.Role{
		Model:    commonmodel.Model{ID: "role-boundary-foreign"},
		TenantID: "role-boundary-b",
		Name:     "其他租户角色",
	}
	if err := store.DB().Create(foreignRole).Error; err != nil {
		t.Fatalf("create foreign role: %v", err)
	}

	crossTenantManager := &apipb.CurrentUser{Id: "role-manager-b", TenantID: "role-boundary-b", UserName: "role-manager-b"}
	crossTenant := decodeCommonResponse(t, doJSONRequest(
		t,
		newTestEngine(crossTenantManager),
		http.MethodPut,
		"/api/core/auth/user/roles",
		map[string]any{"id": targetID, "roleIDs": []string{foreignRole.ID}},
	))
	if crossTenant.Code != apipb.Code_NoPermission {
		t.Fatalf("cross-tenant manager should be denied, got %v", crossTenant.Code)
	}

	platformManager := &apipb.CurrentUser{Id: "platform-admin", TenantID: platformTenant, UserName: "platform-admin"}
	foreignAssignment := decodeCommonResponse(t, doJSONRequest(
		t,
		newTestEngine(platformManager),
		http.MethodPut,
		"/api/core/auth/user/roles",
		map[string]any{"id": targetID, "roleIDs": []string{foreignRole.ID}},
	))
	if foreignAssignment.Code != apipb.Code_BadRequest {
		t.Fatalf("foreign role should be rejected even for platform manager, got %v", foreignAssignment.Code)
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
