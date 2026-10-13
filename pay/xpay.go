package pay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// XPayConfig 小程序虚拟支付配置（企业/事业单位/个体工商户与个人主体小程序通用）
type XPayConfig struct {
	AppID         string // 小程序 AppID
	AppSecret     string // 小程序 AppSecret（获取 access_token 与 code2Session；已提供 AccessToken/SessionKey 时可空）
	OfferID       string // 虚拟支付 → 基本配置 → OfferID
	AppKey        string // 现网 AppKey（env=0）
	SandboxAppKey string // 沙箱 AppKey（env=1）
	Env           int    // 0 现网（个人主体固定 0；Apple 支付不支持沙箱）/ 1 沙箱

	// 消息推送（开发管理 → 消息推送）：Token 必填；EncodingAESKey 安全/兼容模式必填
	PushToken  string
	PushAESKey string

	// IOSMinFen iOS（Apple 支付）最低金额，默认 100 分（1 元）
	IOSMinFen int64

	// AccessToken 接口调用凭证来源；为空时用 AppID/AppSecret 走 stable_token
	AccessToken func(ctx context.Context) (string, error)
	// OpenIDOf 按业务单号返回下单用户 openid（query_order / refund_order 必填）
	OpenIDOf func(ctx context.Context, orderID string) (string, error)

	BaseURL    string // 测试用，默认 https://api.weixin.qq.com
	HTTPClient *http.Client
}

// ErrIOSMinAmount iOS 端 Apple 支付最低 1 元
var ErrIOSMinAmount = errors.New("xpay: iOS 端（Apple 支付）最低支付金额为 1 元")

// outTradeNo：8-32 位数字、大小写字母、_-|*@，不能以下划线开头（wx.requestVirtualPayment 文档）
var xpayTradeNo = regexp.MustCompile(`^[0-9A-Za-z\-|*@][0-9A-Za-z_\-|*@]{7,31}$`)

// XPay 小程序虚拟支付渠道（道具直购 short_series_goods）
type XPay struct {
	cfg    XPayConfig
	api    *XPayClient
	crypto *PushCrypto
}

// NewXPay 校验配置并创建渠道
func NewXPay(cfg XPayConfig) (*XPay, error) {
	if cfg.AppID == "" || cfg.OfferID == "" || cfg.PushToken == "" {
		return nil, fmt.Errorf("xpay: 配置不完整（AppID、OfferID、消息推送 Token）")
	}
	if cfg.Env != 0 && cfg.Env != 1 {
		return nil, fmt.Errorf("xpay: env 只能为 0（现网）或 1（沙箱）")
	}
	key := cfg.AppKey
	if cfg.Env == 1 {
		key = cfg.SandboxAppKey
	}
	if key == "" {
		return nil, fmt.Errorf("xpay: 缺少 env=%d 对应的 AppKey", cfg.Env)
	}
	if cfg.AccessToken == nil {
		if cfg.AppSecret == "" {
			return nil, fmt.Errorf("xpay: 需要 AppSecret 或自定义 AccessToken")
		}
		ts := &StableTokenSource{AppID: cfg.AppID, AppSecret: cfg.AppSecret, BaseURL: cfg.BaseURL, HTTPClient: cfg.HTTPClient}
		cfg.AccessToken = ts.Token
	}
	if cfg.IOSMinFen <= 0 {
		cfg.IOSMinFen = 100
	}
	pc, err := NewPushCrypto(cfg.PushToken, cfg.PushAESKey, cfg.AppID)
	if err != nil {
		return nil, err
	}
	return &XPay{cfg: cfg, crypto: pc, api: &XPayClient{AppKey: key, Env: cfg.Env, AccessToken: cfg.AccessToken, BaseURL: cfg.BaseURL, HTTPClient: cfg.HTTPClient}}, nil
}

// Name 渠道名
func (x *XPay) Name() string { return "xpay" }

