package auth

import (
	scrypt "github.com/elithrar/simple-scrypt"
	"strings"
	"testing"
)

func TestValidPasswdStrength(t *testing.T) {
	valid := []string{
		"Abcdefg1",     // 8 位恰好
		"Passw0rdLong", // 长密码
		"中Aa1密码强度测试",    // rune 计数(中文按 1 字符)
	}
	for _, p := range valid {
		if !ValidPasswdStrength(p) {
			t.Fatalf("%q should be valid", p)
		}
	}
	invalid := []string{
		"",         // 空
		"Aa1",      // 长度不足
		"abcdefg1", // 无大写
		"ABCDEFG1", // 无小写
		"abcdefga", // 无数字
		"12345678", // 纯数字
	}
	for _, p := range invalid {
		if ValidPasswdStrength(p) {
			t.Fatalf("%q should be invalid", p)
		}
	}
}

func TestEncryptedPasswordRoundTrip(t *testing.T) {
	hash, err := EncryptedPassword("s3cret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// scrypt 自描述格式($scrypt$...),可用库校验
	if err := scrypt.CompareHashAndPassword([]byte(hash), []byte("s3cret")); err != nil {
		t.Fatalf("correct password should verify: %v", err)
	}
	if err := scrypt.CompareHashAndPassword([]byte(hash), []byte("wrong")); err == nil {
		t.Fatal("wrong password should fail")
	}
}

func TestGeneratePasswd(t *testing.T) {
	// 长度正确
	for _, length := range []int{1, 8, 16, 64} {
		if got := GeneratePasswd(length, PwdStrengthMix); len(got) != length {
			t.Fatalf("expected length %d, got %d (%q)", length, len(got), got)
		}
	}
	// 字符集约束:纯数字模式不含字母
	nums := GeneratePasswd(32, PwdStrengthOnliyNumber)
	if strings.ContainsAny(nums, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Fatalf("number mode leaked letters: %q", nums)
	}
	// 带特殊字符模式可能包含特殊字符(统计意义上);至少全部字符合法
	spec := GeneratePasswd(64, PwdStrengthAdvance)
	legal := NUmStr + CharStr + SpecStr
	for _, ch := range spec {
		if !strings.ContainsRune(legal, ch) {
			t.Fatalf("illegal character %q in advanced password", ch)
		}
	}
	// 随机性:两次生成不应相同(64 位长度碰撞概率可忽略)
	if GeneratePasswd(64, PwdStrengthMix) == GeneratePasswd(64, PwdStrengthMix) {
		t.Fatal("two random passwords should differ")
	}
}
