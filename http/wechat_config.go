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

// AddWechatConfig godoc
// @Summary 新增
// @Description 新增
// @Tags 微信应用配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param account body apipb.WechatConfigInfo true "Add WechatConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/wechat/config/add [post]
func AddWechatConfig(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.WechatConfigInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建微信应用配置请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}

	id, err := model.CreateWechatConfig(model.PBToWechatConfig(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateWechatConfig godoc
// @Summary 更新
// @Description 更新
// @Tags 微信应用配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param account body apipb.WechatConfigInfo true "Update WechatConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/wechat/config/update [put]
func UpdateWechatConfig(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.WechatConfigInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,更新微信应用配置请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.UpdateWechatConfig(model.PBToWechatConfig(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteWechatConfig godoc
// @Summary 删除
// @Description 删除
// @Tags 微信应用配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete WechatConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/wechat/config/delete [delete]
func DeleteWechatConfig(c *gin.Context) {
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
		log.Warnf(context.Background(), "TransID:%s,删除微信应用配置请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.DeleteWechatConfig(req.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryWechatConfig godoc
// @Summary 分页查询
// @Description 分页查询
// @Tags 微信应用配置管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param appName query string false "APP名称"
// @Param tenantID query string false "租户ID"
// @Param appType query int false "类型"
// @Success 200 {object} apipb.QueryWechatConfigResponse
// @Router /api/core/wechat/config/query [get]
func QueryWechatConfig(c *gin.Context) {
	req := &apipb.QueryWechatConfigRequest{}
	resp := &apipb.QueryWechatConfigResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	model.QueryWechatConfig(req, resp, false)

	c.JSON(http.StatusOK, resp)
}

// GetWechatConfigDetail godoc
// @Summary 查询明细
// @Description 查询明细
// @Tags 微信应用配置管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetWechatConfigDetailResponse
// @Router /api/core/wechat/config/detail [get]
func GetWechatConfigDetail(c *gin.Context) {
	resp := &apipb.GetWechatConfigDetailResponse{
		Code: apipb.Code_Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := model.GetWechatConfigByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.WechatConfigToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// GetAllWechatConfig godoc
// @Summary 查询所有
// @Description 查询所有
// @Tags 微信应用配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAllWechatConfigResponse
// @Router /api/core/wechat/config/all [get]
func GetAllWechatConfig(c *gin.Context) {
	resp := &apipb.GetAllWechatConfigResponse{
		Code: apipb.Code_Success,
	}
	list, err := model.GetAllWechatConfigs()
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = model.WechatConfigsToPB(list)
	c.JSON(http.StatusOK, resp)
}

func RegisterWechatConfigRouter(r *gin.Engine) {
	g := r.Group("/api/core/wechat/config")

	g.POST("add", AddWechatConfig)
	g.PUT("update", UpdateWechatConfig)
	g.GET("query", QueryWechatConfig)
	g.DELETE("delete", DeleteWechatConfig)
	g.GET("detail", GetWechatConfigDetail)
}
