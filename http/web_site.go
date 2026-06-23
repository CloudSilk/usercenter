package http

import (
	"net/http"

	"github.com/CloudSilk/usercenter/internal/website"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// AddWebSite godoc
// @Summary 新增
// @Tags 网站配置管理
// @Param authorization header string true "jwt token"
// @Param account body apipb.WebSiteInfo true "Add WebSite"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/web_site/add [post]
func AddWebSite(c *gin.Context, req *apipb.WebSiteInfo) (*apipb.CommonResponse, error) {
	req.TenantID = ucm.GetTenantID(c)
	id, err := website.CreateWebSite(website.PBToWebSite(req))
	if err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success, Message: id}, nil
}

// UpdateWebSite godoc
// @Summary 更新
// @Tags 网站配置管理
// @Param authorization header string true "jwt token"
// @Param account body apipb.WebSiteInfo true "Update WebSite"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/web_site/update [put]
func UpdateWebSite(c *gin.Context, req *apipb.WebSiteInfo) (*apipb.CommonResponse, error) {
	if err := website.UpdateWebSite(website.PBToWebSite(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteWebSite godoc
// @Summary 删除
// @Tags 网站配置管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete WebSite"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/web_site/delete [delete]
func DeleteWebSite(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	if err := website.DeleteWebSite(req.Id); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QueryWebSite godoc
// @Summary 分页查询
// @Tags 网站配置管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QueryWebSiteResponse
// @Router /api/core/web_site/query [get]
func QueryWebSite(c *gin.Context, req *apipb.QueryWebSiteRequest) (*apipb.QueryWebSiteResponse, error) {
	resp := &apipb.QueryWebSiteResponse{Code: apipb.Code_Success}
	website.QueryWebSite(req, resp, false)
	return resp, nil
}

// GetWebSiteDetail godoc
// @Summary 查询明细
// @Tags 网站配置管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetWebSiteDetailResponse
// @Router /api/core/web_site/detail [get]
func GetWebSiteDetail(c *gin.Context) {
	resp := &apipb.GetWebSiteDetailResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	data, err := website.GetWebSiteByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = website.WebSiteToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterWebSiteRouter(r *gin.Engine) {
	g := r.Group("/api/core/website")
	g.POST("add", AutoHandler(AddWebSite))
	g.PUT("update", AutoHandler(UpdateWebSite))
	g.GET("query", AutoQueryHandler(QueryWebSite))
	g.DELETE("delete", AutoHandler(DeleteWebSite))
	g.GET("detail", GetWebSiteDetail)
}
