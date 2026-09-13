package wechatpay

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
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

// wechat 商户订单号规则:6-32位数字/字母/-_。
var tradeNoPattern = regexp.MustCompile(`^[0-9A-Za-z_-]{6,32}$`)

// CreateOrderInput JSAPI 下单业务入参。
type CreateOrderInput struct {
	TenantID    string
	UserID      string
	App         string
	Description string
	Attach      string
	AmountFen   int64
	OutTradeNo  string
}

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
		}
		if _, err := CreatePayOrder(order); err != nil {
			return nil, nil, err
		}
	} else if order.PrepayID != params.PrepayID {
		if err := UpdatePayOrderPrepay(order.ID, params.PrepayID); err != nil {
			return nil, nil, err
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
func loadOrderWithOwnership(tenantID, userID, outTradeNo string) (*PayOrder, error) {
	order, err := GetPayOrderByOutTradeNo(outTradeNo)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.New("订单不存在")
	}
	if order.TenantID != tenantID || (userID != "" && order.UserID != userID) {
		return nil, ErrOrderNotOwned
	}
	return order, nil
}

// SyncOrderStatus 查询订单状态;CREATED 状态下主动向微信侧查单对账。
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
	if tx, err := api.Query(ctx, cfg.MchID, outTradeNo); err == nil {
		applyTransaction(order, tx)
	}
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

// HandlePayNotify 支付结果回调:验签+解密+幂等更新订单。
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
	// 退款/关闭等其他事件:仅留痕,退款单管理为后续迭代。
	return UpdatePayOrderEvent(order.ID, content.TradeState, content.TradeStateDesc, content.EventType)
}
