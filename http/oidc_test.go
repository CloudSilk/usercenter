package http

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/CloudSilk/usercenter/internal/auth"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/golang-jwt/jwt/v5"
)

func TestVerifyPKCE_S256(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if !verifyPKCE(verifier, challenge, "S256") {
		t.Fatal("S256 校验应通过")
	}
	if verifyPKCE(verifier+"tampered", challenge, "S256") {
		t.Fatal("篡改 verifier 后应失败")
	}
	if verifyPKCE("", challenge, "S256") {
		t.Fatal("空 verifier 应失败")
	}
	// S5:plain method 必须拒绝
	if verifyPKCE(verifier, challenge, "plain") {
		t.Fatal("plain method 应被拒绝(强制 S256)")
	}
	if verifyPKCE(verifier, challenge, "") {
		t.Fatal("空 method 应被拒绝(强制 S256)")
	}
}

func TestClientSecret_BcryptRoundTrip(t *testing.T) {
	plain := "super-secret-123"
	hashed, err := hashClientSecret(plain)
	if err != nil {
		t.Fatal(err)
	}
	client := &auth.OAuthClient{Secret: hashed}
	if !verifyClientSecret(client, plain) {
		t.Fatal("正确密钥应校验通过")
	}
	if verifyClientSecret(client, "wrong") {
		t.Fatal("错误密钥应失败")
	}
}

func TestIssueIDToken_SignsWithActiveKey(t *testing.T) {
	_, err := auth.InitKeyManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer auth.ResetKeys()

	user := &apipb.CurrentUser{Id: "u1", UserName: "alice", TenantID: "t1", RoleIDs: []string{"r1"}}
	idTok, err := issueIDToken("http://localhost:48180", user, "client-x", "nonce-abc")
	if err != nil {
		t.Fatal(err)
	}

	// 用活跃密钥的公钥验签并校验声明
	active := auth.GetActiveKey()
	parsed, err := jwt.Parse(idTok, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodRSA); !ok {
			t.Fatalf("unexpected method: %v (expected RS256)", tok.Header["alg"])
		}
		return active.Public, nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("id_token 验签失败: %v", err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != "u1" || claims["aud"] != "client-x" || claims["nonce"] != "nonce-abc" {
		t.Fatalf("声明不正确: %v", claims)
	}
	if parsed.Header["kid"] != active.Kid {
		t.Fatalf("kid 应在 header: got %v want %v", parsed.Header["kid"], active.Kid)
	}
}

func TestJWKS_ExposesActiveKey(t *testing.T) {
	_, err := auth.InitKeyManager("")
	if err != nil {
		t.Fatal(err)
	}
	defer auth.ResetKeys()

	jwks := auth.GetJWKS()
	if len(jwks) == 0 {
		t.Fatal("JWKS 应至少暴露活跃密钥")
	}
	key := jwks[0]
	if key.Kid == "" || key.Alg != "RS256" {
		t.Fatalf("JWKS key 字段异常: %+v", key)
	}
	if key.Kty != "RSA" || key.N == "" || key.E == "" {
		t.Fatalf("JWKS RSA 公钥 n/e 不应为空: %+v", key)
	}
	if !strings.HasPrefix(key.N, "A") && len(key.N) < 100 {
		t.Fatalf("JWKS n 应包含标准 RSA 公钥编码(长度不足): %s", key.N)
	}
}

func TestURIAllowed(t *testing.T) {
	if !uriAllowed("https://a.com/cb,https://b.com/cb", "https://a.com/cb") {
		t.Fatal("白名单内应通过")
	}
	if uriAllowed("https://a.com/cb", "https://evil.com/cb") {
		t.Fatal("白名单外应拒绝")
	}
}
