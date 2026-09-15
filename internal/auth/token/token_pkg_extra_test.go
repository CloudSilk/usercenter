package token

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/CloudSilk/usercenter/internal/principal"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/golang-jwt/jwt/v5"
)

// newTestUser 构造全字段的 CurrentUser。
func newTestUser(id string) *apipb.CurrentUser {
	return &apipb.CurrentUser{
		Id:         id,
		UserName:   "alice",
		Domain:     "example.com",
		DeviceType: 2,
		ClientIP:   "10.0.0.1",
		SessionID:  "sess-123",
		TenantID:   "tenant-1",
		Key:        "k1",
		RoleIDs:    []string{"r1", "r2"},
		Type:       1,
		Group:      "g1",
		Nickname:   "Alice",
		Avatar:     "https://a/i.png",
		IsVip:      true,
	}
}

func setupMemoryCache(t *testing.T) {
	t.Helper()
	InitTokenCache("pkg-extra-secret", "", "", "", 120)
}

func TestEncodeDecodeTokenRoundTrip(t *testing.T) {
	setupMemoryCache(t)
	user := newTestUser("user-1")

	token, err := EncodeToken(user)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeToken(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Id != user.Id || got.UserName != "alice" || got.TenantID != "tenant-1" ||
		got.SessionID != "sess-123" || got.ClientIP != "10.0.0.1" ||
		got.DeviceType != 2 || got.Type != 1 || got.Group != "g1" ||
		got.Nickname != "Alice" || got.Avatar != "https://a/i.png" || !got.IsVip {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if len(got.RoleIDs) != 2 || got.RoleIDs[0] != "r1" {
		t.Fatalf("role ids mismatch: %v", got.RoleIDs)
	}

	// 便捷访问器
	uid, err := GetUserID(token)
	if err != nil || uid != "user-1" {
		t.Fatalf("GetUserID: %q err=%v", uid, err)
	}
	sid, err := GetSessionID(token)
	if err != nil || sid != "sess-123" {
		t.Fatalf("GetSessionID: %q err=%v", sid, err)
	}
	if sig := GetTokenSignature(token); sig == "" || strings.Contains(sig, ".") {
		t.Fatalf("unexpected signature segment: %q", sig)
	}
	// 非法结构
	if sid, err := GetSessionID("not-a-jwt"); err != nil || sid != "" {
		t.Fatalf("malformed session id: %q err=%v", sid, err)
	}
	if uid, err := GetUserID("not-a-jwt"); err != nil || uid != "<nil>" {
		t.Fatalf("malformed GetUserID: %q err=%v", uid, err)
	}
}

func TestDecodeTokenErrors(t *testing.T) {
	setupMemoryCache(t)
	if _, err := DecodeToken(""); err == nil {
		t.Fatal("empty token should fail")
	}
	// 篡改签名
	user := newTestUser("u2")
	token, err := EncodeToken(user)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	parts := strings.Split(token, ".")
	tampered := parts[0] + "." + parts[1] + ".ZmFrZQ"
	if _, err := DecodeToken(tampered); err == nil {
		t.Fatal("tampered signature should fail")
	}
	// 过期 token:返回用户 + ErrTokenExpired
	expired := jwt.New(jwt.SigningMethodHS256)
	claims := jwt.MapClaims{"id": "u3", "exp": 1, "iat": 1}
	expired.Claims = claims
	expiredToken, err := expired.SignedString([]byte(secretKey))
	if err != nil {
		t.Fatalf("sign expired: %v", err)
	}
	got, err := DecodeToken(expiredToken)
	if got == nil || !errors.Is(err, jwt.ErrTokenExpired) {
		t.Fatalf("expected expired token to yield user+ErrTokenExpired, got %+v err=%v", got, err)
	}
	if got.Id != "u3" {
		t.Fatalf("expired token user mismatch: %+v", got)
	}
}

func TestRotateToken(t *testing.T) {
	setupMemoryCache(t)
	user := newTestUser("u-rotate")
	oldToken, err := EncodeToken(user)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	newToken, err := RotateToken(user, oldToken)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if newToken == oldToken {
		t.Fatal("rotation should produce a new token")
	}
	// 旧 token 已被替换,新 token 存在
	if ok, _ := DefaultTokenCache.Exists(fmt.Sprint(user.Id), oldToken); ok {
		t.Fatal("old token should no longer exist")
	}
	if ok, _ := DefaultTokenCache.Exists(fmt.Sprint(user.Id), newToken); !ok {
		t.Fatal("new token should exist")
	}
	// 连续轮换:每次签发 jti 不同,均为合法轮换且旧 token 立即失效
	third, err := RotateToken(user, newToken)
	if err != nil {
		t.Fatalf("second rotation: %v", err)
	}
	if ok, _ := DefaultTokenCache.Exists(fmt.Sprint(user.Id), newToken); ok {
		t.Fatal("rotated-out token should be replaced")
	}
	if ok, _ := DefaultTokenCache.Exists(fmt.Sprint(user.Id), third); !ok {
		t.Fatal("third token should exist")
	}
	// 未知旧 token 的轮换
	if _, err := RotateToken(user, "a.b.c"); err == nil {
		t.Fatal("rotating unknown old token should fail")
	}
}

func TestEncodeTokenFromPrincipal(t *testing.T) {
	setupMemoryCache(t)
	if _, err := EncodeTokenFromPrincipal(nil); err == nil {
		t.Fatal("nil principal should fail")
	}
	p := principal.NewHuman("u-p", "tenant-p", []string{"r1"})
	token, err := EncodeTokenFromPrincipal(p)
	if err != nil {
		t.Fatalf("encode from principal: %v", err)
	}
	got, err := DecodeToken(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Id != "u-p" || got.TenantID != "tenant-p" || got.Type != 0 {
		t.Fatalf("principal token mismatch: %+v", got)
	}
}

func TestMemoryCachePrimitiveOps(t *testing.T) {
	setupMemoryCache(t)
	cache := DefaultTokenCache

	// Del 的实现会对 payload 段做 base64+JSON 解析(提取 sessionID),
	// 因此必须使用真实签发的 token 而非任意三段式字符串
	user := newTestUser("u9")
	tok, err := EncodeToken(user)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if ok, _ := cache.Exists("u9", tok); !ok {
		t.Fatal("token should exist")
	}
	// 会话密钥材料存取删
	if err := cache.StorePrivateKey("sess-9", "priv"); err != nil {
		t.Fatalf("store priv: %v", err)
	}
	if v, ok := cache.GetPrivateKey("sess-9"); !ok || v != "priv" {
		t.Fatalf("get priv: %q %v", v, ok)
	}
	if err := cache.StorePublicKey("sess-9", "pub"); err != nil {
		t.Fatalf("store pub: %v", err)
	}
	if v, ok := cache.GetPublicKey("sess-9"); !ok || v != "pub" {
		t.Fatalf("get pub: %q %v", v, ok)
	}
	if err := cache.DelPrivateKey("sess-9"); err != nil {
		t.Fatalf("del priv: %v", err)
	}
	if err := cache.DelPublicKey("sess-9"); err != nil {
		t.Fatalf("del pub: %v", err)
	}
	if _, ok := cache.GetPrivateKey("sess-9"); ok {
		t.Fatal("private key should be deleted")
	}

	// 单 token 删除
	if err := cache.Del("u9", tok); err != nil {
		t.Fatalf("del: %v", err)
	}
	if ok, _ := cache.Exists("u9", tok); ok {
		t.Fatal("token should be deleted")
	}

	// DelByUserID 清空该用户全部 token(逐个签发,签名段各不相同)
	var tokens []string
	for i := 0; i < 3; i++ {
		u := newTestUser("u10")
		tk, err := EncodeToken(u)
		if err != nil {
			t.Fatalf("encode %d: %v", i, err)
		}
		tokens = append(tokens, tk)
	}
	if err := cache.DelByUserID("u10"); err != nil {
		t.Fatalf("del by user: %v", err)
	}
	for _, tk := range tokens {
		if ok, _ := cache.Exists("u10", tk); ok {
			t.Fatal("token should be removed by user")
		}
	}
}
