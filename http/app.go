package http

import (
	"context"
	"net/http"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/pkg/utils/middleware"
	ucmodel "github.com/CloudSilk/usercenter/model"
	"github.com/gin-gonic/gin"
)

func AddAPP(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &ucmodel.APP{}
	resp := &model.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建APP请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.CreateAPP(req)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

func UpdateAPP(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &ucmodel.APP{}
	resp := &model.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建APP请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.UpdateAPP(req)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

func DeleteAPP(c *gin.Context) {
	transID := middleware.GetTransID(c)
	req := &ucmodel.APP{}
	resp := &model.CommonResponse{
		Code: model.Success,
	}
	err := c.BindJSON(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		log.Warnf(context.Background(), "TransID:%s,新建APP请求参数无效:%v", transID, err)
		return
	}
	err = middleware.Validate.Struct(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	err = ucmodel.DeleteAPP(req.ID)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

func QueryAPP(c *gin.Context) {
	req := &ucmodel.QueryAPPRequest{}
	resp := &ucmodel.QueryAPPResponse{
		CommonResponse: model.CommonResponse{
			Code: model.Success,
		},
	}
	err := c.BindQuery(req)
	if err != nil {
		resp.Code = model.BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	ucmodel.QueryAPP(req, resp)

	c.JSON(http.StatusOK, resp)
}

func GetAllAPP(c *gin.Context) {
	resp := &ucmodel.QueryAPPResponse{
		CommonResponse: model.CommonResponse{
			Code: model.Success,
		},
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
		CommonResponse: model.CommonResponse{
			Code: model.Success,
		},
	}
	idStr := c.Query("id")
	if idStr == "" {
		resp.Code = model.BadRequest
		c.JSON(http.StatusOK, resp)
		return
	}
	var err error

	resp.Data, err = ucmodel.GetAPPById(idStr)
	if err != nil {
		resp.Code = model.InternalServerError
		resp.Message = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

func RegisterAPPRouter(r *gin.Engine) {
	appGroup := r.Group("/api/core/auth/app")
	appGroup.POST("add", AddAPP)
	appGroup.PUT("update", UpdateAPP)
	appGroup.GET("query", QueryAPP)
	appGroup.DELETE("delete", DeleteAPP)
	appGroup.GET("all", GetAllAPP)
	appGroup.GET("detail", GetAPPDetail)
}
