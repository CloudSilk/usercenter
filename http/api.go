package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/usercenter/internal/permission"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func apiMutationResponse(err error) *apipb.CommonResponse {
	if err == nil {
		return &apipb.CommonResponse{Code: apipb.Code_Success}
	}
	code := apipb.Code_InternalServerError
	if errors.Is(err, permission.ErrInvalidAPIResource) ||
		errors.Is(err, permission.ErrProtectedAPIResource) ||
		errors.Is(err, permission.ErrBoundAPIResource) {
		code = apipb.Code_BadRequest
	}
	return &apipb.CommonResponse{Code: code, Message: err.Error()}
}

func canManageAPIResource(c *gin.Context, apiID string) bool {
	currentTenantID := ucm.GetTenantID(c)
	if currentTenantID == constants.PlatformTenantID {
		return true
	}
	api, err := permission.GetAPIById(apiID)
	return err == nil && api.TenantID == currentTenantID
}

func noAPIResourcePermissionResponse() *apipb.CommonResponse {
	return &apipb.CommonResponse{Code: apipb.Code_NoPermission, Message: "no permission to manage this API resource"}
}

// AddAPI godoc
// @Summary 新增API
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.APIInfo true "Add API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/add [post]
func AddAPI(c *gin.Context, req *permission.API) (*apipb.CommonResponse, error) {
	req.IsMust = false
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	} else if req.TenantID == "" {
		req.TenantID = constants.PlatformTenantID
	}
	if err := permission.CreateAPIResource(req); err != nil {
		return apiMutationResponse(err), nil
	}
	recordAudit(c, "create_api_resource", req.ID, fmt.Sprintf("%s %s", req.Method, req.Path))
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// UpdateAPI godoc
// @Summary 更新API
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.APIInfo true "Update API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/update [put]
func UpdateAPI(c *gin.Context, req *permission.API) (*apipb.CommonResponse, error) {
	if !canManageAPIResource(c, req.ID) {
		return noAPIResourcePermissionResponse(), nil
	}
	current, err := permission.GetAPIById(req.ID)
	if err != nil {
		return apiMutationResponse(err), nil
	}
	req.TenantID = current.TenantID
	req.ProjectID = current.ProjectID
	req.IsMust = current.IsMust
	if err := permission.UpdateAPIResource(req); err != nil {
		return apiMutationResponse(err), nil
	}
	recordAudit(c, "update_api_resource", req.ID, fmt.Sprintf("%s %s", req.Method, req.Path))
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteAPI godoc
// @Summary 删除API
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/delete [delete]
func DeleteAPI(c *gin.Context, req *permission.API) (*apipb.CommonResponse, error) {
	if !canManageAPIResource(c, req.ID) {
		return noAPIResourcePermissionResponse(), nil
	}
	if err := permission.DeleteAPIResource(req.ID); err != nil {
		return apiMutationResponse(err), nil
	}
	recordAudit(c, "delete_api_resource", req.ID, "")
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// EnableAPI godoc
// @Summary 禁用/启用API
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.EnableRequest true "Enable/Disable API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/enable [post]
func EnableAPI(c *gin.Context, req *permission.API) (*apipb.CommonResponse, error) {
	if !canManageAPIResource(c, req.ID) {
		return noAPIResourcePermissionResponse(), nil
	}
	if err := permission.EnableAPIResource(req.ID, req.Enable); err != nil {
		return apiMutationResponse(err), nil
	}
	recordAudit(c, "set_api_resource_enabled", req.ID, fmt.Sprintf("enable=%t", req.Enable))
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QueryAPI godoc
// @Summary 分页查询
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QueryAPIResponse
// @Router /api/core/auth/api/query [get]
func QueryAPI(c *gin.Context, req *apipb.QueryAPIRequest) (*apipb.QueryAPIResponse, error) {
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	resp := &apipb.QueryAPIResponse{Code: apipb.Code_Success}
	var enable *bool
	if raw := c.Query("enable"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			resp.Code = apipb.Code_BadRequest
			resp.Message = "enable must be true or false"
			return resp, nil
		}
		enable = &value
	}
	permission.QueryAPIResources(req, resp, c.Query("keyword"), enable)
	return resp, nil
}

// GetAllAPI godoc
// @Summary 查询所有API
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAllAPIResponse
// @Router /api/core/auth/api/all [get]
func GetAllAPI(c *gin.Context) {
	req := &apipb.QueryAPIRequest{}
	if err := c.BindQuery(req); err != nil {
		writeBadRequest(c, err)
		return
	}
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	apis, err := permission.GetAllAPIs(req)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    apipb.Code_Success,
		"data":    permission.APIsToPB(apis),
		"records": int64(len(apis)),
		"pages":   1,
	})
}

