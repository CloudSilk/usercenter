package http_test

import (
	"bytes"
	"encoding/json"
	"errors"
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
	"github.com/CloudSilk/usercenter/internal/session"
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

func newAPITestEngine(currentUser *apipb.CurrentUser) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", currentUser)
		c.Next()
	})
	userhttp.RegisterAPIRouter(r)
	return r
}

func newMenuTestEngine(currentUser *apipb.CurrentUser) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", currentUser)
		c.Next()
	})
	userhttp.RegisterMenuRouter(r)
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

type roleAuthorizationEnvelope struct {
	Code    apipb.Code `json:"code"`
	Message string     `json:"message"`
	Data    json.RawMessage
}

type roleAuthorizationFixture struct {
	ParentMenuID     string
	ChildMenuID      string
	ParentViewFunc   string
	ParentManageFunc string
	ChildEditFunc    string
}

func createRoleAuthorizationFixture(t *testing.T, prefix string) roleAuthorizationFixture {
	t.Helper()
	parentMenuID := prefix + "-menu-parent"
	childMenuID := prefix + "-menu-child"
	parentViewFunc := prefix + "-parent-view"
	parentManageFunc := prefix + "-parent-manage"
	childEditFunc := prefix + "-child-edit"

	apis := []*permission.API{
		{
			Model:       commonmodel.Model{ID: prefix + "-api-parent-view"},
			Path:        "/" + prefix + "/documents",
			Method:      http.MethodGet,
			Description: "list documents",
			Enable:      true,
			CheckAuth:   true,
		},
		{
			Model:       commonmodel.Model{ID: prefix + "-api-parent-manage"},
			Path:        "/" + prefix + "/documents",
			Method:      http.MethodPost,
			Description: "create document",
			Enable:      true,
			CheckAuth:   true,
		},
		{
			Model:       commonmodel.Model{ID: prefix + "-api-child-edit"},
			Path:        "/" + prefix + "/documents/:id",
			Method:      http.MethodPut,
			Description: "edit document",
			Enable:      true,
			CheckAuth:   true,
		},
		{
			Model:       commonmodel.Model{ID: prefix + "-api-child-edit-duplicate"},
			Path:        "/" + prefix + "/documents/:id",
			Method:      http.MethodPut,
			Description: "duplicate binding",
			Enable:      true,
			CheckAuth:   true,
		},
		{
			Model:       commonmodel.Model{ID: prefix + "-api-child-disabled"},
			Path:        "/" + prefix + "/documents/:id/archive",
			Method:      http.MethodPost,
			Description: "disabled archive API",
			Enable:      false,
			CheckAuth:   true,
		},
	}
	if err := store.DB().Create(&apis).Error; err != nil {
		t.Fatalf("create authorization APIs: %v", err)
	}
	menus := []*permission.Menu{
		{
			Model: commonmodel.Model{ID: parentMenuID},
			Name:  prefix + "-parent",
			Title: "Document center",
			Path:  "/" + prefix,
			Sort:  10,
		},
		{
			Model:    commonmodel.Model{ID: childMenuID},
			ParentID: parentMenuID,
			Level:    1,
			Name:     prefix + "-child",
			Title:    "Document editing",
			Path:     "/" + prefix + "/edit",
			Sort:     20,
		},
	}
	if err := store.DB().Create(&menus).Error; err != nil {
		t.Fatalf("create authorization menus: %v", err)
	}
	functions := []*permission.MenuFunc{
		{
			Model:  commonmodel.Model{ID: prefix + "-func-parent-view"},
			MenuID: parentMenuID,
			Name:   parentViewFunc,
			Title:  "View documents",
		},
		{
			Model:  commonmodel.Model{ID: prefix + "-func-parent-manage"},
			MenuID: parentMenuID,
			Name:   parentManageFunc,
			Title:  "Manage documents",
		},
		{
			Model:  commonmodel.Model{ID: prefix + "-func-child-edit"},
			MenuID: childMenuID,
			Name:   childEditFunc,
			Title:  "Edit documents",
		},
	}
	if err := store.DB().Create(&functions).Error; err != nil {
		t.Fatalf("create authorization functions: %v", err)
	}
	links := []*permission.MenuFuncApi{
		{
			Model:      commonmodel.Model{ID: prefix + "-link-parent-view"},
			MenuFuncID: functions[0].ID,
			APIID:      apis[0].ID,
		},
		{
			Model:      commonmodel.Model{ID: prefix + "-link-parent-manage"},
			MenuFuncID: functions[1].ID,
			APIID:      apis[1].ID,
		},
		{
			Model:      commonmodel.Model{ID: prefix + "-link-child-edit"},
			MenuFuncID: functions[2].ID,
			APIID:      apis[2].ID,
		},
		{
			Model:      commonmodel.Model{ID: prefix + "-link-child-edit-duplicate"},
			MenuFuncID: functions[2].ID,
			APIID:      apis[3].ID,
		},
		{
			Model:      commonmodel.Model{ID: prefix + "-link-child-disabled"},
			MenuFuncID: functions[2].ID,
			APIID:      apis[4].ID,
		},
	}
	if err := store.DB().Create(&links).Error; err != nil {
		t.Fatalf("create function API links: %v", err)
	}
	return roleAuthorizationFixture{
		ParentMenuID:     parentMenuID,
		ChildMenuID:      childMenuID,
		ParentViewFunc:   parentViewFunc,
		ParentManageFunc: parentManageFunc,
		ChildEditFunc:    childEditFunc,
	}
}

func decodeRoleAuthorizationEnvelope(t *testing.T, response *httptest.ResponseRecorder) roleAuthorizationEnvelope {
	t.Helper()
	var envelope roleAuthorizationEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode role authorization response: %v (body=%s)", err, response.Body.String())
	}
	return envelope
}

