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
	g.POST("batch-close", AutoHandler(BatchCloseWechatOrders))
	g.GET("trade-bill", TradeBill)
	g.GET("trade-bill/range", TradeBillRange)
	s := r.Group("/api/core/wechat/pay/stats")
	s.GET("daily", AutoQueryHandler(QueryWechatPayDailyStats))
	s.GET("loop-status", GetWechatPayLoopStatus)
	s.PUT("config", AutoHandler(UpdateWechatPayStatsConfig))
	s.DELETE("config", ResetWechatPayStatsConfig)
	s.POST("reconcile-now", ReconcileNow)
	s.GET("refund-reason", AutoQueryHandler(QueryWechatRefundReasonStats))
	s.GET("refund-reason/trend", AutoQueryHandler(QueryWechatRefundReasonTrend))
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

// RefundReasonStatsQueryRequest 退款原因统计查询请求。
type RefundReasonStatsQueryRequest struct {
	// Days 统计最近 N 天(含今日),默认 30,范围 1-365。
	Days     int    `form:"days" binding:"omitempty,gt=0,lte=365"`
	TenantID string `form:"tenantID"`
}

// RefundReasonStatsResponse 退款原因统计响应。
type RefundReasonStatsResponse struct {
	Code    apipb.Code                  `json:"code"`
	Message string                      `json:"message,omitempty"`
	Data    []*wechatpay.ReasonCodeStat `json:"data,omitempty"`
}

// QueryWechatRefundReasonStats 按退款原因类别聚合退款统计(金额降序)。
//
//	@Summary 退款原因类别统计
//	@Tags 微信支付退款管理
//	@Param authorization header string true "jwt token"
//	@Param days query int false "统计最近 N 天(含今日),默认 30,范围 1-365"
//	@Param tenantID query string false "租户ID,留空为全租户汇总"
//	@Success 200 {object} RefundReasonStatsResponse
//	@Router /api/core/wechat/pay/stats/refund-reason [get]
func QueryWechatRefundReasonStats(c *gin.Context, req *RefundReasonStatsQueryRequest) (*RefundReasonStatsResponse, error) {
	resp := &RefundReasonStatsResponse{Code: apipb.Code_Success}
	stats, err := wechatpay.QueryRefundReasonStats(req.TenantID, req.Days)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = stats
	return resp, nil
}

// RefundReasonTrendQueryRequest 退款原因月度趋势查询请求。
type RefundReasonTrendQueryRequest struct {
	// Months 统计最近 N 个月(含当月),默认 6,范围 1-24。
	Months   int    `form:"months" binding:"omitempty,gt=0,lte=24"`
	TenantID string `form:"tenantID"`
}

// RefundReasonTrendResponse 退款原因月度趋势响应。
type RefundReasonTrendResponse struct {
	Code    apipb.Code                          `json:"code"`
	Message string                              `json:"message,omitempty"`
	Data    []*wechatpay.RefundReasonTrendPoint `json:"data,omitempty"`
}

// QueryWechatRefundReasonTrend 退款原因月度趋势:按月聚合各类别退款笔数与金额。
//
//	@Summary 退款原因月度趋势
//	@Tags 微信支付退款管理
//	@Param authorization header string true "jwt token"
//	@Param months query int false "统计最近 N 个月(含当月),默认 6,范围 1-24"
//	@Param tenantID query string false "租户ID,留空为全租户汇总"
//	@Success 200 {object} RefundReasonTrendResponse
//	@Router /api/core/wechat/pay/stats/refund-reason/trend [get]
func QueryWechatRefundReasonTrend(c *gin.Context, req *RefundReasonTrendQueryRequest) (*RefundReasonTrendResponse, error) {
	resp := &RefundReasonTrendResponse{Code: apipb.Code_Success}
	stats, err := wechatpay.QueryRefundReasonTrend(req.TenantID, req.Months)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = stats
	return resp, nil
}

