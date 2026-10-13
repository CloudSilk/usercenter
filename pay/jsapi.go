package pay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// WechatConfig 微信支付 APIv3 商户配置（全部来自系统配置 pay.wechat.*，密钥只写不读）
type WechatConfig struct {
	AppID           string // 小程序 appid
	MchID           string
	SerialNo        string // 商户 API 证书序列号
	PrivateKeyPEM   string // 商户 API 私钥
	APIv3Key        string
	PubKeyID        string // 微信支付公钥 ID（PUB_KEY_ID_…）
	PubKeyPEM       string // 微信支付公钥
	NotifyURL       string // 支付回调 https://…/pay/wechat/notify
	RefundNotifyURL string // 退款回调 https://…/pay/wechat/refund-notify
	HTTPClient      *http.Client
}

// Wechat 微信支付 APIv3 JSAPI（小程序）渠道
type Wechat struct {
	cfg    WechatConfig
	client *core.Client
	notify *notify.Handler
}

func NewWechat(ctx context.Context, cfg WechatConfig) (*Wechat, error) {
	if cfg.MchID == "" || cfg.SerialNo == "" || cfg.PrivateKeyPEM == "" || cfg.APIv3Key == "" || cfg.PubKeyID == "" || cfg.PubKeyPEM == "" || cfg.AppID == "" {
		return nil, fmt.Errorf("wechatpay: 商户配置不完整（appid、mchid、证书序列号、私钥、APIv3 密钥、微信支付公钥 ID 与公钥）")
	}
	pk, err := utils.LoadPrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("wechatpay: 商户私钥无效：%w", err)
	}
	pub, err := utils.LoadPublicKey(cfg.PubKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("wechatpay: 微信支付公钥无效：%w", err)
	}
	opts := []core.ClientOption{option.WithWechatPayPublicKeyAuthCipher(cfg.MchID, cfg.SerialNo, pk, cfg.PubKeyID, pub)}
	if cfg.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(cfg.HTTPClient))
	}
	cl, err := core.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	h, err := notify.NewRSANotifyHandler(cfg.APIv3Key, verifiers.NewSHA256WithRSAPubkeyVerifier(cfg.PubKeyID, *pub))
	if err != nil {
		return nil, err
	}
	return &Wechat{cfg: cfg, client: cl, notify: h}, nil
}

// Name 渠道名固定为 "wechat"（与历史订单的 channel 值兼容）
func (w *Wechat) Name() string { return "wechat" }

func (w *Wechat) svc() jsapi.JsapiApiService { return jsapi.JsapiApiService{Client: w.client} }

// Prepay JSAPI 统一下单 + 生成小程序调起支付参数（paySign 由商户私钥签名）
func (w *Wechat) Prepay(ctx context.Context, r PrepayReq) (string, map[string]string, error) {
	svc := w.svc()
	req := jsapi.PrepayRequest{
		Appid: core.String(w.cfg.AppID), Mchid: core.String(w.cfg.MchID),
		Description: core.String(r.Description), OutTradeNo: core.String(r.OrderID),
		NotifyUrl: core.String(w.cfg.NotifyURL),
		Amount:    &jsapi.Amount{Total: core.Int64(r.AmountFen), Currency: core.String("CNY")},
		Payer:     &jsapi.Payer{Openid: core.String(r.OpenID)},
	}
	if !r.ExpireAt.IsZero() {
		req.TimeExpire = core.Time(r.ExpireAt)
	}
	resp, _, err := svc.PrepayWithRequestPayment(ctx, req)
	if err != nil {
		return "", nil, err
	}
	return *resp.PrepayId, map[string]string{"provider": "wechat", "appId": *resp.Appid, "timeStamp": *resp.TimeStamp, "nonceStr": *resp.NonceStr,
		"package": *resp.Package, "signType": *resp.SignType, "paySign": *resp.PaySign}, nil
}

func txnOf(t *payments.Transaction) *Txn {
	out := &Txn{OrderID: str(t.OutTradeNo), TransactionID: str(t.TransactionId), State: str(t.TradeState)}
	if t.Amount != nil && t.Amount.Total != nil {
		out.AmountFen = *t.Amount.Total
	}
	if t.SuccessTime != nil {
		if ts, err := time.Parse(time.RFC3339, *t.SuccessTime); err == nil {
			out.PaidAt = &ts
		}
	}
	return out
}

// Query 按商户订单号查单
func (w *Wechat) Query(ctx context.Context, orderID string) (*Txn, error) {
	svc := w.svc()
	t, _, err := svc.QueryOrderByOutTradeNo(ctx, jsapi.QueryOrderByOutTradeNoRequest{OutTradeNo: core.String(orderID), Mchid: core.String(w.cfg.MchID)})
	if err != nil {
		return nil, err
	}
	return txnOf(t), nil
}

// Close 关单
func (w *Wechat) Close(ctx context.Context, orderID string) error {
	svc := w.svc()
	_, err := svc.CloseOrder(ctx, jsapi.CloseOrderRequest{OutTradeNo: core.String(orderID), Mchid: core.String(w.cfg.MchID)})
	return err
}

// Refund 申请退款（结果以退款回调或查询为准）
func (w *Wechat) Refund(ctx context.Context, r RefundReq) (*RefundResult, error) {
	svc := refunddomestic.RefundsApiService{Client: w.client}
	resp, _, err := svc.Create(ctx, refunddomestic.CreateRequest{
		OutTradeNo: core.String(r.OrderID), OutRefundNo: core.String(r.RefundID), Reason: core.String(r.Reason),
		NotifyUrl: core.String(w.cfg.RefundNotifyURL),
		Amount:    &refunddomestic.AmountReq{Refund: core.Int64(r.RefundFen), Total: core.Int64(r.TotalFen), Currency: core.String("CNY")},
	})
	if err != nil {
		return nil, err
	}
	st := ""
	if resp.Status != nil {
		st = string(*resp.Status)
	}
	return &RefundResult{OrderID: r.OrderID, RefundID: r.RefundID, ProviderRefundID: str(resp.RefundId), State: st}, nil
}

type refundNotice struct {
	OutTradeNo   string `json:"out_trade_no"`
	OutRefundNo  string `json:"out_refund_no"`
	RefundID     string `json:"refund_id"`
	RefundStatus string `json:"refund_status"`
}

// ParseNotify 验签（微信支付公钥）+ AES-256-GCM 解密（APIv3 密钥）；支付与退款回调共用
func (w *Wechat) ParseNotify(ctx context.Context, req *http.Request) (*Notify, error) {
	var raw map[string]any
	nr, err := w.notify.ParseNotifyRequest(ctx, req, &raw)
	if err != nil {
		return nil, fmt.Errorf("%w：%v", ErrBadSignature, err)
	}
	n := &Notify{EventID: nr.ID}
	switch {
	case nr.EventType == "TRANSACTION.SUCCESS":
		var t payments.Transaction
		if err := json.Unmarshal([]byte(nr.Resource.Plaintext), &t); err != nil {
			return nil, err
		}
		n.Kind, n.Txn = "pay", txnOf(&t)
	case len(nr.EventType) > 7 && nr.EventType[:7] == "REFUND.":
		var r refundNotice
		if err := json.Unmarshal([]byte(nr.Resource.Plaintext), &r); err != nil {
			return nil, err
		}
		n.Kind = "refund"
		n.Refund = &RefundResult{OrderID: r.OutTradeNo, RefundID: r.OutRefundNo, ProviderRefundID: r.RefundID, State: r.RefundStatus}
	default:
		n.Kind = "other"
	}
	return n, nil
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