func TestRoleAuthorizationPreviewAndPublishLifecycle(t *testing.T) {
	fixture := createRoleAuthorizationFixture(t, "role-auth-lifecycle")
	role := &permission.Role{
		Model:         commonmodel.Model{ID: "role-auth-lifecycle-role"},
		TenantID:      platformTenant,
		Name:          "Authorization lifecycle role",
		Description:   "metadata must survive authorization publishing",
		DefaultRouter: "dashboard",
		CanDel:        true,
		Enable:        true,
	}
	if err := store.DB().Create(role).Error; err != nil {
		t.Fatalf("create authorization role: %v", err)
	}
	if err := store.DB().Create(&permission.RoleMenu{
		Model:  commonmodel.Model{ID: "role-auth-lifecycle-initial-menu"},
		RoleID: role.ID,
		MenuID: fixture.ParentMenuID,
		Funcs:  fixture.ParentViewFunc,
		Show:   true,
	}).Error; err != nil {
		t.Fatalf("create initial role authorization: %v", err)
	}

	targetUserID := mustCreateUser(t, "role-auth-target", platformTenant, "Abc12345")
	if err := store.DB().Create(&user.UserRole{
		Model:  commonmodel.Model{ID: "role-auth-lifecycle-user-role"},
		UserID: targetUserID,
		RoleID: role.ID,
	}).Error; err != nil {
		t.Fatalf("assign role to target user: %v", err)
	}
	activeSession := &session.Session{
		Model:       commonmodel.Model{ID: "role-auth-lifecycle-session"},
		PrincipalID: targetUserID,
		TenantID:    platformTenant,
		TokenSig:    "role-auth-lifecycle-token-signature",
	}
	if err := store.DB().Create(activeSession).Error; err != nil {
		t.Fatalf("create active session: %v", err)
	}
	staleToken, err := token.EncodeToken(&apipb.CurrentUser{
		Id: targetUserID, TenantID: platformTenant, UserName: "role-auth-target",
	})
	if err != nil {
		t.Fatalf("encode stale token: %v", err)
	}
	if exists, err := token.DefaultTokenCache.Exists(targetUserID, staleToken); err != nil || !exists {
		t.Fatalf("stale token should exist before publish: exists=%v err=%v", exists, err)
	}

	current := &apipb.CurrentUser{
		Id: "role-auth-platform-admin", TenantID: platformTenant, UserName: "role-auth-platform-admin",
	}
	engine := newRoleTestEngine(current)
	detailEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		engine,
		http.MethodGet,
		"/api/core/auth/role/authorization?id="+role.ID,
		nil,
	))
	if detailEnvelope.Code != apipb.Code_Success {
		t.Fatalf("load role authorization failed: %v (%s)", detailEnvelope.Code, detailEnvelope.Message)
	}
	var detail permission.RoleAuthorizationDetail
	if err := json.Unmarshal(detailEnvelope.Data, &detail); err != nil {
		t.Fatalf("decode role authorization detail: %v", err)
	}
	if len(detail.Menus) != 2 || detail.Summary.SelectedMenuCount != 1 || detail.Summary.PolicyCount != 1 {
		t.Fatalf("unexpected current authorization detail: %#v", detail)
	}
	initialRevision := detail.Revision

	proposedSelections := []map[string]any{
		{
			"menuID": fixture.ParentMenuID,
			"show":   true,
			"funcs":  []string{fixture.ParentViewFunc, fixture.ParentManageFunc},
		},
		{
			"menuID": fixture.ChildMenuID,
			"show":   true,
			"funcs":  []string{fixture.ChildEditFunc},
		},
	}
	previewEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		engine,
		http.MethodPost,
		"/api/core/auth/role/authorization/preview",
		map[string]any{"roleID": role.ID, "selections": proposedSelections},
	))
	if previewEnvelope.Code != apipb.Code_Success {
		t.Fatalf("preview role authorization failed: %v (%s)", previewEnvelope.Code, previewEnvelope.Message)
	}
	var preview permission.RoleAuthorizationPreview
	if err := json.Unmarshal(previewEnvelope.Data, &preview); err != nil {
		t.Fatalf("decode role authorization preview: %v", err)
	}
	if preview.CurrentRevision != initialRevision ||
		preview.Summary.PolicyCount != 3 ||
		preview.Summary.SkippedDisabledAPICount != 1 {
		t.Fatalf("unexpected authorization preview: %#v", preview)
	}
	var unchangedMenuCount int64
	if err := store.DB().Model(&permission.RoleMenu{}).
		Where("role_id = ?", role.ID).
		Count(&unchangedMenuCount).Error; err != nil {
		t.Fatalf("count unchanged role menus: %v", err)
	}
	if unchangedMenuCount != 1 {
		t.Fatalf("preview mutated role menus, count=%d", unchangedMenuCount)
	}

	publishEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		engine,
		http.MethodPut,
		"/api/core/auth/role/authorization",
		map[string]any{
			"roleID":       role.ID,
			"baseRevision": initialRevision,
			"selections":   proposedSelections,
		},
	))
	if publishEnvelope.Code != apipb.Code_Success {
		t.Fatalf("publish role authorization failed: %v (%s)", publishEnvelope.Code, publishEnvelope.Message)
	}
	var publishResult struct {
		Revision        string `json:"revision"`
		SessionsRevoked int64  `json:"sessionsRevoked"`
	}
	if err := json.Unmarshal(publishEnvelope.Data, &publishResult); err != nil {
		t.Fatalf("decode role authorization publish result: %v", err)
	}
	if publishResult.Revision != preview.ProposedRevision || publishResult.SessionsRevoked != 1 {
		t.Fatalf("unexpected publish result: %#v", publishResult)
	}

	var storedRole permission.Role
	if err := store.DB().First(&storedRole, "id = ?", role.ID).Error; err != nil {
		t.Fatalf("reload role metadata: %v", err)
	}
	if storedRole.Name != role.Name ||
		storedRole.TenantID != role.TenantID ||
		storedRole.Description != role.Description ||
		storedRole.DefaultRouter != role.DefaultRouter {
		t.Fatalf("authorization publish changed role metadata: %#v", storedRole)
	}
	var storedRoleMenus []*permission.RoleMenu
	if err := store.DB().Where("role_id = ?", role.ID).Order("menu_id").Find(&storedRoleMenus).Error; err != nil {
		t.Fatalf("reload published role menus: %v", err)
	}
	if len(storedRoleMenus) != 2 {
		t.Fatalf("published role menu count=%d, want 2", len(storedRoleMenus))
	}
	var policyCount int64
	if err := store.DB().Model(&permission.CasbinRule{}).
		Where("ptype = ? AND v0 = ?", "p", role.ID).
		Count(&policyCount).Error; err != nil {
		t.Fatalf("count published Casbin policies: %v", err)
	}
	if policyCount != 3 {
		t.Fatalf("published Casbin policy count=%d, want 3", policyCount)
	}
	if err := store.DB().First(activeSession, "id = ?", activeSession.ID).Error; err != nil {
		t.Fatalf("reload revoked session: %v", err)
	}
	if !activeSession.Revoked || activeSession.RevokedReason != "role authorization published" {
		t.Fatalf("role authorization publish did not revoke session: %#v", activeSession)
	}
	if exists, err := token.DefaultTokenCache.Exists(targetUserID, staleToken); err != nil || exists {
		t.Fatalf("stale token must be cleared after publish: exists=%v err=%v", exists, err)
	}
	var auditCount int64
	if err := store.DB().Model(&audit.AuditLog{}).
		Where("action = ? AND target_id = ?", audit.AuditActionPublishRoleAuth, role.ID).
		Count(&auditCount).Error; err != nil {
		t.Fatalf("count role authorization audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("role authorization audit count=%d, want 1", auditCount)
	}

	conflictEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		engine,
		http.MethodPut,
		"/api/core/auth/role/authorization",
		map[string]any{
			"roleID":       role.ID,
			"baseRevision": initialRevision,
			"selections":   []map[string]any{},
		},
	))
	if conflictEnvelope.Code != apipb.Code_BadRequest {
		t.Fatalf("stale revision must be rejected, got %v", conflictEnvelope.Code)
	}
	var conflict struct {
		Conflict bool `json:"conflict"`
	}
	if err := json.Unmarshal(conflictEnvelope.Data, &conflict); err != nil {
		t.Fatalf("decode revision conflict: %v", err)
	}
	if !conflict.Conflict {
		t.Fatalf("revision conflict flag missing: %#v", conflict)
	}
}

