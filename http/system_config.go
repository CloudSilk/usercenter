package http

import (
	"context"
	"net/http"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/pkg/utils/middleware"
	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

// AddSystemConfig godoc
// @Summary 新增
// @Description 新增
// @Tags 系统配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param account body apipb.SystemConfigInfo true "Add SystemConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/system/config/add [post]
func AddSystemConfig(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.SystemConfigInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建系统配置请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}

	id, err := model.CreateSystemConfig(model.PBToSystemConfig(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateSystemConfig godoc
// @Summary 更新
// @Description 更新
// @Tags 系统配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param account body apipb.SystemConfigInfo true "Update SystemConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/system/config/update [put]
func UpdateSystemConfig(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.SystemConfigInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,更新系统配置请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.UpdateSystemConfig(model.PBToSystemConfig(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteSystemConfig godoc
// @Summary 删除
// @Description 删除
// @Tags 系统配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete SystemConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/system/config/delete [delete]
func DeleteSystemConfig(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.DelRequest{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,删除系统配置请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.DeleteSystemConfig(req.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QuerySystemConfig godoc
// @Summary 分页查询
// @Description 分页查询
// @Tags 系统配置管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Success 200 {object} apipb.QuerySystemConfigResponse
// @Router /api/core/system/config/query [get]
func QuerySystemConfig(c *gin.Context) {
	req := &apipb.QuerySystemConfigRequest{}
	resp := &apipb.QuerySystemConfigResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	model.QuerySystemConfig(req, resp, false)

	c.JSON(http.StatusOK, resp)
}

// GetSystemConfigDetail godoc
// @Summary 查询明细
// @Description 查询明细
// @Tags 系统配置管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetSystemConfigDetailResponse
// @Router /api/core/system/config/detail [get]
func GetSystemConfigDetail(c *gin.Context) {
	resp := &apipb.GetSystemConfigDetailResponse{
		Code: apipb.Code_Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := model.GetSystemConfigByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.SystemConfigToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// GetAllSystemConfig godoc
// @Summary 查询所有
// @Description 查询所有
// @Tags 系统配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAllSystemConfigResponse
// @Router /api/core/system/config/all [get]
func GetAllSystemConfig(c *gin.Context) {
	resp := &apipb.GetAllSystemConfigResponse{
		Code: apipb.Code_Success,
	}
	list, err := model.GetAllSystemConfigs()
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = model.SystemConfigsToPB(list)
	c.JSON(http.StatusOK, resp)
}

func RegisterSystemConfigRouter(r *gin.Engine) {
	g := r.Group("/api/core/system/config")

	g.POST("add", AddSystemConfig)
	g.PUT("update", UpdateSystemConfig)
	g.GET("query", QuerySystemConfig)
	g.DELETE("delete", DeleteSystemConfig)
	g.GET("detail", GetSystemConfigDetail)
}
