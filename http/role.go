package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/tenant"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AddRole godoc
// @Summary 新增角色
// @Tags 角色管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.RoleInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/add [post]
func AddRole(c *gin.Context, req *apipb.RoleInfo) (*model.CommonResponse, error) {
	//只有平台租户才能为其他租户创建角色
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	if err := permission.CreateRole(permission.PBToRole(req), tenant.GetTenantUserCount); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

// UpdateRole godoc
// @Summary 更新角色
// @Tags 角色管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.RoleInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/update [put]
func UpdateRole(c *gin.Context, req *apipb.RoleInfo) (*model.CommonResponse, error) {
	//只有平台租户才能更改角色的租户
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	if err := permission.UpdateRole(permission.PBToRole(req)); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

// DeleteRole godoc
// @Summary 删除角色
// @Tags 角色管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/role/delete [delete]
func DeleteRole(c *gin.Context, req *apipb.DelRequest) (*model.CommonResponse, error) {
	if err := permission.DeleteRole(req.Id); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
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
	for _, f := range list {
		if err := permission.UpdateRole(permission.PBToRole(f)); err != nil {
			if err == gorm.ErrRecordNotFound {
				err = permission.CreateRole(permission.PBToRole(f), tenant.GetTenantUserCount)
			}
		}
		if err != nil {
			failCount++
		} else {
			successCount++
		}
	}
	resp.Message = fmt.Sprintf("导入成功数量:%d,导入失败数量:%d", successCount, failCount)
	c.JSON(http.StatusOK, resp)
}

func RegisterRoleRouter(r *gin.Engine) {
	roleGroup := r.Group("/api/core/auth/role")
	roleGroup.POST("add", AutoHandler(AddRole))
	roleGroup.PUT("update", AutoHandler(UpdateRole))
	roleGroup.GET("query", AutoQueryHandler(QueryRole))
	roleGroup.DELETE("delete", AutoHandler(DeleteRole))
	roleGroup.GET("all", GetAllRole)
	roleGroup.GET("detail", GetRoleDetail)
	roleGroup.GET("export", ExportRole)
	roleGroup.POST("import", ImportRole)
}
