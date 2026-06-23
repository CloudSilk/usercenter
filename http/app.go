package http

import (
	"net/http"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/app"
	"github.com/gin-gonic/gin"
)

func AddAPP(c *gin.Context, req *app.APP) (*model.CommonResponse, error) {
	if err := app.CreateAPP(req); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

func UpdateAPP(c *gin.Context, req *app.APP) (*model.CommonResponse, error) {
	if err := app.UpdateAPP(req); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

func DeleteAPP(c *gin.Context, req *app.APP) (*model.CommonResponse, error) {
	if err := app.DeleteAPP(req.ID); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

func QueryAPP(c *gin.Context, req *app.QueryAPPRequest) (*app.QueryAPPResponse, error) {
	resp := &app.QueryAPPResponse{CommonResponse: model.CommonResponse{Code: model.Success}}
	app.QueryAPP(req, resp)
	return resp, nil
}

func GetAllAPP(c *gin.Context) {
	resp := &app.QueryAPPResponse{
		CommonResponse: model.CommonResponse{Code: model.Success},
	}
	metadatas, err := app.GetAllAPPs()
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = metadatas
	resp.Records = int64(len(metadatas))
	resp.Pages = 1
	c.JSON(http.StatusOK, resp)
}

func GetAPPDetail(c *gin.Context) {
	resp := model.CommonDetailResponse{
		CommonResponse: model.CommonResponse{Code: model.Success},
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = model.BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	data, err := app.GetAPPById(idStr)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = data
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterAPPRouter(r *gin.Engine) {
	appGroup := r.Group("/api/core/auth/app")
	appGroup.POST("add", AutoHandler(AddAPP))
	appGroup.PUT("update", AutoHandler(UpdateAPP))
	appGroup.GET("query", AutoQueryHandler(QueryAPP))
	appGroup.DELETE("delete", AutoHandler(DeleteAPP))
	appGroup.GET("all", GetAllAPP)
	appGroup.GET("detail", GetAPPDetail)
}
