package http

import (
	"net/http"
	"time"

	"github.com/CloudSilk/usercenter/internal/wechatpay"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// CreatePayOrderRequest 小程序下单请求,amount 单位为分。
type CreatePayOrderRequest struct {
	App         string `json:"app" binding:"required"`
	Description string `json:"description" binding:"required,max=120"`
	Amount      int64  `json:"amount" binding:"required,gt=0"`
	Attach      string `json:"attach" binding:"omitempty,max=100"`
	OutTradeNo  string `json:"outTradeNo" binding:"omitempty,min=6,max=32"`
	// ExpireMinutes 订单有效期(分钟),留空默认120,最长1440。
	ExpireMinutes int `json:"expireMinutes" binding:"omitempty,gt=0,lte=1440"`
}

// PayOrderPayload 订单状态与调起支付所需数据。
type PayOrderPayload struct {
	OutTradeNo string               `json:"outTradeNo"`
	Status     string               `json:"status"`
	TradeState string               `json:"tradeState,omitempty"`
	Amount     int64                `json:"amount"`
	ExpireAt   *time.Time           `json:"expireAt,omitempty"`
	PaidAt     *time.Time           `json:"paidAt,omitempty"`
	PayParams  *wechatpay.PayParams `json:"payParams,omitempty"`
}

// PayOrderResponse 统一响应结构。
type PayOrderResponse struct {
	Code    apipb.Code       `json:"code"`
	Message string           `json:"message,omitempty"`
	Data    *PayOrderPayload `json:"data,omitempty"`
}

// CreatePayOrder 创建小程序 JSAPI 支付订单,返回 wx.requestPayment 参数。
//
//	@Summary 微信小程序支付下单
//	@Tags 微信支付
//	@Param authorization header string true "jwt token"
//	@Param body body CreatePayOrderRequest true "下单请求"
//	@Success 200 {object} PayOrderResponse
//	@Router /api/wechat/pay/order [post]
func CreatePayOrder(c *gin.Context, req *CreatePayOrderRequest) (*PayOrderResponse, error) {
	order, params, err := wechatpay.CreateJSAPIPayment(c.Request.Context(), wechatpay.CreateOrderInput{
		TenantID:      middleware.GetTenantID(c),
		UserID:        middleware.GetUserID(c),
		App:           req.App,
		Description:   req.Description,
		Attach:        req.Attach,
		AmountFen:     req.Amount,
		OutTradeNo:    req.OutTradeNo,
		ExpireMinutes: req.ExpireMinutes,
	})
	if err != nil {
		return &PayOrderResponse{Code: apipb.Code_BadRequest, Message: err.Error()}, nil
	}
	return &PayOrderResponse{
		Code: apipb.Code_Success,
		Data: &PayOrderPayload{
			OutTradeNo: order.OutTradeNo,
			Status:     order.Status,
			Amount:     order.Amount,
			ExpireAt:   order.ExpireAt,
			PayParams:  params,
		},
	}, nil
}

// GetPayOrder 查询订单状态;CREATED 订单会主动向微信侧查单对账。
//
//	@Summary 微信支付订单状态查询
//	@Tags 微信支付
//	@Param authorization header string true "jwt token"
//	@Param outTradeNo query string true "商户订单号"
//	@Success 200 {object} PayOrderResponse
//	@Router /api/wechat/pay/order [get]
func GetPayOrder(c *gin.Context) {
	resp := &PayOrderResponse{Code: apipb.Code_Success}
	outTradeNo := c.Query("outTradeNo")
	if outTradeNo == "" {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "outTradeNo不能为空"
		c.JSON(http.StatusOK, resp)
		return
	}
	order, err := wechatpay.SyncOrderStatus(c.Request.Context(),
		middleware.GetTenantID(c), middleware.GetUserID(c), outTradeNo)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = &PayOrderPayload{
		OutTradeNo: order.OutTradeNo,
		Status:     order.Status,
		TradeState: order.TradeState,
		Amount:     order.Amount,
		ExpireAt:   order.ExpireAt,
		PaidAt:     order.PaidAt,
	}
	c.JSON(http.StatusOK, resp)
}

// ClosePayOrderRequest 关单请求。
type ClosePayOrderRequest struct {
	OutTradeNo string `json:"outTradeNo" binding:"required"`
}

// ClosePayOrder 关闭未支付订单。
//
//	@Summary 微信支付关单
//	@Tags 微信支付
//	@Param authorization header string true "jwt token"
//	@Param body body ClosePayOrderRequest true "关单请求"
//	@Success 200 {object} apipb.CommonResponse
//	@Router /api/wechat/pay/order/close [post]
func ClosePayOrder(c *gin.Context, req *ClosePayOrderRequest) (*apipb.CommonResponse, error) {
	if err := wechatpay.ClosePayOrder(c.Request.Context(),
		middleware.GetTenantID(c), middleware.GetUserID(c), req.OutTradeNo); err != nil {
		return &apipb.CommonResponse{Code: apipb.Code_BadRequest, Message: err.Error()}, nil
	}
	return &apipb.CommonResponse{Code: apipb.Code_Success}, nil
}

// WechatPayNotify 微信支付结果回调(公开端点,SDK 验签+解密)。
// 路径位于 /api/wechat/notify/ 前缀下,命中公开路径白名单。
func WechatPayNotify(c *gin.Context) {
	if err := wechatpay.HandlePayNotify(c.Request, c.Param("app")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

// GetPayRefunds 用户查询自己订单的退款记录。
//
//	@Summary 微信支付退款记录查询
//	@Tags 微信支付
//	@Param authorization header string true "jwt token"
//	@Param outTradeNo query string true "商户订单号"
//	@Success 200 {object} RefundAdminQueryResponse
//	@Router /api/wechat/pay/refund [get]
func GetPayRefunds(c *gin.Context) {
	resp := &RefundAdminQueryResponse{Code: apipb.Code_Success, Data: []*RefundAdminItem{}}
	outTradeNo := c.Query("outTradeNo")
	if outTradeNo == "" {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "outTradeNo不能为空"
		c.JSON(http.StatusOK, resp)
		return
	}
	list, err := wechatpay.ListUserRefunds(middleware.GetTenantID(c), middleware.GetUserID(c), outTradeNo)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	for _, m := range list {
		resp.Data = append(resp.Data, refundToAdminItem(m))
	}
	resp.Records = int64(len(list))
	resp.Total = int64(len(list))
	c.JSON(http.StatusOK, resp)
}

// ListMyPayOrders 用户分页查询自己的支付订单。
//
//	@Summary 我的微信支付订单列表
//	@Tags 微信支付
//	@Param authorization header string true "jwt token"
//	@Param pageIndex query int false "从1开始"
//	@Param pageSize query int false "默认每页10条"
//	@Param status query string false "状态 CREATED/PAID/CLOSED"
//	@Success 200 {object} PayOrderAdminQueryResponse
//	@Router /api/wechat/pay/orders [get]
func ListMyPayOrders(c *gin.Context) {
	req := &PayOrderAdminQueryRequest{}
	if err := c.ShouldBindQuery(req); err != nil {
		c.JSON(http.StatusOK, &PayOrderAdminQueryResponse{Code: apipb.Code_BadRequest, Message: err.Error()})
		return
	}
	resp := &PayOrderAdminQueryResponse{Code: apipb.Code_Success, Data: []*PayOrderAdminItem{}}
	result, err := wechatpay.ListUserOrders(middleware.GetTenantID(c), middleware.GetUserID(c),
		req.Status, req.PageIndex, req.PageSize)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	for _, o := range result.Records {
		resp.Data = append(resp.Data, &PayOrderAdminItem{
			ID: o.ID, TenantID: o.TenantID, UserID: o.UserID, AppID: o.AppID, MchID: o.MchID,
			OutTradeNo: o.OutTradeNo, TransactionID: o.TransactionID, Description: o.Description,
			Attach: o.Attach, Amount: o.Amount, Status: o.Status, TradeState: o.TradeState,
			TradeStateDesc: o.TradeStateDesc, LastEvent: o.LastEvent,
			CreatedAt: o.CreatedAt.Format(time.RFC3339), PaidAt: o.PaidAt, ClosedAt: o.ClosedAt,
		})
	}
	resp.Records, resp.Pages, resp.Total = result.Total, result.Pages, result.Total
	c.JSON(http.StatusOK, resp)
}

// RegisterWechatPayRouter 挂载微信支付端点。
func RegisterWechatPayRouter(r *gin.Engine) {
	g := r.Group("/api/wechat")
	g.POST("pay/order", AutoHandler(CreatePayOrder))
	g.GET("pay/order", GetPayOrder)
	g.GET("pay/orders", ListMyPayOrders)
	g.POST("pay/order/close", AutoHandler(ClosePayOrder))
	g.GET("pay/refund", GetPayRefunds)
	g.POST("notify/pay/:app", WechatPayNotify)
}
