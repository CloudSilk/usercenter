package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/model"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucmodel "github.com/CloudSilk/usercenter/model"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AddAPI godoc
// @Summary 新增API
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.APIInfo true "Add API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/add [post]
func AddAPI(c *gin.Context, req *ucmodel.API) (*model.CommonResponse, error) {
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	if err := ucmodel.CreateAPI(req); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

// UpdateAPI godoc
// @Summary 更新API
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.APIInfo true "Update API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/update [put]
func UpdateAPI(c *gin.Context, req *ucmodel.API) (*model.CommonResponse, error) {
	if err := ucmodel.UpdateAPI(req); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

// DeleteAPI godoc
// @Summary 删除API
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/delete [delete]
func DeleteAPI(c *gin.Context, req *ucmodel.API) (*model.CommonResponse, error) {
	if err := ucmodel.DeleteApi(req.ID); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

// EnableAPI godoc
// @Summary 禁用/启用API
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.EnableRequest true "Enable/Disable API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/enable [post]
func EnableAPI(c *gin.Context, req *ucmodel.API) (*model.CommonResponse, error) {
	if err := ucmodel.EnableAPI(req.ID, req.Enable); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
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
	ucmodel.QueryAPI(req, resp)
	return resp, nil
}

// GetAllAPI godoc
// @Summary 查询所有API
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAllAPIResponse
// @Router /api/core/auth/api/all [get]
func GetAllAPI(c *gin.Context) {
	resp := &apipb.QueryAPIResponse{Code: apipb.Code_Success}
	req := &apipb.QueryAPIRequest{}
	if err := c.BindQuery(req); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	if tenantID := ucm.GetTenantID(c); tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	apis, err := ucmodel.GetAllAPIs(req)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = ucmodel.APIsToPB(apis)
	resp.Records = int64(len(apis))
	resp.Pages = 1
	c.JSON(http.StatusOK, resp)
}

// GetAPIDetail godoc
// @Summary 查询明细
// @Tags API管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAPIDetailResponse
// @Router /api/core/auth/api/detail [get]
func GetAPIDetail(c *gin.Context) {
	resp := model.CommonDetailResponse{CommonResponse: model.CommonResponse{Code: model.Success}}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = model.BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	data, err := ucmodel.GetAPIById(idStr)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = data
	}
	c.JSON(http.StatusOK, resp)
}

// ImportAPI godoc
// @Summary 导入
// @Tags API管理
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/import [post]
func ImportAPI(c *gin.Context) {
	resp := &apipb.QueryAPIResponse{Code: apipb.Code_Success}
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
	var list []*apipb.APIInfo
	if err := json.Unmarshal(buf, &list); err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	successCount, failCount := 0, 0
	for _, f := range list {
		if err := ucmodel.UpdateAPI(ucmodel.PBToAPI(f)); err != nil {
			if err == gorm.ErrRecordNotFound {
				err = ucmodel.CreateAPI(ucmodel.PBToAPI(f))
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

// ExportAPI godoc
// @Summary 导出
// @Tags API管理
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/export [get]
func ExportAPI(c *gin.Context) {
	req := &apipb.QueryAPIRequest{}
	resp := &apipb.QueryAPIResponse{Code: apipb.Code_Success}
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
	ucmodel.QueryAPI(req, resp)
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
	apiGroup.GET("export", ExportAPI)
	apiGroup.POST("import", ImportAPI)
}
