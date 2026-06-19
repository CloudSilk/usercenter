package http

import (
	"net/http"

	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/model"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// AddDictionaries godoc
// @Summary 新增
// @Tags 字典管理
// @Param authorization header string true "jwt token"
// @Param account body apipb.DictionariesInfo true "Add Dictionaries"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/dictionaries/add [post]
func AddDictionaries(c *gin.Context, req *apipb.DictionariesInfo) (*apipb.CommonResponse, error) {
	req.TenantID = ucm.GetTenantID(c)
	id, err := model.CreateDictionaries(model.PBToDictionariesArray(req))
	if err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success, Message: id}, nil
}

// UpdateDictionaries godoc
// @Summary 更新
// @Tags 字典管理
// @Param authorization header string true "jwt token"
// @Param account body apipb.DictionariesInfo true "Update Dictionaries"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/dictionaries/update [put]
func UpdateDictionaries(c *gin.Context, req *apipb.DictionariesInfo) (*apipb.CommonResponse, error) {
	if err := model.UpdateDictionaries(model.PBToDictionariesArray(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteDictionaries godoc
// @Summary 删除
// @Tags 字典管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete Dictionaries"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/dictionaries/delete [delete]
func DeleteDictionaries(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	if err := model.DeleteDictionaries(req.Id); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QueryDictionaries godoc
// @Summary 分页查询
// @Tags 字典管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Param tenantID query string false "租户ID"
// @Success 200 {object} apipb.QueryDictionariesResponse
// @Router /api/core/dictionaries/query [get]
func QueryDictionaries(c *gin.Context, req *apipb.QueryDictionariesRequest) (*apipb.QueryDictionariesResponse, error) {
	resp := &apipb.QueryDictionariesResponse{Code: apipb.Code_Success}
	model.QueryDictionaries(req, resp, false)
	return resp, nil
}

// GetDictionariesDetail godoc
// @Summary 查询明细
// @Tags 字典管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetDictionariesDetailResponse
// @Router /api/core/dictionaries/detail [get]
func GetDictionariesDetail(c *gin.Context) {
	resp := &apipb.GetDictionariesDetailResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	data, err := model.GetDictionariesByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.DictionariesToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterDictionariesRouter(r *gin.Engine) {
	g := r.Group("/api/core/dictionaries")
	g.POST("add", AutoHandler(AddDictionaries))
	g.PUT("update", AutoHandler(UpdateDictionaries))
	g.GET("query", AutoQueryHandler(QueryDictionaries))
	g.DELETE("delete", AutoHandler(DeleteDictionaries))
	g.GET("detail", GetDictionariesDetail)
}
