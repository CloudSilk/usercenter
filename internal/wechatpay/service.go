package wechatpay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	"github.com/google/uuid"
)

var (
	// ErrOrderNotOwned 订单不属于当前租户/用户。
	ErrOrderNotOwned = errors.New("无权操作该订单")
	// ErrOrderAlreadyPaid 订单已支付,不可重复支付或关单。
	ErrOrderAlreadyPaid = errors.New("订单已支付")
	// ErrOrderClosed 订单已关闭。
	ErrOrderClosed = errors.New("订单已关闭")
	// ErrUnknownNotifyOrder 回调中的订单在本系统不存在或与回调应用不匹配。
	ErrUnknownNotifyOrder = errors.New("支付回调订单不存在")
)

// OpenIDResolver 通过用户ID+微信配置ID解析支付 openID,测试可替换。
var OpenIDResolver = user.GetOpenIDByUserIDAndConfigID

// wechat 商户订单号规则:6-32位数字/字母/-_;退款单号允许更长(上限64)。
var tradeNoPattern = regexp.MustCompile(`^[0-9A-Za-z_-]{6,32}$`)
var refundNoPattern = regexp.MustCompile(`^[0-9A-Za-z_-]{6,64}$`)

// CreateOrderInput JSAPI 下单业务入参。
type CreateOrderInput struct {
	TenantID      string
	UserID        string
	App           string
	Description   string
	Attach        string
	AmountFen     int64
	OutTradeNo    string
	ExpireMinutes int // 订单有效期(分钟),<=0 使用默认值
}

// defaultOrderExpireMinutes 默认订单有效期:2小时。
const defaultOrderExpireMinutes = 120

func newOutTradeNo() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

