package auth

import (
	"testing"
)

func TestGetDiscovery(t *testing.T) {
	issuer := "https://id.example.com"
	d := GetDiscovery(issuer)
	if d.Issuer != issuer {
		t.Fatalf("issuer mismatch: %q", d.Issuer)
	}
	for _, want := range []string{
		issuer + "/oauth/authorize",
		issuer + "/oauth/token",
		issuer + "/oauth/userinfo",
		issuer + "/oauth/revoke",
		issuer + "/.well-known/jwks.json",
	} {
		found := false
		for _, s := range []string{
			d.AuthorizationEndpoint, d.TokenEndpoint, d.UserInfoEndpoint,
			d.RevocationEndpoint, d.JWKSURI,
		} {
			if s == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("discovery missing endpoint %q", want)
		}
	}
	if len(d.ResponseTypes) != 3 || len(d.Scopes) != 5 || len(d.Claims) != 9 {
		t.Fatalf("unexpected discovery arrays: %+v", d)
	}
	// RS256 必须在支持算法中(签名密钥经 JWKS 暴露)
	algOK := false
	for _, a := range d.IDTokenSigningAlgs {
		if a == "RS256" {
			algOK = true
		}
	}
	if !algOK {
		t.Fatal("RS256 should be supported")
	}
}

func TestOIDCTableNames(t *testing.T) {
	if (RefreshToken{}).TableName() != "refresh_token" {
		t.Fatal("refresh_token table name mismatch")
	}
	if (OAuthClient{}).TableName() != "oauth_client" {
		t.Fatal("oauth_client table name mismatch")
	}
	if (ConsentRecord{}).TableName() != "oauth_consent" {
		t.Fatal("oauth_consent table name mismatch")
	}
}

func TestKidFromPublicDeterministic(t *testing.T) {
	// kidFromPublic 在 SPKI 编码失败时走 (n,e) 哈希退化路径——
	// 正常公钥走 SPKI 路径;两条路径对同一密钥应产生一致 kid
	key, err := parseRSAPrivatePEM(genPEM(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	k1 := kidFromPublic(&key.PublicKey)
	k2 := kidFromPublic(&key.PublicKey)
	if k1 == "" || k1 != k2 {
		t.Fatalf("kid should be deterministic: %q vs %q", k1, k2)
	}
	if len(k1) != 32 { // 16 字节 hex
		t.Fatalf("kid length = %d, want 32 hex chars", len(k1))
	}
	// 不同密钥 kid 不同
	key2, _ := parseRSAPrivatePEM(genPEM(t))
	if kidFromPublic(&key2.PublicKey) == k1 {
		t.Fatal("different keys should have different kids")
	}
}

func TestEncodeHex(t *testing.T) {
	// 65537 = 0x010001 → 大端字节 0x01 0x00 0x01 → hex "010001"
	if got := encodeHex(bigEndBytes(65537)); got != "010001" {
		t.Fatalf("encodeHex(65537) = %q", got)
	}
}

func TestParseRSADERErrors(t *testing.T) {
	if _, err := parseRSADER([]byte("junk")); err == nil {
		t.Fatal("junk der should fail")
	}
	// PKCS8 非 RSA 密钥(EC)会被拒绝
	// 构造 PKCS8 包装的 ed25519 私钥超出演示范围;此处仅验证 junk 路径
}
