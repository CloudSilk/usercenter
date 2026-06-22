package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dubbo.apache.org/dubbo-go/v3/config"
	_ "dubbo.apache.org/dubbo-go/v3/imports"
	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/pkg/db/mysql"
	"github.com/CloudSilk/pkg/db/sqlite"
	"github.com/CloudSilk/pkg/utils"
	ucconfig "github.com/CloudSilk/usercenter/config"
	"github.com/CloudSilk/usercenter/docs"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/model"
	"github.com/CloudSilk/usercenter/internal/alert"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/provider"
	"github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/CloudSilk/usercenter/web"
	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
)

// gin-swagger middleware
// swagger embed files
func main() {
	config.SetProviderService(&provider.UserProvider{})
	config.SetProviderService(&provider.TenantProvider{})
	config.SetProviderService(&provider.RoleProvider{})
	config.SetProviderService(&provider.MenuProvider{})
	config.SetProviderService(&provider.APIProvider{})
	config.SetProviderService(&provider.IdentityProvider{})
	config.SetProviderService(&provider.FormComponentProvider{})
	config.SetProviderService(&provider.ProjectProvider{})
	config.SetProviderService(&provider.DictionariesProvider{})
	config.SetProviderService(&provider.LanguageProvider{})
	config.SetProviderService(&provider.SystemConfigProvider{})
	config.SetProviderService(&provider.WebSiteProvider{})
	config.SetProviderService(&provider.WechatConfigProvider{})
	config.SetProviderService(&provider.WechatProvider{})
	if err := config.Load(); err != nil {
		panic(err)
	}
	configCenter := config.GetRootConfig().ConfigCenter
	nacosAddr := configCenter.Address
	list := strings.Split(nacosAddr, ":")
	port, err := strconv.ParseUint(list[1], 10, 64)
	if err != nil {
		panic(err)
	}
	ucconfig.Init(configCenter.Namespace, list[0], port, configCenter.Username, configCenter.Password)

	var dbClient db.DBClientInterface
	if ucconfig.DefaultConfig.DBType == "sqlite" {
		dbClient = sqlite.NewSqlite2("", "", ucconfig.DefaultConfig.Sqlite, "usercenter", ucconfig.DefaultConfig.Debug)
	} else {
		dbClient = mysql.NewMysql(ucconfig.DefaultConfig.Mysql, ucconfig.DefaultConfig.Debug)
	}

	// 配置 Casbin watcher 用的 Redis（必须在 InitDB 之前，因为 NewEnforcer 在初始化时读取）
	if ucconfig.DefaultConfig.Token.RedisAddr != "" {
		model.SetCasbinRedis(ucconfig.DefaultConfig.Token.RedisAddr, ucconfig.DefaultConfig.Token.RedisName, ucconfig.DefaultConfig.Token.RedisPwd)
	}
	model.InitDB(dbClient, true)
	token.InitTokenCache(ucconfig.DefaultConfig.Token.Key, ucconfig.DefaultConfig.Token.RedisAddr, ucconfig.DefaultConfig.Token.RedisName, ucconfig.DefaultConfig.Token.RedisPwd, ucconfig.DefaultConfig.Token.Expired)
	// AI Key 加密密钥：优先显式配置，否则从 token.key 派生（SHA-256），保证部署内确定。
	if !apikey.SetEncryptionKeyFrom(ucconfig.DefaultConfig.APIKeyEncKey) {
		apikey.SetEncryptionKeyFrom(ucconfig.DefaultConfig.Token.Key)
	}
	// OIDC 密钥管理器（id_token 签名 + JWKS）。
	auth.InitKeyManager(ucconfig.DefaultConfig.Token.Key)
	// PII 字段加密密钥（MFA secret / 敏感字段），派生自 token.key
	auth.SetPIIKeyFrom(ucconfig.DefaultConfig.Token.Key)
	// 告警 Webhook（可选）
	alert.SetWebhookURL(ucconfig.DefaultConfig.AlertWebhookURL)
	// 社交登录配置（GitHub/Google 等）
	socialCfgs := make([]userhttp.SocialLoginConfig, 0, len(ucconfig.DefaultConfig.SocialLogins))
	for _, s := range ucconfig.DefaultConfig.SocialLogins {
		socialCfgs = append(socialCfgs, userhttp.SocialLoginConfig{
			Provider: s.Provider, ClientID: s.ClientID, ClientSecret: s.ClientSecret, RedirectURI: s.RedirectURI,
		})
	}
	userhttp.SetSocialLogins(socialCfgs)
	constants.SetPlatformTenantID(ucconfig.DefaultConfig.PlatformTenantID)
	constants.SetSuperAdminRoleID(ucconfig.DefaultConfig.SuperAdminRoleID)
	constants.SetDefaultRoleID(ucconfig.DefaultConfig.DefaultRoleID)
	constants.SetEnabelTenant(ucconfig.DefaultConfig.EnableTenant)
	model.SetDefaultPwd(ucconfig.DefaultConfig.DefaultPwd)
	model.SetLoginLock(ucconfig.DefaultConfig.LoginLock.MaxErrCount, ucconfig.DefaultConfig.LoginLock.LockMinutes)
	// 首次部署：users 表为空时自动播种平台租户 + 超级管理员角色 + 初始管理员。
	// 口令取 defaultPwd 配置，未配置则随机生成并打印；初始账号强制首登改密。
	seedBootstrapAdmin(ucconfig.DefaultConfig.PlatformTenantID, ucconfig.DefaultConfig.SuperAdminRoleID, ucconfig.DefaultConfig.DefaultPwd)
	Start(GetPort("ATALI_PORT", 48080))
}