func TestRoleAuthorizationEnforcesTenantAndHierarchyBoundaries(t *testing.T) {
	fixture := createRoleAuthorizationFixture(t, "role-auth-boundary")
	tenantA := &tenant.Tenant{
		Model:     commonmodel.Model{ID: "role-auth-tenant-a"},
		Name:      "role-auth-tenant-a",
		Enable:    true,
		Expired:   time.Now().Add(24 * time.Hour),
		UserCount: 100,
	}
	tenantB := &tenant.Tenant{
		Model:     commonmodel.Model{ID: "role-auth-tenant-b"},
		Name:      "role-auth-tenant-b",
		Enable:    true,
		Expired:   time.Now().Add(24 * time.Hour),
		UserCount: 100,
	}
	if err := store.DB().Create([]*tenant.Tenant{tenantA, tenantB}).Error; err != nil {
		t.Fatalf("create authorization tenants: %v", err)
	}
	if err := store.DB().Create([]*tenant.TenantMenu{
		{
			Model:    commonmodel.Model{ID: "role-auth-tenant-parent-menu"},
			TenantID: tenantA.ID,
			MenuID:   fixture.ParentMenuID,
			Funcs:    fixture.ParentViewFunc,
		},
		{
			Model:    commonmodel.Model{ID: "role-auth-tenant-child-menu"},
			TenantID: tenantA.ID,
			MenuID:   fixture.ChildMenuID,
			Funcs:    fixture.ChildEditFunc,
		},
	}).Error; err != nil {
		t.Fatalf("create tenant menu authorization: %v", err)
	}
	ownedRole := &permission.Role{
		Model:    commonmodel.Model{ID: "role-auth-boundary-owned-role"},
		TenantID: tenantA.ID,
		Name:     "Owned tenant role",
		CanDel:   true,
		Enable:   true,
	}
	foreignRole := &permission.Role{
		Model:    commonmodel.Model{ID: "role-auth-boundary-foreign-role"},
		TenantID: tenantB.ID,
		Name:     "Foreign tenant role",
		CanDel:   true,
		Enable:   true,
	}
	disabledRole := &permission.Role{
		Model:    commonmodel.Model{ID: "role-auth-boundary-disabled-role"},
		TenantID: platformTenant,
		Name:     "Disabled platform role",
		CanDel:   true,
		Enable:   false,
	}
	if err := store.DB().Create([]*permission.Role{ownedRole, foreignRole, disabledRole}).Error; err != nil {
		t.Fatalf("create boundary roles: %v", err)
	}
	if err := store.DB().Model(&permission.Role{}).
		Where("id = ?", disabledRole.ID).
		Update("enable", false).Error; err != nil {
		t.Fatalf("disable boundary role: %v", err)
	}

	tenantEngine := newRoleTestEngine(&apipb.CurrentUser{
		Id: "role-auth-tenant-admin", TenantID: tenantA.ID, UserName: "role-auth-tenant-admin",
	})
	foreignEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		tenantEngine,
		http.MethodGet,
		"/api/core/auth/role/authorization?id="+foreignRole.ID,
		nil,
	))
	if foreignEnvelope.Code != apipb.Code_NoPermission {
		t.Fatalf("cross-tenant authorization detail should be denied, got %v", foreignEnvelope.Code)
	}

	outOfScopeFunction := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		tenantEngine,
		http.MethodPost,
		"/api/core/auth/role/authorization/preview",
		map[string]any{
			"roleID": ownedRole.ID,
			"selections": []map[string]any{{
				"menuID": fixture.ParentMenuID,
				"show":   true,
				"funcs":  []string{fixture.ParentManageFunc},
			}},
		},
	))
	if outOfScopeFunction.Code != apipb.Code_BadRequest {
		t.Fatalf("function outside tenant menu scope should be rejected, got %v", outOfScopeFunction.Code)
	}

	missingAncestor := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		tenantEngine,
		http.MethodPost,
		"/api/core/auth/role/authorization/preview",
		map[string]any{
			"roleID": ownedRole.ID,
			"selections": []map[string]any{{
				"menuID": fixture.ChildMenuID,
				"show":   true,
				"funcs":  []string{fixture.ChildEditFunc},
			}},
		},
	))
	if missingAncestor.Code != apipb.Code_BadRequest {
		t.Fatalf("visible child without visible ancestor should be rejected, got %v", missingAncestor.Code)
	}

	platformEngine := newRoleTestEngine(&apipb.CurrentUser{
		Id: "role-auth-platform-boundary-admin", TenantID: platformTenant, UserName: "role-auth-platform-boundary-admin",
	})
	disabledDetailEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		platformEngine,
		http.MethodGet,
		"/api/core/auth/role/authorization?id="+disabledRole.ID,
		nil,
	))
	if disabledDetailEnvelope.Code != apipb.Code_Success {
		t.Fatalf("load disabled role authorization failed: %v (%s)", disabledDetailEnvelope.Code, disabledDetailEnvelope.Message)
	}
	var disabledDetail permission.RoleAuthorizationDetail
	if err := json.Unmarshal(disabledDetailEnvelope.Data, &disabledDetail); err != nil {
		t.Fatalf("decode disabled role detail: %v", err)
	}
	disabledPublish := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		platformEngine,
		http.MethodPut,
		"/api/core/auth/role/authorization",
		map[string]any{
			"roleID":       disabledRole.ID,
			"baseRevision": disabledDetail.Revision,
			"selections": []map[string]any{{
				"menuID": fixture.ParentMenuID,
				"show":   true,
				"funcs":  []string{fixture.ParentViewFunc},
			}},
		},
	))
	if disabledPublish.Code != apipb.Code_Success {
		t.Fatalf("disabled role selections should be saved: %v (%s)", disabledPublish.Code, disabledPublish.Message)
	}
	var disabledMenuCount, disabledPolicyCount int64
	if err := store.DB().Model(&permission.RoleMenu{}).
		Where("role_id = ?", disabledRole.ID).
		Count(&disabledMenuCount).Error; err != nil {
		t.Fatalf("count disabled role selections: %v", err)
	}
	if err := store.DB().Model(&permission.CasbinRule{}).
		Where("ptype = ? AND v0 = ?", "p", disabledRole.ID).
		Count(&disabledPolicyCount).Error; err != nil {
		t.Fatalf("count disabled role policies: %v", err)
	}
	if disabledMenuCount != 1 || disabledPolicyCount != 0 {
		t.Fatalf("disabled role must save selections without active policy, menus=%d policies=%d", disabledMenuCount, disabledPolicyCount)
	}
}

