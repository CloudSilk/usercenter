package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/CloudSilk/usercenter/authorization"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

type runtimeAuthorizationRequest struct {
	Path   string `json:"path" binding:"required"`
	Method string `json:"method" binding:"required"`
}

type runtimeAuthorizationIdentity struct {
	SubjectID string   `json:"subjectID"`
	TenantID  string   `json:"tenantID"`
	UserName  string   `json:"userName"`
	RoleIDs   []string `json:"roleIDs"`
}

type runtimeAuthorizationResult struct {
	authorization.Decision
	Path      string                          `json:"path"`
	Method    string                          `json:"method"`
	Identity  runtimeAuthorizationIdentity    `json:"identity"`
	DataScope authorization.DataScopeDecision `json:"dataScope"`
}

func CheckRuntimeAuthorization(c *gin.Context) {
	principal, ok := ucm.GetPrincipal(c)
	if !ok || principal == nil {
		c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Unauthorized, "message": "current login identity is unavailable"})
		return
	}
	req := &runtimeAuthorizationRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": apipb.Code_BadRequest, "message": err.Error()})
		return
	}
	decision, err := authorization.EvaluateRoles(principal.Roles(), req.Path, req.Method)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": apipb.Code_InternalServerError, "message": err.Error()})
		return
	}
	dataScope := authorization.DataScopeDecision{Rules: []authorization.DataScopeRule{}}
	if decision.Allow {
		scopeRoleIDs := decision.MatchedRoleIDs
		dataScope, err = authorization.EvaluateDataScopes(scopeRoleIDs, principal.TenantID(), req.Path, req.Method)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": apipb.Code_InternalServerError, "message": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": apipb.Code_Success,
		"data": runtimeAuthorizationResult{
			Decision:  decision,
			Path:      strings.TrimSpace(req.Path),
			Method:    strings.ToUpper(strings.TrimSpace(req.Method)),
			DataScope: dataScope,
			Identity: runtimeAuthorizationIdentity{
				SubjectID: principal.Subject(),
				TenantID:  principal.TenantID(),
				UserName:  principal.DisplayName(),
				RoleIDs:   append([]string{}, principal.Roles()...),
			},
		},
	})
}

func ApplyAuthorizationCatalog(c *gin.Context) {
	principal, ok := ucm.GetPrincipal(c)
	if !ok || principal == nil || !hasAuthorizationRole(principal.Roles(), "1") {
		c.JSON(http.StatusOK, gin.H{"code": apipb.Code_NoPermission, "message": "super administrator role is required"})
		return
	}
	catalog := authorization.Catalog{}
	if err := c.ShouldBindJSON(&catalog); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": apipb.Code_BadRequest, "message": err.Error()})
		return
	}
	summary, err := applyAuthorizationCatalog(catalog)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": apipb.Code_BadRequest, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": summary})
}

func applyAuthorizationCatalog(catalog authorization.Catalog) (authorization.Summary, error) {
	for attempt := 0; ; attempt++ {
		summary, err := authorization.Apply(catalog)
		if err == nil || attempt >= 4 || !isSQLiteBusy(err) {
			return summary, err
		}
		time.Sleep(time.Duration(1<<attempt) * 20 * time.Millisecond)
	}
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "sqlite_busy")
}

func hasAuthorizationRole(roleIDs []string, expected string) bool {
	for _, roleID := range roleIDs {
		if strings.TrimSpace(roleID) == expected {
			return true
		}
	}
	return false
}

func RegisterAuthorizationRouter(r *gin.Engine) {
	r.POST(authorization.RuntimeCheckPath, CheckRuntimeAuthorization)
	r.POST(authorization.CatalogApplyPath, ApplyAuthorizationCatalog)
}
