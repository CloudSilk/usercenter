package http

import (
	"net/http"

	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/model"
	"github.com/gin-gonic/gin"
)

// AddWechatConfig godoc
// @Summary 新增
// @Tags 微信应用配置管理
// @Param authorization header string true "jwt token"
// @Param account body apipb.WechatConfigInfo true "Add WechatConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/wechat/config/add [post]
func AddWechatConfig(c *gin.Context, req *apipb.WechatConfigInfo) (*apipb.CommonResponse, error) {
	id, err := model.CreateWechatConfig(model.PBToWechatConfig(req))
	if err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success, Message: id}, nil
}

// UpdateWechatConfig godoc
// @Summary 更新
// @Tags 微信应用配置管理
// @Param authorization header string true "jwt token"
// @Param account body apipb.WechatConfigInfo true "Update WechatConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/wechat/config/update [put]
func UpdateWechatConfig(c *gin.Context, req *apipb.WechatConfigInfo) (*apipb.CommonResponse, error) {
	if err := model.UpdateWechatConfig(model.PBToWechatConfig(req)); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// DeleteWechatConfig godoc
// @Summary 删除
// @Tags 微信应用配置管理
// @Param authorization header string true "jwt token"
// @Param data body apipb.DelRequest true "Delete WechatConfig"
// @Success 200 {object} apipb.CommonResponse
// @Router /api/core/wechat/config/delete [delete]
func DeleteWechatConfig(c *gin.Context, req *apipb.DelRequest) (*apipb.CommonResponse, error) {
	if err := model.DeleteWechatConfig(req.Id); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// QueryWechatConfig godoc
// @Summary 分页查询
// @Tags 微信应用配置管理
// @Param authorization header string true "jwt token"
// @Param pageIndex query int false "从1开始"
// @Param pageSize query int false "默认每页10条"
// @Success 200 {object} apipb.QueryWechatConfigResponse
// @Router /api/core/wechat/config/query [get]
func QueryWechatConfig(c *gin.Context, req *apipb.QueryWechatConfigRequest) (*apipb.QueryWechatConfigResponse, error) {
	resp := &apipb.QueryWechatConfigResponse{Code: apipb.Code_Success}
	model.QueryWechatConfig(req, resp, false)
	return resp, nil
}

// GetWechatConfigDetail godoc
// @Summary 查询明细
// @Tags 微信应用配置管理
// @Param id query string true "ID"
// @Param authorization header string true "jwt token"
// @Success 200 {object} apipb.GetWechatConfigDetailResponse
// @Router /api/core/wechat/config/detail [get]
func GetWechatConfigDetail(c *gin.Context) {
	resp := &apipb.GetWechatConfigDetailResponse{Code: apipb.Code_Success}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = apipb.Code_BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	data, err := model.GetWechatConfigByID(idStr)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = model.WechatConfigToPB(data)
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterWechatConfigRouter(r *gin.Engine) {
	g := r.Group("/api/core/wechat/config")
	g.POST("add", AutoHandler(AddWechatConfig))
	g.PUT("update", AutoHandler(UpdateWechatConfig))
	g.GET("query", AutoQueryHandler(QueryWechatConfig))
	g.DELETE("delete", AutoHandler(DeleteWechatConfig))
	g.GET("detail", GetWechatConfigDetail)
}