// API 暴露底层 /xpay/* 客户端（下载账单、补报发货等）
func (x *XPay) API() *XPayClient { return x.api }

// Env 当前环境
func (x *XPay) Env() int { return x.cfg.Env }

// Prepay 生成 wx.requestVirtualPayment 参数。虚拟支付没有服务端预下单，prepayID 即业务单号。
func (x *XPay) Prepay(ctx context.Context, r PrepayReq) (string, map[string]string, error) {
	if !xpayTradeNo.MatchString(r.OrderID) {
		return "", nil, fmt.Errorf("xpay: 业务单号需 8-32 位（数字/字母/_-|*@，不能以 _ 开头）")
	}
	if r.ProductID == "" {
		return "", nil, fmt.Errorf("xpay: 未配置道具 ID（MP 后台虚拟支付 → 道具管理）")
	}
	q := r.Quantity
	if q <= 0 {
		q = 1
	}
	list := r.ListFen
	if list <= 0 {
		list = r.AmountFen
	}
	if r.AmountFen <= 0 || r.AmountFen > list*int64(q) {
		return "", nil, fmt.Errorf("xpay: 金额无效")
	}
	if strings.EqualFold(r.Platform, "ios") && r.AmountFen < x.cfg.IOSMinFen {
		return "", nil, ErrIOSMinAmount
	}
	sk := r.SessionKey
	if sk == "" {
		if r.LoginCode == "" || x.cfg.AppSecret == "" {
			return "", nil, fmt.Errorf("xpay: 缺少用户 session_key（请先 wx.login 并上传 code）")
		}
		s, err := Code2Session(ctx, x.cfg.HTTPClient, x.cfg.BaseURL, x.cfg.AppID, x.cfg.AppSecret, r.LoginCode)
		if err != nil {
			return "", nil, err
		}
		if r.OpenID != "" && s.OpenID != r.OpenID {
			return "", nil, fmt.Errorf("xpay: 登录凭证与当前用户不符")
		}
		sk = s.SessionKey
	}
	attach := r.Attach
	if attach == "" {
		attach = r.OrderID
	}
	d := XPaySignData{OfferID: x.cfg.OfferID, BuyQuantity: q, Env: x.cfg.Env, CurrencyType: "CNY", ProductID: r.ProductID, GoodsPrice: list, OutTradeNo: r.OrderID, Attach: attach}
	if unit := r.AmountFen / int64(q); unit < list {
		if r.AmountFen%int64(q) != 0 {
			return "", nil, fmt.Errorf("xpay: 优惠后总价须能被数量整除")
		}
		d.ActivitySellingPrice = unit
	}
	p, err := BuildXPayClientParams(x.api.AppKey, sk, XPayModeGoods, d)
	if err != nil {
		return "", nil, err
	}
	return r.OrderID, map[string]string{"provider": "xpay", "signData": p.SignData, "mode": p.Mode, "paySig": p.PaySig, "signature": p.Signature, "env": fmt.Sprint(x.cfg.Env)}, nil
}

func (x *XPay) openID(ctx context.Context, orderID string) (string, error) {
	if x.cfg.OpenIDOf == nil {
		return "", fmt.Errorf("xpay: 未配置 OpenIDOf，无法调用 query_order")
	}
	return x.cfg.OpenIDOf(ctx, orderID)
}

// TxnOfXPayOrder query_order 状态映射为通用状态
func TxnOfXPayOrder(o *XPayOrder) *Txn {
	t := &Txn{OrderID: o.OrderID, TransactionID: o.WxOrderID, AmountFen: o.PaidFee}
	switch o.Status {
	case XPayStatusPaid, XPayStatusProviding, XPayStatusProvided:
		t.State = "SUCCESS"
	case XPayStatusRefundMch, XPayStatusRefundDone, XPayStatusRefundFailed:
		t.State = "REFUND"
	case XPayStatusClosed:
		t.State = "CLOSED"
	default:
		t.State = "NOTPAY"
	}
	if o.PaidTime > 0 && t.State != "NOTPAY" {
		ts := time.Unix(o.PaidTime, 0)
		t.PaidAt = &ts
	}
	return t
}