func TestDeleteMenuRebuildsRoleAuthorizationAndRevokesSessions(t *testing.T) {
	const prefix = "role-auth-menu-delete"
	fixture := createRoleAuthorizationFixture(t, prefix)
	role := &permission.Role{
		Model:    commonmodel.Model{ID: prefix + "-role"},
		TenantID: platformTenant,
		Name:     "Menu deletion role",
		CanDel:   true,
		Enable:   true,
	}
	if err := store.DB().Create(role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	detail, err := permission.GetRoleAuthorization(role.ID)
	if err != nil {
		t.Fatalf("load initial authorization: %v", err)
	}
	selections := []permission.RoleAuthorizationSelection{
		{MenuID: fixture.ParentMenuID, Show: true, Funcs: []string{fixture.ParentViewFunc}},
		{MenuID: fixture.ChildMenuID, Show: true, Funcs: []string{fixture.ChildEditFunc}},
	}
	if _, err := permission.PublishRoleAuthorization(role.ID, detail.Revision, selections); err != nil {
		t.Fatalf("publish initial authorization: %v", err)
	}
	const userID = prefix + "-user"
	userRole := &user.UserRole{
		Model:  commonmodel.Model{ID: prefix + "-user-role"},
		UserID: userID,
		RoleID: role.ID,
	}
	activeSession := &session.Session{
		Model:       commonmodel.Model{ID: prefix + "-session"},
		PrincipalID: userID,
		TenantID:    platformTenant,
		TokenSig:    prefix + "-token",
	}
	if err := store.DB().Create(userRole).Error; err != nil {
		t.Fatalf("create user role: %v", err)
	}
	if err := store.DB().Create(activeSession).Error; err != nil {
		t.Fatalf("create active session: %v", err)
	}

	if err := permission.DeleteMenu(fixture.ChildMenuID); err != nil {
		t.Fatalf("delete authorized child menu: %v", err)
	}

	var childMenuCount, childFunctionCount, childLinkCount int64
	if err := store.DB().Model(&permission.Menu{}).
		Where("id = ?", fixture.ChildMenuID).
		Count(&childMenuCount).Error; err != nil {
		t.Fatalf("count deleted menu: %v", err)
	}
	if err := store.DB().Model(&permission.MenuFunc{}).
		Where("menu_id = ?", fixture.ChildMenuID).
		Count(&childFunctionCount).Error; err != nil {
		t.Fatalf("count deleted menu functions: %v", err)
	}
	if err := store.DB().Model(&permission.MenuFuncApi{}).
		Where("menu_func_id = ?", prefix+"-func-child-edit").
		Count(&childLinkCount).Error; err != nil {
		t.Fatalf("count deleted function API links: %v", err)
	}
	if childMenuCount != 0 || childFunctionCount != 0 || childLinkCount != 0 {
		t.Fatalf(
			"deleted menu associations remain: menus=%d functions=%d links=%d",
			childMenuCount,
			childFunctionCount,
			childLinkCount,
		)
	}

	var storedRoleMenus []*permission.RoleMenu
	if err := store.DB().Where("role_id = ?", role.ID).Find(&storedRoleMenus).Error; err != nil {
		t.Fatalf("load remaining role menus: %v", err)
	}
	if len(storedRoleMenus) != 1 || storedRoleMenus[0].MenuID != fixture.ParentMenuID {
		t.Fatalf("unexpected remaining role menus: %#v", storedRoleMenus)
	}
	var policies []*permission.CasbinRule
	if err := store.DB().Where("ptype = ? AND v0 = ?", "p", role.ID).Find(&policies).Error; err != nil {
		t.Fatalf("load rebuilt policies: %v", err)
	}
	if len(policies) != 1 ||
		policies[0].Path != "/"+prefix+"/documents" ||
		policies[0].Method != http.MethodGet {
		t.Fatalf("unexpected rebuilt policies: %#v", policies)
	}
	if err := store.DB().Where("id = ?", activeSession.ID).First(activeSession).Error; err != nil {
		t.Fatalf("reload affected session: %v", err)
	}
	if !activeSession.Revoked || activeSession.RevokedReason != "role menu deleted" {
		t.Fatalf("menu deletion did not revoke affected session: %#v", activeSession)
	}
}

func TestMenuManagementEnforcesTenantHierarchyAndProtection(t *testing.T) {
	const prefix = "menu-management-boundary"
	tenantAdmin := &apipb.CurrentUser{Id: prefix + "-admin", TenantID: prefix + "-tenant"}
	engine := newMenuTestEngine(tenantAdmin)

	createRoot := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/menu/add", map[string]any{
		"id":          prefix + "-root",
		"tenantID":    "foreign-tenant",
		"projectID":   "workspace",
		"name":        " root-workspace ",
		"title":       " Root workspace ",
		"path":        " /" + prefix + "/root ",
		"component":   " admin/root ",
		"icon":        " layout-grid ",
		"sort":        10,
		"hidden":      false,
		"cache":       true,
		"defaultMenu": false,
		"closeTab":    true,
		"isMust":      true,
	}))
	if createRoot.Code != commonmodel.Success {
		t.Fatalf("create root menu failed: %v (%s)", createRoot.Code, createRoot.Message)
	}
	root, err := permission.GetMenuByID(prefix + "-root")
	if err != nil {
		t.Fatalf("load root menu: %v", err)
	}
	if root.TenantID != tenantAdmin.TenantID ||
		root.Name != "root-workspace" ||
		root.Title != "Root workspace" ||
		root.Path != "/"+prefix+"/root" ||
		root.Component != "admin/root" ||
		root.Icon != "layout-grid" ||
		root.Level != 0 ||
		root.IsMust ||
		!root.CloseTab {
		t.Fatalf("menu normalization, tenant scoping or closeTab conversion failed: %#v", root)
	}

	duplicate := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/menu/add", map[string]any{
		"id":        prefix + "-duplicate",
		"projectID": root.ProjectID,
		"name":      root.Name,
		"title":     "Duplicate",
		"path":      "/" + prefix + "/duplicate",
	}))
	if duplicate.Code != apipb.Code_BadRequest {
		t.Fatalf("duplicate menu identity should be rejected, got %v (%s)", duplicate.Code, duplicate.Message)
	}

	createChild := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/menu/add", map[string]any{
		"id":        prefix + "-child",
		"projectID": root.ProjectID,
		"parentID":  root.ID,
		"name":      "child-workspace",
		"title":     "Child workspace",
		"path":      "/" + prefix + "/child",
		"component": "admin/child",
		"sort":      20,
	}))
	createGrandchild := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/menu/add", map[string]any{
		"id":        prefix + "-grandchild",
		"projectID": root.ProjectID,
		"parentID":  prefix + "-child",
		"name":      "grandchild-workspace",
		"title":     "Grandchild workspace",
		"path":      "/" + prefix + "/grandchild",
		"component": "admin/grandchild",
		"sort":      30,
	}))
	if createChild.Code != commonmodel.Success || createGrandchild.Code != commonmodel.Success {
		t.Fatalf("create hierarchy failed: child=%v grandchild=%v", createChild.Code, createGrandchild.Code)
	}
	child, err := permission.GetMenuByID(prefix + "-child")
	if err != nil {
		t.Fatalf("load child menu: %v", err)
	}
	grandchild, err := permission.GetMenuByID(prefix + "-grandchild")
	if err != nil {
		t.Fatalf("load grandchild menu: %v", err)
	}
	if child.Level != 1 || grandchild.Level != 2 {
		t.Fatalf("unexpected initial hierarchy levels: child=%d grandchild=%d", child.Level, grandchild.Level)
	}

	parameter := &permission.MenuParameter{
		Model:  commonmodel.Model{ID: prefix + "-parameter"},
		MenuID: child.ID,
		Type:   "query",
		Key:    "from",
		Value:  "menu-test",
	}
	function := &permission.MenuFunc{
		Model:  commonmodel.Model{ID: prefix + "-function"},
		MenuID: child.ID,
		Name:   "view",
		Title:  "View",
	}
	if err := store.DB().Create(parameter).Error; err != nil {
		t.Fatalf("create menu parameter fixture: %v", err)
	}
	if err := store.DB().Create(function).Error; err != nil {
		t.Fatalf("create menu function fixture: %v", err)
	}

	updateChild := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPut, "/api/core/auth/menu/update", map[string]any{
		"id":          child.ID,
		"name":        child.Name,
		"title":       "Hidden workspace",
		"path":        child.Path,
		"component":   child.Component,
		"parentID":    "",
		"icon":        "panel-top",
		"sort":        5,
		"hidden":      true,
		"cache":       true,
		"defaultMenu": false,
		"closeTab":    true,
	}))
	if updateChild.Code != commonmodel.Success {
		t.Fatalf("update menu metadata failed: %v (%s)", updateChild.Code, updateChild.Message)
	}
	child, err = permission.GetMenuByID(child.ID)
	if err != nil {
		t.Fatalf("reload updated child: %v", err)
	}
	grandchild, err = permission.GetMenuByID(grandchild.ID)
	if err != nil {
		t.Fatalf("reload descendant: %v", err)
	}
	if child.ParentID != "" || child.Level != 0 || !child.Hidden || !child.CloseTab ||
		len(child.Parameters) != 1 || len(child.MenuFuncs) != 1 || grandchild.Level != 1 {
		t.Fatalf("metadata update did not preserve associations or cascade levels: child=%#v grandchild=%#v", child, grandchild)
	}

	cycle := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPut, "/api/core/auth/menu/update", map[string]any{
		"id":        child.ID,
		"name":      child.Name,
		"title":     child.Title,
		"path":      child.Path,
		"component": child.Component,
		"parentID":  grandchild.ID,
		"hidden":    child.Hidden,
		"sort":      child.Sort,
		"closeTab":  child.CloseTab,
	}))
	if cycle.Code != apipb.Code_BadRequest {
		t.Fatalf("cyclic hierarchy should be rejected, got %v (%s)", cycle.Code, cycle.Message)
	}

	foreignParent := &permission.Menu{
		Model:     commonmodel.Model{ID: prefix + "-foreign-parent"},
		TenantID:  prefix + "-foreign-tenant",
		ProjectID: root.ProjectID,
		Name:      "foreign-parent",
		Title:     "Foreign parent",
		Path:      "/" + prefix + "/foreign-parent",
	}
	if err := permission.AddMenu(foreignParent); err != nil {
		t.Fatalf("create foreign parent fixture: %v", err)
	}
	crossTenantParent := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/menu/add", map[string]any{
		"id":        prefix + "-cross-tenant-child",
		"projectID": root.ProjectID,
		"parentID":  foreignParent.ID,
		"name":      "cross-tenant-child",
		"title":     "Cross tenant child",
		"path":      "/" + prefix + "/cross-tenant-child",
	}))
	if crossTenantParent.Code != apipb.Code_BadRequest {
		t.Fatalf("cross-tenant parent should be rejected, got %v (%s)", crossTenantParent.Code, crossTenantParent.Message)
	}

	for _, request := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/core/auth/menu/detail?id=" + foreignParent.ID, nil},
		{http.MethodGet, "/api/core/auth/menu/impact?id=" + foreignParent.ID, nil},
		{http.MethodPut, "/api/core/auth/menu/update", map[string]any{
			"id": foreignParent.ID, "name": foreignParent.Name, "title": foreignParent.Title, "path": foreignParent.Path,
		}},
		{http.MethodDelete, "/api/core/auth/menu/delete", map[string]any{"id": foreignParent.ID}},
	} {
		response := decodeCommonResponse(t, doJSONRequest(t, engine, request.method, request.path, request.body))
		if response.Code != apipb.Code_NoPermission {
			t.Fatalf("%s %s should be tenant denied, got %v", request.method, request.path, response.Code)
		}
	}

	blockerParent := &permission.Menu{
		Model:     commonmodel.Model{ID: prefix + "-blocker"},
		TenantID:  tenantAdmin.TenantID,
		ProjectID: root.ProjectID,
		Name:      "blocker-parent",
		Title:     "Blocker parent",
		Path:      "/" + prefix + "/blocker",
	}
	blockerChild := &permission.Menu{
		Model:     commonmodel.Model{ID: prefix + "-blocker-child"},
		TenantID:  tenantAdmin.TenantID,
		ProjectID: root.ProjectID,
		ParentID:  blockerParent.ID,
		Name:      "blocker-child",
		Title:     "Blocker child",
		Path:      "/" + prefix + "/blocker-child",
	}
	if err := permission.AddMenu(blockerParent); err != nil {
		t.Fatalf("create blocker parent: %v", err)
	}
	if err := permission.AddMenu(blockerChild); err != nil {
		t.Fatalf("create blocker child: %v", err)
	}
	impactRecorder := doJSONRequest(t, engine, http.MethodGet, "/api/core/auth/menu/impact?id="+blockerParent.ID, nil)
	impactEnvelope := struct {
		Code apipb.Code            `json:"code"`
		Data permission.MenuImpact `json:"data"`
	}{}
	if err := json.Unmarshal(impactRecorder.Body.Bytes(), &impactEnvelope); err != nil {
		t.Fatalf("decode menu impact: %v (body=%s)", err, impactRecorder.Body.String())
	}
	if impactEnvelope.Code != commonmodel.Success ||
		impactEnvelope.Data.DirectChildCount != 1 ||
		impactEnvelope.Data.CanDelete {
		t.Fatalf("unexpected menu impact: %#v", impactEnvelope)
	}
	blockedDelete := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodDelete, "/api/core/auth/menu/delete", map[string]any{
		"id": blockerParent.ID,
	}))
	if blockedDelete.Code != apipb.Code_BadRequest {
		t.Fatalf("parent menu deletion should be blocked, got %v (%s)", blockedDelete.Code, blockedDelete.Message)
	}

	systemMenu := &permission.Menu{
		Model:     commonmodel.Model{ID: prefix + "-system"},
		TenantID:  tenantAdmin.TenantID,
		ProjectID: root.ProjectID,
		Name:      "system-menu",
		Title:     "System menu",
		Path:      "/" + prefix + "/system",
		IsMust:    true,
	}
	if err := permission.AddMenu(systemMenu); err != nil {
		t.Fatalf("create system menu fixture: %v", err)
	}
	protectedUpdate := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPut, "/api/core/auth/menu/update", map[string]any{
		"id":     systemMenu.ID,
		"name":   systemMenu.Name,
		"title":  systemMenu.Title,
		"path":   systemMenu.Path,
		"hidden": true,
	}))
	protectedDelete := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodDelete, "/api/core/auth/menu/delete", map[string]any{
		"id": systemMenu.ID,
	}))
	if protectedUpdate.Code != apipb.Code_BadRequest || protectedDelete.Code != apipb.Code_BadRequest {
		t.Fatalf("system menu protection failed: update=%v delete=%v", protectedUpdate.Code, protectedDelete.Code)
	}

	queryRecorder := doJSONRequest(
		t,
		engine,
		http.MethodGet,
		"/api/core/auth/menu/query?pageIndex=1&pageSize=50&keyword=Hidden%20workspace&visibility=hidden",
		nil,
	)
	queryResponse := &apipb.QueryMenuResponse{}
	if err := json.Unmarshal(queryRecorder.Body.Bytes(), queryResponse); err != nil {
		t.Fatalf("decode menu query: %v (body=%s)", err, queryRecorder.Body.String())
	}
	if queryResponse.Code != commonmodel.Success ||
		queryResponse.Records != 1 ||
		len(queryResponse.Data) != 1 ||
		queryResponse.Data[0].Id != child.ID ||
		!queryResponse.Data[0].Hidden ||
		!queryResponse.Data[0].CloseTab {
		t.Fatalf("keyword/visibility query or closeTab response mismatch: %#v", queryResponse)
	}
}