// CreateJSAPIPayment 小程序 JSAPI 下单:解析商户配置与 openID、预支付,
// 持久化订单并返回 wx.requestPayment 签名参数。
// 复用已存在的 CREATED 订单号会重新预支付(微信侧 prepay_id 保持一致),保证客户端安全重试。
func CreateJSAPIPayment(ctx context.Context, in CreateOrderInput) (*PayOrder, *PayParams, error) {
	outTradeNo := in.OutTradeNo
	if outTradeNo == "" {
		outTradeNo = newOutTradeNo()
	} else if !tradeNoPattern.MatchString(outTradeNo) {
		return nil, nil, errors.New("商户订单号需为6-32位数字/字母/-_")
	}
	if in.AmountFen <= 0 {
		return nil, nil, errors.New("订单金额必须大于0")
	}
	if strings.TrimSpace(in.Description) == "" {
		return nil, nil, errors.New("订单描述不能为空")
	}

	cfg, wc, err := GetEnabledPayConfigByApp(in.App)
	if err != nil {
		return nil, nil, err
	}
	if cfg.TenantID != in.TenantID {
		return nil, nil, ErrOrderNotOwned
	}
	if cfg.NotifyURL == "" {
		return nil, nil, errors.New("支付配置缺少回调地址")
	}

	order, err := GetPayOrderByOutTradeNo(outTradeNo)
	if err != nil {
		return nil, nil, err
	}
	if order != nil {
		if order.TenantID != in.TenantID || (in.UserID != "" && order.UserID != in.UserID) {
			return nil, nil, ErrOrderNotOwned
		}
		switch order.Status {
		case PayOrderPaid:
			return nil, nil, ErrOrderAlreadyPaid
		case PayOrderClosed:
			return nil, nil, ErrOrderClosed
		}
	}

	openID, err := OpenIDResolver(in.UserID, wc.ID)
	if err != nil {
		return nil, nil, err
	}
	if openID == "" {
		return nil, nil, errors.New("当前用户未绑定该微信应用,请先通过微信登录")
	}

	if in.ExpireMinutes <= 0 {
		in.ExpireMinutes = defaultOrderExpireMinutes
	}
	expireAt := time.Now().Add(time.Duration(in.ExpireMinutes) * time.Minute)

	api, err := GetPayAPI(cfg)
	if err != nil {
		return nil, nil, err
	}
	params, err := api.Prepay(ctx, PrepayInput{
		AppID:       cfg.AppID,
		MchID:       cfg.MchID,
		Description: in.Description,
		OutTradeNo:  outTradeNo,
		NotifyURL:   cfg.NotifyURL,
		Attach:      in.Attach,
		OpenID:      openID,
		AmountFen:   in.AmountFen,
		TimeExpire:  expireAt,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("微信下单失败: %w", err)
	}

	if order == nil {
		order = &PayOrder{
			TenantID:       cfg.TenantID,
			UserID:         in.UserID,
			WechatConfigID: cfg.WechatConfigID,
			AppID:          cfg.AppID,
			MchID:          cfg.MchID,
			OutTradeNo:     outTradeNo,
			OpenID:         openID,
			Description:    in.Description,
			Attach:         in.Attach,
			Amount:         in.AmountFen,
			Currency:       "CNY",
			Status:         PayOrderCreated,
			PrepayID:       params.PrepayID,
			ExpireAt:       &expireAt,
		}
		if _, err := CreatePayOrder(order); err != nil {
			return nil, nil, err
		}
	} else {
		// 复用 CREATED 订单:对齐最新有效期
		if err := store.DB().Model(&PayOrder{}).Where("id = ?", order.ID).
			Update("expire_at", expireAt).Error; err != nil {
			return nil, nil, err
		}
		order.ExpireAt = &expireAt
		if order.PrepayID != params.PrepayID {
			if err := UpdatePayOrderPrepay(order.ID, params.PrepayID); err != nil {
				return nil, nil, err
			}
		}
	}
	return order, params, nil
}

// applyTransaction 用上游查单结果推进本地订单状态。
func applyTransaction(order *PayOrder, tx *TransactionResult) {
	switch tx.TradeState {
	case "SUCCESS":
		paidAt := tx.SuccessTime
		if paidAt.IsZero() {
			paidAt = time.Now()
		}
		if _, err := MarkOrderPaid(order.ID, tx.TransactionID, paidAt); err != nil {
			log.Errorf(context.Background(), "同步订单 %s 支付状态失败:%v", order.OutTradeNo, err)
			return
		}
		order.Status = PayOrderPaid
		order.TradeState = "SUCCESS"
		order.TransactionID = tx.TransactionID
		order.PaidAt = &paidAt
	case "CLOSED", "REVOKED", "PAYERROR":
		if err := MarkOrderClosed(order.ID, tx.TradeStateDesc); err != nil {
			log.Errorf(context.Background(), "同步订单 %s 关闭状态失败:%v", order.OutTradeNo, err)
			return
		}
		order.Status = PayOrderClosed
		order.TradeState = tx.TradeState
		order.TradeStateDesc = tx.TradeStateDesc
	default:
		_ = UpdatePayOrderEvent(order.ID, tx.TradeState, tx.TradeStateDesc, "")
		order.TradeState = tx.TradeState
		order.TradeStateDesc = tx.TradeStateDesc
	}
}

// loadOrderWithOwnership 校验归属并加载订单。
// tenantID 为空表示平台侧操作,跳过租户校验;userID 为空表示管理端操作,跳过用户校验。
func loadOrderWithOwnership(tenantID, userID, outTradeNo string) (*PayOrder, error) {
	order, err := GetPayOrderByOutTradeNo(outTradeNo)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.New("订单不存在")
	}
	if (tenantID != "" && order.TenantID != tenantID) ||
		(userID != "" && order.UserID != userID) {
		return nil, ErrOrderNotOwned
	}
	return order, nil
}

// SyncOrderStatus 查询订单状态;CREATED 订单复用对账逻辑:
// 先向微信侧查单对账,仍未支付且已过失效时间则关单兜底。
func SyncOrderStatus(ctx context.Context, tenantID, userID, outTradeNo string) (*PayOrder, error) {
	order, err := loadOrderWithOwnership(tenantID, userID, outTradeNo)
	if err != nil {
		return nil, err
	}
	if order.Status != PayOrderCreated {
		return order, nil
	}
	cfg, err := GetPayConfigByWechatConfigID(order.WechatConfigID)
	if err != nil {
		return order, nil // 配置已删除等场景:仅返回本地状态
	}
	api, err := GetPayAPI(cfg)
	if err != nil {
		return order, nil
	}
	reconcileOrder(ctx, cfg.MchID, api, order)
	return order, nil
}

