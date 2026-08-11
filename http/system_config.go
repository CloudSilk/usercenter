package http

import (
	"net/http"

	"github.com/CloudSilk/usercenter/internal/systemconfig"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

// AddSystemConfig godoc
// @Summary 新增
// @Tags 系统配置管理
// @Param authorization header string true "jwt token"
// @Param account body apipb.SystemConfigInfo true "Add SystemConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/system/config/add [post]
func AddSystemConfig(c *gin.Context, req *apipb.SystemConfigInfo) (*apipb.CommonResponse, error) {
	req.TenantID = scopedUserTenantID(c, req.TenantID)
	id, err := systemconfig.CreateSystemConfig(systemconfig.PBToSystemConfig(req))
	if err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success, Message: id}, nil
}

// UpdateSystemConfig godoc
// @Summary 更新
// @Tags 系统配置管理
// @Param authorization header string true "jwt token"
// @Param account body apipb.SystemConfigInfo true "Update SystemConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/system/config/update [put]
func UpdateSystemConfig(c *gin.Context, req *apipb.SystemConfigInfo) (*apipb.CommonResponse, error) {
	current, err := systemconfig.GetSystemConfigByID(req.Id)
	if err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	scope := scopedUserTenantID(c, req.TenantID)
	if current.TenantID != scope {
		return &apipb.CommonResponse{Code: apipb.Code_NoPermission, Message: "无权管理该租户的系统配置"}, nil
	}
	req.TenantID = current.TenantID
	if err := systemconfig.UpdateSystemConfig(systemconfig.PBToSystemConfig(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteSystemConfig godoc
// @Summary 删除
// @Tags 系统配置管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete SystemConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/system/config/delete [delete]
func DeleteSystemConfig(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	current, err := systemconfig.GetSystemConfigByID(req.Id)
	if err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	if current.TenantID != scopedUserTenantID(c, c.Query("tenantID")) {
		return &apipb.CommonResponse{Code: apipb.Code_NoPermission, Message: "无权管理该租户的系统配置"}, nil
	}
	if err := systemconfig.DeleteSystemConfig(req.Id); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QuerySystemConfig godoc
// @Summary 分页查询
// @Tags 系统配置管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QuerySystemConfigResponse
// @Router /api/core/system/config/query [get]
func QuerySystemConfig(c *gin.Context, req *apipb.QuerySystemConfigRequest) (*apipb.QuerySystemConfigResponse, error) {
	tenantID := scopedUserTenantID(c, c.Query("tenantID"))
	resp := &apipb.QuerySystemConfigResponse{Code: apipb.Code_Success}
	systemconfig.QuerySystemConfigForTenant(req, resp, false, tenantID)
	return resp, nil
}

// GetSystemConfigDetail godoc
// @Summary 查询明细
// @Tags 系统配置管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetSystemConfigDetailResponse
// @Router /api/core/system/config/detail [get]
func GetSystemConfigDetail(c *gin.Context) {
	resp := &apipb.GetSystemConfigDetailResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	data, err := systemconfig.GetSystemConfigByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else if data.TenantID != scopedUserTenantID(c, c.Query("tenantID")) {
		resp.Code = apipb.Code_NoPermission
		resp.Message = "无权查看该租户的系统配置"
	} else {
		resp.Data = systemconfig.SystemConfigToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterSystemConfigRouter(r *gin.Engine) {
	g := r.Group("/api/core/system/config")
	g.POST("add", AutoHandler(AddSystemConfig))
	g.PUT("update", AutoHandler(UpdateSystemConfig))
	g.GET("query", AutoQueryHandler(QuerySystemConfig))
	g.DELETE("delete", AutoHandler(DeleteSystemConfig))
	g.GET("detail", GetSystemConfigDetail)
}
