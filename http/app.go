package http

import (
	"net/http"

	"github.com/CloudSilk/pkg/model"
	ucmodel "github.com/CloudSilk/usercenter/model"
	"github.com/gin-gonic/gin"
)

func AddAPP(c *gin.Context, req *ucmodel.APP) (*model.CommonResponse, error) {
	if err := ucmodel.CreateAPP(req); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

func UpdateAPP(c *gin.Context, req *ucmodel.APP) (*model.CommonResponse, error) {
	if err := ucmodel.UpdateAPP(req); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

func DeleteAPP(c *gin.Context, req *ucmodel.APP) (*model.CommonResponse, error) {
	if err := ucmodel.DeleteAPP(req.ID); err != nil {
		return &model.CommonResponse{Code: model.InternalServerError, Message: err.Error()}, nil
	}
	return &model.CommonResponse{Code: model.Success}, nil
}

func QueryAPP(c *gin.Context, req *ucmodel.QueryAPPRequest) (*ucmodel.QueryAPPResponse, error) {
	resp := &ucmodel.QueryAPPResponse{CommonResponse: model.CommonResponse{Code: model.Success}}
	ucmodel.QueryAPP(req, resp)
	return resp, nil
}

func GetAllAPP(c *gin.Context) {
	resp := &ucmodel.QueryAPPResponse{
		CommonResponse: model.CommonResponse{Code: model.Success},
	}
	metadatas, err := ucmodel.GetAllAPPs()
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
	data, err := ucmodel.GetAPPById(idStr)
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
