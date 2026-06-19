package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/CloudSilk/pkg/constants"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucmodel "github.com/CloudSilk/usercenter/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AddTenant godoc
// @Summary 新增租户
// @Tags 租户管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.TenantInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/tenant/add [post]
func AddTenant(c *gin.Context, req *apipb.TenantInfo) (*apipb.CommonResponse, error) {
	if err := ucmodel.CreateTenant(ucmodel.PBToTenant(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// UpdateTenant godoc
// @Summary 更新租户
// @Tags 租户管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.TenantInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/tenant/update [put]
func UpdateTenant(c *gin.Context, req *apipb.TenantInfo) (*apipb.CommonResponse, error) {
	if req.Id == constants.PlatformTenantID {
		return &apipb.CommonResponse{Code: apipb.Code_BadRequest, Message: "平台租户不允许更新"}, nil
	}
	if err := ucmodel.UpdateTenant(ucmodel.PBToTenant(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteTenant godoc
// @Summary 删除租户
// @Tags 租户管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/tenant/delete [delete]
func DeleteTenant(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	if req.Id == "" {
		return &apipb.CommonResponse{Code: apipb.Code_BadRequest, Message: "id不能为空"}, nil
	}
	if req.Id == constants.PlatformTenantID {
		return &apipb.CommonResponse{Code: apipb.Code_BadRequest, Message: "平台租户不允许删除"}, nil
	}
	if err := ucmodel.DeleteTenant(req.Id); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// EnableTenant godoc
// @Summary 禁用/启用租户
// @Tags 租户管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.EnableRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/tenant/enable [post]
func EnableTenant(c *gin.Context, req *apipb.EnableRequest) (*apipb.CommonResponse, error) {
	if req.Id == constants.PlatformTenantID {
		return &apipb.CommonResponse{Code: apipb.Code_BadRequest, Message: "平台租户不允许更新"}, nil
	}
	if err := ucmodel.EnableTenant(req.Id, req.Enable); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QueryTenant godoc
// @Summary 分页查询
// @Tags 租户管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QueryTenantResponse
// @Router /api/core/auth/tenant/query [get]
func QueryTenant(c *gin.Context, req *apipb.QueryTenantRequest) (*apipb.QueryTenantResponse, error) {
	resp := &apipb.QueryTenantResponse{Code: apipb.Code_Success}
	ucmodel.QueryTenant(req, resp)
	return resp, nil
}

// GetAllTenant godoc
// @Summary 查询所有租户
// @Tags 租户管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.QueryTenantResponse
// @Router /api/core/auth/tenant/all [get]
func GetAllTenant(c *gin.Context) {
	resp := &apipb.GetAllTenantResponse{Code: apipb.Code_Success}
	data, err := ucmodel.GetAllTenant()
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = ucmodel.TenantsToPB(data)
	c.JSON(http.StatusOK, resp)
}

// GetTenantDetail godoc
// @Summary 查询明细
// @Tags 租户管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetTenantDetailResponse
// @Router /api/core/auth/tenant/detail [get]
func GetTenantDetail(c *gin.Context) {
	resp := &apipb.GetTenantDetailResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	data, err := ucmodel.GetTenantByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = ucmodel.TenantToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// CopyTenant godoc
// @Summary 复制租户
// @Tags 租户管理
// @Param authorization header string true "jwt token"
// @Param id query string true "ID"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/tenant/copy [post]
func CopyTenant(c *gin.Context) {
	resp := &apipb.CommonResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	if err := ucmodel.CopyTenant(idStr); err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// ExportTenant godoc
// @Summary 导出
// @Tags 租户管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/auth/tenant/export [get]
func ExportTenant(c *gin.Context) {
	req := &apipb.QueryTenantRequest{}
	resp := &apipb.QueryTenantResponse{Code: apipb.Code_Success}
	if err := c.BindQuery(req); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	req.PageIndex = 1
	req.PageSize = 1000
	ucmodel.QueryTenant(req, resp)
	if resp.Code != apipb.Code_Success {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment;filename=Tenant.json")
	c.Header("Content-Transfer-Encoding", "binary")
	buf, _ := json.Marshal(resp.Data)
	c.Writer.Write(buf)
}

// ImportTenant godoc
// @Summary 导入
// @Tags 租户管理
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/auth/tenant/import [post]
func ImportTenant(c *gin.Context) {
	resp := &apipb.QueryTenantResponse{Code: apipb.Code_Success}
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
	var list []*apipb.TenantInfo
	if err := json.Unmarshal(buf, &list); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	successCount, failCount := 0, 0
	for _, f := range list {
		if err := ucmodel.UpdateTenant(ucmodel.PBToTenant(f)); err != nil {
			if err == gorm.ErrRecordNotFound {
				err = ucmodel.CreateTenant(ucmodel.PBToTenant(f))
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

func RegisterTenantRouter(r *gin.Engine) {
	g := r.Group("/api/core/auth/tenant")
	g.POST("add", AutoHandler(AddTenant))
	g.PUT("update", AutoHandler(UpdateTenant))
	g.GET("query", AutoQueryHandler(QueryTenant))
	g.DELETE("delete", AutoHandler(DeleteTenant))
	g.GET("all", GetAllTenant)
	g.GET("detail", GetTenantDetail)
	g.POST("copy", CopyTenant)
	g.POST("enable", AutoHandler(EnableTenant))
	g.GET("export", ExportTenant)
	g.POST("import", ImportTenant)
}
