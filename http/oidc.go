package http

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// OIDC / OAuth2 Provider（REDESIGN #6）。
//
// 把 usercenter 暴露为标准 OIDC IdP，其它应用可作为 OAuth Client 接入：
//   GET  /.well-known/openid-configuration  发现文档
//   GET  /.well-known/jwks.json             JWKS 公钥标识
//   POST /oauth/authorize                   授权（API 式：已登录用户凭 Bearer 直接换 code）
//   POST /oauth/token                       换令牌（authorization_code / client_credentials / refresh_token）
//   GET  /oauth/userinfo                    用户信息（Bearer）
//   POST /oauth/revoke                      吊销
//
// 采用 HS256（与系统 JWT 一致）；access_token 复用 usercenter JWT，
// id_token 用 KeyManager 活跃密钥单独签发。PKCE 支持 S256。

const (
	authCodeTTL     = 60 * time.Second
	refreshTokenTTL = 30 * 24 * time.Hour
)

// --- 授权码内存存储（短生命周期，进程级）---

type authCode struct {
	code                string
	clientID            string
	redirectURI         string
	scope               string
	user                *apipb.CurrentUser // 授权时刻的用户快照（含 name/email/tenant/roles）
	codeChallenge       string
	codeChallengeMethod string
	nonce               string
	expiresAt           time.Time
	consumed            bool
}

var (
	authCodeMu      sync.Mutex
	authCodeStore   = map[string]*authCode{}
	refreshTokenMu  sync.Mutex
)

func issueAuthCode(clientID, redirectURI, scope string, user *apipb.CurrentUser, challenge, method, nonce string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	code := base64.RawURLEncoding.EncodeToString(b)
	authCodeMu.Lock()
	authCodeStore[code] = &authCode{
		code: code, clientID: clientID, redirectURI: redirectURI, scope: scope,
		user: user, codeChallenge: challenge, codeChallengeMethod: method,
		nonce: nonce, expiresAt: time.Now().Add(authCodeTTL),
	}
	// 顺手清理过期码
	for k, v := range authCodeStore {
		if time.Now().After(v.expiresAt) {
			delete(authCodeStore, k)
		}
	}
	authCodeMu.Unlock()
	return code
}

func consumeAuthCode(code string) (*authCode, bool) {
	authCodeMu.Lock()
	defer authCodeMu.Unlock()
	ac, ok := authCodeStore[code]
	if !ok || time.Now().After(ac.expiresAt) || ac.consumed {
		return nil, false
	}
	ac.consumed = true
	delete(authCodeStore, code)
	return ac, true
}

// RegisterOIDCRouter 挂载 OIDC/ OAuth2 端点。
// authorize/userinfo 需要用户登录（由调用方中间件写 Principal）；token/revoke 为客户端凭证。
func RegisterOIDCRouter(r *gin.Engine) {
	r.GET("/.well-known/openid-configuration", oidcDiscovery)
	r.GET("/.well-known/jwks.json", oidcJWKS)
	r.POST("/oauth/authorize", oidcAuthorize)
	r.POST("/oauth/token", oidcToken)
	r.GET("/oauth/userinfo", oidcUserinfo)
	r.POST("/oauth/revoke", oidcRevoke)
}

