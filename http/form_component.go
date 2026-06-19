package http

import (
	"net/http"

	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/model"
	"github.com/gin-gonic/gin"
)

// AddFormComponent godoc
// @Summary 新增
// @Tags 表单组件管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.FormComponentInfo true "Add FormComponent"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/form/component/add [post]
func AddFormComponent(c *gin.Context, req *apipb.FormComponentInfo) (*apipb.CommonResponse, error) {
	id, err := model.CreateFormComponent(model.PBToFormComponent(req))
	if err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success, Message: id}, nil
}

// UpdateFormComponent godoc
// @Summary 更新
// @Tags 表单组件管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.FormComponentInfo true "Update FormComponent"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/form/component/update [put]
func UpdateFormComponent(c *gin.Context, req *apipb.FormComponentInfo) (*apipb.CommonResponse, error) {
	if err := model.UpdateFormComponent(model.PBToFormComponent(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteFormComponent godoc
// @Summary 删除
// @Tags 表单组件管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete FormComponent"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/auth/form/component/delete [delete]
func DeleteFormComponent(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	if err := model.DeleteFormComponent(req.Id); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QueryFormComponent godoc
// @Summary 分页查询
// @Tags 表单组件管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QueryFormComponentResponse
// @Router /api/core/auth/form/component/query [get]
func QueryFormComponent(c *gin.Context, req *apipb.QueryFormComponentRequest) (*apipb.QueryFormComponentResponse, error) {
	resp := &apipb.QueryFormComponentResponse{Code: apipb.Code_Success}
	model.QueryFormComponent(req, resp, false)
	return resp, nil
}

// GetFormComponentDetail godoc
// @Summary 查询明细
// @Tags 表单组件管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetFormComponentDetailResponse
// @Router /api/core/auth/form/component/detail [get]
func GetFormComponentDetail(c *gin.Context) {
	resp := &apipb.GetFormComponentDetailResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
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
	g.POST("add", AutoHandler(AddFormComponent))
	g.PUT("update", AutoHandler(UpdateFormComponent))
	g.GET("query", AutoQueryHandler(QueryFormComponent))
	g.DELETE("delete", AutoHandler(DeleteFormComponent))
	g.GET("detail", GetFormComponentDetail)
}
