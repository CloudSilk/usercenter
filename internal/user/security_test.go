package user_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CloudSilk/pkg/db"
	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/bootstrap"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	glebsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestMain 使用 sqlite 临时库初始化 store，使安全相关单元测试不依赖外部 MySQL。
// 依赖真实 MySQL 的集成测试原 model/*_test.go（build tag: integration）随 model 包删除而移除；
// 这里的单元测试覆盖登录失败/锁定、staff_no 免密回归、重置/修改口令等纯内存路径。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "usercenter_user_test_")
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
	// 初始化 PII 加密密钥（MFA factor secret 用 AES-GCM 加密）
	auth.SetPIIKeyFrom("test-secret-key")
	store.SetDB(db.NewDBClient(gdb, false))
	if err := bootstrap.RunMigration(); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// mustCreateUser 直接写入一条启用用户（绕过 CreateUser 的租户/密码强度业务规则），
// 密码用 scrypt 哈希，便于 Login 校验。
func mustCreateUser(t *testing.T, userName, password string) *user.User {
	t.Helper()
	hash, err := auth.EncryptedPassword(password)
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	u := &user.User{
		UserName: userName,
		Password: hash,
		Nickname: userName,
		Enable:   true,
	}
	if err := store.DB().Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestGeneratePasswordRandomAndInCharset(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		p := auth.GeneratePasswd(16, auth.PwdStrengthAdvance)
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
	user.Login(&apipb.LoginRequest{UserName: "okuser", Password: "wrong"}, &apipb.LoginResponse{})
	// 正确密码登录（Code 由调用方预设，与 provider/user.go 一致）
	resp := &apipb.LoginResponse{Code: commonmodel.Success}
	user.Login(&apipb.LoginRequest{UserName: "okuser", Password: "Abc12345"}, resp)
	if resp.Code != apipb.Code_Success {
		t.Fatalf("expected login success, got %v (%s)", resp.Code, resp.Message)
	}
	var dbu user.User
	store.DB().First(&dbu, "id = ?", u.ID)
	if dbu.ErrNumber != 0 || dbu.LockedExpired != 0 {
		t.Fatalf("expected cleared counters, got errNumber=%d lockedExpired=%d", dbu.ErrNumber, dbu.LockedExpired)
	}
}

func TestLoginLockoutAfterMaxFailures(t *testing.T) {
	u := mustCreateUser(t, "lockuser", "Abc12345")
	wrong := &apipb.LoginRequest{UserName: "lockuser", Password: "wrong"}

	// 连续 MaxErrCount 次错误密码
	for i := 0; i < int(int(5)); i++ {
		resp := &apipb.LoginResponse{}
		user.Login(wrong, resp)
		if resp.Code != apipb.Code_UserNameOrPasswordIsWrong {
			t.Fatalf("attempt %d: expected wrong-password code, got %v", i+1, resp.Code)
		}
	}

	// 达到阈值后应已锁定：即使正确密码也返回 UserDisabled
	resp := &apipb.LoginResponse{}
	user.Login(&apipb.LoginRequest{UserName: "lockuser", Password: "Abc12345"}, resp)
	if resp.Code != apipb.Code_UserDisabled {
		t.Fatalf("expected locked/disabled after %d failures, got %v", int(5), resp.Code)
	}

	var dbu user.User
	store.DB().First(&dbu, "id = ?", u.ID)
	if dbu.LockedExpired <= time.Now().Unix() {
		t.Fatalf("expected LockedExpired in the future, got %d (now %d)", dbu.LockedExpired, time.Now().Unix())
	}
}

func TestLoginByStaffNoRequiresPassword(t *testing.T) {
	// S1 回归：修复前 staff_no 入口免密即可登录；修复后必须校验密码
	u := mustCreateUser(t, "staffuser", "Abc12345")
	store.DB().Model(&user.User{}).Where("id = ?", u.ID).Update("staff_no", "staff001")

	// 错误密码 → 必须失败（修复前会成功签发 token）
	wrong := &apipb.LoginByStaffNoResponse{Code: commonmodel.Success}
	user.LoginByStaffNo(&apipb.LoginByStaffNoRequest{StaffNo: "staff001", Password: "wrong"}, wrong)
	if wrong.Code != apipb.Code_UserNameOrPasswordIsWrong {
		t.Fatalf("expected wrong-password code for staff_no with wrong password, got %v", wrong.Code)
	}

	// 正确密码 → 成功
	ok := &apipb.LoginByStaffNoResponse{Code: commonmodel.Success}
	user.LoginByStaffNo(&apipb.LoginByStaffNoRequest{StaffNo: "staff001", Password: "Abc12345"}, ok)
	if ok.Code != apipb.Code_Success {
		t.Fatalf("expected success for staff_no with correct password, got %v (%s)", ok.Code, ok.Message)
	}
}

func TestResetPwdForcesChangeAndAllowsNewPassword(t *testing.T) {
	u := mustCreateUser(t, "resetuser", "Abc12345")
	newPwd := "Xyz98765"
	if err := user.ResetPwd(u.ID, newPwd); err != nil {
		t.Fatalf("ResetPwd: %v", err)
	}
	// 新密码可登录
	resp := &apipb.LoginResponse{Code: commonmodel.Success}
	user.Login(&apipb.LoginRequest{UserName: "resetuser", Password: newPwd}, resp)
	if resp.Code != apipb.Code_Success {
		t.Fatalf("expected login success with reset password, got %v (%s)", resp.Code, resp.Message)
	}
	// 旧密码应失效
	old := &apipb.LoginResponse{}
	user.Login(&apipb.LoginRequest{UserName: "resetuser", Password: "Abc12345"}, old)
	if old.Code == apipb.Code_Success {
		t.Fatal("old password should no longer work after reset")
	}
	// 重置后强制改密标记置位
	var dbu user.User
	store.DB().First(&dbu, "id = ?", u.ID)
	if !dbu.ForceChangePwd {
		t.Fatal("expected ForceChangePwd=true after reset")
	}
}

func TestResetPwdRandomWhenDefaultEmpty(t *testing.T) {
	u := mustCreateUser(t, "resetuser2", "Abc12345")
	saved := user.DefaultPwd
	user.DefaultPwd = ""
	defer func() { user.DefaultPwd = saved }()

	if err := user.ResetPwd(u.ID, user.DefaultPwd); err != nil {
		t.Fatalf("ResetPwd with empty default: %v", err)
	}
	var dbu user.User
	store.DB().First(&dbu, "id = ?", u.ID)
	if dbu.Password == "" {
		t.Fatal("expected non-empty (random) password when default is empty")
	}
	if !dbu.ForceChangePwd {
		t.Fatal("expected ForceChangePwd=true after reset")
	}
}

func TestUpdatePwdRejectsWeakPassword(t *testing.T) {
	u := mustCreateUser(t, "weakpwduser", "Abc12345")
	// 弱密码（纯数字，无大小写）应被拒绝
	if err := user.UpdatePwd(u.ID, "Abc12345", "12345678"); err == nil {
		t.Fatal("expected error for weak new password")
	}
	// 旧密码未变更（弱密码被拒），强密码应成功
	if err := user.UpdatePwd(u.ID, "Abc12345", "Xyz98765"); err != nil {
		t.Fatalf("strong password should succeed, got %v", err)
	}
}

func TestUpdateUserRolesReplacesLinksAndInvalidatesActiveContext(t *testing.T) {
	u := mustCreateUser(t, "role-assignment-user", "Abc12345")
	if err := store.DB().Model(&user.User{}).Where("id = ?", u.ID).Update("tenant_id", "tenant-role-a").Error; err != nil {
		t.Fatalf("set user tenant: %v", err)
	}
	roles := []*permission.Role{
		{Model: commonmodel.Model{ID: "role-assignment-1"}, TenantID: "tenant-role-a", Name: "材料起草"},
		{Model: commonmodel.Model{ID: "role-assignment-2"}, TenantID: "tenant-role-a", Name: "材料审校"},
	}
	if err := store.DB().Create(&roles).Error; err != nil {
		t.Fatalf("create roles: %v", err)
	}
	if err := store.DB().Create(&user.UserRole{UserID: u.ID, RoleID: "role-assignment-1"}).Error; err != nil {
		t.Fatalf("create old role link: %v", err)
	}

	accessToken, err := token.EncodeToken(&apipb.CurrentUser{
		Id: u.ID, UserName: u.UserName, TenantID: "tenant-role-a", RoleIDs: []string{"role-assignment-1"},
	})
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	if exists, err := token.DefaultTokenCache.Exists(u.ID, accessToken); err != nil || !exists {
		t.Fatalf("token must exist before role update: exists=%v err=%v", exists, err)
	}
	activeSession := &session.Session{
		Model:       commonmodel.Model{ID: "role-assignment-session"},
		PrincipalID: u.ID,
		TenantID:    "tenant-role-a",
	}
	if err := store.DB().Create(activeSession).Error; err != nil {
		t.Fatalf("create active session: %v", err)
	}

	result, err := user.UpdateUserRoles(u.ID, []string{"role-assignment-2", "role-assignment-2"})
	if err != nil {
		t.Fatalf("update roles: %v", err)
	}
	if len(result.RoleIDs) != 1 || result.RoleIDs[0] != "role-assignment-2" {
		t.Fatalf("unexpected normalized roles: %#v", result.RoleIDs)
	}
	if len(result.PreviousRoleIDs) != 1 || result.PreviousRoleIDs[0] != "role-assignment-1" {
		t.Fatalf("unexpected previous roles: %#v", result.PreviousRoleIDs)
	}
	if result.SessionsRevoked != 1 {
		t.Fatalf("sessions revoked = %d, want 1", result.SessionsRevoked)
	}

	var links []*user.UserRole
	if err := store.DB().Where("user_id = ?", u.ID).Find(&links).Error; err != nil {
		t.Fatalf("query new role links: %v", err)
	}
	if len(links) != 1 || links[0].RoleID != "role-assignment-2" {
		t.Fatalf("role links were not replaced: %#v", links)
	}
	if exists, err := token.DefaultTokenCache.Exists(u.ID, accessToken); err != nil || exists {
		t.Fatalf("stale token must be invalidated: exists=%v err=%v", exists, err)
	}
	var storedSession session.Session
	if err := store.DB().First(&storedSession, "id = ?", activeSession.ID).Error; err != nil {
		t.Fatalf("reload session: %v", err)
	}
	if !storedSession.Revoked || storedSession.RevokedReason != "user roles updated" {
		t.Fatalf("active session was not revoked: %#v", storedSession)
	}
}

func TestUpdateUserRolesRejectsForeignTenantRoleWithoutChangingLinks(t *testing.T) {
	u := mustCreateUser(t, "role-assignment-tenant-user", "Abc12345")
	if err := store.DB().Model(&user.User{}).Where("id = ?", u.ID).Update("tenant_id", "tenant-role-owner").Error; err != nil {
		t.Fatalf("set user tenant: %v", err)
	}
	roles := []*permission.Role{
		{Model: commonmodel.Model{ID: "role-assignment-owned"}, TenantID: "tenant-role-owner", Name: "本租户角色"},
		{Model: commonmodel.Model{ID: "role-assignment-foreign"}, TenantID: "tenant-role-foreign", Name: "其他租户角色"},
	}
	if err := store.DB().Create(&roles).Error; err != nil {
		t.Fatalf("create roles: %v", err)
	}
	if err := store.DB().Create(&user.UserRole{UserID: u.ID, RoleID: "role-assignment-owned"}).Error; err != nil {
		t.Fatalf("create old role link: %v", err)
	}

	if _, err := user.UpdateUserRoles(u.ID, []string{"role-assignment-foreign"}); err == nil {
		t.Fatal("foreign-tenant role assignment must be rejected")
	}
	var links []*user.UserRole
	if err := store.DB().Where("user_id = ?", u.ID).Find(&links).Error; err != nil {
		t.Fatalf("query preserved role links: %v", err)
	}
	if len(links) != 1 || links[0].RoleID != "role-assignment-owned" {
		t.Fatalf("failed assignment changed existing links: %#v", links)
	}
}

func TestGetUserTenantID(t *testing.T) {
	u := mustCreateUser(t, "tenantuser", "Abc12345")
	store.DB().Model(&user.User{}).Where("id = ?", u.ID).Update("tenant_id", "tenant-xyz")

	tid, err := user.GetUserTenantID(u.ID)
	if err != nil || tid != "tenant-xyz" {
		t.Fatalf("expected tenant-xyz, got %q err=%v", tid, err)
	}
	// 不存在的用户应返回错误，避免越权校验被绕过
	if _, err := user.GetUserTenantID("nonexistent-id"); err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestDeleteUserUsesOneTransactionAndRemovesRoleLinks(t *testing.T) {
	u := mustCreateUser(t, "deleteuser", "Abc12345")
	if err := store.DB().Model(&user.User{}).Where("id = ?", u.ID).Update("can_del", true).Error; err != nil {
		t.Fatalf("mark user deletable: %v", err)
	}
	link := &user.UserRole{UserID: u.ID, RoleID: "delete-test-role"}
	if err := store.DB().Create(link).Error; err != nil {
		t.Fatalf("create user role link: %v", err)
	}

	if err := user.DeleteUser(u.ID); err != nil {
		t.Fatalf("DeleteUser must not self-lock its sqlite transaction: %v", err)
	}

	var activeUsers int64
	if err := store.DB().Model(&user.User{}).Where("id = ?", u.ID).Count(&activeUsers).Error; err != nil {
		t.Fatalf("count active users: %v", err)
	}
	if activeUsers != 0 {
		t.Fatalf("expected user to be soft deleted, active count=%d", activeUsers)
	}
	var roleLinks int64
	if err := store.DB().Unscoped().Model(&user.UserRole{}).Where("user_id = ?", u.ID).Count(&roleLinks).Error; err != nil {
		t.Fatalf("count role links: %v", err)
	}
	if roleLinks != 0 {
		t.Fatalf("expected user role links to be removed, count=%d", roleLinks)
	}
}

func TestUserToPBRedactsPasswordAndKeepsManagementFields(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	u := &user.User{
		TenantModel: commonmodel.TenantModel{
			Model:    commonmodel.Model{ID: "user-redaction", CreatedAt: now},
			TenantID: "tenant-redaction",
		},
		UserName:    "redaction",
		Password:    "sensitive-password-hash",
		Nickname:    "脱敏用户",
		Title:       "办公室主任",
		Description: "负责公文审核",
		RealName:    "测试姓名",
		Enable:      true,
	}

	got := user.UserToPB(u)
	if got.Password != "" {
		t.Fatalf("password hash must never be serialized, got %q", got.Password)
	}
	if got.Title != u.Title || got.Description != u.Description || got.RealName != u.RealName {
		t.Fatalf("management fields were lost: %#v", got)
	}
	if got.CreatedAt == "" {
		t.Fatal("createdAt must be available to the management list")
	}
}

func TestEnforceCacheHitIsConsistent(t *testing.T) {
	// 同一 (sub,obj,act) 连续判定两次，结果应一致（命中缓存或未命中都应一致）
	url := "/api/cachecheck/test"
	c1, err1 := permission.EnforceCached("nonexistent-role", url, "GET")
	c2, err2 := permission.EnforceCached("nonexistent-role", url, "GET")
	if err1 != nil || err2 != nil {
		t.Fatalf("enforceCached errors: %v %v", err1, err2)
	}
	if c1 != c2 {
		t.Fatalf("cache consistency: %v vs %v", c1, c2)
	}
}
