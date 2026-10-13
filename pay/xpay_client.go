package pay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// 微信服务端 API：access_token（stable_token）、code2Session 与虚拟支付 /xpay/* 接口

// DefaultWxAPIBase 微信开放接口域名
const DefaultWxAPIBase = "https://api.weixin.qq.com"

// XPayAPIError /xpay/* 或 cgi-bin 接口的业务错误
type XPayAPIError struct {
	Code int    `json:"errcode"`
	Msg  string `json:"errmsg"`
	URI  string `json:"-"`
}

func (e *XPayAPIError) Error() string {
	return fmt.Sprintf("wx %s: errcode=%d %s", e.URI, e.Code, e.Msg)
}

// 虚拟支付常见错误码（query_order / refund_order 文档）
const (
	XPayErrSignature       = 268490003
	XPayErrDuplicate       = 268490004
	XPayErrSessionExpired  = 268490009
	XPayErrRefundRunning   = 268490014
	XPayErrRateLimit       = 268490015
	XPayErrLeftFeeMismatch = 268490016
)

type wxHTTP struct {
	base string
	hc   *http.Client
}

func (w wxHTTP) client() *http.Client {
	if w.hc != nil {
		return w.hc
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (w wxHTTP) url(path string, q url.Values) string {
	b := w.base
	if b == "" {
		b = DefaultWxAPIBase
	}
	if len(q) == 0 {
		return b + path
	}
	return b + path + "?" + q.Encode()
}

func (w wxHTTP) do(ctx context.Context, method, u string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := w.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("wx: HTTP %d", resp.StatusCode)
	}
	var e XPayAPIError
	if json.Unmarshal(rb, &e) == nil && e.Code != 0 {
		e.URI = req.URL.Path
		return &e
	}
	if out != nil {
		return json.Unmarshal(rb, out)
	}
	return nil
}

// StableTokenSource 通过 /cgi-bin/stable_token 获取并缓存 access_token（提前 5 分钟刷新）
type StableTokenSource struct {
	AppID, AppSecret string
	BaseURL          string
	HTTPClient       *http.Client

	mu     sync.Mutex
	token  string
	expire time.Time
}

// Token 实现 XPayConfig.AccessToken
func (s *StableTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Now().Before(s.expire) {
		return s.token, nil
	}
	if s.AppID == "" || s.AppSecret == "" {
		return "", fmt.Errorf("wx: 获取 access_token 需要小程序 AppID 与 AppSecret")
	}
	body, _ := json.Marshal(map[string]any{"grant_type": "client_credential", "appid": s.AppID, "secret": s.AppSecret, "force_refresh": false})
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	h := wxHTTP{s.BaseURL, s.HTTPClient}
	if err := h.do(ctx, "POST", h.url("/cgi-bin/stable_token", nil), body, &out); err != nil {
		return "", err
	}
	s.token, s.expire = out.AccessToken, time.Now().Add(time.Duration(out.ExpiresIn-300)*time.Second)
	return s.token, nil
}

// Session code2Session 结果
type Session struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
}