// ClosePayOrder 关闭订单:先调微信侧关单(未预支付的订单直接本地关闭)。
func ClosePayOrder(ctx context.Context, tenantID, userID, outTradeNo string) error {
	order, err := loadOrderWithOwnership(tenantID, userID, outTradeNo)
	if err != nil {
		return err
	}
	if order.Status == PayOrderPaid {
		return ErrOrderAlreadyPaid
	}
	if order.Status == PayOrderClosed {
		return nil
	}
	if order.PrepayID != "" {
		cfg, err := GetPayConfigByWechatConfigID(order.WechatConfigID)
		if err != nil {
			return fmt.Errorf("支付配置不可用: %w", err)
		}
		api, err := GetPayAPI(cfg)
		if err != nil {
			return err
		}
		if err := api.Close(ctx, cfg.MchID, outTradeNo); err != nil {
			return fmt.Errorf("微信关单失败: %w", err)
		}
	}
	order.Status = PayOrderClosed
	return MarkOrderClosed(order.ID, "商户关单")
}

// ApplyRefundInput 申请退款业务入参(管理端操作)。
type ApplyRefundInput struct {
	TenantID     string
	OutTradeNo   string
	RefundAmount int64  // 退款金额,单位:分
	OutRefundNo  string // 可选,留空自动生成
	ReasonCode   string // 退款原因类别,留空默认 other
	Reason       string // 原因补充说明(自由文本)
}

// ApplyRefund 对已支付订单发起退款:校验可退余额后调微信侧申请退款并落库。
// 同一 outRefundNo 重复申请直接返回已有退款单(幂等)。
func ApplyRefund(ctx context.Context, in ApplyRefundInput) (*PayRefund, error) {
	if in.RefundAmount <= 0 {
		return nil, errors.New("退款金额必须大于0")
	}
	reasonCode, err := NormalizeReasonCode(in.ReasonCode)
	if err != nil {
		return nil, err
	}
	in.ReasonCode = reasonCode
	order, err := GetPayOrderByOutTradeNo(in.OutTradeNo)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.New("订单不存在")
	}
	if order.TenantID != in.TenantID {
		return nil, ErrOrderNotOwned
	}
	if order.Status != PayOrderPaid {
		return nil, errors.New("仅已支付订单可退款")
	}

	outRefundNo := in.OutRefundNo
	if outRefundNo == "" {
		outRefundNo = "rf" + newOutTradeNo()
	} else if !refundNoPattern.MatchString(outRefundNo) {
		return nil, errors.New("退款单号需为6-64位数字/字母/-_")
	}
	if existing, err := GetPayRefundByOutRefundNo(outRefundNo); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.PayOrderID != order.ID {
			return nil, errors.New("退款单号已被其他订单使用")
		}
		return existing, nil // 幂等:同一退款单号重复申请
	}

	refunded, err := SumActiveRefundAmount(order.ID)
	if err != nil {
		return nil, err
	}
	if in.RefundAmount > order.Amount-refunded {
		return nil, fmt.Errorf("退款金额超过可退余额(已退/退款中 %d 分)", refunded)
	}

	cfg, err := GetPayConfigByWechatConfigID(order.WechatConfigID)
	if err != nil {
		return nil, fmt.Errorf("支付配置不可用: %w", err)
	}

	refund := &PayRefund{
		TenantID:    order.TenantID,
		UserID:      order.UserID,
		PayOrderID:  order.ID,
		OutTradeNo:  order.OutTradeNo,
		OutRefundNo: outRefundNo,
		Amount:      in.RefundAmount,
		Total:       order.Amount,
		ReasonCode:  in.ReasonCode,
		Reason:      in.Reason,
	}

	// 审核流开启:仅落库待审核,审核通过后才调用微信侧退款
	if cfg.RefundApprovalRequired {
		refund.Status = RefundPending
		if _, err := CreatePayRefund(refund); err != nil {
			return nil, err
		}
		log.Infof(ctx, "订单 %s 退款申请待审核(退款单 %s, %d 分)", order.OutTradeNo, outRefundNo, in.RefundAmount)
		return refund, nil
	}

	api, err := GetPayAPI(cfg)
	if err != nil {
		return nil, err
	}
	refundNotifyURL := cfg.RefundNotifyURL
	if refundNotifyURL == "" {
		refundNotifyURL = cfg.NotifyURL // 未配置独立退款回调时复用支付回调
	}
	result, err := api.Refund(ctx, RefundInput{
		MchID:           cfg.MchID,
		OutTradeNo:      order.OutTradeNo,
		OutRefundNo:     outRefundNo,
		Reason:          in.Reason,
		NotifyURL:       refundNotifyURL,
		RefundAmountFen: in.RefundAmount,
		TotalFen:        order.Amount,
	})
	if err != nil {
		return nil, fmt.Errorf("微信退款申请失败: %w", err)
	}
	status := result.Status
	if status == "" {
		status = RefundProcessing
	}
	refund.Status = status
	refund.RefundID = result.RefundID
	if status == RefundSuccess && !result.SuccessTime.IsZero() {
		refund.SuccessTime = &result.SuccessTime
	}
	if _, err := CreatePayRefund(refund); err != nil {
		return nil, err
	}
	log.Infof(ctx, "订单 %s 退款申请受理(退款单 %s, %d 分)", order.OutTradeNo, outRefundNo, in.RefundAmount)
	return refund, nil
}