// BatchCloseOrdersRequest 管理端批量关单请求。
type BatchCloseOrdersRequest struct {
	// OutTradeNos 显式订单号列表(上限100)。
	OutTradeNos []string `json:"outTradeNos" binding:"omitempty,gt=0,max=100,dive,required,min=6,max=32"`
	// 模式二:租户ID + 创建超过 OlderThanMinutes 的 CREATED 订单批量清理。
	TenantID         string `json:"tenantID" binding:"omitempty,max=36"`
	OlderThanMinutes int    `json:"olderThanMinutes" binding:"omitempty,gt=0,lte=43200"`
}

// BatchCloseOrdersResponse 批量关单响应。
type BatchCloseOrdersResponse struct {
	Code    apipb.Code                  `json:"code"`
	Message string                      `json:"message,omitempty"`
	Data    *wechatpay.BatchCloseResult `json:"data,omitempty"`
}

// BatchCloseWechatOrders 管理端批量关单:显式订单号列表,或按租户+滞留时长清理。
//
//	@Summary 微信支付订单批量关单
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Param body body BatchCloseOrdersRequest true "批量关单请求(二选一:outTradeNos 或 tenantID+olderThanMinutes)"
//	@Success 200 {object} BatchCloseOrdersResponse
//	@Router /api/core/wechat/pay/order/batch-close [post]
func BatchCloseWechatOrders(c *gin.Context, req *BatchCloseOrdersRequest) (*BatchCloseOrdersResponse, error) {
	resp := &BatchCloseOrdersResponse{Code: apipb.Code_Success}
	nos := req.OutTradeNos
	if len(nos) == 0 {
		// 模式二:租户 + 滞留时长
		if req.TenantID == "" || req.OlderThanMinutes <= 0 {
			resp.Code = apipb.Code_BadRequest
			resp.Message = "请提供 outTradeNos 列表,或 tenantID+olderThanMinutes"
			return resp, nil
		}
		before := time.Now().Add(-time.Duration(req.OlderThanMinutes) * time.Minute)
		list, err := wechatpay.ListCreatedTradeNosByAge(req.TenantID, before, 500)
		if err != nil {
			resp.Code = apipb.Code_InternalServerError
			resp.Message = err.Error()
			return resp, nil
		}
		if len(list) == 0 {
			resp.Data = &wechatpay.BatchCloseResult{Failures: []wechatpay.BatchCloseFailure{}}
			return resp, nil
		}
		nos = list
	}
	result, err := wechatpay.BatchCloseOrders(c.Request.Context(), middleware.GetTenantID(c), nos)
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = result
	return resp, nil
}

// TradeBillQueryRequest 交易账单下载请求。
type TradeBillQueryRequest struct {
	// ConfigID 商户配置ID。
	ConfigID string `form:"configID" binding:"required"`
	// BillDate 账单日期(YYYY-MM-DD),仅可申请昨日及更早。
	BillDate string `form:"billDate" binding:"required"`
	// BillType 账单类型 ALL(所有)/SUCCESS(成功)/REFUND(退款),留空 ALL。
	BillType string `form:"billType" binding:"omitempty,oneof=ALL SUCCESS REFUND"`
}

// TradeBill 交易账单下载:申请微信侧交易账单并流式返回解压后的 CSV。
//
//	@Summary 下载微信交易账单
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Param configID query string true "商户配置ID"
//	@Param billDate query string true "账单日期(YYYY-MM-DD,仅昨日及更早)"
//	@Param billType query string false "账单类型 ALL/SUCCESS/REFUND,默认 ALL"
//	@Success 200 {string} string
//	@Router /api/core/wechat/pay/order/trade-bill [get]
func TradeBill(c *gin.Context) {
	req := &TradeBillQueryRequest{}
	if err := c.ShouldBindQuery(req); err != nil {
		c.JSON(http.StatusOK, &PayOrderAdminQueryResponse{Code: apipb.Code_BadRequest, Message: err.Error()})
		return
	}
	csvData, err := wechatpay.GetTradeBill(c.Request.Context(),
		middleware.GetTenantID(c), middleware.GetUserID(c), req.ConfigID, req.BillDate, req.BillType)
	if err != nil {
		c.JSON(http.StatusOK, &PayOrderAdminQueryResponse{Code: apipb.Code_BadRequest, Message: err.Error()})
		return
	}
	filename := "trade_bill_" + req.BillDate + "_" + req.ConfigID + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", csvData)
}

