package auth

import (
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	webauthn "github.com/go-webauthn/webauthn/webauthn"
)

func TestWebAuthnUserInterface(t *testing.T) {
	u := &WebAuthnUser{ID: []byte("user-id-1"), Name: "alice",
		DisplayName: "Alice", Credentials: []webauthn.Credential{{ID: []byte("cred")}}}
	if string(u.WebAuthnID()) != "user-id-1" {
		t.Fatalf("WebAuthnID mismatch: %q", u.WebAuthnID())
	}
	if u.WebAuthnName() != "alice" || u.WebAuthnDisplayName() != "Alice" {
		t.Fatalf("name mismatch: %+v", u)
	}
	if len(u.WebAuthnCredentials()) != 1 {
		t.Fatalf("credentials mismatch: %+v", u.WebAuthnCredentials())
	}
	if u.WebAuthnIcon() != "" {
		t.Fatal("icon should be empty")
	}
}

func TestInitWebAuthn(t *testing.T) {
	t.Cleanup(func() { webAuthnInstance = nil })

	// nil / 空 RPID:不启用且不报错
	if err := InitWebAuthn(nil); err != nil {
		t.Fatalf("nil config: %v", err)
	}
	if IsWebAuthnEnabled() {
		t.Fatal("should be disabled after nil config")
	}
	if err := InitWebAuthn(&WebAuthnConfig{RPID: ""}); err != nil {
		t.Fatalf("empty rpid: %v", err)
	}
	if IsWebAuthnEnabled() {
		t.Fatal("should be disabled after empty rpid")
	}
	// 正常配置
	if err := InitWebAuthn(&WebAuthnConfig{
		RPID: "example.com", RPDisplayName: "UserCenter",
		RPOrigins: []string{"https://example.com"},
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if !IsWebAuthnEnabled() {
		t.Fatal("expected webauthn enabled")
	}
}

func TestWebAuthnDisabledGuards(t *testing.T) {
	t.Cleanup(func() { webAuthnInstance = nil })
	webAuthnInstance = nil

	if _, _, err := BeginRegistration("u", "n", "d", nil); err == nil ||
		!strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("BeginRegistration guard: %v", err)
	}
	if _, err := FinishRegistration("u", "n", "d", nil, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "") && !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("FinishRegistration guard: %v", err)
	}
	if _, _, err := BeginLogin("u", "n", "d", nil); err == nil {
		t.Fatal("BeginLogin guard missing")
	}
	if _, err := FinishLogin("u", nil, nil, nil); err == nil {
		t.Fatal("FinishLogin guard missing")
	}
}

func TestSerializeDeserializeCredential(t *testing.T) {
	cred := &webauthn.Credential{
		ID:              []byte("cred-id-bytes"),
		PublicKey:       []byte("pub-key-bytes"),
		AttestationType: "none",
		Authenticator:   webauthn.Authenticator{SignCount: 7},
		Transport:       []protocol.AuthenticatorTransport{protocol.Internal, "usb"},
	}
	enc, err := SerializeCredential(cred)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	// JSON 自描述字段
	for _, want := range []string{`"signCount":7`, `"attestationType":"none"`, `"transport":["internal","usb"]`} {
		if !strings.Contains(enc, want) {
			t.Fatalf("serialized missing %q: %s", want, enc)
		}
	}
	// 反序列化往返
	creds, err := DeserializeCredentials([]string{enc})
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	got := creds[0]
	if string(got.ID) != "cred-id-bytes" || string(got.PublicKey) != "pub-key-bytes" ||
		got.AttestationType != "none" || got.Authenticator.SignCount != 7 ||
		len(got.Transport) != 2 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	// 非法 JSON 项被跳过
	creds, err = DeserializeCredentials([]string{"{bad-json", enc})
	if err != nil {
		t.Fatalf("deserialize with bad item: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("bad item should be skipped, got %d", len(creds))
	}
	// 空列表
	if creds, err = DeserializeCredentials(nil); err != nil || creds != nil {
		t.Fatalf("empty list: %+v err=%v", creds, err)
	}
}