// ApproveRefundInput 审核退款业务入参。
type ApproveRefundInput struct {
	TenantID    string
	ApproverID  string
	OutRefundNo string
	Approved    bool
	Comment     string
}

// ApproveRefund 审核待审核退款单:通过则调用微信侧退款,拒绝则置 REJECTED。
// 仅 PENDING 状态可审核,状态守卫防并发重复审核。
func ApproveRefund(ctx context.Context, in ApproveRefundInput) (*PayRefund, error) {
	refund, err := GetPayRefundByOutRefundNo(in.OutRefundNo)
	if err != nil {
		return nil, err
	}
	if refund == nil {
		return nil, errors.New("退款单不存在")
	}
	if in.TenantID != "" && refund.TenantID != in.TenantID {
		return nil, ErrOrderNotOwned
	}
	if refund.Status != RefundPending {
		return nil, fmt.Errorf("仅待审核退款单可审核,当前状态 %s", refund.Status)
	}

	if !in.Approved {
		if _, err := MarkRefundApproved(refund.ID, in.ApproverID, in.Comment, false, "", "", time.Time{}); err != nil {
			return nil, err
		}
		recordRefundAudit(in, refund, "")
		log.Infof(ctx, "退款单 %s 审核拒绝(审核人 %s)", in.OutRefundNo, in.ApproverID)
		return GetPayRefundByOutRefundNo(in.OutRefundNo)
	}

	order, err := GetPayOrderByOutTradeNo(refund.OutTradeNo)
	if err != nil || order == nil {
		return nil, fmt.Errorf("原支付订单不可用")
	}
	cfg, err := GetPayConfigByWechatConfigID(order.WechatConfigID)
	if err != nil {
		return nil, fmt.Errorf("支付配置不可用: %w", err)
	}
	api, err := GetPayAPI(cfg)
	if err != nil {
		return nil, err
	}
	refundNotifyURL := cfg.RefundNotifyURL
	if refundNotifyURL == "" {
		refundNotifyURL = cfg.NotifyURL
	}
	result, err := api.Refund(ctx, RefundInput{
		MchID:           cfg.MchID,
		OutTradeNo:      order.OutTradeNo,
		OutRefundNo:     refund.OutRefundNo,
		Reason:          refund.Reason,
		NotifyURL:       refundNotifyURL,
		RefundAmountFen: refund.Amount,
		TotalFen:        order.Amount,
	})
	if err != nil {
		return nil, fmt.Errorf("微信退款申请失败: %w", err)
	}
	if _, err := MarkRefundApproved(refund.ID, in.ApproverID, in.Comment, true, result.Status, result.RefundID, result.SuccessTime); err != nil {
		return nil, err
	}
	recordRefundAudit(in, refund, result.RefundID)
	log.Infof(ctx, "退款单 %s 审核通过并已提交微信(审核人 %s)", in.OutRefundNo, in.ApproverID)
	return GetPayRefundByOutRefundNo(in.OutRefundNo)
}

