// Package bootstrap encapsulates the startup initialization sequence shared by
// main.go (production) and cmd/devserver (development).
//
// Each entry point provides its own config source (Nacos vs env vars) and auth
// middleware, but the init sequence is identical:
//
//	DB → Keys → Constants → Seed → Alert/Social → Routers → SPA → Health → Shutdown
//
// This package exposes composable building blocks; each entry point calls them in
// the correct order with its own parameter values.
package bootstrap

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/alert"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/model"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/web"
	"github.com/gin-gonic/gin"
)

// Keys collects the three independent cryptographic key slots.
// Production fills this from Nacos; devserver from env vars / generated values.
type Keys struct {
	// TokenKey is the HS256 JWT access_token signing secret.
	TokenKey string
	// TokenRedisAddr when non-empty routes token cache through Redis (cluster deployments).
	TokenRedisAddr string
	TokenRedisName string
	TokenRedisPwd  string
	// TokenExpired is the access_token lifetime in minutes.
	TokenExpired int
	// APIKeyEncKey is an independent AES-GCM key for AI provider API keys;
	// left empty, it is derived from TokenKey via SHA-256.
	APIKeyEncKey string
	// PIIEncKey is an independent AES-GCM key for PII field encryption;
	// left empty, it is derived from TokenKey via SHA-256.
	PIIEncKey string
	// OIDCSigningKey is a PEM-encoded RSA private key for id_token RS256 signing;
	// left empty, an ephemeral 2048-bit key is generated at startup.
	OIDCSigningKey string
}

// InitKeys initialises the three independent key slots + the OIDC RSA key manager.
// Call this exactly once per process after the DB is ready.
func InitKeys(k Keys) {
	// 1. JWT access_token signing cache (HS256, internal use)
	token.InitTokenCache(k.TokenKey, k.TokenRedisAddr, k.TokenRedisName, k.TokenRedisPwd, k.TokenExpired)

	// 2. AI Key encryption key: prefer explicit config, fall back to TokenKey.
	if !apikey.SetEncryptionKeyFrom(k.APIKeyEncKey) {
		apikey.SetEncryptionKeyFrom(k.TokenKey)
	}

	// 3. PII encryption key: prefer explicit config, fall back to TokenKey.
	if !auth.SetPIIKeyFrom(k.PIIEncKey) {
		auth.SetPIIKeyFrom(k.TokenKey)
	}

	// 4. OIDC id_token RSA key (RS256 + JWKS).
	if kid, err := auth.InitKeyManager(k.OIDCSigningKey); err != nil {
		// PEM parse failed or empty → auto-generate RSA-2048.
		if kid2, err2 := auth.InitKeyManager(""); err2 != nil {
			panic(fmt.Sprintf("RSA 密钥初始化失败: %v", err2))
		} else {
			fmt.Printf("[oidc] RSA 签名密钥就绪 kid=%s (auto-generated)\n", kid2)
		}
	} else {
		fmt.Printf("[oidc] RSA 签名密钥就绪 kid=%s\n", kid)
	}
}

// Constants collects the global platform constants injected from config.
type Constants struct {
	PlatformTenantID string
	SuperAdminRoleID string
	DefaultRoleID    string
	EnableTenant     bool
	DefaultPwd       string
	LoginLockMaxErr  int
	LoginLockMinutes int
}

// InitConstants sets the global platform constants (tenant, roles, login lock, etc.).
func InitConstants(c Constants) {
	constants.SetPlatformTenantID(c.PlatformTenantID)
	constants.SetSuperAdminRoleID(c.SuperAdminRoleID)
	constants.SetDefaultRoleID(c.DefaultRoleID)
	constants.SetEnabelTenant(c.EnableTenant)
	model.SetDefaultPwd(c.DefaultPwd)
	model.SetLoginLock(c.LoginLockMaxErr, c.LoginLockMinutes)
}

// SeedAdmin creates the initial admin user on first deploy (idempotent).
func SeedAdmin(platformTenantID, superAdminRoleID, defaultPwd string) {
	seeded, generated, err := model.SeedBootstrapAdmin(platformTenantID, superAdminRoleID, defaultPwd)
	if err != nil {
		fmt.Printf("[bootstrap] 初始管理员播种失败: %v\n", err)
		return
	}
	if !seeded {
		return
	}
	if generated != "" {
		fmt.Println("==========================================================")
		fmt.Printf("[bootstrap] 首次部署：已创建初始管理员 admin / %s\n", generated)
		fmt.Println("[bootstrap] 请立即登录管理后台并修改密码！")
		fmt.Println("==========================================================")
	} else {
		fmt.Println("[bootstrap] 首次部署：已创建初始管理员 admin（口令取自 defaultPwd 配置）")
	}
}

// ---------- Database ----------

