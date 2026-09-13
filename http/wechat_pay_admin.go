package http

import (
	"time"

	"github.com/CloudSilk/usercenter/internal/wechatpay"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// PayOrderAdminItem 管理端订单视图。
type PayOrderAdminItem struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenantID"`
	UserID         string     `json:"userID"`
	AppID          string     `json:"appID"`
	MchID          string     `json:"mchID"`
	OutTradeNo     string     `json:"outTradeNo"`
	TransactionID  string     `json:"transactionID"`
	Description    string     `json:"description"`
	Attach         string     `json:"attach"`
	Amount         int64      `json:"amount"`
	Status         string     `json:"status"`
	TradeState     string     `json:"tradeState"`
	TradeStateDesc string     `json:"tradeStateDesc"`
	LastEvent      string     `json:"lastEvent"`
	CreatedAt      string     `json:"createdAt"`
	PaidAt         *time.Time `json:"paidAt,omitempty"`
	ClosedAt       *time.Time `json:"closedAt,omitempty"`
}

// PayOrderAdminQueryRequest 管理端订单分页查询请求。
type PayOrderAdminQueryRequest struct {
	PageIndex  int    `form:"pageIndex" binding:"omitempty,gt=0"`
	PageSize   int    `form:"pageSize" binding:"omitempty,gt=0,lt=1000"`
	TenantID   string `form:"tenantID"`
	UserID     string `form:"userID"`
	MchID      string `form:"mchID"`
	Status     string `form:"status" binding:"omitempty,oneof=INIT CREATED PAID CLOSED"`
	OutTradeNo string `form:"outTradeNo"`
}

// PayOrderAdminQueryResponse 管理端订单分页查询响应。
type PayOrderAdminQueryResponse struct {
	Code    apipb.Code           `json:"code"`
	Message string               `json:"message,omitempty"`
	Data    []*PayOrderAdminItem `json:"data,omitempty"`
	Records int64                `json:"records"`
	Pages   int64                `json:"pages"`
	Total   int64                `json:"total"`
}

// QueryWechatPayOrders 管理端分页查询支付订单。
//
//	@Summary 分页查询微信支付订单
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Param pageIndex query int false "从1开始"
//	@Param pageSize query int false "默认每页10条"
//	@Param tenantID query string false "租户ID"
//	@Param userID query string false "用户ID"
//	@Param mchID query string false "商户号"
//	@Param status query string false "状态 INIT/CREATED/PAID/CLOSED"
//	@Param outTradeNo query string false "商户订单号"
//	@Success 200 {object} PayOrderAdminQueryResponse
//	@Router /api/core/wechat/pay/order/query [get]
func QueryWechatPayOrders(c *gin.Context, req *PayOrderAdminQueryRequest) (*PayOrderAdminQueryResponse, error) {
	resp := &PayOrderAdminQueryResponse{Code: apipb.Code_Success, Data: []*PayOrderAdminItem{}}
	result, err := wechatpay.QueryPayOrders(&wechatpay.PayOrderQuery{
		PageIndex:  req.PageIndex,
		PageSize:   req.PageSize,
		TenantID:   req.TenantID,
		UserID:     req.UserID,
		MchID:      req.MchID,
		Status:     req.Status,
		OutTradeNo: req.OutTradeNo,
	})
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return resp, nil
	}
	for _, o := range result.Records {
		resp.Data = append(resp.Data, &PayOrderAdminItem{
			ID:             o.ID,
			TenantID:       o.TenantID,
			UserID:         o.UserID,
			AppID:          o.AppID,
			MchID:          o.MchID,
			OutTradeNo:     o.OutTradeNo,
			TransactionID:  o.TransactionID,
			Description:    o.Description,
			Attach:         o.Attach,
			Amount:         o.Amount,
			Status:         o.Status,
			TradeState:     o.TradeState,
			TradeStateDesc: o.TradeStateDesc,
			LastEvent:      o.LastEvent,
			CreatedAt:      o.CreatedAt.Format(time.RFC3339),
			PaidAt:         o.PaidAt,
			ClosedAt:       o.ClosedAt,
		})
	}
	resp.Records = result.Total
	resp.Pages = result.Pages
	resp.Total = result.Total
	return resp, nil
}

// RegisterWechatPayOrderRouter 挂载管理端支付订单端点。
func RegisterWechatPayOrderRouter(r *gin.Engine) {
	g := r.Group("/api/core/wechat/pay/order")
	g.GET("query", AutoQueryHandler(QueryWechatPayOrders))
}

// ApplyRefundRequest 管理端退款申请请求。
type ApplyRefundRequest struct {
	OutTradeNo   string `json:"outTradeNo" binding:"required"`
	RefundAmount int64  `json:"refundAmount" binding:"required,gt=0"`
	OutRefundNo  string `json:"outRefundNo" binding:"omitempty,min=6,max=64"`
	Reason       string `json:"reason" binding:"omitempty,max=120"`
}

