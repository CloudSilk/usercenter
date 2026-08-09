package http

import (
	"github.com/gin-gonic/gin"
)

func RegisterAuthRouter(r *gin.Engine) {
	RegisterLoginOptionsRouter(r)
	RegisterUserRouter(r)
	RegisterTenantRouter(r)
	RegisterWechatRouter(r)
	RegisterAPIRouter(r)
	RegisterAPPRouter(r)
	RegisterMenuRouter(r)
	RegisterRoleRouter(r)
	RegisterFormComponentRouter(r)
	RegisterProjectRouter(r)
	RegisterDictionariesRouter(r)
	RegisterLanguageRouter(r)
	RegisterSystemConfigRouter(r)
	RegisterWebSiteRouter(r)
	RegisterWechatConfigRouter(r)
	RegisterPermissionEffectivenessRouter(r)
	RegisterAuthorizationRouter(r)
	registerAPIKeyAuthRoutes(r.Group("/admin/api"))
}
