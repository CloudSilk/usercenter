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

// AddLanguage godoc
// @Summary 新增
// @Description 新增
// @Tags 多语言管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param account body apipb.LanguageInfo true "Add Language"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/language/add [post]
func AddLanguage(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.LanguageInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建多语言请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}

	id, err := model.CreateLanguage(model.PBToLanguage(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateLanguage godoc
// @Summary 更新
// @Description 更新
// @Tags 多语言管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param account body apipb.LanguageInfo true "Update Language"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/language/update [put]
func UpdateLanguage(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.LanguageInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,更新多语言请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.UpdateLanguage(model.PBToLanguage(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteLanguage godoc
// @Summary 删除
// @Description 删除
// @Tags 多语言管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete Language"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/language/delete [delete]
func DeleteLanguage(c *gin.Context) {
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
		log.Warnf(context.Background(), "TransID:%s,删除多语言请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.DeleteLanguage(req.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryLanguage godoc
// @Summary 分页查询
// @Description 分页查询
// @Tags 多语言管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param webSiteID query string false "所属站点"
// @Success 200 {object} apipb.QueryLanguageResponse
// @Router /api/core/language/query [get]
func QueryLanguage(c *gin.Context) {
	req := &apipb.QueryLanguageRequest{}
	resp := &apipb.QueryLanguageResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	model.QueryLanguage(req, resp, false)

	c.JSON(http.StatusOK, resp)
}

// GetLanguageDetail godoc
// @Summary 查询明细
// @Description 查询明细
// @Tags 多语言管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetLanguageDetailResponse
// @Router /api/core/language/detail [get]
func GetLanguageDetail(c *gin.Context) {
	resp := &apipb.GetLanguageDetailResponse{
		Code: apipb.Code_Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := model.GetLanguageByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.LanguageToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// GetAllLanguage godoc
// @Summary 查询所有
// @Description 查询所有
// @Tags 多语言管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAllLanguageResponse
// @Router /api/core/language/all [get]
func GetAllLanguage(c *gin.Context) {
	resp := &apipb.GetAllLanguageResponse{
		Code: apipb.Code_Success,
	}
	list, err := model.GetAllLanguages()
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = model.LanguagesToPB(list)
	c.JSON(http.StatusOK, resp)
}

func RegisterLanguageRouter(r *gin.Engine) {
	g := r.Group("/api/core/language")

	g.POST("add", AddLanguage)
	g.PUT("update", UpdateLanguage)
	g.GET("query", QueryLanguage)
	g.DELETE("delete", DeleteLanguage)
	g.GET("detail", GetLanguageDetail)
}
