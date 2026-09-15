package auth

import (
	"strings"
	"testing"
)

func TestPIIKeyFromInput(t *testing.T) {
	if SetPIIKeyFrom("") {
		t.Fatal("empty input should return false")
	}
	if !SetPIIKeyFrom("master-secret") {
		t.Fatal("non-empty input should return true")
	}
	t.Cleanup(func() { piiKey = nil })
}

func TestEncryptDecryptPIIRoundTrip(t *testing.T) {
	SetPIIKeyFrom("master-secret")
	t.Cleanup(func() { piiKey = nil })

	enc, err := EncryptPII("13800138000")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// 同明文两次加密密文不同(GCM 随机 nonce)
	enc2, _ := EncryptPII("13800138000")
	if enc == enc2 {
		t.Fatal("expected different ciphertexts (random nonce)")
	}
	dec, err := DecryptPII(enc)
	if err != nil || dec != "13800138000" {
		t.Fatalf("round trip: %q err=%v", dec, err)
	}
}

func TestEncryptDecryptPIIWithoutKey(t *testing.T) {
	piiKey = nil
	if _, err := EncryptPII("x"); err == nil || !strings.Contains(err.Error(), "key not set") {
		t.Fatalf("expected key-not-set error, got %v", err)
	}
	if _, err := DecryptPII("x"); err == nil {
		t.Fatal("expected decrypt without key to fail")
	}
}

func TestDecryptPIITampered(t *testing.T) {
	SetPIIKeyFrom("master-secret")
	t.Cleanup(func() { piiKey = nil })
	enc, _ := EncryptPII("secret")
	tampered := enc[:len(enc)-2] + "XX"
	if _, err := DecryptPII(tampered); err == nil {
		t.Fatal("tampered ciphertext should fail to decrypt")
	}
	if _, err := DecryptPII("short"); err == nil {
		t.Fatal("short ciphertext should fail")
	}
}

func TestMaskPII(t *testing.T) {
	cases := []struct {
		in, typ, want string
	}{
		{"13800138000", "mobile", "138****8000"},
		{"ab@example.com", "email", "ab***@example.com"},
		{"110101199001011234", "idCard", "110101********1234"},
		{"12345", "mobile", "12***45"},         // 过短按通用规则截断
		{"a@b.com", "email", "a@***om"},        // @ 前不足 2 字符不脱敏
		{"whatever", "unknownType", "wh***er"}, // 未知类型走通用截断
	}
	for _, tc := range cases {
		if got := MaskPII(tc.in, tc.typ); got != tc.want {
			t.Fatalf("MaskPII(%q,%q) = %q, want %q", tc.in, tc.typ, got, tc.want)
		}
	}
}
