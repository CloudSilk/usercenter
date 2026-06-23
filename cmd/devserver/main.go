// Command devserver 启动一个**仅 HTTP** 的本地开发服务，用于在浏览器中验证
// 内嵌管理后台（/web/admin）。
//
// 与生产 main.go 的区别：
//   - 不依赖 Nacos / Dubbo Triple，跳过 config.Load() 与 provider 注册；
//   - 直连本地 MySQL（DSN 与端口可用环境变量覆盖）；
//   - 始终执行 AutoMigrate 自动建表（无 debug 开关依赖）；
//   - 若 users 表为空，自动播种一个超级管理员账号，方便直接登录面板。
//
// 用法（确保本地 MySQL 已起、usercenter 库已建）：
//
//	NO_PROXY=localhost,127.0.0.1 go run ./cmd/devserver
//
// 然后浏览器打开 http://localhost:48080/web/admin 。
// 默认管理员：admin / Admin@123456
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/db/mysql"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/scim"
	"github.com/CloudSilk/usercenter/internal/store"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/model"
	"github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/CloudSilk/usercenter/web"
	"github.com/gin-gonic/gin"
)

// 本地默认值；均可通过环境变量覆盖。
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

const (
	devPlatformTenantID = "platform"
	devSuperAdminRoleID = "1"
	devAdminUser        = "admin"
	// 安全修复 S4:不再硬编码默认密码，启动时若种子播种且用到默认口令则随机生成并打印到 stdout。
	devAdminPwd         = ""
	// devserver 无需固定 token key，每次启动随机生成。
	// 若需持久化 session(如已登录 token 重启后仍有效)，可通过 UC_TOKEN_KEY 环境变量指定。
	devSCIMToken        = "scim-dev-token"
)

// devShutdownState 用于在 SIGTERM 后标记 devserver 实例正在退出，使 /health 返回 503。
var devShutdownState int32 // atomic: 0=running, 1=draining

func main() {
	// 安全修复 S4:硬守卫 — 生产检测
	if os.Getenv("GIN_MODE") == "release" {
		fmt.Println("[devserver] 禁止在 GIN_MODE=release 下运行。生产请使用 main.go。")
		os.Exit(1)
	}
	dsn := env("UC_MYSQL_DSN", "root:root123@tcp(127.0.0.1:13306)/usercenter?charset=utf8mb4&parseTime=True&loc=Local")

	// 1. 连库 + AutoMigrate（始终建表，含 REDESIGN 新增域表）
	dbClient := mysql.NewMysql(dsn, true)
	model.InitDB(dbClient, true)

	// 2. token 缓存（内存，无 Redis）
	tokenKey := env("UC_TOKEN_KEY", "")
	if tokenKey == "" {
		tokenKey = randKey()
		fmt.Printf("[devserver] 启动时随机生成 token key: %s\n", tokenKey)
	}
	token.InitTokenCache(tokenKey, "", "", "", 1440)
	// AI Key 加密密钥
	apikey.SetEncryptionKeyFrom(tokenKey)
	// PII 加密密钥
	auth.SetPIIKeyFrom(tokenKey + "-pii")
	// OIDC RSA 密钥管理器（id_token RS256 签名 + JWKS）
	if kid, err := auth.InitKeyManager(""); err != nil {
		panic(fmt.Sprintf("RSA 密钥初始化失败: %v", err))
	} else {
		fmt.Printf("[devserver] OIDC RSA 签名密钥就绪 kid=%s\n", kid)
	}

	// 3. 全局常量
	constants.SetPlatformTenantID(devPlatformTenantID)
	constants.SetSuperAdminRoleID(devSuperAdminRoleID)
	constants.SetDefaultRoleID(devSuperAdminRoleID)
	constants.SetEnabelTenant(true)
	model.SetDefaultPwd("")
	model.SetLoginLock(5, 15)

	// 4. 首次播种初始管理员（复用生产同款 model.SeedBootstrapAdmin）
	if seeded, g, err := model.SeedBootstrapAdmin(devPlatformTenantID, devSuperAdminRoleID, devAdminPwd); err != nil {
		fmt.Println("[devserver] 播种失败:", err)
	} else if !seeded {
		fmt.Println("[devserver] 已有用户，跳过播种")
	} else {
		pwd := devAdminPwd
		if g != "" {
			pwd = g
		}
		fmt.Printf("[devserver] 已播种管理员：%s / %s\n", devAdminUser, pwd)
	}

	// 5. HTTP 服务（与生产 Start() 路由一致，去掉 Dubbo/Swagger）
	port := 48080
	if v := os.Getenv("UC_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &port)
	}
	startHTTP(port)
}