// 退款审核审计动作。
const (
	AuditActionRefundApprove = "pay_refund_approve"
	AuditActionRefundReject  = "pay_refund_reject"
)

// recordRefundAudit 退款审核操作落审计日志:人工主体(PrincipalKind=0),
// 不受对账告警静默窗口影响,每次人工审核均留痕。
func recordRefundAudit(in ApproveRefundInput, refund *PayRefund, wxRefundID string) {
	action := AuditActionRefundApprove
	if !in.Approved {
		action = AuditActionRefundReject
	}
	detail, err := json.Marshal(map[string]any{
		"outRefundNo": refund.OutRefundNo,
		"outTradeNo":  refund.OutTradeNo,
		"amount":      refund.Amount,
		"reasonCode":  refund.ReasonCode,
		"approved":    in.Approved,
		"comment":     in.Comment,
		"wxRefundID":  wxRefundID,
	})
	if err != nil {
		detail = []byte("{}")
	}
	auditRecorder(store.DB(), in.ApproverID, "", 0, action, refund.ID, "", string(detail))
}

// SyncRefundStatus 向微信侧查退款单并对账更新本地状态。
// tenantID 为空表示平台侧操作,不校验租户归属;非空时校验退款单归属。
func SyncRefundStatus(ctx context.Context, tenantID, outRefundNo string) (*PayRefund, error) {
	refund, err := GetPayRefundByOutRefundNo(outRefundNo)
	if err != nil {
		return nil, err
	}
	if refund == nil {
		return nil, errors.New("退款单不存在")
	}
	if tenantID != "" && refund.TenantID != tenantID {
		return nil, ErrOrderNotOwned
	}
	if refund.Status == RefundSuccess || refund.Status == RefundClosed {
		return refund, nil // 终态不再查单
	}
	cfg, err := GetPayConfigByWechatConfigID(refundOrderConfigID(refund))
	if err != nil {
		return refund, nil
	}
	api, err := GetPayAPI(cfg)
	if err != nil {
		return refund, nil
	}
	if result, err := api.QueryRefund(ctx, outRefundNo); err == nil && result.Status != "" {
		updated, err := MarkRefundStatus(refund.ID, result.RefundID, result.Status, result.SuccessTime)
		if err != nil {
			log.Errorf(ctx, "同步退款单 %s 状态失败:%v", outRefundNo, err)
			return refund, nil
		}
		if updated {
			refund.Status = result.Status
			if result.Status == RefundSuccess && !result.SuccessTime.IsZero() {
				refund.SuccessTime = &result.SuccessTime
			}
		}
	}
	return refund, nil
}

// refundOrderConfigID 退款单关联的微信应用配置ID(通过原支付订单)。
func refundOrderConfigID(refund *PayRefund) string {
	order, err := GetPayOrderByOutTradeNo(refund.OutTradeNo)
	if err != nil || order == nil {
		return ""
	}
	return order.WechatConfigID
}

