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
	constants.SetPlatformTenantID(ucconfig.DefaultConfig.PlatformTenantID)
	constants.SetSuperAdminRoleID(ucconfig.DefaultConfig.SuperAdminRoleID)
	constants.SetDefaultRoleID(ucconfig.DefaultConfig.DefaultRoleID)
	constants.SetEnabelTenant(ucconfig.DefaultConfig.EnableTenant)
	model.SetDefaultPwd(ucconfig.DefaultConfig.DefaultPwd)
	model.SetLoginLock(ucconfig.DefaultConfig.LoginLock.MaxErrCount, ucconfig.DefaultConfig.LoginLock.LockMinutes)
	Start(GetPort("ATALI_PORT", 48080))
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
	r.Use(middleware.AuthRequired)
	r.Use(utils.Cors())
	userhttp.RegisterAuthRouter(r)
	userhttp.RegisterAdminRouter(r)

	// 管理后台单页应用（Vue 3 + Element Plus CDN）。
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

// registerAdminWeb 挂载内嵌的管理后台单页应用。
// 同时响应 /web/admin、/web/admin.html 与 /web/admin/，统一返回 admin.html。
// /web/ 前缀已被 AuthRequired 放行，因此此处不触发鉴权。
func registerAdminWeb(r *gin.Engine) {
	r.GET("/web/admin", serveAdminHTML)
	r.GET("/web/admin.html", serveAdminHTML)
	r.GET("/web/admin/", serveAdminHTML)
}

// serveAdminHTML 返回内嵌的 admin.html，并设置正确的 Content-Type 与禁止缓存的响应头。
func serveAdminHTML(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", web.AdminHTML)
}
