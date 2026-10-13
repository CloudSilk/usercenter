// Package pay 是 usercenter 对外提供的可复用支付渠道包，供嵌入 usercenter 的业务服务直接使用：
//
//   - Wechat：微信支付 APIv3 JSAPI（普通商户，小程序 wx.requestPayment）
//   - XPay：小程序虚拟支付（wx.requestVirtualPayment，企业/个体户与个人主体均适用）
//   - Mock：本地开发与演示
//
// 渠道只负责“与支付机构交互 + 验签解密”；订单状态机与履约由调用方实现，
// 虚拟支付的“每个微信单号只发货一次”由 DeliveryLedger（GORM 模型 PayDelivery）保证。
// 通过 Config + New 按配置选择渠道。该包不改变 internal/wechatpay 既有的 JSAPI 下单接口与表结构。
package pay

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// PrepayReq 统一下单
type PrepayReq struct {
	OrderID     string
	Description string
	AmountFen   int64
	OpenID      string // 小程序用户 openid（JSAPI 必填）
	ExpireAt    time.Time

	// 以下字段仅虚拟支付（XPay）使用
	ProductID  string // MP 后台「道具管理」中已发布的道具 ID
	ListFen    int64  // 道具标价（分，须与后台道具价格一致，即 signData.goodsPrice）；AmountFen 低于它时作为 activitySellingPrice 传入
	Quantity   int    // 购买数量，默认 1
	SessionKey string // 当前用户有效的 session_key（用户态签名）；为空时用 LoginCode 经 code2Session 换取
	LoginCode  string // 小程序 wx.login 拿到的 code
	Platform   string // 客户端平台：ios / android / windows / devtools…；iOS 走 Apple 支付，最低 1 元
	Attach     string // 透传数据，发货推送时原样返回；默认订单号
}

// Txn 查单 / 支付回调结果
type Txn struct {
	OrderID       string
	TransactionID string
	State         string // SUCCESS / NOTPAY / CLOSED / REFUND / USERPAYING / PAYERROR
	AmountFen     int64
	PaidAt        *time.Time
}

// RefundReq 申请退款
type RefundReq struct {
	OrderID, RefundID, Reason string
	RefundFen, TotalFen       int64
}

// RefundResult 退款结果 / 退款回调
type RefundResult struct {
	OrderID, RefundID, ProviderRefundID string
	State                               string // SUCCESS / PROCESSING / ABNORMAL / CLOSED
}

// Notify 已验签解密的回调
type Notify struct {
	EventID string
	Kind    string // pay / refund / verify（消息推送 URL 校验）/ other
	Txn     *Txn
	Refund  *RefundResult
	// OpenID 付款用户（虚拟支付推送携带）
	OpenID string
	// Raw 解密后的原始推送内容（调试与审计）
	Raw []byte
	// Echo URL 校验时需原样返回的 echostr
	Echo string
}

// Acker 由需要特定回包格式的渠道实现（如虚拟支付消息推送须回 {"ErrCode":0,"ErrMsg":"success"}）。
// 未实现时调用方自行回 2xx。
type Acker interface {
	Ack(n *Notify, err error) (status int, contentType string, body []byte)
}

// Provider 支付渠道
type Provider interface {
	Name() string
	// Prepay 下单并返回小程序 wx.requestPayment 参数
	Prepay(ctx context.Context, r PrepayReq) (prepayID string, params map[string]string, err error)
	Query(ctx context.Context, orderID string) (*Txn, error)
	Close(ctx context.Context, orderID string) error
	Refund(ctx context.Context, r RefundReq) (*RefundResult, error)
	// ParseNotify 验签 + 解密；签名不通过返回 ErrBadSignature
	ParseNotify(ctx context.Context, req *http.Request) (*Notify, error)
}

var (
	ErrBadSignature = errors.New("pay: 回调验签失败")
	// ErrNotSupported 渠道不支持该操作（如虚拟支付无关单接口、iOS 订单无法主动退款）
	ErrNotSupported = errors.New("pay: 渠道不支持该操作")
)
