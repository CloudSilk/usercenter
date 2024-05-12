package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	ucmodel "github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/utils/middleware"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AddAPI godoc
// @Summary 新增API
// @Description 新增API
// @Tags API管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.APIInfo true "Add API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/add [post]
func AddAPI(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &ucmodel.API{}
	resp := &model.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建API请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	tenantID := ucm.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	err = ucmodel.CreateAPI(req)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateAPI godoc
// @Summary 更新API
// @Description 更新API
// @Tags API管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.APIInfo true "Update API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/update [put]
func UpdateAPI(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &ucmodel.API{}
	resp := &model.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建API请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.UpdateAPI(req)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteAPI godoc
// @Summary 删除API
// @Description 软删除API
// @Tags API管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/delete [delete]
func DeleteAPI(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &ucmodel.API{}
	resp := &model.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建API请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.DeleteApi(req.ID)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// EnableAPI godoc
// @Summary 禁用/启用API
// @Description 禁用/启用API
// @Tags API管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.EnableRequest true "Enable/Disable API"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/enable [post]
func EnableAPI(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &ucmodel.API{}
	resp := &model.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建API请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.EnableAPI(req.ID, req.Enable)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryAPI godoc
// @Summary 分页查询
// @Description 分页查询
// @Tags API管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param path query string false "路径"
// @Param method query string false "方法"
// @Param group query string false "分组"
// @Param checkAuth query string false "是否检查权限"
// @Param checkLogin query string false "是否需要登录"
// @Success 200 {object} apipb.QueryAPIResponse
// @Router /api/core/auth/api/query [get]
func QueryAPI(c *gin.Context) {
	req := &apipb.QueryAPIRequest{}
	resp := &apipb.QueryAPIResponse{
		Code: model.Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	tenantID := ucm.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	ucmodel.QueryAPI(req, resp)

	c.JSON(http.StatusOK, resp)
}

// GetAllAPI godoc
// @Summary 查询所有API
// @Description 查询所有API
// @Tags API管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAllAPIResponse
// @Router /api/core/auth/api/all [get]
func GetAllAPI(c *gin.Context) {
	resp := &apipb.QueryAPIResponse{
		Code: model.Success,
	}
	req := &apipb.QueryAPIRequest{}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	apis, err := ucmodel.GetAllAPIs(req)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	tenantID := ucm.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
		req.TenantID = tenantID
	}
	resp.Data = ucmodel.APIsToPB(apis)
	resp.Records = int64(len(apis))
	resp.Pages = 1
	c.JSON(http.StatusOK, resp)
}

// GetAPIDetail godoc
// @Summary 查询明细
// @Description 查询明细
// @Tags API管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAPIDetailResponse
// @Router /api/core/auth/api/detail [get]
func GetAPIDetail(c *gin.Context) {
	resp := model.CommonDetailResponse{
		CommonResponse: model.CommonResponse{
			Code: model.Success,
		},
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = model.BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	resp.Data, err = ucmodel.GetAPIById(idStr)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// ImportAPI
// @Summary 导入
// @Description 导入
// @Tags API管理
// @Accept  mpfd
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/import [post]
func ImportAPI(c *gin.Context) {
	resp := &apipb.QueryAPIResponse{
		Code: apipb.Code_Success,
	}
	//从API中读取文件
	file, fileHeader, err := c.Request.FormFile("files")
	if err != nil {
		fmt.Println(err)
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	//defer 结束时关闭文件
	defer file.Close()
	fmt.Println("filename: " + fileHeader.Filename)
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	var list []*apipb.APIInfo
	err = json.Unmarshal(buf, &list)
	if err != nil {
		fmt.Println(err)
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	successCount := 0
	failCount := 0
	for _, f := range list {
		err = ucmodel.UpdateAPI(ucmodel.PBToAPI(f))
		if err == gorm.ErrRecordNotFound {
			err = ucmodel.CreateAPI(ucmodel.PBToAPI(f))
		}
		if err != nil {
			failCount++
			fmt.Println(err)
		} else {
			successCount++
		}
	}
	resp.Message = fmt.Sprintf("导入成功数量:%d,导入失败数量:%d", successCount, failCount)
	c.JSON(http.StatusOK, resp)
}

// ExportAPI godoc
// @Summary 导出
// @Description 导出
// @Tags API管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param group query string false "Group"
// @Param method query string false "Method"
// @Param path query string false "Path"
// @Param ids query []string false "IDs"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/api/export [get]
func ExportAPI(c *gin.Context) {
	req := &apipb.QueryAPIRequest{}
	resp := &apipb.QueryAPIResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	tenantID := ucm.GetTenantID(c)
	if tenantID != constants.PlatformTenantID {
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
	apiGroup.POST("add", AddAPI)
	apiGroup.PUT("update", UpdateAPI)
	apiGroup.GET("query", QueryAPI)
	apiGroup.DELETE("delete", DeleteAPI)
	apiGroup.POST("enable", EnableAPI)
	apiGroup.GET("all", GetAllAPI)
	apiGroup.GET("detail", GetAPIDetail)
	apiGroup.GET("export", ExportAPI)
	apiGroup.POST("import", ImportAPI)
}