// seedBootstrapAdmin 调用领域层播种初始管理员；非首次部署（已有用户）为 no-op。
// 未配置 defaultPwd 时生成的随机口令会显著打印，提示运维立即登录改密。
func seedBootstrapAdmin(platformTenantID, superAdminRoleID, defaultPwd string) {
	seeded, generated, err := model.SeedBootstrapAdmin(platformTenantID, superAdminRoleID, defaultPwd)
	if err != nil {
		fmt.Printf("[bootstrap] 初始管理员播种失败: %v\n", err)
		return
	}
	if !seeded {
		return // 非首次部署，已有用户
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

// 从环境变量中获取端口号
func GetPort(envName string, defaultPort int) int {
	port := defaultPort
	if os.Getenv(envName) != "" {
		var err error
		port, err = strconv.Atoi(os.Getenv(envName))
		if err != nil {
			fmt.Println("get port from env failed, err:", err)
			port = defaultPort
		}
	}
	return port
}

func Start(port int) {
	// programatically set swagger info
	docs.SwaggerInfo.Title = "UserCenter API"
	docs.SwaggerInfo.Description = "This is a UserCenter server."
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Schemes = []string{"http", "https"}

	r := gin.Default()
	r.Use(userhttp.MetricsMiddleware()) // Prometheus 指标采集（HTTP 量/延迟）
	r.Use(middleware.AuthRequired)
	r.Use(utils.Cors())
	userhttp.RegisterAuthRouter(r)
	userhttp.RegisterAdminRouter(r)
	userhttp.RegisterAIGatewayRouter(r)  // OpenAI 兼容 AI 网关：/v1/chat/completions、/v1/models
	userhttp.RegisterOIDCRouter(r)       // OIDC/OAuth2 Provider：/.well-known/* /oauth/*
	userhttp.RegisterSocialLoginRouter(r) // 社交登录画廊：/api/oauth/:provider/{login,callback}
	userhttp.RegisterMetricsRouter(r)    // /metrics Prometheus 抓取端点

	// 管理后台单页应用（React + Vite 构建产物，go:embed 打包进二进制）。
	// /web/ 前缀已在 middleware.AuthRequired 中放行，无需鉴权即可加载页面；
	// 页面内部通过 /api/core/auth/user/login 获取 Token 后访问受保护接口。
	registerAdminWeb(r)

	// 健康检查端点（供 K8s liveness/readiness probe 使用）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 仅在 Debug 模式下暴露 swagger 文档，生产环境（debug=false）不对外暴露 API 文档
	if ucconfig.DefaultConfig.Debug {
		r.GET("/swagger/usercenter/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// 优雅关闭：捕获 SIGTERM/SIGINT，等待现有请求完成后再退出
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	go func() {
		fmt.Printf("started server on :%d\n", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("server error: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("server forced to shutdown: %v\n", err)
	}
	fmt.Println("server exited")
}

// registerAdminWeb 挂载内嵌的管理后台单页应用（React + Vite 构建产物）。
// 使用 http.FS(DistFS) 嵌入 dist 目录，以 /web/admin 为前缀对外暴露静态资源。
// /web/ 前缀已被 AuthRequired 放行，因此此处不触发鉴权。
func registerAdminWeb(r *gin.Engine) {
	// SPA fallback：返回 index.html
	r.GET("/web/admin", func(c *gin.Context) { webSPA("index.html", c) })
	r.GET("/web/admin/*any", func(c *gin.Context) {
		p := c.Param("any")
		if p == "" || p == "/" || !strings.Contains(p, ".") {
			webSPA("index.html", c)
			return
		}
		rel := strings.TrimPrefix(p, "/")
		webSPA(rel, c)
	})
}

func webSPA(name string, c *gin.Context) {
	data, err := web.ReadFile(name)
	if err != nil {
		if name != "index.html" {
			webSPA("index.html", c)
			return
		}
		c.String(http.StatusNotFound, "not found")
		return
	}
	contentType := "text/html; charset=utf-8"
	if strings.HasSuffix(name, ".js") {
		contentType = "application/javascript"
	} else if strings.HasSuffix(name, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(name, ".svg") {
		contentType = "image/svg+xml"
	} else if strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".ico") {
		contentType = "image/png"
	}
	c.Data(http.StatusOK, contentType, data)
}