// RefundAdminItem 退款单视图。
type RefundAdminItem struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenantID"`
	UserID      string     `json:"userID"`
	OutTradeNo  string     `json:"outTradeNo"`
	OutRefundNo string     `json:"outRefundNo"`
	RefundID    string     `json:"refundID"`
	Amount      int64      `json:"amount"`
	Total       int64      `json:"total"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"`
	SuccessTime *time.Time `json:"successTime,omitempty"`
	CreatedAt   string     `json:"createdAt"`
}

func refundToAdminItem(m *wechatpay.PayRefund) *RefundAdminItem {
	return &RefundAdminItem{
		ID: m.ID, TenantID: m.TenantID, UserID: m.UserID,
		OutTradeNo: m.OutTradeNo, OutRefundNo: m.OutRefundNo, RefundID: m.RefundID,
		Amount: m.Amount, Total: m.Total, Reason: m.Reason, Status: m.Status,
		SuccessTime: m.SuccessTime, CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
}

// ApplyWechatRefund 对已支付订单发起退款(管理端)。
//
//	@Summary 微信支付退款申请
//	@Tags 微信支付退款管理
//	@Param authorization header string true "jwt token"
//	@Param body body ApplyRefundRequest true "退款申请"
//	@Success 200 {object} RefundAdminQueryResponse
//	@Router /api/core/wechat/pay/refund/apply [post]
func ApplyWechatRefund(c *gin.Context, req *ApplyRefundRequest) (*RefundAdminQueryResponse, error) {
	resp := &RefundAdminQueryResponse{Code: apipb.Code_Success}
	refund, err := wechatpay.ApplyRefund(c.Request.Context(), wechatpay.ApplyRefundInput{
		TenantID:     middleware.GetTenantID(c),
		OutTradeNo:   req.OutTradeNo,
		RefundAmount: req.RefundAmount,
		OutRefundNo:  req.OutRefundNo,
		Reason:       req.Reason,
	})
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = []*RefundAdminItem{refundToAdminItem(refund)}
	resp.Records, resp.Total, resp.Pages = 1, 1, 1
	return resp, nil
}

// RefundAdminQueryRequest 管理端退款单分页查询请求。
type RefundAdminQueryRequest struct {
	PageIndex   int    `form:"pageIndex" binding:"omitempty,gt=0"`
	PageSize    int    `form:"pageSize" binding:"omitempty,gt=0,lt=1000"`
	TenantID    string `form:"tenantID"`
	Status      string `form:"status" binding:"omitempty,oneof=PROCESSING SUCCESS CLOSED ABNORMAL"`
	OutTradeNo  string `form:"outTradeNo"`
	OutRefundNo string `form:"outRefundNo"`
}

// RefundAdminQueryResponse 退款单查询响应。
type RefundAdminQueryResponse struct {
	Code    apipb.Code         `json:"code"`
	Message string             `json:"message,omitempty"`
	Data    []*RefundAdminItem `json:"data,omitempty"`
	Records int64              `json:"records"`
	Pages   int64              `json:"pages"`
	Total   int64              `json:"total"`
}

// QueryWechatRefunds 管理端分页查询退款单。
//
//	@Summary 分页查询微信支付退款单
//	@Tags 微信支付退款管理
//	@Param authorization header string true "jwt token"
//	@Param pageIndex query int false "从1开始"
//	@Param pageSize query int false "默认每页10条"
//	@Param tenantID query string false "租户ID"
//	@Param status query string false "状态 PROCESSING/SUCCESS/CLOSED/ABNORMAL"
//	@Param outTradeNo query string false "商户订单号"
//	@Param outRefundNo query string false "商户退款单号"
//	@Success 200 {object} RefundAdminQueryResponse
//	@Router /api/core/wechat/pay/refund/query [get]
func QueryWechatRefunds(c *gin.Context, req *RefundAdminQueryRequest) (*RefundAdminQueryResponse, error) {
	resp := &RefundAdminQueryResponse{Code: apipb.Code_Success, Data: []*RefundAdminItem{}}
	result, err := wechatpay.QueryPayRefunds(&wechatpay.PayRefundQuery{
		PageIndex:   req.PageIndex,
		PageSize:    req.PageSize,
		TenantID:    req.TenantID,
		Status:      req.Status,
		OutTradeNo:  req.OutTradeNo,
		OutRefundNo: req.OutRefundNo,
	})
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return resp, nil
	}
	for _, m := range result.Records {
		resp.Data = append(resp.Data, refundToAdminItem(m))
	}
	resp.Records = result.Total
	resp.Pages = result.Pages
	resp.Total = result.Total
	return resp, nil
}

// RegisterWechatPayRefundRouter 挂载管理端退款端点。
func RegisterWechatPayRefundRouter(r *gin.Engine) {
	g := r.Group("/api/core/wechat/pay/refund")
	g.POST("apply", AutoHandler(ApplyWechatRefund))
	g.GET("query", AutoQueryHandler(QueryWechatRefunds))
}
