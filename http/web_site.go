package http

import (
	"context"
	"net/http"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/pkg/utils/middleware"
	"github.com/CloudSilk/usercenter/model"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// AddWebSite godoc
// @Summary 新增
// @Description 新增
// @Tags 网站配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param account body apipb.WebSiteInfo true "Add WebSite"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/web_site/add [post]
func AddWebSite(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.WebSiteInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建网站配置请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	req.TenantID = ucm.GetTenantID(c)
	id, err := model.CreateWebSite(model.PBToWebSite(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateWebSite godoc
// @Summary 更新
// @Description 更新
// @Tags 网站配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param account body apipb.WebSiteInfo true "Update WebSite"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/web_site/update [put]
func UpdateWebSite(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.WebSiteInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,更新网站配置请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.UpdateWebSite(model.PBToWebSite(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteWebSite godoc
// @Summary 删除
// @Description 删除
// @Tags 网站配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete WebSite"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/web_site/delete [delete]
func DeleteWebSite(c *gin.Context) {
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
		log.Warnf(context.Background(), "TransID:%s,删除网站配置请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.DeleteWebSite(req.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryWebSite godoc
// @Summary 分页查询
// @Description 分页查询
// @Tags 网站配置管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param name query string false "网站名称"
// @Param code query string false "网站编号"
// @Param projectID query string false "所属项目"
// @Param tenantID query string false "租户ID"
// @Success 200 {object} apipb.QueryWebSiteResponse
// @Router /api/core/web_site/query [get]
func QueryWebSite(c *gin.Context) {
	req := &apipb.QueryWebSiteRequest{}
	resp := &apipb.QueryWebSiteResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	model.QueryWebSite(req, resp, false)

	c.JSON(http.StatusOK, resp)
}

// GetWebSiteDetail godoc
// @Summary 查询明细
// @Description 查询明细
// @Tags 网站配置管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetWebSiteDetailResponse
// @Router /api/core/web_site/detail [get]
func GetWebSiteDetail(c *gin.Context) {
	resp := &apipb.GetWebSiteDetailResponse{
		Code: apipb.Code_Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := model.GetWebSiteByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.WebSiteToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// GetAllWebSite godoc
// @Summary 查询所有
// @Description 查询所有
// @Tags 网站配置管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAllWebSiteResponse
// @Router /api/core/web_site/all [get]
func GetAllWebSite(c *gin.Context) {
	resp := &apipb.GetAllWebSiteResponse{
		Code: apipb.Code_Success,
	}
	list, err := model.GetAllWebSites()
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = model.WebSitesToPB(list)
	c.JSON(http.StatusOK, resp)
}

func RegisterWebSiteRouter(r *gin.Engine) {
	g := r.Group("/api/core/website")

	g.POST("add", AddWebSite)
	g.PUT("update", UpdateWebSite)
	g.GET("query", QueryWebSite)
	g.DELETE("delete", DeleteWebSite)
	g.GET("detail", GetWebSiteDetail)
}
