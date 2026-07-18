package server

import (
	"net/http"
	"strings"

	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// DevAuthRequired 是开发期鉴权中间件，供宿主（如 CapsiFarm 单体）在全新空库时使用。
//
// 背景：生产 utils/middleware.AuthRequired 依赖 Casbin（规则来自 api 表）。全新本地
// 库 api 表为空，会导致除登录外的所有接口被拦截。开发期目的是验证业务功能，
// 无需完整权限体系，因此本中间件：
//   - 放行静态资源（/web、/swagger、/uploads 等）与免登录公开端点；
//   - 其余请求用 token.DecodeToken 真实验签，成功则写 Principal/User 到 context；
//   - 不做 Casbin 鉴权——已登录即放行。
//
// 生产环境请改用 middleware.AuthRequired 并完成 api 表 + Casbin 规则播种。
func DevAuthRequired(c *gin.Context) {
	path := c.Request.URL.Path
	// 静态资源与免登录公开端点。
	if strings.HasPrefix(path, "/swagger/") || strings.HasPrefix(path, "/web/") ||
		strings.HasPrefix(path, "/uploads/") || strings.HasPrefix(path, "/scim/") ||
		strings.HasPrefix(path, "/.well-known/") || strings.HasPrefix(path, "/api/oauth/") ||
		path == "/api/social/providers" {
		return
	}
	if path == "/api/core/auth/user/login" || path == "/api/core/auth/user/mfa/verify" ||
		path == "/health" || path == "/readyz" ||
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