// Code2Session 用 wx.login 的 code 换取 openid 与 session_key（/sns/jscode2session）
func Code2Session(ctx context.Context, hc *http.Client, baseURL, appID, secret, code string) (*Session, error) {
	h := wxHTTP{baseURL, hc}
	var s Session
	q := url.Values{"appid": {appID}, "secret": {secret}, "js_code": {code}, "grant_type": {"authorization_code"}}
	if err := h.do(ctx, "GET", h.url("/sns/jscode2session", q), nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// XPayOrder query_order 返回的 order 对象
type XPayOrder struct {
	OrderID        string `json:"order_id"`
	CreateTime     int64  `json:"create_time"`
	UpdateTime     int64  `json:"update_time"`
	Status         int    `json:"status"`
	BizType        int    `json:"biz_type"`
	OrderFee       int64  `json:"order_fee"`
	CouponFee      int64  `json:"coupon_fee"`
	PaidFee        int64  `json:"paid_fee"`
	OrderType      int    `json:"order_type"`
	RefundFee      int64  `json:"refund_fee"`
	PaidTime       int64  `json:"paid_time"`
	ProvideTime    int64  `json:"provide_time"`
	BizMeta        string `json:"biz_meta"`
	EnvType        int    `json:"env_type"`
	Token          string `json:"token"`
	LeftFee        int64  `json:"left_fee"`
	WxOrderID      string `json:"wx_order_id"`
	ChannelOrderID string `json:"channel_order_id"`
	WxpayOrderID   string `json:"wxpay_order_id"`
	SettTime       int64  `json:"sett_time"`
	SettState      int    `json:"sett_state"`
	PlatformFeeFen int64  `json:"platform_fee_fen"`
	CpsFeeFen      int64  `json:"cps_fee_fen"`
}

// query_order 订单状态 status
const (
	XPayStatusInit         = 0 // 订单初始化（未创建成功）
	XPayStatusCreated      = 1 // 订单创建成功
	XPayStatusPaid         = 2 // 已支付，待发货
	XPayStatusProviding    = 3 // 发货中
	XPayStatusProvided     = 4 // 已发货
	XPayStatusRefundMch    = 5 // 退款：商户侧扣款完成
	XPayStatusClosed       = 6 // 已关闭
	XPayStatusRefundFailed = 7 // 退款失败
	XPayStatusRefundDone   = 8 // 退款：用户侧收款完成
)

// query_order 订单类型 order_type
const (
	XPayOrderTypePay       = 0 // 普通虚拟支付
	XPayOrderTypeRefund    = 1 // 普通退款
	XPayOrderTypeIOSPay    = 7 // 苹果 iOS 支付
	XPayOrderTypeIOSRefund = 8 // 苹果 iOS 退款
)

// XPayRefundOrderReq refund_order 请求体
type XPayRefundOrderReq struct {
	OpenID        string `json:"openid"`
	OrderID       string `json:"order_id,omitempty"`
	WxOrderID     string `json:"wx_order_id,omitempty"`
	RefundOrderID string `json:"refund_order_id"`
	LeftFee       int64  `json:"left_fee"`
	RefundFee     int64  `json:"refund_fee"`
	BizMeta       string `json:"biz_meta"`
	RefundReason  string `json:"refund_reason"` // 0-暂无描述 1-产品问题 2-售后问题 3-意愿问题 4-价格问题 5-其他
	ReqFrom       string `json:"req_from"`      // 1-人工客服退款 2-用户自己发起 3-其它
	Env           int    `json:"env"`
}

// XPayRefundOrderResp refund_order 返回
type XPayRefundOrderResp struct {
	RefundOrderID   string `json:"refund_order_id"`
	RefundWxOrderID string `json:"refund_wx_order_id"`
	PayOrderID      string `json:"pay_order_id"`
	PayWxOrderID    string `json:"pay_wx_order_id"`
}

// XPayClient 虚拟支付服务端接口（POST https://api.weixin.qq.com/xpay/...?access_token=&pay_sig=）
type XPayClient struct {
	AppKey      string // 与 Env 对应的 AppKey
	Env         int
	AccessToken func(ctx context.Context) (string, error)
	BaseURL     string
	HTTPClient  *http.Client
}

// Call 发起带 pay_sig 的 /xpay/* 调用；参与签名的 body 与实际发送的完全一致
func (c *XPayClient) Call(ctx context.Context, uri string, req, out any) error {
	return c.call(ctx, uri, true, req, out)
}

func (c *XPayClient) call(ctx context.Context, uri string, sign bool, req, out any) error {
	if c.AccessToken == nil {
		return fmt.Errorf("xpay: 未配置 access_token 来源")
	}
	tok, err := c.AccessToken(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	h := wxHTTP{c.BaseURL, c.HTTPClient}
	q := url.Values{"access_token": {tok}}
	if sign {
		q.Set("pay_sig", XPayPaySig(c.AppKey, uri, string(body)))
	}
	return h.do(ctx, "POST", h.url(uri, q), body, out)
}

// QueryOrder /xpay/query_order：orderID 与 wxOrderID 二选一
func (c *XPayClient) QueryOrder(ctx context.Context, openID, orderID, wxOrderID string) (*XPayOrder, error) {
	req := map[string]any{"openid": openID, "env": c.Env}
	if orderID != "" {
		req["order_id"] = orderID
	}
	if wxOrderID != "" {
		req["wx_order_id"] = wxOrderID
	}
	var out struct {
		Order *XPayOrder `json:"order"`
	}
	if err := c.Call(ctx, "/xpay/query_order", req, &out); err != nil {
		return nil, err
	}
	if out.Order == nil {
		return nil, fmt.Errorf("xpay: query_order 未返回订单")
	}
	return out.Order, nil
}

// RefundOrder /xpay/refund_order：仅启动退款任务，最终结果以 query_order 或 xpay_refund_notify 为准
func (c *XPayClient) RefundOrder(ctx context.Context, r XPayRefundOrderReq) (*XPayRefundOrderResp, error) {
	r.Env = c.Env
	var out XPayRefundOrderResp
	if err := c.Call(ctx, "/xpay/refund_order", r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// NotifyProvideGoods /xpay/notify_provide_goods：推送未成功应答时补报“已发货”（现金单；文档仅要求 access_token，不带 pay_sig）
func (c *XPayClient) NotifyProvideGoods(ctx context.Context, orderID, wxOrderID string) error {
	req := map[string]any{"env": c.Env}
	if orderID != "" {
		req["order_id"] = orderID
	}
	if wxOrderID != "" {
		req["wx_order_id"] = wxOrderID
	}
	return c.call(ctx, "/xpay/notify_provide_goods", false, req, nil)
}
