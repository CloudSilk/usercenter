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
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AddTenant
// @Summary 新增租户
// @Description 新增租户
// @Tags 租户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.TenantInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/tenant/add [post]
func AddTenant(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.TenantInfo{}
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建租户请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.CreateTenant(ucmodel.PBToTenant(req))
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateTenant
// @Summary 更新租户
// @Description 更新租户
// @Tags 租户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.TenantInfo true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/tenant/update [put]
func UpdateTenant(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.TenantInfo{}
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建租户请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	if req.Id == constants.PlatformTenantID {
		resp.Code = model.BadRequest
		resp.Message = "平台租户不允许更新"
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.UpdateTenant(ucmodel.PBToTenant(req))
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteTenant
// @Summary 删除租户
// @Description 软删除租户
// @Tags 租户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/tenant/delete [delete]
func DeleteTenant(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.DelRequest{}
	resp := &apipb.CommonResponse{
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

	if req.Id == "" {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	if req.Id == constants.PlatformTenantID {
		resp.Code = model.BadRequest
		resp.Message = "平台租户不允许删除"
		c.JSON(http.StatusOK, resp)
		return
	}

	err = ucmodel.DeleteTenant(req.Id)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// EnableTenant
// @Summary 禁用/启用租户
// @Description 禁用/启用租户
// @Tags 租户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.EnableRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/tenant/enable [post]
func EnableTenant(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.EnableRequest{}
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建租户请求参数无效:%v", transID, err)
		return
	}
	if req.Id == constants.PlatformTenantID {
		resp.Code = model.BadRequest
		resp.Message = "平台租户不允许更新"
		c.JSON(http.StatusOK, resp)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.EnableTenant(req.Id, req.Enable)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryTenant
// @Summary 分页查询
// @Description 分页查询
// @Tags 租户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param tenantID query string false "租户ID"
// @Param name query string false "名称"
// @Success 200 {object} apipb.QueryTenantResponse
// @Router /api/core/auth/tenant/query [get]
func QueryTenant(c *gin.Context) {
	req := &apipb.QueryTenantRequest{}
	resp := &apipb.QueryTenantResponse{
		Code: model.Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	ucmodel.QueryTenant(req, resp)

	c.JSON(http.StatusOK, resp)
}

// GetAllTenant
// @Summary 查询所有租户
// @Description 查询所有租户
// @Tags 租户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.QueryTenantResponse
// @Router /api/core/auth/tenant/all [get]
func GetAllTenant(c *gin.Context) {
	resp := &apipb.GetAllTenantResponse{
		Code: model.Success,
	}
	data, err := ucmodel.GetAllTenant()
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = ucmodel.TenantsToPB(data)
	c.JSON(http.StatusOK, resp)
}

// GetTenantDetail
// @Summary 查询明细
// @Description 查询明细
// @Tags 租户管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetTenantDetailResponse
// @Router /api/core/auth/tenant/detail [get]
func GetTenantDetail(c *gin.Context) {
	resp := &apipb.GetTenantDetailResponse{
		Code: model.Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = model.BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := ucmodel.GetTenantByID(idStr)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = ucmodel.TenantToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// CopyTenant
// @Summary 禁用/启用租户
// @Description 禁用/启用租户
// @Tags 租户管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.EnableRequest true "请求参数"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/tenant/enable [post]
func CopyTenant(c *gin.Context) {
	resp := &apipb.CommonResponse{
		Code: model.Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = model.BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}

	err := ucmodel.CopyTenant(idStr)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// ExportTenant godoc
// @Summary 导出
// @Description 导出
// @Tags 租户管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param name query string false "租户名称"
// @Param ids query []string false "IDs"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/auth/tenant/export [get]
func ExportTenant(c *gin.Context) {
	req := &apipb.QueryTenantRequest{}
	resp := &apipb.QueryTenantResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
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

// ImportTenant
// @Summary 导入
// @Description 导入
// @Tags 租户管理
// @Accept  mpfd
// @Produce  json
// @Param authorization header string true "Bearer+空格+Token"
// @Param files formData file true "要上传的文件"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/auth/tenant/import [post]
func ImportTenant(c *gin.Context) {
	resp := &apipb.QueryTenantResponse{
		Code: apipb.Code_Success,
	}
	//从租户中读取文件
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

	var list []*apipb.TenantInfo
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
		err = ucmodel.UpdateTenant(ucmodel.PBToTenant(f))
		if err == gorm.ErrRecordNotFound {
			err = ucmodel.CreateTenant(ucmodel.PBToTenant(f))
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

func RegisterTenantRouter(r *gin.Engine) {
	g := r.Group("/api/core/auth/tenant")

	g.POST("add", AddTenant)
	g.PUT("update", UpdateTenant)
	g.GET("query", QueryTenant)
	g.DELETE("delete", DeleteTenant)
	g.GET("all", GetAllTenant)
	g.GET("detail", GetTenantDetail)
	g.POST("copy", CopyTenant)
	g.POST("enable", EnableTenant)
	g.GET("export", ExportTenant)
	g.POST("import", ImportTenant)
}