// TradeBillRangeQueryRequest 按日期范围批量拉取账单请求。
type TradeBillRangeQueryRequest struct {
	ConfigID  string `form:"configID" binding:"required"`
	StartDate string `form:"startDate" binding:"required"`
	EndDate   string `form:"endDate" binding:"required"`
	// BillType 账单类型 ALL/SUCCESS/REFUND,留空 ALL。
	BillType string `form:"billType" binding:"omitempty,oneof=ALL SUCCESS REFUND"`
}

// TradeBillRange 按日期范围批量拉取交易账单并合并为单个 CSV(跨度上限31天,单日失败不中断)。
//
//	@Summary 按日期范围批量下载微信交易账单
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Param configID query string true "商户配置ID"
//	@Param startDate query string true "开始日期(YYYY-MM-DD)"
//	@Param endDate query string true "结束日期(YYYY-MM-DD,仅昨日及更早)"
//	@Param billType query string false "账单类型 ALL/SUCCESS/REFUND,默认 ALL"
//	@Success 200 {string} string
//	@Router /api/core/wechat/pay/order/trade-bill/range [get]
func TradeBillRange(c *gin.Context) {
	req := &TradeBillRangeQueryRequest{}
	if err := c.ShouldBindQuery(req); err != nil {
		c.JSON(http.StatusOK, &PayOrderAdminQueryResponse{Code: apipb.Code_BadRequest, Message: err.Error()})
		return
	}
	csvData, err := wechatpay.GetTradeBillRange(c.Request.Context(),
		middleware.GetTenantID(c), middleware.GetUserID(c),
		req.ConfigID, req.StartDate, req.EndDate, req.BillType)
	if err != nil {
		c.JSON(http.StatusOK, &PayOrderAdminQueryResponse{Code: apipb.Code_BadRequest, Message: err.Error()})
		return
	}
	filename := "trade_bill_" + req.StartDate + "_to_" + req.EndDate + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", csvData)
}

// BillFileQueryRequest 已归档账单查询请求。
type BillFileQueryRequest struct {
	// ConfigID 商户配置ID,留空查全部(平台侧)。
	ConfigID string `form:"configID"`
	// Days 查询最近 N 天,默认 30,范围 1-365。
	Days     int    `form:"days" binding:"omitempty,gt=0,lte=365"`
	TenantID string `form:"tenantID"`
	// PageIndex/PageSize 可选分页,均不传时返回全量。
	PageIndex int `form:"pageIndex" binding:"omitempty,gt=0"`
	PageSize  int `form:"pageSize" binding:"omitempty,gt=0,lte=500"`
}

// BillFileItem 账单元数据视图(不含内容)。
type BillFileItem struct {
	ID       string `json:"id"`
	TenantID string `json:"tenantID"`
	ConfigID string `json:"configID"`
	BillDate string `json:"billDate"`
	BillType string `json:"billType"`
}

// BillFileListResponse 已归档账单列表响应。
type BillFileListResponse struct {
	Code    apipb.Code      `json:"code"`
	Message string          `json:"message,omitempty"`
	Data    []*BillFileItem `json:"data,omitempty"`
	Records int64           `json:"records"`
	Pages   int64           `json:"pages"`
	Total   int64           `json:"total"`
}