func TestAPIResourceManagementEnforcesTenantValidationAndAtomicDuplicates(t *testing.T) {
	const prefix = "api-resource-boundary"
	tenantAdmin := &apipb.CurrentUser{Id: prefix + "-admin", TenantID: prefix + "-tenant"}
	engine := newAPITestEngine(tenantAdmin)

	create := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/api/add", map[string]any{
		"id":          prefix + "-api",
		"tenantID":    "foreign-tenant",
		"path":        " /api/" + prefix + "/documents ",
		"group":       " document ",
		"method":      "get",
		"description": " tenant-owned endpoint ",
		"enable":      true,
		"checkAuth":   true,
		"checkLogin":  false,
	}))
	if create.Code != commonmodel.Success {
		t.Fatalf("create API resource failed: %v (%s)", create.Code, create.Message)
	}
	stored, err := permission.GetAPIById(prefix + "-api")
	if err != nil {
		t.Fatalf("load created API: %v", err)
	}
	if stored.TenantID != tenantAdmin.TenantID ||
		stored.Path != "/api/"+prefix+"/documents" ||
		stored.Method != http.MethodGet ||
		!stored.CheckAuth ||
		!stored.CheckLogin {
		t.Fatalf("API normalization or tenant scoping failed: %#v", stored)
	}

	duplicate := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/api/add", map[string]any{
		"id":         prefix + "-duplicate",
		"path":       stored.Path,
		"method":     "GET",
		"enable":     true,
		"checkLogin": true,
	}))
	if duplicate.Code != apipb.Code_BadRequest {
		t.Fatalf("duplicate API should be rejected as bad request, got %v (%s)", duplicate.Code, duplicate.Message)
	}
	invalid := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/api/add", map[string]any{
		"id":     prefix + "-invalid",
		"path":   "api/without-leading-slash",
		"method": "TRACE",
	}))
	if invalid.Code != apipb.Code_BadRequest {
		t.Fatalf("invalid API should be rejected, got %v (%s)", invalid.Code, invalid.Message)
	}

	foreign := &permission.API{
		Model:       commonmodel.Model{ID: prefix + "-foreign"},
		TenantID:    prefix + "-foreign-tenant",
		Path:        "/api/" + prefix + "/foreign",
		Method:      http.MethodGet,
		Description: "foreign tenant endpoint",
		Enable:      true,
		CheckLogin:  true,
	}
	if err := permission.CreateAPIResource(foreign); err != nil {
		t.Fatalf("create foreign API fixture: %v", err)
	}
	for _, request := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/core/auth/api/detail?id=" + foreign.ID, nil},
		{http.MethodGet, "/api/core/auth/api/impact?id=" + foreign.ID, nil},
		{http.MethodPut, "/api/core/auth/api/update", map[string]any{
			"id": foreign.ID, "path": foreign.Path, "method": foreign.Method, "enable": true, "checkLogin": true,
		}},
		{http.MethodPost, "/api/core/auth/api/enable", map[string]any{"id": foreign.ID, "enable": false}},
		{http.MethodDelete, "/api/core/auth/api/delete", map[string]any{"id": foreign.ID}},
	} {
		response := decodeCommonResponse(t, doJSONRequest(t, engine, request.method, request.path, request.body))
		if response.Code != apipb.Code_NoPermission {
			t.Fatalf("%s %s should be tenant denied, got %v", request.method, request.path, response.Code)
		}
	}

	second := &permission.API{
		Model:       commonmodel.Model{ID: prefix + "-second"},
		TenantID:    tenantAdmin.TenantID,
		Path:        "/api/" + prefix + "/second",
		Method:      http.MethodPost,
		Description: "searchable atomic duplicate target",
		Enable:      true,
		CheckLogin:  true,
	}
	if err := permission.CreateAPIResource(second); err != nil {
		t.Fatalf("create second API: %v", err)
	}
	updateDuplicate := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPut, "/api/core/auth/api/update", map[string]any{
		"id": second.ID, "path": stored.Path, "method": stored.Method, "enable": true, "checkLogin": true,
	}))
	if updateDuplicate.Code != apipb.Code_BadRequest {
		t.Fatalf("duplicate update should be rejected, got %v (%s)", updateDuplicate.Code, updateDuplicate.Message)
	}
	unchanged, err := permission.GetAPIById(second.ID)
	if err != nil {
		t.Fatalf("reload duplicate target: %v", err)
	}
	if unchanged.Path != second.Path || unchanged.Method != second.Method {
		t.Fatalf("failed duplicate update changed stored API: %#v", unchanged)
	}

	queryRecorder := doJSONRequest(
		t,
		engine,
		http.MethodGet,
		"/api/core/auth/api/query?pageIndex=1&pageSize=20&keyword=searchable&enable=true",
		nil,
	)
	queryResponse := &apipb.QueryAPIResponse{}
	if err := json.Unmarshal(queryRecorder.Body.Bytes(), queryResponse); err != nil {
		t.Fatalf("decode API query: %v (body=%s)", err, queryRecorder.Body.String())
	}
	if queryResponse.Code != commonmodel.Success || queryResponse.Records != 1 ||
		len(queryResponse.Data) != 1 || queryResponse.Data[0].Id != second.ID {
		t.Fatalf("keyword/status query mismatch: %#v", queryResponse)
	}

	systemAPI := &permission.API{
		Model:      commonmodel.Model{ID: prefix + "-system"},
		TenantID:   tenantAdmin.TenantID,
		Path:       "/api/" + prefix + "/system",
		Method:     http.MethodGet,
		Enable:     true,
		CheckLogin: true,
		IsMust:     true,
	}
	if err := permission.CreateAPIResource(systemAPI); err != nil {
		t.Fatalf("create system API: %v", err)
	}
	disableSystem := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/api/enable", map[string]any{
		"id": systemAPI.ID, "enable": false,
	}))
	deleteSystem := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodDelete, "/api/core/auth/api/delete", map[string]any{
		"id": systemAPI.ID,
	}))
	if disableSystem.Code != apipb.Code_BadRequest || deleteSystem.Code != apipb.Code_BadRequest {
		t.Fatalf("system API protection failed: disable=%v delete=%v", disableSystem.Code, deleteSystem.Code)
	}
}

