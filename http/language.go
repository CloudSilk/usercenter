package http

import (
	"net/http"

	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/model"
	"github.com/gin-gonic/gin"
)

// AddLanguage godoc
// @Summary 新增
// @Tags 多语言管理
// @Param authorization header string true "jwt token"
// @Param account body apipb.LanguageInfo true "Add Language"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/language/add [post]
func AddLanguage(c *gin.Context, req *apipb.LanguageInfo) (*apipb.CommonResponse, error) {
	id, err := model.CreateLanguage(model.PBToLanguage(req))
	if err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success, Message: id}, nil
}

// UpdateLanguage godoc
// @Summary 更新
// @Tags 多语言管理
// @Param authorization header string true "jwt token"
// @Param account body apipb.LanguageInfo true "Update Language"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/language/update [put]
func UpdateLanguage(c *gin.Context, req *apipb.LanguageInfo) (*apipb.CommonResponse, error) {
	if err := model.UpdateLanguage(model.PBToLanguage(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteLanguage godoc
// @Summary 删除
// @Tags 多语言管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete Language"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/language/delete [delete]
func DeleteLanguage(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	if err := model.DeleteLanguage(req.Id); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QueryLanguage godoc
// @Summary 分页查询
// @Tags 多语言管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QueryLanguageResponse
// @Router /api/core/language/query [get]
func QueryLanguage(c *gin.Context, req *apipb.QueryLanguageRequest) (*apipb.QueryLanguageResponse, error) {
	resp := &apipb.QueryLanguageResponse{Code: apipb.Code_Success}
	model.QueryLanguage(req, resp, false)
	return resp, nil
}

// GetLanguageDetail godoc
// @Summary 查询明细
// @Tags 多语言管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetLanguageDetailResponse
// @Router /api/core/language/detail [get]
func GetLanguageDetail(c *gin.Context) {
	resp := &apipb.GetLanguageDetailResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	data, err := model.GetLanguageByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.LanguageToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterLanguageRouter(r *gin.Engine) {
	g := r.Group("/api/core/language")
	g.POST("add", AutoHandler(AddLanguage))
	g.PUT("update", AutoHandler(UpdateLanguage))
	g.GET("query", AutoQueryHandler(QueryLanguage))
	g.DELETE("delete", AutoHandler(DeleteLanguage))
	g.GET("detail", GetLanguageDetail)
}
