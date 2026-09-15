package auth

import (
	"strings"
	"testing"
)

// TestCompareArgon2EdgeCases 补齐 CompareArgon2 的边界分支覆盖。
func TestCompareArgon2EdgeCases(t *testing.T) {
	hash, _ := EncryptedPasswordArgon2("testpass")

	// 正确密码
	if err := CompareArgon2(hash, "testpass"); err != nil {
		t.Fatalf("correct password: %v", err)
	}
	// 错误密码
	if err := CompareArgon2(hash, "wrongpass"); err == nil || err.Error() != "password mismatch" {
		t.Fatalf("expected mismatch error, got %v", err)
	}
	// 非法格式:少段
	if err := CompareArgon2("$argon2id$v=19", "x"); err == nil || !strings.Contains(err.Error(), "invalid argon2id format") {
		t.Fatalf("expected format error, got %v", err)
	}
	// 非法格式:错误算法标识
	badAlg := strings.Replace(hash, "argon2id", "bcrypt", 1)
	if err := CompareArgon2(badAlg, "x"); err == nil || !strings.Contains(err.Error(), "invalid argon2id format") {
		t.Fatalf("expected wrong alg error, got %v", err)
	}
	// 非法 base64 salt
	badSalt := "$argon2id$v=19$m=65536,t=3,p=2$!!!$!!!"
	if err := CompareArgon2(badSalt, "x"); err == nil {
		t.Fatal("expected base64 decode error for invalid salt")
	}
}

// TestIsArgon2AndScryptHash 补齐格式判断分支。
func TestIsArgon2AndScryptHash(t *testing.T) {
	cases := []struct {
		input    string
		isArgon2 bool
		isScrypt bool
	}{
		{"$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA", true, false},
		{"some-scrypt-hash", false, true},
		{"", false, false},
	}
	for _, tc := range cases {
		if IsArgon2Hash(tc.input) != tc.isArgon2 {
			t.Fatalf("IsArgon2Hash(%q) mismatch", tc.input)
		}
		if IsScryptHash(tc.input) != tc.isScrypt {
			t.Fatalf("IsScryptHash(%q) mismatch", tc.input)
		}
	}
}

// TestGenerateTOTPSecretUniqueness 补齐 TOTP 密钥唯一性验证。
func TestGenerateTOTPSecretUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		secret, err := GenerateTOTPSecret()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if seen[secret] {
			t.Fatalf("duplicate secret generated: %s", secret)
		}
		seen[secret] = true
	}
}

// TestGenerateTOTPURIMultiple 测试 URI 生成格式。
func TestGenerateTOTPURIMultiple(t *testing.T) {
	uri := GenerateTOTPURI("ABC234DEF", "bob@corp.io", "MyApp")
	for _, want := range []string{
		"otpauth://totp/MyApp:bob@corp.io",
		"secret=ABC234DEF",
		"issuer=MyApp",
	} {
		if !strings.Contains(uri, want) {
			t.Fatalf("uri missing %q: %s", want, uri)
		}
	}
}

// TestRequiredACRForActionAllActions 补齐所有 action 分支。
func TestRequiredACRForActionAllActions(t *testing.T) {
	level2Actions := []string{"delete_user", "reset_password", "change_password", "update_role"}
	for _, a := range level2Actions {
		if got := RequiredACRForAction(a); got != ACRLevel2 {
			t.Fatalf("action %q: expected %s, got %s", a, ACRLevel2, got)
		}
	}
	level3Actions := []string{"export_data", "delete_tenant"}
	for _, a := range level3Actions {
		if got := RequiredACRForAction(a); got != ACRLevel3 {
			t.Fatalf("action %q: expected %s, got %s", a, ACRLevel3, got)
		}
	}
	if got := RequiredACRForAction("unknown_action"); got != ACRLevel1 {
		t.Fatalf("unknown action: expected %s, got %s", ACRLevel1, got)
	}
}