// GetAPIDetail godoc
// @Summary 查询明细
// @Tags API管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAPIDetailResponse
// @Router /api/core/auth/api/detail [get]
func GetAPIDetail(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		writeBadRequest(c, errStr("id required"))
		return
	}
	if !canManageAPIResource(c, idStr) {
		c.JSON(http.StatusOK, noAPIResourcePermissionResponse())
		return
	}
	data, err := permission.GetAPIById(idStr)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeOK(c, gin.H{"data": data})
}

func GetAPIImpact(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		writeBadRequest(c, errStr("id required"))
		return
	}
	if !canManageAPIResource(c, idStr) {
		c.JSON(http.StatusOK, noAPIResourcePermissionResponse())
		return
	}
	impact, err := permission.GetAPIResourceImpact(idStr)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeOK(c, gin.H{"data": impact})
}

// ImportAPI godoc
// @Summary 导入
// @Tags API管理
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/import [post]
func ImportAPI(c *gin.Context) {
	file, _, err := c.Request.FormFile("files")
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	defer file.Close()
	buf, err := io.ReadAll(file)
	if err != nil {
		writeBadRequest(c, err)
		return
	}
	var list []*apipb.APIInfo
	if err := json.Unmarshal(buf, &list); err != nil {
		writeBadRequest(c, err)
		return
	}
	successCount, failCount := 0, 0
	for _, f := range list {
		if err := permission.UpdateAPI(permission.PBToAPI(f)); err != nil {
			if err == gorm.ErrRecordNotFound {
				err = permission.CreateAPI(permission.PBToAPI(f))
			}
		}
		if err != nil {
			failCount++
		} else {
			successCount++
		}
	}
	writeOK(c, gin.H{
		"code":    apipb.Code_Success,
		"message": fmt.Sprintf("导入成功数量:%d,导入失败数量:%d", successCount, failCount),
	})
}

// ExportAPI godoc
// @Summary 导出
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/export [get]
func ExportAPI(c *gin.Context) {
	req := &apipb.QueryAPIRequest{}
	if err := c.BindQuery(req); err != nil {
		writeBadRequest(c, err)
		return
	}
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	req.PageIndex = 1
	req.PageSize = 1000
	resp := &apipb.QueryAPIResponse{Code: apipb.Code_Success}
	permission.QueryAPI(req, resp)
	if resp.Code != apipb.Code_Success {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment;filename=API.json")
	c.Header("Content-Transfer-Encoding", "binary")
	buf, _ := json.Marshal(resp.Data)
	c.Writer.Write(buf)
}

func RegisterAPIRouter(r *gin.Engine) {
	apiGroup := r.Group("/api/core/auth/api")
	apiGroup.POST("add", AutoHandler(AddAPI))
	apiGroup.PUT("update", AutoHandler(UpdateAPI))
	apiGroup.GET("query", AutoQueryHandler(QueryAPI))
	apiGroup.DELETE("delete", AutoHandler(DeleteAPI))
	apiGroup.POST("enable", AutoHandler(EnableAPI))
	apiGroup.GET("all", GetAllAPI)
	apiGroup.GET("detail", GetAPIDetail)
	apiGroup.GET("impact", GetAPIImpact)
	apiGroup.GET("export", ExportAPI)
	apiGroup.POST("import", ImportAPI)
}
