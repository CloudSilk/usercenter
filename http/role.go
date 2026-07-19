package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func canManageRole(c *gin.Context, roleID string) bool {
	currentTenantID := ucm.GetTenantID(c)
	if currentTenantID == constants.PlatformTenantID {
		return true
	}
	target, err := permission.GetRoleByID(roleID)
	return err == nil && !target.Public && target.TenantID == currentTenantID
}

func noRolePermissionResponse() *apipb.CommonResponse {
	return &apipb.CommonResponse{Code: apipb.Code_NoPermission, Message: "无权管理该角色"}
}

// AddRole godoc
// @Summary 新增角色
// @Tags 角色管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.RoleInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/add [post]
func AddRole(c *gin.Context, req *apipb.RoleInfo) (*apipb.CommonResponse, error) {
	//只有平台租户才能为其他租户创建角色
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
		req.Public = false
	}
	if err := permission.CreateRole(permission.PBToRole(req), tenant.GetTenantUserCount); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// UpdateRole godoc
// @Summary 更新角色
// @Tags 角色管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.RoleInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/update [put]
func UpdateRole(c *gin.Context, req *apipb.RoleInfo) (*apipb.CommonResponse, error) {
	if !canManageRole(c, req.Id) {
		return noRolePermissionResponse(), nil
	}
	//只有平台租户才能更改角色的租户
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
		req.Public = false
	}
	if err := permission.UpdateRole(permission.PBToRole(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteRole godoc
// @Summary 删除角色
// @Tags 角色管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/delete [delete]
func DeleteRole(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	if !canManageRole(c, req.Id) {
		return noRolePermissionResponse(), nil
	}
	if err := permission.DeleteRole(req.Id); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// EnableRole godoc
// @Summary 禁用/启用角色
// @Description 系统角色不允许停用；状态变化会撤销受影响用户的旧登录上下文并同步 Casbin 权限。
// @Tags 角色管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.EnableRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/enable [post]
func EnableRole(c *gin.Context, req *apipb.EnableRequest) (*apipb.CommonResponse, error) {
	if !canManageRole(c, req.Id) {
		return noRolePermissionResponse(), nil
	}
	if err := permission.SetRoleEnabled(req.Id, req.Enable); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QueryRole godoc
// @Summary 分页查询
// @Tags 角色管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QueryRoleResponse
// @Router /api/core/auth/role/query [get]
func QueryRole(c *gin.Context, req *apipb.QueryRoleRequest) (*apipb.QueryRoleResponse, error) {
	//只有平台租户才能查询其他租户的角色
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	resp := &apipb.QueryRoleResponse{Code: apipb.Code_Success}
	permission.QueryRole(req, resp, false)
	return resp, nil
}

// GetRoleDetail godoc
// @Summary 查询明细
// @Tags 角色管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetRoleDetailResponse
// @Router /api/core/auth/role/detail [get]
func GetRoleDetail(c *gin.Context) {
	resp := &apipb.GetRoleDetailResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	if !canManageRole(c, idStr) {
		resp.Code = apipb.Code_NoPermission
		resp.Message = "无权查看该角色"
		c.JSON(http.StatusOK, resp)
		return
	}
	data, err := permission.GetRoleByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = permission.RoleToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// GetAllRole godoc
// @Summary 查询所有角色
// @Tags 角色管理
// @Param authorization header string true "jwt token"
// @Param tenantID query string false "租户ID"
// @Param containerComm query bool false "是否包含公共角色"
// @Success 200 {object} apipb.QueryRoleResponse
// @Router /api/core/auth/role/all [get]
func GetAllRole(c *gin.Context) {
	resp := &apipb.QueryRoleResponse{Code: apipb.Code_Success}
	req := &apipb.GetAllRoleRequest{}
	if err := c.BindQuery(req); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	roles, err := permission.GetAllRole(req.TenantID, req.ContainerComm)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = permission.RolesToPB(roles)
	resp.Records = int64(len(roles))
	resp.Pages = 1
	c.JSON(http.StatusOK, resp)
}

// ExportRole godoc
// @Summary 导出
// @Tags 角色管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/export [get]
func ExportRole(c *gin.Context) {
	req := &apipb.QueryRoleRequest{}
	resp := &apipb.QueryRoleResponse{Code: apipb.Code_Success}
	if err := c.BindQuery(req); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	req.PageIndex = 1
	req.PageSize = 1000
	permission.QueryRole(req, resp, true)
	if resp.Code != apipb.Code_Success {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment;filename=Role.json")
	c.Header("Content-Transfer-Encoding", "binary")
	buf, _ := json.Marshal(resp.Data)
	c.Writer.Write(buf)
}

// ImportRole godoc
// @Summary 导入
// @Tags 角色管理
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/import [post]
func ImportRole(c *gin.Context) {
	resp := &apipb.QueryRoleResponse{Code: apipb.Code_Success}
	file, _, err := c.Request.FormFile("files")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	defer file.Close()
	buf, err := io.ReadAll(file)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	var list []*apipb.RoleInfo
	if err := json.Unmarshal(buf, &list); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	successCount, failCount := 0, 0
	currentTenantID := ucm.GetTenantID(c)
	for _, f := range list {
		if currentTenantID != constants.PlatformTenantID {
			f.TenantID = currentTenantID
			f.Public = false
			existing, lookupErr := permission.GetRoleByID(f.Id)
			if lookupErr != nil && lookupErr != gorm.ErrRecordNotFound {
				failCount++
				continue
			}
			if lookupErr == nil && (existing.Public || existing.TenantID != currentTenantID) {
				failCount++
				continue
			}
		}
		itemErr := permission.UpdateRole(permission.PBToRole(f))
		if itemErr != nil {
			if itemErr == gorm.ErrRecordNotFound {
				itemErr = permission.CreateRole(permission.PBToRole(f), tenant.GetTenantUserCount)
			}
		}
		if itemErr != nil {
			failCount++
		} else {
			successCount++
		}
	}
	resp.Message = fmt.Sprintf("导入成功数量:%d,导入失败数量:%d", successCount, failCount)
	c.JSON(http.StatusOK, resp)
}

type roleAuthorizationRequest struct {
	RoleID     string                                  `json:"roleID" binding:"required"`
	Selections []permission.RoleAuthorizationSelection `json:"selections"`
}

type publishRoleAuthorizationRequest struct {
	RoleID       string                                  `json:"roleID" binding:"required"`
	BaseRevision string                                  `json:"baseRevision" binding:"required"`
	Selections   []permission.RoleAuthorizationSelection `json:"selections"`
}

func roleAuthorizationError(c *gin.Context, err error) {
	message := err.Error()
	if errors.Is(err, permission.ErrRoleAuthorizationRevisionConflict) {
		c.JSON(http.StatusOK, gin.H{
			"code":    apipb.Code_BadRequest,
			"message": message,
			"data": gin.H{
				"conflict": true,
			},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    apipb.Code_BadRequest,
		"message": message,
	})
}

// GetRoleAuthorization returns the native role authorization source, tenant
// scoped candidates, current revision and the exact active Casbin projection.
func GetRoleAuthorization(c *gin.Context) {
	roleID := c.Query("id")
	if strings.TrimSpace(roleID) == "" {
		roleAuthorizationError(c, errors.New("role ID cannot be empty"))
		return
	}
	if !canManageRole(c, roleID) {
		c.JSON(http.StatusOK, noRolePermissionResponse())
		return
	}
	detail, err := permission.GetRoleAuthorization(roleID)
	if err != nil {
		roleAuthorizationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": detail})
}

// PreviewRoleAuthorization validates a proposed selection and projects its
// Casbin policies without modifying role_menus, sessions or policy storage.
func PreviewRoleAuthorization(c *gin.Context) {
	req := &roleAuthorizationRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		roleAuthorizationError(c, err)
		return
	}
	if !canManageRole(c, req.RoleID) {
		c.JSON(http.StatusOK, noRolePermissionResponse())
		return
	}
	preview, err := permission.PreviewRoleAuthorization(req.RoleID, req.Selections)
	if err != nil {
		roleAuthorizationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": preview})
}

// PublishRoleAuthorization atomically replaces role menu selections and their
// Casbin projection, then revokes every affected login context.
func PublishRoleAuthorization(c *gin.Context) {
	req := &publishRoleAuthorizationRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		roleAuthorizationError(c, err)
		return
	}
	if !canManageRole(c, req.RoleID) {
		c.JSON(http.StatusOK, noRolePermissionResponse())
		return
	}
	result, err := permission.PublishRoleAuthorization(req.RoleID, req.BaseRevision, req.Selections)
	if err != nil {
		roleAuthorizationError(c, err)
		return
	}
	currentSessionRevoked := false
	currentUserID := ucm.GetUserID(c)
	for _, userID := range result.AffectedUserIDs {
		if userID == currentUserID {
			currentSessionRevoked = true
			break
		}
	}
	detail, _ := json.Marshal(map[string]interface{}{
		"revision":        result.Revision,
		"summary":         result.Summary,
		"sessionsRevoked": result.SessionsRevoked,
	})
	audit.RecordAuditWithKind(
		store.DB(),
		ucm.GetUserID(c),
		ucm.GetUserName(c),
		int32(ucm.GetPrincipalKind(c)),
		audit.AuditActionPublishRoleAuth,
		req.RoleID,
		c.ClientIP(),
		string(detail),
	)
	c.JSON(http.StatusOK, gin.H{
		"code": apipb.Code_Success,
		"data": gin.H{
			"revision":              result.Revision,
			"summary":               result.Summary,
			"sessionsRevoked":       result.SessionsRevoked,
			"currentSessionRevoked": currentSessionRevoked,
		},
	})
}

func RegisterRoleRouter(r *gin.Engine) {
	roleGroup := r.Group("/api/core/auth/role")
	roleGroup.POST("add", AutoHandler(AddRole))
	roleGroup.PUT("update", AutoHandler(UpdateRole))
	roleGroup.POST("enable", AutoHandler(EnableRole))
	roleGroup.GET("query", AutoQueryHandler(QueryRole))
	roleGroup.DELETE("delete", AutoHandler(DeleteRole))
	roleGroup.GET("all", GetAllRole)
	roleGroup.GET("detail", GetRoleDetail)
	roleGroup.GET("authorization", GetRoleAuthorization)
	roleGroup.POST("authorization/preview", PreviewRoleAuthorization)
	roleGroup.PUT("authorization", PublishRoleAuthorization)
	roleGroup.GET("export", ExportRole)
	roleGroup.POST("import", ImportRole)
}