// QueryWechatBillFiles 列出已归档的交易账单(元数据),支持可选分页。
//
//	@Summary 已归档交易账单列表
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Param configID query string false "商户配置ID"
//	@Param days query int false "最近 N 天,默认 30"
//	@Param pageIndex query int false "页码,从 1 开始"
//	@Param pageSize query int false "每页条数,上限 500"
//	@Success 200 {object} BillFileListResponse
//	@Router /api/core/wechat/pay/bill/list [get]
func QueryWechatBillFiles(c *gin.Context, req *BillFileQueryRequest) (*BillFileListResponse, error) {
	resp := &BillFileListResponse{Code: apipb.Code_Success, Data: []*BillFileItem{}}
	list, total, err := wechatpay.ListBillFiles(wechatpay.BillFileQuery{
		TenantID:  req.TenantID,
		ConfigID:  req.ConfigID,
		Days:      req.Days,
		PageIndex: req.PageIndex,
		PageSize:  req.PageSize,
	})
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		return resp, nil
	}
	for _, b := range list {
		resp.Data = append(resp.Data, &BillFileItem{
			ID: b.ID, TenantID: b.TenantID, ConfigID: b.ConfigID,
			BillDate: b.BillDate, BillType: b.BillType,
		})
	}
	pages := int64(1)
	if req.PageSize > 0 {
		pages = (total + int64(req.PageSize) - 1) / int64(req.PageSize)
	}
	resp.Records, resp.Total, resp.Pages = total, total, pages
	return resp, nil
}

// DownloadWechatBillFile 下载已归档的交易账单 CSV。
//
//	@Summary 下载已归档交易账单
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Param id query string true "账单ID"
//	@Success 200 {string} string
//	@Router /api/core/wechat/pay/bill/download [get]
func DownloadWechatBillFile(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusOK, &BillFileListResponse{Code: apipb.Code_BadRequest, Message: "id不能为空"})
		return
	}
	bill, err := wechatpay.GetBillFileByID(id)
	if err != nil {
		c.JSON(http.StatusOK, &BillFileListResponse{Code: apipb.Code_BadRequest, Message: err.Error()})
		return
	}
	filename := "trade_bill_" + bill.BillDate + "_" + bill.BillType + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", bill.Content)
}

// RegisterWechatBillRouter 挂载已归档账单端点。
func RegisterWechatBillRouter(r *gin.Engine) {
	g := r.Group("/api/core/wechat/pay/bill")
	g.GET("list", AutoQueryHandler(QueryWechatBillFiles))
	g.GET("download", DownloadWechatBillFile)
}

// LoopStatusResponse 对账循环状态响应。
type LoopStatusResponse struct {
	Code    apipb.Code            `json:"code"`
	Message string                `json:"message,omitempty"`
	Data    *wechatpay.LoopStatus `json:"data,omitempty"`
}

// GetWechatPayLoopStatus 查询对账循环运行状态与参数。
//
//	@Summary 对账循环运行状态
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Success 200 {object} LoopStatusResponse
//	@Router /api/core/wechat/pay/stats/loop-status [get]
func GetWechatPayLoopStatus(c *gin.Context) {
	status := wechatpay.GetLoopStatus()
	c.JSON(http.StatusOK, &LoopStatusResponse{Code: apipb.Code_Success, Data: &status})
}

// ReconcileNowResult 手动触发对账的结果。
type ReconcileNowResult struct {
	Processed  int                   `json:"processed"`
	LoopStatus *wechatpay.LoopStatus `json:"loopStatus"`
}

// ReconcileNowResponse 手动触发对账响应。
type ReconcileNowResponse struct {
	Code    apipb.Code          `json:"code"`
	Message string              `json:"message,omitempty"`
	Data    *ReconcileNowResult `json:"data,omitempty"`
}

// ReconcileNow 管理端立即执行一轮对账兜底(与循环 tick 相同逻辑,幂等)。
//
//	@Summary 立即执行一轮对账
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Success 200 {object} ReconcileNowResponse
//	@Router /api/core/wechat/pay/stats/reconcile-now [post]
func ReconcileNow(c *gin.Context) {
	n, err := wechatpay.ReconcileStaleOrders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, &ReconcileNowResponse{Code: apipb.Code_InternalServerError, Message: err.Error()})
		return
	}
	status := wechatpay.GetLoopStatus()
	c.JSON(http.StatusOK, &ReconcileNowResponse{
		Code: apipb.Code_Success,
		Data: &ReconcileNowResult{Processed: n, LoopStatus: &status},
	})
}

