package http

import (
	"net/http"

	"github.com/CloudSilk/usercenter/internal/app"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

func AddAPP(c *gin.Context, req *app.APP) (*apipb.CommonResponse, error) {
	if err := app.CreateAPP(req); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

func UpdateAPP(c *gin.Context, req *app.APP) (*apipb.CommonResponse, error) {
	if err := app.UpdateAPP(req); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

func DeleteAPP(c *gin.Context, req *app.APP) (*apipb.CommonResponse, error) {
	if err := app.DeleteAPP(req.ID); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

func QueryAPP(c *gin.Context, req *app.QueryAPPRequest) (*app.QueryAPPResponse, error) {
	resp := &app.QueryAPPResponse{}
	app.QueryAPP(req, resp)
	return resp, nil
}

func GetAllAPP(c *gin.Context) {
	metadatas, err := app.GetAllAPPs()
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    apipb.Code_Success,
		"data":    metadatas,
		"records": int64(len(metadatas)),
		"pages":   1,
	})
}

func GetAPPDetail(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		writeBadRequest(c, errStr("id required"))
		return
	}
	data, err := app.GetAPPById(idStr)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeOK(c, gin.H{"data": data})
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