// applyRefundEvent 用退款回调/查单结果推进退款单状态。
func applyRefundEvent(order *PayOrder, content *NotifyContent) error {
	refund, err := GetPayRefundByOutRefundNo(content.OutRefundNo)
	if err != nil {
		return err
	}
	if refund == nil || refund.PayOrderID != order.ID {
		return ErrUnknownNotifyOrder
	}
	switch content.RefundStatus {
	case RefundSuccess, RefundClosed, RefundAbnormal:
		successTime := content.RefundSuccessTime
		updated, err := MarkRefundStatus(refund.ID, content.RefundID, content.RefundStatus, successTime)
		if err != nil {
			return err
		}
		if updated {
			log.Infof(context.Background(), "退款单 %s 状态更新为 %s", content.OutRefundNo, content.RefundStatus)
		}
		return nil
	default:
		// PROCESSING 等中间态:仅留痕到原订单
		return UpdatePayOrderEvent(order.ID, "", content.EventType, content.EventType)
	}
}

// ListUserOrders 用户分页查询自己的支付订单(仅限当前租户)。
func ListUserOrders(tenantID, userID, status string, pageIndex, pageSize int) (*PayOrderListResult, error) {
	if userID == "" {
		return nil, errors.New("缺少用户身份")
	}
	return QueryPayOrders(&PayOrderQuery{
		TenantID:  tenantID,
		UserID:    userID,
		Status:    status,
		PageIndex: pageIndex,
		PageSize:  pageSize,
	})
}

// ListUserRefunds 用户查询自己订单的退款记录(校验订单归属)。
func ListUserRefunds(tenantID, userID, outTradeNo string) ([]*PayRefund, error) {
	if _, err := loadOrderWithOwnership(tenantID, userID, outTradeNo); err != nil {
		return nil, err
	}
	return ListRefundsByOutTradeNo(outTradeNo)
}

// HandlePayNotify 微信支付/退款结果回调:验签+解密+幂等更新订单或退款单。
// 返回 nil 表示处理成功(应答微信 SUCCESS);返回错误时调用方应答 FAIL,微信会重试。
func HandlePayNotify(req *http.Request, app string) error {
	cfg, _, err := GetEnabledPayConfigByApp(app)
	if err != nil {
		return err
	}
	api, err := GetPayAPI(cfg)
	if err != nil {
		return err
	}
	content, err := api.ParseNotify(req)
	if err != nil {
		return err
	}
	order, err := GetPayOrderByOutTradeNo(content.OutTradeNo)
	if err != nil {
		return err
	}
	if order == nil || order.WechatConfigID != cfg.WechatConfigID {
		return ErrUnknownNotifyOrder
	}

	// 退款事件:推进退款单状态(幂等),不影响原订单支付状态
	if strings.HasPrefix(content.EventType, "REFUND.") {
		return applyRefundEvent(order, content)
	}

	if content.AmountFen > 0 && content.AmountFen != order.Amount {
		log.Warnf(req.Context(), "订单 %s 回调金额(%d分)与订单金额(%d分)不一致", order.OutTradeNo, content.AmountFen, order.Amount)
	}

	if content.EventType == "TRANSACTION.SUCCESS" || content.TradeState == "SUCCESS" {
		paidAt := content.SuccessTime
		if paidAt.IsZero() {
			paidAt = time.Now()
		}
		updated, err := MarkOrderPaid(order.ID, content.TransactionID, paidAt)
		if err != nil {
			return err
		}
		if updated {
			log.Infof(req.Context(), "订单 %s 支付成功(微信单号 %s)", order.OutTradeNo, content.TransactionID)
		}
		return nil
	}
	// 关闭等其他事件:仅留痕
	return UpdatePayOrderEvent(order.ID, content.TradeState, content.TradeStateDesc, content.EventType)
}

// BatchCloseFailure 单笔关单失败记录。
type BatchCloseFailure struct {
	OutTradeNo string `json:"outTradeNo"`
	Error      string `json:"error"`
}

// BatchCloseResult 批量关单结果。
type BatchCloseResult struct {
	Closed   int                 `json:"closed"`
	Skipped  int                 `json:"skipped"`
	Failures []BatchCloseFailure `json:"failures,omitempty"`
}

// maxBatchCloseSize 显式订单号批量关单的上限。
const maxBatchCloseSize = 100

