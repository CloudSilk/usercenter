package http

import (
	"net/http"

	"github.com/CloudSilk/pkg/model"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// HandlerFunc 泛型业务函数签名:只写核心业务逻辑,绑定/校验/响应由 AutoHandler 处理。
type HandlerFunc[TReq any, TResp any] func(c *gin.Context, req *TReq) (*TResp, error)

// AutoHandler 泛型中间件:自动 BindJSON + Validate + 响应填充,减少样板代码。
// 使用方式:r.POST("add", AutoHandler(AddUser))
func AutoHandler[TReq any, TResp any](h HandlerFunc[TReq, TResp]) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, errorResp(model.BadRequest, err.Error()))
			return
		}
		// 支持 validator 结构体 tag 校验
		if err := middleware.Validate.Struct(&req); err != nil {
			c.JSON(http.StatusOK, errorResp(model.BadRequest, err.Error()))
			return
		}

		resp, err := h(c, &req)
		if err != nil {
			c.JSON(http.StatusOK, errorResp(model.InternalServerError, err.Error()))
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// QueryHandlerFunc 泛型查询函数签名(GET 请求,参数从 query 绑定)
type QueryHandlerFunc[TReq any, TResp any] func(c *gin.Context, req *TReq) (*TResp, error)

// AutoQueryHandler 自动 BindQuery + Validate + 响应填充
func AutoQueryHandler[TReq any, TResp any](h QueryHandlerFunc[TReq, TResp]) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TReq
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusOK, errorResp(model.BadRequest, err.Error()))
			return
		}
		if err := middleware.Validate.Struct(&req); err != nil {
			c.JSON(http.StatusOK, errorResp(model.BadRequest, err.Error()))
			return
		}
		resp, err := h(c, &req)
		if err != nil {
			c.JSON(http.StatusOK, errorResp(model.InternalServerError, err.Error()))
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// errorResp 构造标准错误响应
func errorResp(code int32, message string) *apipb.CommonResponse {
	return &apipb.CommonResponse{Code: apipb.Code(code), Message: message}
}
