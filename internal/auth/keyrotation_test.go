package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

// genPEM 生成 RSA 私钥的 PKCS1 PEM(供密钥管理测试)。
func genPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
}

// --- Argon2id 密码哈希 ---

func TestEncryptedPasswordArgon2Format(t *testing.T) {
	hash, err := EncryptedPasswordArgon2("s3cret-密码")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !IsArgon2Hash(hash) {
		t.Fatalf("expected argon2id prefix, got %q", hash)
	}
	if IsScryptHash(hash) {
		t.Fatal("argon2 hash should not be classified as scrypt")
	}
	// 相同密码校验通过
	if err := CompareArgon2(hash, "s3cret-密码"); err != nil {
		t.Fatalf("compare correct password: %v", err)
	}
	// 错误密码拒绝
	if err := CompareArgon2(hash, "wrong"); err == nil || err.Error() != "password mismatch" {
		t.Fatalf("expected password mismatch, got %v", err)
	}
	// 同密码两次加密盐不同,哈希不同
	hash2, _ := EncryptedPasswordArgon2("s3cret-密码")
	if hash2 == hash {
		t.Fatal("expected random salt to produce different hashes")
	}
}

func TestCompareArgon2InvalidFormat(t *testing.T) {
	for _, bad := range []string{"", "plain", "$argon2id$bad", "$bcrypt$v=19$a$b$c$d"} {
		if err := CompareArgon2(bad, "x"); err == nil {
			t.Fatalf("expected invalid format error for %q", bad)
		}
	}
	// 非 argon2 的非空哈希归类为 scrypt(需 rehash)
	if !IsScryptHash("some-scrypt-legacy") {
		t.Fatal("legacy hash should be classified as scrypt")
	}
	if IsScryptHash("") {
		t.Fatal("empty hash should not be scrypt")
	}
}

// --- RSA 密钥管理与轮换 ---

func TestKeyManagerInitAutoGenerate(t *testing.T) {
	ResetKeys()
	kid, err := InitKeyManager("") // 空串自动生成
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if kid == "" {
		t.Fatal("expected non-empty kid")
	}
	active := GetActiveKey()
	if active == nil || active.Kid != kid || !active.IsActive {
		t.Fatalf("unexpected active key: %+v", active)
	}
	if GetKeyByID(kid) == nil {
		t.Fatal("GetKeyByID should return active key")
	}
	if GetKeyByID("missing") != nil {
		t.Fatal("unknown kid should return nil")
	}
	t.Cleanup(ResetKeys)
}

func TestKeyManagerRotateAndGracePeriod(t *testing.T) {
	ResetKeys()
	SetGracePeriod(24 * 365 * time.Hour) // 实际不会过期
	oldKid, err := InitKeyManager("")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	newKid, err := RotateKey(genPEM(t))
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if newKid == oldKid {
		t.Fatal("rotate should produce a new kid")
	}
	if GetActiveKey().Kid != newKid {
		t.Fatalf("active kid = %s, want %s", GetActiveKey().Kid, newKid)
	}
	// 旧密钥降级但仍在(grace period 内可验签)
	old := GetKeyByID(oldKid)
	if old == nil || old.IsActive {
		t.Fatalf("old key should remain but inactive: %+v", old)
	}
	// JWKS 含新旧两把
	jwks := GetJWKS()
	if len(jwks) != 2 {
		t.Fatalf("expected 2 JWKS entries, got %d", len(jwks))
	}
	jwksMap := GetJWKSMap()
	if _, ok := jwksMap["keys"]; !ok {
		t.Fatal("JWKS map should contain keys")
	}
	t.Cleanup(ResetKeys)
}

func TestKeyManagerRotatePrunesBeyondGrace(t *testing.T) {
	ResetKeys()
	SetGracePeriod(-time.Second) // 立即过期
	if _, err := InitKeyManager(""); err != nil {
		t.Fatalf("init: %v", err)
	}
	firstKid := GetActiveKey().Kid
	if _, err := RotateKey(genPEM(t)); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if GetKeyByID(firstKid) != nil {
		t.Fatal("expired old key should be pruned after rotate")
	}
	if _, err := RotateKey(genPEM(t)); err != nil {
		t.Fatalf("rotate again: %v", err)
	}
	if _, err := RotateKey(genPEM(t)); err != nil {
		t.Fatalf("rotate third: %v", err)
	}
	if GetActiveKey() == nil {
		t.Fatal("active key should exist after rotations")
	}
	t.Cleanup(ResetKeys)
}

func TestParseRSAPrivatePEM(t *testing.T) {
	valid := genPEM(t)
	if _, err := parseRSAPrivatePEM(valid); err != nil {
		t.Fatalf("valid pem: %v", err)
	}
	// PKCS8 路径
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	der8, _ := x509.MarshalPKCS8PrivateKey(key)
	pem8 := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der8}))
	if _, err := parseRSAPrivatePEM(pem8); err != nil {
		t.Fatalf("pkcs8 pem: %v", err)
	}
	// 裸 base64 DER 路径
	if _, err := parseRSAPrivatePEM("bm90LWFuLWtleQ=="); err == nil {
		t.Fatal("expected invalid der error")
	}
	// 空串自动生成
	if _, err := parseRSAPrivatePEM(""); err != nil {
		t.Fatalf("empty should auto generate: %v", err)
	}
}
