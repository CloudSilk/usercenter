package auth

import (
	"strings"
	"testing"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	glebsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// --- TOTP ---

func TestTOTPGenerateAndVerify(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	if strings.ContainsAny(secret, "=/+") {
		t.Fatalf("secret should be base32 std without padding, got %q", secret)
	}

	code := GenerateTOTPCode(secret)
	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got %q", code)
	}
	if !VerifyTOTP(secret, code) {
		t.Fatal("current window code should verify")
	}
	// 错误码/长度不符拒绝
	if VerifyTOTP(secret, "000000") && code != "000000" {
		t.Fatal("wrong code should not verify")
	}
	if VerifyTOTP(secret, "12345") {
		t.Fatal("5-digit code should not verify")
	}
	// 非法 base32 secret:任何码都不验证
	if VerifyTOTP("!!!not-base32!!!", "123456") {
		t.Fatal("invalid secret should not verify")
	}
}

func TestGenerateTOTPURI(t *testing.T) {
	uri := GenerateTOTPURI("SECRET234", "alice@example.com", "UserCenter")
	for _, want := range []string{
		"otpauth://totp/UserCenter:alice@example.com",
		"secret=SECRET234",
		"issuer=UserCenter",
	} {
		if !strings.Contains(uri, want) {
			t.Fatalf("uri missing %q: %s", want, uri)
		}
	}
}

func TestRequiredACRForAction(t *testing.T) {
	l2 := []string{"delete_user", "reset_password", "change_password", "update_role"}
	for _, a := range l2 {
		if got := RequiredACRForAction(a); got != ACRLevel2 {
			t.Fatalf("action %q: expected %s, got %s", a, ACRLevel2, got)
		}
	}
	l3 := []string{"export_data", "delete_tenant"}
	for _, a := range l3 {
		if got := RequiredACRForAction(a); got != ACRLevel3 {
			t.Fatalf("action %q: expected %s, got %s", a, ACRLevel3, got)
		}
	}
	if got := RequiredACRForAction("login"); got != ACRLevel1 {
		t.Fatalf("default action: expected %s, got %s", ACRLevel1, got)
	}
}

// --- MFA challenge(一次性令牌)与因子校验(DB) ---

func TestMFAChallengeIssueAndConsume(t *testing.T) {
	tok, err := IssueMFAChallenge("user-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !strings.HasPrefix(tok, "mfa_") {
		t.Fatalf("expected mfa_ prefix, got %q", tok)
	}
	// 首次消费成功且返回绑定的 userID
	id, ok := ConsumeMFAChallenge(tok)
	if !ok || id != "user-1" {
		t.Fatalf("consume: id=%q ok=%v", id, ok)
	}
	// 单次使用:第二次消费失败
	if _, ok := ConsumeMFAChallenge(tok); ok {
		t.Fatal("challenge should be single-use")
	}
	// 未知令牌
	if _, ok := ConsumeMFAChallenge("mfa_unknown"); ok {
		t.Fatal("unknown challenge should fail")
	}
}

func setupMFADB(t *testing.T) {
	t.Helper()
	realDB, err := gorm.Open(glebsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := realDB.AutoMigrate(&MFAFactor{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store.SetDB(db.NewDBClient(realDB, false))
	t.Cleanup(func() { store.SetDB(nil) })
}

func TestHasEnabledMFA(t *testing.T) {
	setupMFADB(t)
	if HasEnabledMFA("nobody") {
		t.Fatal("unknown principal should have no MFA")
	}
	if err := store.DB().Create(&MFAFactor{
		ID: "f-p1", PrincipalID: "p1", Type: "totp", SecretEnc: "enc", Enable: true,
	}).Error; err != nil {
		t.Fatalf("seed factor: %v", err)
	}
	if !HasEnabledMFA("p1") {
		t.Fatal("expected enabled MFA for p1")
	}
	// 禁用因子不计入
	if err := store.DB().Create(&MFAFactor{
		ID: "f-p2", PrincipalID: "p2", Type: "totp", SecretEnc: "enc", Enable: false,
	}).Error; err != nil {
		t.Fatalf("seed disabled factor: %v", err)
	}
	// Enable 带 default:true 标签,创建时 false 会被回填为 true,显式更新为禁用
	if err := store.DB().Model(&MFAFactor{}).Where("id = ?", "f-p2").
		Update("enable", false).Error; err != nil {
		t.Fatalf("disable factor: %v", err)
	}
	if HasEnabledMFA("p2") {
		t.Fatal("disabled factor should not count")
	}
}

func TestVerifyMFACode(t *testing.T) {
	setupMFADB(t)
	SetPIIKeyFrom("mfa-pii-key")
	t.Cleanup(func() { piiKey = nil })

	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	enc, err := EncryptPII(secret)
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	if err := store.DB().Create(&MFAFactor{
		ID: "f-totp", PrincipalID: "p-totp", Type: "totp", SecretEnc: enc, Enable: true,
	}).Error; err != nil {
		t.Fatalf("seed factor: %v", err)
	}

	code := GenerateTOTPCode(secret)
	if !VerifyMFACode("p-totp", code) {
		t.Fatal("correct code should verify")
	}
	// 命中后 last_used_at 更新
	var factor MFAFactor
	store.DB().First(&factor, "principal_id = ?", "p-totp")
	if factor.LastUsedAt == 0 {
		t.Fatal("last_used_at should be updated on hit")
	}
	if VerifyMFACode("p-totp", "000000") && code != "000000" {
		t.Fatal("wrong code should not verify")
	}
	// 解密失败的因子跳过
	if err := store.DB().Create(&MFAFactor{
		ID: "f-badenc", PrincipalID: "p-badenc", Type: "totp", SecretEnc: "not-encrypted", Enable: true,
	}).Error; err != nil {
		t.Fatalf("seed bad factor: %v", err)
	}
	if VerifyMFACode("p-badenc", GenerateTOTPCode("another-secret")) {
		t.Fatal("undecryptable factor should be skipped")
	}
}