// UpdatePayStatsConfigRequest 运行时更新对账参数请求(仅提交需要修改的字段)。
type UpdatePayStatsConfigRequest struct {
	// IntervalSeconds 对账轮询间隔(秒),最小 10。
	IntervalSeconds *int `json:"intervalSeconds,omitempty" binding:"omitempty,gt=0"`
	// ScanAgeMinutes 只扫描创建超过该分钟数的未决订单,最小 1。
	ScanAgeMinutes *int `json:"scanAgeMinutes,omitempty" binding:"omitempty,gt=0"`
	// BatchSize 单轮对账最大订单数,范围 1-1000。
	BatchSize *int `json:"batchSize,omitempty" binding:"omitempty,gt=0,lte=1000"`
	// AlertAgeHours 订单滞留告警阈值(小时),最小 1。
	AlertAgeHours *int `json:"alertAgeHours,omitempty" binding:"omitempty,gt=0"`
	// AlertSilenceMinutes 同类告警静默窗口(分钟),最小 1。
	AlertSilenceMinutes *int `json:"alertSilenceMinutes,omitempty" binding:"omitempty,gt=0"`
	// BillRetentionDays 归档账单保留天数,最小 7。
	BillRetentionDays *int `json:"billRetentionDays,omitempty" binding:"omitempty,gt=0"`
}

// UpdateWechatPayStatsConfig 运行时更新对账参数并返回最新状态快照。
//
//	@Summary 更新对账参数(运行时生效)
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Param body body UpdatePayStatsConfigRequest true "对账参数"
//	@Success 200 {object} LoopStatusResponse
//	@Router /api/core/wechat/pay/stats/config [put]
func UpdateWechatPayStatsConfig(c *gin.Context, req *UpdatePayStatsConfigRequest) (*LoopStatusResponse, error) {
	if req.IntervalSeconds != nil {
		wechatpay.ConfigureReconcile(
			time.Duration(*req.IntervalSeconds)*time.Second,
			0, 0, 0, 0,
		)
	}
	if req.ScanAgeMinutes != nil {
		wechatpay.SetReconcileScanAge(*req.ScanAgeMinutes)
	}
	if req.BatchSize != nil {
		wechatpay.SetReconcileBatchSize(*req.BatchSize)
	}
	if req.AlertAgeHours != nil {
		wechatpay.SetReconcileAlertAge(*req.AlertAgeHours)
	}
	if req.AlertSilenceMinutes != nil {
		wechatpay.SetReconcileAlertSilence(*req.AlertSilenceMinutes)
	}
	if req.BillRetentionDays != nil {
		wechatpay.SetBillRetentionDays(*req.BillRetentionDays)
	}
	// 持久化当前生效参数:重启后仍优先于 Nacos 基线生效
	status := wechatpay.GetLoopStatus()
	resp := &LoopStatusResponse{Code: apipb.Code_Success, Data: &status}
	if err := wechatpay.SaveStatsConfigSnapshot(wechatpay.SnapshotFromLoopStatus(status)); err != nil {
		resp.Message = "参数已生效但持久化失败: " + err.Error()
	}
	return resp, nil
}

// ResetWechatPayStatsConfig 清除 DB 覆盖快照并把对账参数恢复为 Nacos 基线。
//
//	@Summary 重置对账参数为 Nacos 基线
//	@Tags 微信支付订单管理
//	@Param authorization header string true "jwt token"
//	@Success 200 {object} LoopStatusResponse
//	@Router /api/core/wechat/pay/stats/config [delete]
func ResetWechatPayStatsConfig(c *gin.Context) {
	resp := &LoopStatusResponse{Code: apipb.Code_Success}
	if err := wechatpay.ResetStatsConfigToBaseline(); err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}
	status := wechatpay.GetLoopStatus()
	resp.Data = &status
	c.JSON(http.StatusOK, resp)
}
