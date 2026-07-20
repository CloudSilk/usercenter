package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/accesseffect"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func permissionEffectivenessError(c *gin.Context, err error) {
	code := apipb.Code_InternalServerError
	if errors.Is(err, gorm.ErrRecordNotFound) {
		code = apipb.Code_BadRequest
	}
	c.JSON(http.StatusOK, gin.H{"code": code, "message": err.Error()})
}

// GetPermissionEffectiveness returns the native login, authorization, session
// and audit projection for one tenant-scoped user.
func GetPermissionEffectiveness(c *gin.Context) {
	userID := strings.TrimSpace(c.Query("userID"))
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{"code": apipb.Code_BadRequest, "message": "userID is required"})
		return
	}
	if !canManageUser(c, userID) {
		c.JSON(http.StatusOK, noUserPermissionResponse())
		return
	}
	detail, err := accesseffect.GetDetail(userID)
	if err != nil {
		permissionEffectivenessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": detail})
}

// CheckPermissionEffectiveness evaluates one API using the exact public,
// login-only and enabled-role Casbin order, then records the decision through
// usercenter's native audit domain.
func CheckPermissionEffectiveness(c *gin.Context) {
	req := &accesseffect.CheckRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		permissionEffectivenessError(c, err)
		return
	}
	if !canManageUser(c, req.UserID) {
		c.JSON(http.StatusOK, noUserPermissionResponse())
		return
	}
	result, err := accesseffect.Check(req.UserID, req.APIID)
	if err != nil {
		permissionEffectivenessError(c, err)
		return
	}
	operatorName := ucm.GetUserName(c)
	if exists, currentUser := ucm.GetUser(c); exists &&
		currentUser != nil &&
		strings.TrimSpace(currentUser.UserName) != "" {
		operatorName = currentUser.UserName
	}
	audit.RecordAuditWithKind(
		store.DB(),
		ucm.GetUserID(c),
		operatorName,
		int32(ucm.GetPrincipalKind(c)),
		audit.AuditActionVerifyPermissionEffect,
		req.UserID,
		c.ClientIP(),
		accesseffect.AuditDetail(result),
	)
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": result})
}

// GetPermissionEffectivenessSelf exposes the current middleware principal and
// JWT role snapshot. It is a login-only endpoint, not an administrator bypass.
func GetPermissionEffectivenessSelf(c *gin.Context) {
	exists, currentUser := ucm.GetUser(c)
	if !exists || currentUser == nil {
		c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Unauthorized, "message": "current login identity is unavailable"})
		return
	}
	roleIDs := append([]string(nil), currentUser.RoleIDs...)
	if principal, ok := ucm.GetPrincipal(c); ok && principal != nil {
		roleIDs = append([]string(nil), principal.Roles()...)
	}
	c.JSON(http.StatusOK, gin.H{
		"code": apipb.Code_Success,
		"data": gin.H{
			"userID":        ucm.GetUserID(c),
			"userName":      currentUser.UserName,
			"tenantID":      ucm.GetTenantID(c),
			"roleIDs":       roleIDs,
			"principalKind": int32(ucm.GetPrincipalKind(c)),
		},
	})
}

func RegisterPermissionEffectivenessRouter(r *gin.Engine) {
	if err := accesseffect.EnsureAPIResources(); err != nil {
		log.Errorf(context.Background(), "ensure permission effectiveness API resources failed: %v", err)
	}
	group := r.Group("/api/core/auth/permission/effectiveness")
	group.GET("", GetPermissionEffectiveness)
	group.POST("check", CheckPermissionEffectiveness)
	group.GET("self", GetPermissionEffectivenessSelf)
}
