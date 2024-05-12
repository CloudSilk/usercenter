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

// AddDictionaries godoc
// @Summary 新增
// @Description 新增
// @Tags 字典管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param account body apipb.DictionariesInfo true "Add Dictionaries"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/dictionaries/add [post]
func AddDictionaries(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.DictionariesInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建字典请求参数无效:%v", transID, err)
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
	id, err := model.CreateDictionaries(model.PBToDictionariesArray(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Message = id
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateDictionaries godoc
// @Summary 更新
// @Description 更新
// @Tags 字典管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param account body apipb.DictionariesInfo true "Update Dictionaries"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/dictionaries/update [put]
func UpdateDictionaries(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &apipb.DictionariesInfo{}
	resp := &apipb.CommonResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,更新字典请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.UpdateDictionaries(model.PBToDictionariesArray(req))
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteDictionaries godoc
// @Summary 删除
// @Description 删除
// @Tags 字典管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete Dictionaries"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/dictionaries/delete [delete]
func DeleteDictionaries(c *gin.Context) {
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
		log.Warnf(context.Background(), "TransID:%s,删除字典请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = model.DeleteDictionaries(req.Id)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// QueryDictionaries godoc
// @Summary 分页查询
// @Description 分页查询
// @Tags 字典管理
// @Accept  json
// @Produce  octet-stream
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param orderField query string false "排序字段"
// @Param desc query bool false "是否倒序排序"
// @Param tenantID query string false "租户ID"
// @Success 200 {object} apipb.QueryDictionariesResponse
// @Router /api/core/dictionaries/query [get]
func QueryDictionaries(c *gin.Context) {
	req := &apipb.QueryDictionariesRequest{}
	resp := &apipb.QueryDictionariesResponse{
		Code: apipb.Code_Success,
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	model.QueryDictionaries(req, resp, false)

	c.JSON(http.StatusOK, resp)
}

// GetDictionariesDetail godoc
// @Summary 查询明细
// @Description 查询明细
// @Tags 字典管理
// @Accept  json
// @Produce  json
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetDictionariesDetailResponse
// @Router /api/core/dictionaries/detail [get]
func GetDictionariesDetail(c *gin.Context) {
	resp := &apipb.GetDictionariesDetailResponse{
		Code: apipb.Code_Success,
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	data, err := model.GetDictionariesByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.DictionariesToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

// GetAllDictionaries godoc
// @Summary 查询所有
// @Description 查询所有
// @Tags 字典管理
// @Accept  json
// @Produce  json
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetAllDictionariesResponse
// @Router /api/core/dictionaries/all [get]
func GetAllDictionaries(c *gin.Context) {
	resp := &apipb.GetAllDictionariesResponse{
		Code: apipb.Code_Success,
	}
	list, err := model.GetAllDictionaries()
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = model.DictionariesArrayToPB(list)
	c.JSON(http.StatusOK, resp)
}

func RegisterDictionariesRouter(r *gin.Engine) {
	g := r.Group("/api/core/dictionaries")

	g.POST("add", AddDictionaries)
	g.PUT("update", UpdateDictionaries)
	g.GET("query", QueryDictionaries)
	g.DELETE("delete", DeleteDictionaries)
	g.GET("detail", GetDictionariesDetail)
}
