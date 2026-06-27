package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"dubbo.apache.org/dubbo-go/v3/config"
	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/alert"
	"github.com/CloudSilk/usercenter/internal/authn"
	"github.com/CloudSilk/usercenter/internal/principal"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

// --- 旧接口(向后兼容,阶段3 删除) ---

func GetUser(c *gin.Context) (bool, *apipb.CurrentUser) {
	obj, exists := c.Get("User")
	if !exists {
		return false, nil
	}
	user, ok := obj.(*apipb.CurrentUser)
	if !ok {
		return false, nil
	}
	return true, user
}

func GetUserID(c *gin.Context) string {
	if p, ok := GetPrincipal(c); ok && p != nil {
		return p.Subject()
	}
	exists, user := GetUser(c)
	if !exists || user == nil {
		return ""
	}
	return user.Id
}

func GetUserName(c *gin.Context) string {
	if p, ok := GetPrincipal(c); ok && p != nil {
		return p.DisplayName()
	}
	exists, user := GetUser(c)
	if !exists || user == nil {
		return ""
	}
	return user.UserName
}

func GetTenantID(c *gin.Context) string {
	if p, ok := GetPrincipal(c); ok && p != nil {
		return p.TenantID()
	}
	exists, user := GetUser(c)
	if !exists || user == nil {
		return ""
	}
	return user.TenantID
}

// --- 新接口(REDESIGN §4 阶段2:Principal) ---

// GetPrincipal 从 gin.Context 获取鉴权主体 Principal
func GetPrincipal(c *gin.Context) (principal.Principal, bool) {
	obj, exists := c.Get("Principal")
	if !exists {
		return nil, false
	}
	p, ok := obj.(principal.Principal)
	if !ok {
		return nil, false
	}
	return p, true
}

// GetPrincipalKind 返回主体类型(Human/Agent/Service),未登录返回 Unknown
func GetPrincipalKind(c *gin.Context) principal.Kind {
	if p, ok := GetPrincipal(c); ok && p != nil {
		return p.Kind()
	}
	return principal.KindUnknown
}

// IsAgentRequest 判断当前请求是否来自 AI Agent
func IsAgentRequest(c *gin.Context) bool {
	return GetPrincipalKind(c) == principal.KindAgent
}

func GetAccessToken(c *gin.Context) string {
	accessToken := c.GetHeader("Authorization")
	if accessToken == "" {
		accessToken = c.GetHeader("authorization")
	}
	accessToken = strings.Replace(accessToken, "Bearer ", "", 1)
	return accessToken
}


type RateLimiter interface {
	Allow(principalID string) bool
}

func AuthRequired(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/swagger/") || strings.HasPrefix(c.Request.URL.Path, "/web/") {
		return
	}
	// OIDC/OAuth2 公开端点（spec 要求）：发现文档、JWKS、令牌、吊销须免登录。
	// authorize/userinfo 仍需用户 Bearer，走正常鉴权。
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/.well-known/") || path == "/oauth/token" || path == "/oauth/revoke" ||
		path == "/readyz" ||
		path == "/health" || path == "/metrics" || path == "/admin/api/audit/stream" ||
		strings.HasPrefix(path, "/api/oauth/") || path == "/api/social/providers" {
		return
	}
	t := GetAccessToken(c)
	p, currentUser, code, err := authn.AuthenticatePrincipal(t, c.Request.Method, c.Request.URL.Path, true)

	if code != model.Success {
		if code == model.Unauthorized {
			alert.AlertAuthFailure(c.ClientIP(), c.Request.URL.Path)
		}
		message := ""
		if err != nil {
			message = err.Error()
		}
		c.AbortWithStatusJSON(http.StatusOK, model.CommonResponse{
			Code:    code,
			Message: message,
		})
		return
	}

	// 阶段3:Principal 为一等身份,CurrentUser 向后兼容
	if p != nil {
		c.Set("Principal", p)
	}
	c.Set("User", currentUser)
}

var IdentityImpl = new(apipb.IdentityClientImpl)

func InitIdentity() {
	config.SetConsumerService(IdentityImpl)
}

func Authenticate(t, method, url string, checkAuth bool) (*apipb.CurrentUser, int, error) {
	resp, err := IdentityImpl.Authenticate(context.Background(), &apipb.AuthenticateRequest{
		Token:     t,
		Method:    method,
		Url:       url,
		CheckAuth: checkAuth,
	})
	if err != nil {
		return nil, model.InternalServerError, err
	}
	if resp.Code != model.Success {
		return nil, int(resp.Code), errors.New(resp.Message)
	}
	return resp.CurrentUser, model.Success, nil
}

func AuthRequiredWithRPC(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
		return
	}
	t := GetAccessToken(c)
	currentUser, code, err := Authenticate(t, c.Request.Method, c.Request.URL.Path, true)

	if code != model.Success {
		message := ""
		if err != nil {
			message = err.Error()
		}
		c.AbortWithStatusJSON(http.StatusOK, model.CommonResponse{
			Code:    code,
			Message: message,
		})
		return
	}

	p := principal.FromTokenAndUser(t, currentUser)
	if p != nil {
		c.Set("Principal", p)
	}
	c.Set("User", currentUser)
}