func TestAPIResourceMutationRebuildsPoliciesAndRejectsBoundDelete(t *testing.T) {
	const prefix = "api-resource-policy"
	api := &permission.API{
		Model:       commonmodel.Model{ID: prefix + "-api"},
		TenantID:    platformTenant,
		Path:        "/api/" + prefix + "/documents",
		Group:       "documents",
		Method:      http.MethodGet,
		Description: "policy-carrying API",
		Enable:      true,
		CheckAuth:   true,
		CheckLogin:  true,
	}
	if err := permission.CreateAPIResource(api); err != nil {
		t.Fatalf("create API: %v", err)
	}
	menu := &permission.Menu{
		Model:    commonmodel.Model{ID: prefix + "-menu"},
		TenantID: platformTenant,
		Name:     prefix,
		Title:    "Policy workspace",
		Path:     "/" + prefix,
	}
	function := &permission.MenuFunc{
		Model:  commonmodel.Model{ID: prefix + "-function"},
		MenuID: menu.ID,
		Name:   "view",
		Title:  "View documents",
	}
	link := &permission.MenuFuncApi{
		Model:      commonmodel.Model{ID: prefix + "-link"},
		MenuFuncID: function.ID,
		APIID:      api.ID,
	}
	role := &permission.Role{
		Model:    commonmodel.Model{ID: prefix + "-role"},
		TenantID: platformTenant,
		Name:     "Policy role",
		CanDel:   true,
		Enable:   true,
	}
	if err := store.DB().Create(menu).Error; err != nil {
		t.Fatalf("create menu: %v", err)
	}
	if err := store.DB().Create(function).Error; err != nil {
		t.Fatalf("create menu function: %v", err)
	}
	if err := store.DB().Create(link).Error; err != nil {
		t.Fatalf("create function API link: %v", err)
	}
	if err := store.DB().Create(role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	detail, err := permission.GetRoleAuthorization(role.ID)
	if err != nil {
		t.Fatalf("load role authorization: %v", err)
	}
	if _, err := permission.PublishRoleAuthorization(role.ID, detail.Revision, []permission.RoleAuthorizationSelection{
		{MenuID: menu.ID, Show: true, Funcs: []string{function.Name}},
	}); err != nil {
		t.Fatalf("publish role authorization: %v", err)
	}
	const userID = prefix + "-user"
	if err := store.DB().Create(&user.UserRole{
		Model: commonmodel.Model{ID: prefix + "-user-role"}, UserID: userID, RoleID: role.ID,
	}).Error; err != nil {
		t.Fatalf("create user role: %v", err)
	}
	activeSession := &session.Session{
		Model: commonmodel.Model{ID: prefix + "-session-update"}, PrincipalID: userID, TenantID: platformTenant, TokenSig: prefix + "-token-update",
	}
	if err := store.DB().Create(activeSession).Error; err != nil {
		t.Fatalf("create update session: %v", err)
	}

	impact, err := permission.GetAPIResourceImpact(api.ID)
	if err != nil {
		t.Fatalf("load API impact: %v", err)
	}
	if impact.MenuFunctionBindingCount != 1 ||
		impact.ActiveRoleCount != 1 ||
		impact.ActivePolicyCount != 1 ||
		impact.AffectedUserCount != 1 ||
		impact.CanDelete {
		t.Fatalf("unexpected API impact: %#v", impact)
	}

	platformAdmin := &apipb.CurrentUser{Id: prefix + "-admin", TenantID: platformTenant}
	engine := newAPITestEngine(platformAdmin)
	updatedPath := "/api/" + prefix + "/documents/:id"
	update := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPut, "/api/core/auth/api/update", map[string]any{
		"id": api.ID, "path": updatedPath, "group": "documents", "method": "patch",
		"description": "updated policy endpoint", "enable": true, "checkAuth": true, "checkLogin": true,
	}))
	if update.Code != commonmodel.Success {
		t.Fatalf("update bound API failed: %v (%s)", update.Code, update.Message)
	}
	var policies []*permission.CasbinRule
	if err := store.DB().Where("ptype = ? AND v0 = ?", "p", role.ID).Find(&policies).Error; err != nil {
		t.Fatalf("load updated policies: %v", err)
	}
	if len(policies) != 1 || policies[0].Path != updatedPath || policies[0].Method != http.MethodPatch {
		t.Fatalf("bound API update did not rebuild policy: %#v", policies)
	}
	if err := store.DB().Where("id = ?", activeSession.ID).First(activeSession).Error; err != nil {
		t.Fatalf("reload update session: %v", err)
	}
	if !activeSession.Revoked || activeSession.RevokedReason != "API resource updated" {
		t.Fatalf("API update did not revoke session: %#v", activeSession)
	}

	stateSession := &session.Session{
		Model: commonmodel.Model{ID: prefix + "-session-state"}, PrincipalID: userID, TenantID: platformTenant, TokenSig: prefix + "-token-state",
	}
	if err := store.DB().Create(stateSession).Error; err != nil {
		t.Fatalf("create state session: %v", err)
	}
	disable := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/api/enable", map[string]any{
		"id": api.ID, "enable": false,
	}))
	if disable.Code != commonmodel.Success {
		t.Fatalf("disable bound API failed: %v (%s)", disable.Code, disable.Message)
	}
	var policyCount int64
	if err := store.DB().Model(&permission.CasbinRule{}).
		Where("ptype = ? AND v0 = ?", "p", role.ID).
		Count(&policyCount).Error; err != nil {
		t.Fatalf("count disabled API policies: %v", err)
	}
	if policyCount != 0 {
		t.Fatalf("disabled API retained role policy, count=%d", policyCount)
	}
	if err := store.DB().Where("id = ?", stateSession.ID).First(stateSession).Error; err != nil {
		t.Fatalf("reload state session: %v", err)
	}
	if !stateSession.Revoked || stateSession.RevokedReason != "API resource state updated" {
		t.Fatalf("API state change did not revoke session: %#v", stateSession)
	}
	enable := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodPost, "/api/core/auth/api/enable", map[string]any{
		"id": api.ID, "enable": true,
	}))
	if enable.Code != commonmodel.Success {
		t.Fatalf("re-enable bound API failed: %v (%s)", enable.Code, enable.Message)
	}
	if err := store.DB().Model(&permission.CasbinRule{}).
		Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ?", "p", role.ID, updatedPath, http.MethodPatch).
		Count(&policyCount).Error; err != nil {
		t.Fatalf("count re-enabled API policies: %v", err)
	}
	if policyCount != 1 {
		t.Fatalf("re-enabled API policy count=%d, want 1", policyCount)
	}

	boundDelete := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodDelete, "/api/core/auth/api/delete", map[string]any{
		"id": api.ID,
	}))
	if boundDelete.Code != apipb.Code_BadRequest {
		t.Fatalf("bound API delete should be rejected, got %v (%s)", boundDelete.Code, boundDelete.Message)
	}
	if err := store.DB().Unscoped().Delete(&permission.MenuFuncApi{}, "id = ?", link.ID).Error; err != nil {
		t.Fatalf("remove API binding: %v", err)
	}
	unboundImpact, err := permission.GetAPIResourceImpact(api.ID)
	if err != nil {
		t.Fatalf("reload unbound impact: %v", err)
	}
	if !unboundImpact.CanDelete || unboundImpact.MenuFunctionBindingCount != 0 {
		t.Fatalf("unbound API should be deletable: %#v", unboundImpact)
	}
	deleted := decodeCommonResponse(t, doJSONRequest(t, engine, http.MethodDelete, "/api/core/auth/api/delete", map[string]any{
		"id": api.ID,
	}))
	if deleted.Code != commonmodel.Success {
		t.Fatalf("delete unbound API failed: %v (%s)", deleted.Code, deleted.Message)
	}
	if _, err := permission.GetAPIById(api.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted API should be absent, err=%v", err)
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