// BatchCloseOrders 管理端批量关单:仅 CREATED 订单会被关闭;
// 已支付/已关闭订单计入 skipped(非错误)。tenantID 用于租户隔离。
func BatchCloseOrders(ctx context.Context, tenantID string, outTradeNos []string) (*BatchCloseResult, error) {
	if len(outTradeNos) == 0 {
		return nil, errors.New("订单号列表不能为空")
	}
	if len(outTradeNos) > maxBatchCloseSize {
		return nil, fmt.Errorf("单次批量关单上限 %d 笔", maxBatchCloseSize)
	}
	result := &BatchCloseResult{Failures: []BatchCloseFailure{}}
	for _, no := range outTradeNos {
		// 先按状态分类:PAID/CLOSED 属于跳过而非错误;仅 CREATED 需要关单动作
		order, err := GetPayOrderByOutTradeNo(no)
		if err != nil {
			result.Failures = append(result.Failures, BatchCloseFailure{OutTradeNo: no, Error: err.Error()})
			continue
		}
		if order == nil {
			result.Failures = append(result.Failures, BatchCloseFailure{OutTradeNo: no, Error: "订单不存在"})
			continue
		}
		if order.TenantID != tenantID && tenantID != "" {
			result.Failures = append(result.Failures, BatchCloseFailure{OutTradeNo: no, Error: ErrOrderNotOwned.Error()})
			continue
		}
		switch order.Status {
		case PayOrderPaid, PayOrderClosed:
			result.Skipped++
			continue
		}
		if err := ClosePayOrder(ctx, tenantID, "", no); err != nil {
			result.Failures = append(result.Failures, BatchCloseFailure{OutTradeNo: no, Error: err.Error()})
			continue
		}
		result.Closed++
	}
	log.Infof(ctx, "批量关单完成:关闭 %d,跳过 %d,失败 %d", result.Closed, result.Skipped, len(result.Failures))
	return result, nil
}

// ListCreatedTradeNosByAge 取指定租户创建早于 before 的 CREATED 订单号(过期清理场景)。
func ListCreatedTradeNosByAge(tenantID string, before time.Time, limit int) ([]string, error) {
	var nos []string
	db := store.DB().Model(&PayOrder{}).Where("status = ? AND created_at < ?", PayOrderCreated, before)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	err := db.Limit(limit).Order("created_at ASC").Pluck("out_trade_no", &nos).Error
	return nos, err
}

// 账单类型:所有流水/仅成功/仅退款。
const (
	BillTypeAll     = "ALL"
	BillTypeSuccess = "SUCCESS"
	BillTypeRefund  = "REFUND"
)

// normalizeBillType 归一化账单类型:空默认 ALL,非法值报错。
func normalizeBillType(billType string) (string, error) {
	if billType == "" {
		return BillTypeAll, nil
	}
	switch billType {
	case BillTypeAll, BillTypeSuccess, BillTypeRefund:
		return billType, nil
	}
	return "", errors.New("非法账单类型: " + billType)
}

// GetTradeBill 下载指定商户配置在 billDate(YYYY-MM-DD)、指定类型(billType: ALL/SUCCESS/REFUND,
// 留空 ALL)的交易账单 CSV。微信侧账单 T+1 生成,当日/未来日期直接拒绝;
// tenantID 为空表示平台侧操作。
func GetTradeBill(ctx context.Context, tenantID, configID, billDate, billType string) ([]byte, error) {
	billType, err := normalizeBillType(billType)
	if err != nil {
		return nil, err
	}
	d, err := time.ParseInLocation("2006-01-02", billDate, time.Local)
	if err != nil {
		return nil, errors.New("账单日期格式应为 YYYY-MM-DD")
	}
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	if !d.Before(today) {
		return nil, errors.New("交易账单仅可申请昨日及更早日期")
	}
	cfg, err := GetPayConfigByID(configID)
	if err != nil {
		return nil, fmt.Errorf("支付配置不存在")
	}
	if tenantID != "" && cfg.TenantID != tenantID {
		return nil, ErrOrderNotOwned
	}
	api, err := GetPayAPI(cfg)
	if err != nil {
		return nil, err
	}
	return api.DownloadTradeBill(ctx, billDate, billType)
}
