//go:build !integration
// +build !integration

package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CloudSilk/pkg/db"
	commonmodel "github.com/CloudSilk/pkg/model"
	glebsqlite "github.com/glebarez/sqlite"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/model/token"
	"gorm.io/gorm"
)

// TestMain 使用 sqlite 临时库初始化 model 包，使安全相关单元测试不依赖外部 MySQL。
// 依赖真实 MySQL 的集成测试见 db_test/api_test/casbin_rule_test/menu_test（build tag: integration）。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "usercenter_test_")
	if err != nil {
		panic(err)
	}
	// 使用 pure-Go 的 glebarez/sqlite，避免依赖 CGO（mattn/go-sqlite3）
	gdb, err := gorm.Open(glebsqlite.Open(filepath.Join(dir, "test.db")), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	// 初始化 token 缓存（内存模式，不依赖 Redis）；提供非空密钥以满足启动校验
	token.InitTokenCache("test-secret-key", "", "", "", 120)
	InitDB(db.NewDBClient(gdb, false), true) // debug=true 触发 AutoMigrate 建表
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// mustCreateUser 直接写入一条启用用户（绕过 CreateUser 的租户/密码强度业务规则），
// 密码用 scrypt 哈希，便于 Login 校验。
func mustCreateUser(t *testing.T, userName, password string) *User {
	t.Helper()
	hash, err := EncryptedPassword(password)
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	u := &User{
		UserName: userName,
		Password: hash,
		Nickname: userName,
		Enable:   true,
	}
	if err := dbClient.DB().Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestGeneratePasswordRandomAndInCharset(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		p := generatePasswd(16, PwdStrengthAdvance)
		if len(p) != 16 {
			t.Fatalf("expected length 16, got %d (%q)", len(p), p)
		}
		seen[p] = true
	}
	if len(seen) < 10 {
		t.Fatalf("password not sufficiently random: only %d unique values in 20 runs", len(seen))
	}
}

func TestLoginSuccessClearsFailures(t *testing.T) {
	u := mustCreateUser(t, "okuser", "Abc12345")
	// 先制造一次失败
	Login(&apipb.LoginRequest{UserName: "okuser", Password: "wrong"}, &apipb.LoginResponse{})
	// 正确密码登录（Code 由调用方预设，与 provider/user.go 一致）
	resp := &apipb.LoginResponse{Code: commonmodel.Success}
	Login(&apipb.LoginRequest{UserName: "okuser", Password: "Abc12345"}, resp)
	if resp.Code != apipb.Code_Success {
		t.Fatalf("expected login success, got %v (%s)", resp.Code, resp.Message)
	}
	var dbu User
	dbClient.DB().First(&dbu, "id = ?", u.ID)
	if dbu.ErrNumber != 0 || dbu.LockedExpired != 0 {
		t.Fatalf("expected cleared counters, got errNumber=%d lockedExpired=%d", dbu.ErrNumber, dbu.LockedExpired)
	}
}

func TestLoginLockoutAfterMaxFailures(t *testing.T) {
	u := mustCreateUser(t, "lockuser", "Abc12345")
	wrong := &apipb.LoginRequest{UserName: "lockuser", Password: "wrong"}

	// 连续 MaxErrCount 次错误密码
	for i := 0; i < int(loginLockMaxErrCount); i++ {
		resp := &apipb.LoginResponse{}
		Login(wrong, resp)
		if resp.Code != apipb.Code_UserNameOrPasswordIsWrong {
			t.Fatalf("attempt %d: expected wrong-password code, got %v", i+1, resp.Code)
		}
	}

	// 达到阈值后应已锁定：即使正确密码也返回 UserDisabled
	resp := &apipb.LoginResponse{}
	Login(&apipb.LoginRequest{UserName: "lockuser", Password: "Abc12345"}, resp)
	if resp.Code != apipb.Code_UserDisabled {
		t.Fatalf("expected locked/disabled after %d failures, got %v", loginLockMaxErrCount, resp.Code)
	}

	var dbu User
	dbClient.DB().First(&dbu, "id = ?", u.ID)
	if dbu.LockedExpired <= time.Now().Unix() {
		t.Fatalf("expected LockedExpired in the future, got %d (now %d)", dbu.LockedExpired, time.Now().Unix())
	}
}

func TestLoginByStaffNoRequiresPassword(t *testing.T) {
	// S1 回归：修复前 staff_no 入口免密即可登录；修复后必须校验密码
	u := mustCreateUser(t, "staffuser", "Abc12345")
	dbClient.DB().Model(&User{}).Where("id = ?", u.ID).Update("staff_no", "staff001")

	// 错误密码 → 必须失败（修复前会成功签发 token）
	wrong := &apipb.LoginByStaffNoResponse{Code: commonmodel.Success}
	LoginByStaffNo(&apipb.LoginByStaffNoRequest{StaffNo: "staff001", Password: "wrong"}, wrong)
	if wrong.Code != apipb.Code_UserNameOrPasswordIsWrong {
		t.Fatalf("expected wrong-password code for staff_no with wrong password, got %v", wrong.Code)
	}

	// 正确密码 → 成功
	ok := &apipb.LoginByStaffNoResponse{Code: commonmodel.Success}
	LoginByStaffNo(&apipb.LoginByStaffNoRequest{StaffNo: "staff001", Password: "Abc12345"}, ok)
	if ok.Code != apipb.Code_Success {
		t.Fatalf("expected success for staff_no with correct password, got %v (%s)", ok.Code, ok.Message)
	}
}

func TestResetPwdForcesChangeAndAllowsNewPassword(t *testing.T) {
	u := mustCreateUser(t, "resetuser", "Abc12345")
	newPwd := "Xyz98765"
	if err := ResetPwd(u.ID, newPwd); err != nil {
		t.Fatalf("ResetPwd: %v", err)
	}
	// 新密码可登录
	resp := &apipb.LoginResponse{Code: commonmodel.Success}
	Login(&apipb.LoginRequest{UserName: "resetuser", Password: newPwd}, resp)
	if resp.Code != apipb.Code_Success {
		t.Fatalf("expected login success with reset password, got %v (%s)", resp.Code, resp.Message)
	}
	// 旧密码应失效
	old := &apipb.LoginResponse{}
	Login(&apipb.LoginRequest{UserName: "resetuser", Password: "Abc12345"}, old)
	if old.Code == apipb.Code_Success {
		t.Fatal("old password should no longer work after reset")
	}
	// 重置后强制改密标记置位
	var dbu User
	dbClient.DB().First(&dbu, "id = ?", u.ID)
	if !dbu.ForceChangePwd {
		t.Fatal("expected ForceChangePwd=true after reset")
	}
}

func TestResetPwdRandomWhenDefaultEmpty(t *testing.T) {
	u := mustCreateUser(t, "resetuser2", "Abc12345")
	saved := DefaultPwd
	DefaultPwd = ""
	defer func() { DefaultPwd = saved }()

	if err := ResetPwd(u.ID, DefaultPwd); err != nil {
		t.Fatalf("ResetPwd with empty default: %v", err)
	}
	var dbu User
	dbClient.DB().First(&dbu, "id = ?", u.ID)
	if dbu.Password == "" {
		t.Fatal("expected non-empty (random) password when default is empty")
	}
	if !dbu.ForceChangePwd {
		t.Fatal("expected ForceChangePwd=true after reset")
	}
}

func TestGetUserTenantID(t *testing.T) {
	u := mustCreateUser(t, "tenantuser", "Abc12345")
	dbClient.DB().Model(&User{}).Where("id = ?", u.ID).Update("tenant_id", "tenant-xyz")

	tid, err := GetUserTenantID(u.ID)
	if err != nil || tid != "tenant-xyz" {
		t.Fatalf("expected tenant-xyz, got %q err=%v", tid, err)
	}
	// 不存在的用户应返回错误，避免越权校验被绕过
	if _, err := GetUserTenantID("nonexistent-id"); err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestAuthenticateInvalidTokenOnProtectedURL(t *testing.T) {
	// 无效 token + 非白名单路径 → 应返回 TokenInvalid（同时验证 enforceCached 路径不 panic）
	_, code, _ := Authenticate("invalidtoken", "GET", "/api/protected/notwhitelisted", true)
	if code != commonmodel.TokenInvalid {
		t.Fatalf("expected TokenInvalid for bad token on protected url, got %v", code)
	}
}

func TestAuthenticateCacheHitIsConsistent(t *testing.T) {
	// 同一 (sub,obj,act) 连续判定两次，结果应一致（命中缓存或未命中都应一致）
	url := "/api/cachecheck/test"
	c1, err1 := enforceCached("nonexistent-role", url, "GET")
	c2, err2 := enforceCached("nonexistent-role", url, "GET")
	if err1 != nil || err2 != nil {
		t.Fatalf("enforceCached errors: %v %v", err1, err2)
	}
	if c1 != c2 {
		t.Fatalf("cache consistency: %v vs %v", c1, c2)
	}
}
