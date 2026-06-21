// Command devserver 启动一个**仅 HTTP** 的本地开发服务，用于在浏览器中验证
// 内嵌管理后台（/web/admin）。
//
// 与生产 main.go 的区别：
//   - 不依赖 Nacos / Dubbo Triple，跳过 config.Load() 与 provider 注册；
//   - 直连本地 MySQL（DSN 与端口可用环境变量覆盖）；
//   - debug=true 触发 AutoMigrate 自动建表；
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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/db/mysql"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/scim"
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
	devAdminPwd         = "Admin@123456"
	devTokenKey         = "local-dev-signing-key-2026" // 非空且非源码已知默认值，满足 InitTokenCache 校验
	devSCIMToken        = "scim-dev-token"
)

func main() {
	dsn := env("UC_MYSQL_DSN", "root:root123@tcp(127.0.0.1:13306)/usercenter?charset=utf8mb4&parseTime=True&loc=Local")

	// 1. 连库 + AutoMigrate（debug=true 建表，含 REDESIGN 新增域表）
	dbClient := mysql.NewMysql(dsn, true)
	model.InitDB(dbClient, true)

	// 2. token 缓存（内存，无 Redis）；key 必须显式配置，否则拒绝启动
	token.InitTokenCache(devTokenKey, "", "", "", 1440)

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
	r.Use(devAuthRequired) // dev-only：真实验签 + 写 Principal，但跳过 Casbin（全新库 api 表为空）
	r.Use(utils.Cors())
	userhttp.RegisterAuthRouter(r)
	userhttp.RegisterAdminRouter(r)
	scim.RegisterSCIMRouter(r, devSCIMToken) // dev 挂载 SCIM，方便面板「SCIM 配置」页测试

	// 内嵌管理后台单页
	r.GET("/web/admin", serveAdminHTML)
	r.GET("/web/admin.html", serveAdminHTML)
	r.GET("/web/admin/", serveAdminHTML)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: r}
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	fmt.Println("[devserver] 已退出")
}

func serveAdminHTML(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", web.AdminHTML)
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
	if strings.HasPrefix(path, "/swagger/") || strings.HasPrefix(path, "/web/") || strings.HasPrefix(path, "/scim/") {
		return
	}
	// 登录/健康检查等免登录路径
	if path == "/api/core/auth/user/login" || path == "/health" {
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