// Query 发货兜底：按业务单号 query_order（推送丢失时定时轮询，查到 SUCCESS 即补发货）
func (x *XPay) Query(ctx context.Context, orderID string) (*Txn, error) {
	oid, err := x.openID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	o, err := x.api.QueryOrder(ctx, oid, orderID, "")
	if err != nil {
		return nil, err // 用户未拉起支付时平台侧没有该单，调用方按“未支付”继续等待/超时关单
	}
	return TxnOfXPayOrder(o), nil
}

// Close 虚拟支付没有关单接口：业务单号只能使用一次，未支付的单在本地关闭即可（迟到的发货推送须由调用方按订单状态处理）
func (x *XPay) Close(ctx context.Context, orderID string) error { return nil }

// Refund 先 query_order 取 left_fee 再 refund_order 启动退款任务；iOS（Apple 支付）订单不能主动退款
func (x *XPay) Refund(ctx context.Context, r RefundReq) (*RefundResult, error) {
	if !xpayTradeNo.MatchString(r.RefundID) {
		return nil, fmt.Errorf("xpay: 退款单号需 8-32 位")
	}
	oid, err := x.openID(ctx, r.OrderID)
	if err != nil {
		return nil, err
	}
	o, err := x.api.QueryOrder(ctx, oid, r.OrderID, "")
	if err != nil {
		return nil, err
	}
	if o.OrderType == XPayOrderTypeIOSPay {
		return nil, fmt.Errorf("%w：iOS（Apple 支付）订单只能由用户向 App Store 申请退款", ErrNotSupported)
	}
	if r.RefundFen <= 0 || r.RefundFen > o.LeftFee {
		return nil, fmt.Errorf("xpay: 退款金额须在 (0, %d] 分之间", o.LeftFee)
	}
	reason := r.Reason
	if len(reason) > 1024 {
		reason = reason[:1024]
	}
	resp, err := x.api.RefundOrder(ctx, XPayRefundOrderReq{OpenID: oid, OrderID: r.OrderID, RefundOrderID: r.RefundID, LeftFee: o.LeftFee, RefundFee: r.RefundFen,
		BizMeta: reason, RefundReason: "0", ReqFrom: "1"})
	if err != nil {
		var ae *XPayAPIError
		if !errors.As(err, &ae) || ae.Code != XPayErrRefundRunning {
			return nil, err
		}
		resp = &XPayRefundOrderResp{RefundOrderID: r.RefundID}
	}
	return &RefundResult{OrderID: r.OrderID, RefundID: r.RefundID, ProviderRefundID: resp.RefundWxOrderID, State: "PROCESSING"}, nil
}

// QueryRefund 退款单状态：query_order(order_id=退款单号)
func (x *XPay) QueryRefund(ctx context.Context, orderID, refundID string) (*RefundResult, error) {
	oid, err := x.openID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	o, err := x.api.QueryOrder(ctx, oid, refundID, "")
	if err != nil {
		return nil, err
	}
	st := "PROCESSING"
	switch o.Status {
	case XPayStatusRefundDone:
		st = "SUCCESS"
	case XPayStatusRefundFailed:
		st = "ABNORMAL"
	}
	return &RefundResult{OrderID: orderID, RefundID: refundID, ProviderRefundID: o.WxOrderID, State: st}, nil
}

