package http

import (
	"time"

	"github.com/CloudSilk/usercenter/internal/wechatpay"
	apipb "github.com/CloudSilk/usercenter/proto"
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