// InitDBClient wires the shared DB layer. If casbinRedisAddr is non-empty, the
// Casbin Redis watcher is configured before AutoMigrate (required by NewEnforcer).
func InitDBClient(client db.DBClientInterface, casbinRedisAddr, casbinRedisUser, casbinRedisPwd string) {
	if casbinRedisAddr != "" {
		model.SetCasbinRedis(casbinRedisAddr, casbinRedisUser, casbinRedisPwd)
	}
	model.InitDB(client, true)
}

// ---------- Key Isolation ----------

// CheckKeyIsolation panics if any two of tokenKey / apiKeyEncKey / piiEncKey
// are identical (when explicitly configured). Prevents copy-paste config errors
// that would reuse one key for multiple cryptographic purposes.
func CheckKeyIsolation(tokenKey, apiKeyEncKey, piiEncKey string) {
	if apiKeyEncKey != "" && tokenKey != "" && apiKeyEncKey == tokenKey {
		panic("安全检查失败: apiKeyEncKey 与 token.key 相同，请配置独立的 AI Key 加密密钥")
	}
	if piiEncKey != "" && tokenKey != "" && piiEncKey == tokenKey {
		panic("安全检查失败: piiEncKey 与 token.key 相同，请配置独立的 PII 加密密钥")
	}
	if apiKeyEncKey != "" && piiEncKey != "" && apiKeyEncKey == piiEncKey {
		panic("安全检查失败: apiKeyEncKey 与 piiEncKey 相同，请分别为 AI Key 和 PII 配置独立密钥")
	}
}

// ---------- Alert & Social Login ----------

// SetAlertWebhook configures the optional alert webhook URL (Slack/Feishu/etc.).
func SetAlertWebhook(url string) {
	alert.SetWebhookURL(url)
}

// SocialLoginConfig mirrors http.SocialLoginConfig without importing the http package.
type SocialLoginConfig struct {
	Provider     string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// SetSocialLogins registers social login providers (GitHub, Google, etc.).
func SetSocialLogins(cfgs []SocialLoginConfig) {
	out := make([]userhttp.SocialLoginConfig, 0, len(cfgs))
	for _, s := range cfgs {
		out = append(out, userhttp.SocialLoginConfig{
			Provider: s.Provider, ClientID: s.ClientID,
			ClientSecret: s.ClientSecret, RedirectURI: s.RedirectURI,
		})
	}
	userhttp.SetSocialLogins(out)
}

// ---------- Embedded Admin SPA ----------

// RegisterAdminSPA mounts the embedded React+Vite admin panel under /web/admin.
// /web/ prefix is already bypassed by AuthRequired; no auth needed.
func RegisterAdminSPA(r *gin.Engine) {
	r.GET("/web/admin", func(c *gin.Context) { serveAdminSPA("index.html", c) })
	r.GET("/web/admin/*any", func(c *gin.Context) {
		p := c.Param("any")
		if p == "" || p == "/" || !strings.Contains(p, ".") {
			serveAdminSPA("index.html", c)
			return
		}
		serveAdminSPA(strings.TrimPrefix(p, "/"), c)
	})
}

func serveAdminSPA(name string, c *gin.Context) {
	data, err := web.ReadFile(name)
	if err != nil {
		if name != "index.html" {
			serveAdminSPA("index.html", c)
			return
		}
		c.String(http.StatusNotFound, "not found")
		return
	}
	contentType := "text/html; charset=utf-8"
	switch {
	case strings.HasSuffix(name, ".js"):
		contentType = "application/javascript"
	case strings.HasSuffix(name, ".css"):
		contentType = "text/css"
	case strings.HasSuffix(name, ".svg"):
		contentType = "image/svg+xml"
	case strings.HasSuffix(name, ".png"), strings.HasSuffix(name, ".ico"):
		contentType = "image/png"
	}
	c.Data(http.StatusOK, contentType, data)
}

// ---------- Health & Readiness ----------

// ShutdownState is an atomic flag: 0 = running, 1 = draining (after SIGTERM).
// The shutdown goroutine in each entry point sets this before calling Shutdown.
var ShutdownState int32

// RegisterHealthEndpoints mounts /health and /readyz.
// /health returns 503 after SIGTERM so load balancers stop sending traffic.
func RegisterHealthEndpoints(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		if ShutdownState == 1 {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "shutting down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/readyz", func(c *gin.Context) {
		d := store.DB()
		if d != nil {
			if sqlDB, err := d.DB(); err == nil {
				if err := sqlDB.Ping(); err == nil {
					c.JSON(http.StatusOK, gin.H{"status": "ok"})
					return
				}
			}
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
	})
}

// ---------- Utilities ----------

// GetPort reads a port number from the named environment variable,
// falling back to defaultPort if unset or invalid.
func GetPort(envName string, defaultPort int) int {
	port := defaultPort
	if v := os.Getenv(envName); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			port = p
		}
	}
	return port
}
