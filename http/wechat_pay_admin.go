package http

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
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

// ExportWechatPayOrders 导出支付订单 CSV(复用查询过滤条件,无分页,单次上限 5 万行)。
//
//	@Summary 导出微信支付订单 CSV
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Param tenantID query string false "租户ID"
//	@Param userID query string false "用户ID"
//	@Param mchID query string false "商户号"
//	@Param status query string false "状态 INIT/CREATED/PAID/CLOSED"
//	@Param outTradeNo query string false "商户订单号"
//	@Success 200 {string} string
//	@Router /api/core/wechat/pay/order/export [get]
func ExportWechatPayOrders(c *gin.Context) {
	req := &PayOrderAdminQueryRequest{}
	if err := c.ShouldBindQuery(req); err != nil {
		c.JSON(http.StatusOK, &PayOrderAdminQueryResponse{Code: apipb.Code_BadRequest, Message: err.Error()})
		return
	}
	orders, err := wechatpay.ListPayOrdersForExport(&wechatpay.PayOrderQuery{
		TenantID:   req.TenantID,
		UserID:     req.UserID,
		MchID:      req.MchID,
		Status:     req.Status,
		OutTradeNo: req.OutTradeNo,
	})
	if err != nil {
		c.JSON(http.StatusOK, &PayOrderAdminQueryResponse{Code: apipb.Code_InternalServerError, Message: err.Error()})
		return
	}
	filename := "pay_orders_" + time.Now().Format("20060102150405") + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Status(http.StatusOK)
	// UTF-8 BOM:保证 Excel 直接打开中文不乱码
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{
		"订单ID", "租户ID", "用户ID", "AppID", "商户号", "商户订单号", "微信支付单号",
		"商品描述", "附加数据", "金额(分)", "状态", "微信交易状态", "状态说明", "最近事件",
		"创建时间", "支付时间", "关闭时间",
	})
	for _, o := range orders {
		_ = w.Write([]string{
			o.ID, o.TenantID, o.UserID, o.AppID, o.MchID, o.OutTradeNo, o.TransactionID,
			o.Description, o.Attach, strconv.FormatInt(o.Amount, 10), o.Status, o.TradeState,
			o.TradeStateDesc, o.LastEvent,
			o.CreatedAt.Format(time.RFC3339),
			formatTimePtr(o.PaidAt), formatTimePtr(o.ClosedAt),
		})
	}
	w.Flush()
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// RegisterWechatPayOrderRouter 挂载管理端支付订单端点。
func RegisterWechatPayOrderRouter(r *gin.Engine) {
	g := r.Group("/api/core/wechat/pay/order")
	g.GET("query", AutoQueryHandler(QueryWechatPayOrders))
	g.GET("export", ExportWechatPayOrders)
	s := r.Group("/api/core/wechat/pay/stats")
	s.GET("daily", AutoQueryHandler(QueryWechatPayDailyStats))
}

// ApplyRefundRequest 管理端退款申请请求。
type ApplyRefundRequest struct {
	OutTradeNo   string `json:"outTradeNo" binding:"required"`
	RefundAmount int64  `json:"refundAmount" binding:"required,gt=0"`
	OutRefundNo  string `json:"outRefundNo" binding:"omitempty,min=6,max=64"`
	// ReasonCode 退款原因类别(quality/not_received/wrong_order/price/duplicate/customer_service/other),
	// 留空默认 other。
	ReasonCode string `json:"reasonCode" binding:"omitempty,max=32"`
	Reason     string `json:"reason" binding:"omitempty,max=120"`
}