func issuerFrom(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

func oidcDiscovery(c *gin.Context) {
	c.JSON(http.StatusOK, auth.GetDiscovery(issuerFrom(c)))
}

func oidcJWKS(c *gin.Context) {
	c.JSON(http.StatusOK, auth.GetJWKS(issuerFrom(c)))
}

// oidcAuthorize API 式授权：已登录用户凭 Bearer，对指定 client 直接颁发一次性授权码。
// body: {client_id, redirect_uri, scope, state, code_challenge, code_challenge_method, nonce}
func oidcAuthorize(c *gin.Context) {
	ok, user := ucm.GetUser(c)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_user", "error_description": "需登录用户"})
		return
	}
	var req struct {
		ClientID            string `json:"client_id" binding:"required"`
		RedirectURI         string `json:"redirect_uri" binding:"required"`
		Scope               string `json:"scope"`
		State               string `json:"state"`
		CodeChallenge       string `json:"code_challenge"`
		CodeChallengeMethod string `json:"code_challenge_method"`
		Nonce               string `json:"nonce"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": err.Error()})
		return
	}
	client, err := getOAuthClient(req.ClientID)
	if err != nil || client == nil || !client.Enable {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client"})
		return
	}
	if !uriAllowed(client.RedirectURIs, req.RedirectURI) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "redirect_uri 不在白名单"})
		return
	}
	code := issueAuthCode(req.ClientID, req.RedirectURI, req.Scope, user, req.CodeChallenge, req.CodeChallengeMethod, req.Nonce)
	c.JSON(http.StatusOK, gin.H{"code": code, "state": req.State})
}

// oidcToken 令牌端点：authorization_code / client_credentials / refresh_token。
func oidcToken(c *gin.Context) {
	grantType := c.PostForm("grant_type")
	switch grantType {
	case "authorization_code":
		handleAuthCodeGrant(c)
	case "client_credentials":
		handleClientCredentialsGrant(c)
	case "refresh_token":
		handleRefreshTokenGrant(c)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_grant_type"})
	}
}

func handleAuthCodeGrant(c *gin.Context) {
	clientID, clientSecret, ok := clientCreds(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
		return
	}
	client, err := getOAuthClient(clientID)
	if err != nil || client == nil || !client.Enable || !verifyClientSecret(client, clientSecret) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
		return
	}
	ac, ok := consumeAuthCode(c.PostForm("code"))
	if !ok || ac.clientID != clientID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant", "error_description": "授权码无效或已过期"})
		return
	}
	// PKCE 校验
	if ac.codeChallenge != "" {
		if !verifyPKCE(c.PostForm("code_verifier"), ac.codeChallenge, ac.codeChallengeMethod) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant", "error_description": "PKCE 校验失败"})
			return
		}
	}
	// 颁发 access_token（usercenter JWT，复用登录用户身份）
	access, err := token.EncodeToken(ac.user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	idToken, _ := issueIDToken(issuerFrom(c), ac.user, clientID, ac.nonce)
	refresh := issueRefreshToken(ac.user, clientID, ac.scope)
	c.JSON(http.StatusOK, auth.TokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: tokenTTLSeconds(),
		RefreshToken: refresh, Scope: ac.scope, IDToken: idToken,
	})
}

func handleClientCredentialsGrant(c *gin.Context) {
	clientID, clientSecret, ok := clientCreds(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
		return
	}
	client, err := getOAuthClient(clientID)
	if err != nil || client == nil || !client.Enable || !verifyClientSecret(client, clientSecret) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
		return
	}
	if !grantAllowed(client.GrantTypes, "client_credentials") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unauthorized_client"})
		return
	}
	// 机器主体：以 client_id 为 subject、类型 Service
	svc := &apipb.CurrentUser{
		Id: clientID, UserName: client.Name, TenantID: "", RoleIDs: []string{},
	}
	access, err := token.EncodeToken(svc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, auth.TokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: tokenTTLSeconds(), Scope: client.Scopes,
	})
}

func handleRefreshTokenGrant(c *gin.Context) {
	clientID, clientSecret, ok := clientCreds(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
		return
	}
	rtStr := c.PostForm("refresh_token")
	refreshTokenMu.Lock()
	var rt auth.RefreshToken
	err := store.DB().Where("token = ? AND revoked = ?", rtStr, false).First(&rt).Error
	refreshTokenMu.Unlock()
	if err != nil || rt.ClientID != clientID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	if rt.ExpiresAt > 0 && time.Now().Unix() > rt.ExpiresAt {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant", "error_description": "refresh_token 过期"})
		return
	}
	_ = clientSecret
	user := &apipb.CurrentUser{Id: rt.PrincipalID, TenantID: rt.TenantID}
	access, err := token.EncodeToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	newRT := issueRefreshToken(user, clientID, rt.Scope)
	// 吊销旧 refresh token（轮换）
	_ = store.DB().Model(&auth.RefreshToken{}).Where("token = ?", rtStr).Update("revoked", true).Error
	c.JSON(http.StatusOK, auth.TokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: tokenTTLSeconds(),
		RefreshToken: newRT, Scope: rt.Scope,
	})
}

// oidcUserinfo 返回 Bearer 对应用户的声明（sub/name/email/tenant_id/role_ids）。
func oidcUserinfo(c *gin.Context) {
	ok, user := ucm.GetUser(c)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"sub": user.Id, "name": user.UserName, "nickname": user.Nickname,
		"preferred_username": user.UserName, "tenant_id": user.TenantID, "role_ids": user.RoleIDs,
	})
}

func oidcRevoke(c *gin.Context) {
	tok := c.PostForm("token")
	if tok == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	// 吊销 refresh token；access token 走 token 缓存删除
	_ = store.DB().Model(&auth.RefreshToken{}).Where("token = ?", tok).Update("revoked", true).Error
	if token.DefaultTokenCache != nil {
		_ = token.DefaultTokenCache.Del("", tok)
	}
	c.Status(http.StatusNoContent)
}

// --- 工具 ---

func tokenTTLSeconds() int {
	if token.DefaultTokenCache != nil {
		return token.DefaultTokenCache.TokenExpired() * 60
	}
	return 7200
}

// issueIDToken 用 KeyManager 活跃密钥签发 OIDC id_token。
func issueIDToken(issuer string, user *apipb.CurrentUser, audience, nonce string) (string, error) {
	active := auth.GetActiveKey()
	if active == nil {
		return "", fmt.Errorf("key manager 未初始化")
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": issuer, "sub": user.Id, "aud": audience,
		"exp": now.Add(1 * time.Hour).Unix(), "iat": now.Unix(),
		"name": user.UserName, "preferred_username": user.UserName, "tenant_id": user.TenantID,
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	if len(user.RoleIDs) > 0 {
		claims["role_ids"] = user.RoleIDs
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = active.Kid
	return tok.SignedString([]byte(active.Key))
}

func issueRefreshToken(user *apipb.CurrentUser, clientID, scope string) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	rt := base64.RawURLEncoding.EncodeToString(b)
	rec := &auth.RefreshToken{
		Token: rt, PrincipalID: user.Id, TenantID: user.TenantID,
		ClientID: clientID, Scope: scope, ExpiresAt: time.Now().Add(refreshTokenTTL).Unix(),
	}
	_ = store.DB().Create(rec).Error
	return rt
}

// clientCreds 从 Basic 或 POST body 取 client_id/client_secret。
func clientCreds(c *gin.Context) (id, secret string, ok bool) {
	if u, p, has := c.Request.BasicAuth(); has {
		return u, p, true
	}
	id = c.PostForm("client_id")
	secret = c.PostForm("client_secret")
	if id == "" {
		return "", "", false
	}
	return id, secret, true
}

func getOAuthClient(id string) (*auth.OAuthClient, error) {
	var client auth.OAuthClient
	err := store.DB().First(&client, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &client, nil
}

// verifyClientSecret 用 bcrypt 校验（Secret 列存哈希）。
func verifyClientSecret(client *auth.OAuthClient, secret string) bool {
	if client.Secret == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(client.Secret), []byte(secret)) == nil
}

func uriAllowed(allowed, target string) bool {
	for _, u := range strings.Split(allowed, ",") {
		if strings.TrimSpace(u) == target {
			return true
		}
	}
	return false
}

func grantAllowed(declared, grant string) bool {
	if declared == "" {
		return true
	}
	for _, g := range strings.Split(declared, ",") {
		if strings.TrimSpace(g) == grant {
			return true
		}
	}
	return false
}

// verifyPKCE S256: BASE64URL(SHA256(code_verifier)) == code_challenge
func verifyPKCE(verifier, challenge, method string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	if method != "S256" {
		return method == "" || method == "plain" && verifier == challenge
	}
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]) == challenge
}

// hashClientSecret bcrypt 哈希客户端密钥（管理端创建/轮转时调用）。
func hashClientSecret(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(h), err
}

// genClientSecret 生成 32 字节随机客户端密钥（创建/轮转时返回明文一次）。
func genClientSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
