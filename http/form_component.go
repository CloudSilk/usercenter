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

// AddFormComponent godoc
// @Summary 新增
// @Description 新增
// @Tags 表单组件管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.FormComponentInfo true "Add FormComponent"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/form/component/add [post]
func AddFormComponent(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.FormComponentInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建表单组件请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}

	id, err := model.CreateFormComponent(model.PBToFormComponent(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateFormComponent godoc
// @Summary 更新
// @Description 更新
// @Tags 表单组件管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.FormComponentInfo true "Update FormComponent"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/form/component/update [put]
func UpdateFormComponent(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.FormComponentInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,更新表单组件请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.UpdateFormComponent(model.PBToFormComponent(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteFormComponent godoc
// @Summary 删除
// @Description 删除
// @Tags 表单组件管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete FormComponent"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/form/component/delete [delete]
func DeleteFormComponent(c *gin.Context) {
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
		log.Warnf(context.Background(), "TransID:%s,删除表单组件请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.DeleteFormComponent(req.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryFormComponent godoc
// @Summary 分页查询
// @Description 分页查询
// @Tags 表单组件管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param name query string false "组件名称"
// @Success 200 {object} apipb.QueryFormComponentResponse
// @Router /api/core/auth/form/component/query [get]
func QueryFormComponent(c *gin.Context) {
	req := &apipb.QueryFormComponentRequest{}
	resp := &apipb.QueryFormComponentResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	model.QueryFormComponent(req, resp, false)

	c.JSON(http.StatusOK, resp)
}

// GetFormComponentDetail godoc
// @Summary 查询明细
// @Description 查询明细
// @Tags 表单组件管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetFormComponentDetailResponse
// @Router /api/core/auth/form/component/detail [get]
func GetFormComponentDetail(c *gin.Context) {
	resp := &apipb.GetFormComponentDetailResponse{
		Code: apipb.Code_Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := model.GetFormComponentByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.FormComponentToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterFormComponentRouter(r *gin.Engine) {
	g := r.Group("/api/core/auth/form/component")

	g.POST("add", AddFormComponent)
	g.PUT("update", UpdateFormComponent)
	g.GET("query", QueryFormComponent)
	g.DELETE("delete", DeleteFormComponent)
	g.GET("detail", GetFormComponentDetail)
}