// RefundAdminItem 退款单视图。
type RefundAdminItem struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenantID"`
	UserID         string     `json:"userID"`
	OutTradeNo     string     `json:"outTradeNo"`
	OutRefundNo    string     `json:"outRefundNo"`
	RefundID       string     `json:"refundID"`
	Amount         int64      `json:"amount"`
	Total          int64      `json:"total"`
	ReasonCode     string     `json:"reasonCode,omitempty"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	SuccessTime    *time.Time `json:"successTime,omitempty"`
	ApproverID     string     `json:"approverID,omitempty"`
	ApproveComment string     `json:"approveComment,omitempty"`
	ApprovedAt     *time.Time `json:"approvedAt,omitempty"`
	CreatedAt      string     `json:"createdAt"`
}

func refundToAdminItem(m *wechatpay.PayRefund) *RefundAdminItem {
	return &RefundAdminItem{
		ID: m.ID, TenantID: m.TenantID, UserID: m.UserID,
		OutTradeNo: m.OutTradeNo, OutRefundNo: m.OutRefundNo, RefundID: m.RefundID,
		Amount: m.Amount, Total: m.Total, ReasonCode: m.ReasonCode,
		Reason: m.Reason, Status: m.Status,
		SuccessTime: m.SuccessTime, ApproverID: m.ApproverID,
		ApproveComment: m.ApproveComment, ApprovedAt: m.ApprovedAt,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
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
		ReasonCode:   req.ReasonCode,
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

// GetWechatRefundDetail 管理端查询单笔退款单,默认主动向微信侧同步最新状态。
//
//	@Summary 微信支付退款单详情(可同步状态)
//	@Tags 微信支付退款管理
//	@Param authorization header string true "jwt token"
//	@Param outRefundNo query string true "商户退款单号"
//	@Param sync query bool false "是否向微信侧查单同步,默认true"
//	@Success 200 {object} RefundAdminQueryResponse
//	@Router /api/core/wechat/pay/refund/detail [get]
func GetWechatRefundDetail(c *gin.Context) {
	resp := &RefundAdminQueryResponse{Code: apipb.Code_Success}
	outRefundNo := c.Query("outRefundNo")
	if outRefundNo == "" {
		resp.Code = apipb.Code_BadRequest
		resp.Message = "outRefundNo不能为空"
		c.JSON(http.StatusOK, resp)
		return
	}
	var (
		refund *wechatpay.PayRefund
		err    error
	)
	if c.DefaultQuery("sync", "true") == "true" {
		refund, err = wechatpay.SyncRefundStatus(c.Request.Context(), middleware.GetTenantID(c), outRefundNo)
	} else {
		refund, err = wechatpay.GetPayRefundByOutRefundNo(outRefundNo)
		if err == nil && refund == nil {
			err = errors.New("退款单不存在")
		}
	}
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	resp.Data = []*RefundAdminItem{refundToAdminItem(refund)}
	resp.Records, resp.Total, resp.Pages = 1, 1, 1
	c.JSON(http.StatusOK, resp)
}

// ApproveRefundRequest 退款审核请求。
type ApproveRefundRequest struct {
	OutRefundNo string `json:"outRefundNo" binding:"required"`
	Approved    bool   `json:"approved"`
	Comment     string `json:"comment" binding:"omitempty,max=200"`
}

// ApproveWechatRefund 审核待审核退款单(通过后提交微信,拒绝置为 REJECTED)。
//
//	@Summary 微信支付退款审核
//	@Tags 微信支付退款管理
//	@Param authorization header string true "jwt token"
//	@Param body body ApproveRefundRequest true "审核请求"
//	@Success 200 {object} RefundAdminQueryResponse
//	@Router /api/core/wechat/pay/refund/approve [post]
func ApproveWechatRefund(c *gin.Context, req *ApproveRefundRequest) (*RefundAdminQueryResponse, error) {
	resp := &RefundAdminQueryResponse{Code: apipb.Code_Success}
	refund, err := wechatpay.ApproveRefund(c.Request.Context(), wechatpay.ApproveRefundInput{
		TenantID:    middleware.GetTenantID(c),
		ApproverID:  middleware.GetUserID(c),
		OutRefundNo: req.OutRefundNo,
		Approved:    req.Approved,
		Comment:     req.Comment,
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

// RegisterWechatPayRefundRouter 挂载管理端退款端点。
func RegisterWechatPayRefundRouter(r *gin.Engine) {
	g := r.Group("/api/core/wechat/pay/refund")
	g.POST("apply", AutoHandler(ApplyWechatRefund))
	g.POST("approve", AutoHandler(ApproveWechatRefund))
	g.GET("query", AutoQueryHandler(QueryWechatRefunds))
	g.GET("detail", GetWechatRefundDetail)
}

// PayDailyStatsQueryRequest 对账日报查询请求。
type PayDailyStatsQueryRequest struct {
	// Days 统计最近 N 天(含今日),默认 7,范围 1-90。
	Days     int    `form:"days" binding:"omitempty,gt=0,lte=90"`
	TenantID string `form:"tenantID"`
}

// PayDailyStatsResponse 对账日报响应。
type PayDailyStatsResponse struct {
	Code    apipb.Code                `json:"code"`
	Message string                    `json:"message,omitempty"`
	Data    []*wechatpay.DailyPayStat `json:"data,omitempty"`
}

// QueryWechatPayDailyStats 管理端支付对账日报:按日聚合下单/支付/关单/退款。
//
//	@Summary 微信支付对账日报
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Param days query int false "统计最近 N 天(含今日),默认 7,范围 1-90"
//	@Param tenantID query string false "租户ID,留空为全租户汇总"
//	@Success 200 {object} PayDailyStatsResponse
//	@Router /api/core/wechat/pay/stats/daily [get]
func QueryWechatPayDailyStats(c *gin.Context, req *PayDailyStatsQueryRequest) (*PayDailyStatsResponse, error) {
	resp := &PayDailyStatsResponse{Code: apipb.Code_Success}
	stats, err := wechatpay.QueryDailyPayStats(req.TenantID, req.Days)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = stats
	return resp, nil
}
