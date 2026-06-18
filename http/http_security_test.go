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
	glebsqlite "github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/model"
	"github.com/CloudSilk/usercenter/model/token"
	apipb "github.com/CloudSilk/usercenter/proto"
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
	model.InitDB(db.NewDBClient(gdb, false), true)
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
	_ = model.CreateTenant(&model.Tenant{
		Model:     commonmodel.Model{ID: tenantID},
		Name:      tenantID,
		Enable:    true,
		Expired:   time.Now().Add(24 * time.Hour),
		UserCount: 100,
	})
	u := &model.User{
		TenantModel: commonmodel.TenantModel{TenantID: tenantID},
		UserName:    userName,
		Password:    password,
		Nickname:    userName,
		Enable:      true,
	}
	if err := model.CreateUser(u, false); err != nil {
		t.Fatalf("create user %q: %v", userName, err)
	}
	return u.ID
}

func canLogin(t *testing.T, userName, password string) bool {
	t.Helper()
	resp := &apipb.LoginResponse{Code: commonmodel.Success}
	model.Login(&apipb.LoginRequest{UserName: userName, Password: password}, resp)
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