func startHTTP(port int) {
	r := gin.Default()
	r.Use(userhttp.MetricsMiddleware())
	r.Use(devAuthRequired) // dev-only：真实验签 + 写 Principal，但跳过 Casbin（全新库 api 表为空）
	r.Use(utils.Cors())
	userhttp.RegisterAuthRouter(r)
	userhttp.RegisterAdminRouter(r)
	userhttp.RegisterAIGatewayRouter(r) // OpenAI 兼容 AI 网关
	userhttp.RegisterOIDCRouter(r)      // OIDC/OAuth2 Provider
	userhttp.RegisterSocialLoginRouter(r) // 社交登录画廊
	userhttp.RegisterMetricsRouter(r)   // /metrics
	scim.RegisterSCIMRouter(r, devSCIMToken) // dev 挂载 SCIM，方便面板「SCIM 配置」页测试

	// 内嵌管理后台单页（React + Vite SPA）
	r.GET("/web/admin", serveAdminHTML)
	r.GET("/web/admin/*any", serveAdminHTML)

	r.GET("/health", func(c *gin.Context) {
		if atomic.LoadInt32(&devShutdownState) == 1 {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "shutting down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/readyz", func(c *gin.Context) {
		d := store.DB()
		if d != nil {
			sqlDB, err := d.DB()
			if err == nil {
				if err := sqlDB.Ping(); err == nil {
					c.JSON(http.StatusOK, gin.H{"status": "ok"})
					return
				}
			}
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		fmt.Printf("[devserver] 管理后台: http://localhost:%d/web/admin  (admin / %s)\n", port, devAdminPwd)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[devserver] 服务错误: %v\n", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("[devserver] 正在关闭...")
	atomic.StoreInt32(&devShutdownState, 1)
	// 30s 超时匹配 K8s 默认 terminationGracePeriodSeconds
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	fmt.Println("[devserver] 已退出")
}

func serveAdminHTML(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	path := c.Request.URL.Path
	trimmed := strings.TrimPrefix(path, "/web/admin")
	if trimmed == "" || trimmed == "/" {
		trimmed = "index.html"
	} else {
		trimmed = strings.TrimPrefix(trimmed, "/")
	}
	// 无文件扩展名的路径 → 直接 SPA fallback（如 /web/admin/users）
	if !strings.Contains(trimmed, ".") {
		trimmed = "index.html"
	}
	data, err := web.ReadFile(trimmed)
	if err != nil {
		data, err = web.ReadFile("index.html")
		if err != nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		trimmed = "index.html"
	}
	contentType := "text/html; charset=utf-8"
	if strings.HasSuffix(trimmed, ".js") {
		contentType = "application/javascript"
	} else if strings.HasSuffix(trimmed, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(trimmed, ".svg") {
		contentType = "image/svg+xml"
	} else if strings.HasSuffix(trimmed, ".png") || strings.HasSuffix(trimmed, ".ico") {
		contentType = "image/png"
	}
	c.Data(http.StatusOK, contentType, data)
}

// devAuthRequired 是仅供 devserver 使用的鉴权中间件。
//
// 与生产 utils/middleware.AuthRequired 的区别：生产依赖 Casbin（角色 -1/0/具体角色）
// 判定放行，而 Casbin 规则由 api 表播种而来——全新本地库 api 表为空，会导致连登录
// 接口都被拦截。devserver 目的是验证管理后台 UI，无需权限体系，因此这里：
//   - 放行静态资源（/web、/swagger）与登录类免鉴权路径；
//   - 其余请求用 token.DecodeToken 真实验签解析，成功则写 Principal/User 到 context；
//   - 不做 Casbin 鉴权——已登录即放行。
func devAuthRequired(c *gin.Context) {
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/swagger/") || strings.HasPrefix(path, "/web/") ||
		strings.HasPrefix(path, "/scim/") || strings.HasPrefix(path, "/.well-known/") ||
		strings.HasPrefix(path, "/api/oauth/") || path == "/api/social/providers" {
		return
	}
	// 登录/健康检查/OIDC 公开端点免登录（spec 要求 discovery/jwks/token/revoke 公开）
	if path == "/api/core/auth/user/login" || path == "/health" ||
		path == "/oauth/token" || path == "/oauth/revoke" || path == "/metrics" ||
		path == "/admin/api/audit/stream" {
		return
	}
	t := middleware.GetAccessToken(c)
	cu, err := token.DecodeToken(t)
	if err != nil || cu == nil {
		c.AbortWithStatusJSON(http.StatusOK, gin.H{"code": 41004, "message": "未登录或 token 无效"})
		return
	}
	if p := principal.FromTokenAndUser(t, cu); p != nil {
		c.Set("Principal", p)
	}
	c.Set("User", cu)
}

// randKey 生成 32 字节随机密钥(用于 token 签名)。
func randKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