// ParseNotify 处理消息推送：GET 为服务器 URL 校验（Kind=verify），POST 验签解密后解析
// xpay_goods_deliver_notify / xpay_coin_pay_notify（Kind=pay，EventID 以微信单号去重）与 xpay_refund_notify（Kind=refund）
func (x *XPay) ParseNotify(ctx context.Context, req *http.Request) (*Notify, error) {
	qv := req.URL.Query()
	q := PushQuery{Signature: qv.Get("signature"), Timestamp: qv.Get("timestamp"), Nonce: qv.Get("nonce"), Echostr: qv.Get("echostr"), EncryptType: qv.Get("encrypt_type"), MsgSignature: qv.Get("msg_signature")}
	if req.Method == http.MethodGet {
		echo, err := x.crypto.VerifyURL(q)
		if err != nil {
			return nil, err
		}
		return &Notify{Kind: "verify", Echo: echo}, nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	msg, isX, err := x.crypto.Open(q, body)
	if err != nil {
		return &Notify{AckXML: isXML(body)}, err
	}
	e, err := ParseXPayEvent(msg, isX)
	if err != nil {
		return &Notify{AckXML: isX}, err
	}
	n := &Notify{Raw: msg, OpenID: e.OpenID, AckXML: isX, Kind: "other", EventID: fmt.Sprintf("%s:%s:%d", e.Event, e.FromUserName, e.CreateTime)}
	switch e.Event {
	case XPayEventGoodsDeliver, XPayEventCoinPay:
		if e.Env != x.cfg.Env {
			return n, fmt.Errorf("xpay: 推送环境 env=%d 与配置 env=%d 不符，拒绝发货", e.Env, x.cfg.Env)
		}
		t := &Txn{OrderID: e.OutTradeNo, State: "SUCCESS"}
		var actual int64
		var qty int
		if e.GoodsInfo != nil {
			actual, qty = e.GoodsInfo.ActualPrice, e.GoodsInfo.Quantity
		} else if e.CoinInfo != nil {
			actual, qty = e.CoinInfo.ActualPrice, e.CoinInfo.Quantity
		}
		if qty <= 0 {
			qty = 1
		}
		t.AmountFen = actual * int64(qty)
		if e.WeChatPayInfo != nil {
			t.TransactionID = e.WeChatPayInfo.MchOrderNo // 平台单号 wx_order_id：幂等去重以此为准
			if e.WeChatPayInfo.PaidTime > 0 {
				ts := time.Unix(e.WeChatPayInfo.PaidTime, 0)
				t.PaidAt = &ts
			}
		}
		if t.TransactionID == "" { // 非微信支付渠道（如 Apple 支付）可能没有 WeChatPayInfo
			t.TransactionID = "out:" + e.OutTradeNo
		}
		if t.PaidAt == nil && e.CreateTime > 0 {
			ts := time.Unix(e.CreateTime, 0)
			t.PaidAt = &ts
		}
		n.Kind, n.Txn, n.EventID = "pay", t, "deliver:"+t.TransactionID
	case XPayEventRefund:
		st := "SUCCESS"
		if e.RetCode != 0 {
			st = "ABNORMAL"
		}
		n.Kind = "refund"
		n.Refund = &RefundResult{OrderID: e.MchOrderID, RefundID: e.MchRefundID, ProviderRefundID: e.WxRefundID, State: st}
		n.EventID = "refund:" + e.WxRefundID + ":" + st
	}
	return n, nil
}

// Ack 消息推送回包：URL 校验原样回 echostr；验签失败 401；处理失败 ErrCode=1 让平台重推（最多 15 次）
func (x *XPay) Ack(n *Notify, err error) (int, string, []byte) {
	isX := n != nil && n.AckXML
	if err == nil && n != nil && n.Kind == "verify" {
		return 200, "text/plain; charset=utf-8", []byte(n.Echo)
	}
	if errors.Is(err, ErrBadSignature) {
		ct, b := PushAck(isX, 1, "invalid signature")
		return 401, ct, b
	}
	if err != nil {
		ct, b := PushAck(isX, 1, err.Error())
		return 200, ct, b
	}
	ct, b := PushAck(isX, 0, "success")
	return 200, ct, b
}
