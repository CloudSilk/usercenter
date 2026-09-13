package wechatpay

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/consts"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// PrepayInput JSAPI 下单参数。
type PrepayInput struct {
	AppID       string
	MchID       string
	Description string
	OutTradeNo  string
	NotifyURL   string
	Attach      string
	OpenID      string
	AmountFen   int64
	Currency    string
	// TimeExpire 订单失效时间,零值表示不传,由微信侧使用默认有效期。
	TimeExpire time.Time
}

// PayParams 调起小程序 wx.requestPayment 所需的签名参数。
type PayParams struct {
	AppID     string `json:"appId"`
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
	PrepayID  string `json:"prepayID"`
}

// TransactionResult 查单返回的交易状态。
type TransactionResult struct {
	OutTradeNo     string
	TransactionID  string
	TradeState     string
	TradeStateDesc string
	SuccessTime    time.Time
}

// NotifyContent 回调通知验签解密后的内容(交易或退款事件)。
type NotifyContent struct {
	EventType      string
	OutTradeNo     string
	TransactionID  string
	TradeState     string
	TradeStateDesc string
	SuccessTime    time.Time
	Attach         string
	AmountFen      int64
	// 退款事件专有字段
	OutRefundNo       string
	RefundID          string
	RefundStatus      string
	RefundSuccessTime time.Time
	RefundAmountFen   int64
}

// RefundInput 申请退款参数。
type RefundInput struct {
	MchID           string
	OutTradeNo      string
	OutRefundNo     string
	Reason          string
	NotifyURL       string
	RefundAmountFen int64
	TotalFen        int64
	Currency        string
}

// RefundResult 退款申请/查单结果。
type RefundResult struct {
	RefundID    string
	OutRefundNo string
	Status      string
	SuccessTime time.Time
}

// PayAPI 微信支付上游接口抽象,生产实现基于官方 APIv3 SDK,测试可注入 fake。
type PayAPI interface {
	Prepay(ctx context.Context, in PrepayInput) (*PayParams, error)
	Query(ctx context.Context, mchID, outTradeNo string) (*TransactionResult, error)
	Close(ctx context.Context, mchID, outTradeNo string) error
	Refund(ctx context.Context, in RefundInput) (*RefundResult, error)
	QueryRefund(ctx context.Context, outRefundNo string) (*RefundResult, error)
	ParseNotify(req *http.Request) (*NotifyContent, error)
	// DownloadTradeBill 下载指定日期(YYYY-MM-DD)、指定类型(billType: ALL/SUCCESS/REFUND)
	// 的交易账单,返回解压后的 CSV 内容。
	DownloadTradeBill(ctx context.Context, billDate, billType string) ([]byte, error)
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// sdkPayAPI 官方 SDK 实现,按商户配置构建。
type sdkPayAPI struct {
	mchID         string
	client        *core.Client
	notifyHandler *notify.Handler
}

// newSDKPayAPI 构建商户 APIv3 客户端:商户私钥签名 + 平台证书自动下载轮换,
// 同时注册回调验签器与 AES-GCM 解密套件。
func newSDKPayAPI(cfg *PayConfig) (PayAPI, error) {
	privateKey, err := utils.LoadPrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("解析商户私钥失败: %w", err)
	}
	client, err := core.NewClient(context.Background(),
		option.WithWechatPayAutoAuthCipher(cfg.MchID, cfg.MchSerialNo, privateKey, cfg.APIV3Key))
	if err != nil {
		return nil, fmt.Errorf("初始化微信支付客户端失败: %w", err)
	}
	certificateVisitor := downloader.MgrInstance().GetCertificateVisitor(cfg.MchID)
	notifyHandler, err := notify.NewRSANotifyHandler(cfg.APIV3Key,
		verifiers.NewSHA256WithRSAVerifier(certificateVisitor))
	if err != nil {
		return nil, fmt.Errorf("初始化回调处理器失败: %w", err)
	}
	return &sdkPayAPI{mchID: cfg.MchID, client: client, notifyHandler: notifyHandler}, nil
}

func (p *sdkPayAPI) Prepay(ctx context.Context, in PrepayInput) (*PayParams, error) {
	if in.Currency == "" {
		in.Currency = "CNY"
	}
	total := in.AmountFen
	currency := in.Currency
	var timeExpire *time.Time
	if !in.TimeExpire.IsZero() {
		t := in.TimeExpire
		timeExpire = &t
	}
	svc := jsapi.JsapiApiService{Client: p.client}
	resp, _, err := svc.PrepayWithRequestPayment(ctx, jsapi.PrepayRequest{
		Appid:       core.String(in.AppID),
		Mchid:       core.String(in.MchID),
		Description: core.String(in.Description),
		OutTradeNo:  core.String(in.OutTradeNo),
		NotifyUrl:   core.String(in.NotifyURL),
		Attach:      core.String(in.Attach),
		TimeExpire:  timeExpire,
		Amount:      &jsapi.Amount{Total: &total, Currency: &currency},
		Payer:       &jsapi.Payer{Openid: core.String(in.OpenID)},
	})
	if err != nil {
		return nil, err
	}
	return &PayParams{
		AppID:     derefString(resp.Appid),
		TimeStamp: derefString(resp.TimeStamp),
		NonceStr:  derefString(resp.NonceStr),
		Package:   derefString(resp.Package),
		SignType:  derefString(resp.SignType),
		PaySign:   derefString(resp.PaySign),
		PrepayID:  derefString(resp.PrepayId),
	}, nil
}

func (p *sdkPayAPI) Query(ctx context.Context, mchID, outTradeNo string) (*TransactionResult, error) {
	svc := jsapi.JsapiApiService{Client: p.client}
	tx, _, err := svc.QueryOrderByOutTradeNo(ctx, jsapi.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(outTradeNo),
		Mchid:      core.String(mchID),
	})
	if err != nil {
		return nil, err
	}
	result := &TransactionResult{
		OutTradeNo:     derefString(tx.OutTradeNo),
		TransactionID:  derefString(tx.TransactionId),
		TradeState:     derefString(tx.TradeState),
		TradeStateDesc: derefString(tx.TradeStateDesc),
	}
	if t := derefString(tx.SuccessTime); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			result.SuccessTime = parsed
		}
	}
	return result, nil
}

func (p *sdkPayAPI) Close(ctx context.Context, mchID, outTradeNo string) error {
	svc := jsapi.JsapiApiService{Client: p.client}
	_, err := svc.CloseOrder(ctx, jsapi.CloseOrderRequest{
		OutTradeNo: core.String(outTradeNo),
		Mchid:      core.String(mchID),
	})
	return err
}

// notifyTransactionContent 交易/退款通知解密后的资源体。
// 支付事件与退款事件的资源结构不同,按 EventType 取用对应字段。
type notifyTransactionContent struct {
	OutTradeNo     *string `json:"out_trade_no"`
	TransactionID  *string `json:"transaction_id"`
	TradeState     *string `json:"trade_state"`
	TradeStateDesc *string `json:"trade_state_desc"`
	SuccessTime    *string `json:"success_time"`
	Attach         *string `json:"attach"`
	Amount         struct {
		Total  *int64 `json:"total"`
		Refund *int64 `json:"refund"`
	} `json:"amount"`
	// 退款事件字段
	OutRefundNo  *string `json:"out_refund_no"`
	RefundID     *string `json:"refund_id"`
	RefundStatus *string `json:"refund_status"`
}

// tradeBillResp 申请账单接口的响应体。
type tradeBillResp struct {
	DownloadURL string `json:"download_url"`
	HashType    string `json:"hash_type"`
	HashValue   string `json:"hash_value"`
}

// billClient 下载账单压缩包使用的 HTTP 客户端(download_url 无需签名,可直接 GET)。
var billHTTPClient = &http.Client{Timeout: 60 * time.Second}

// DownloadTradeBill 交易账单两步下载:先申请账单拿到 download_url(请求自动签名),
// 再下载 gzip 压缩包并解压,返回账单 CSV 内容(含汇总行)。
func (p *sdkPayAPI) DownloadTradeBill(ctx context.Context, billDate, billType string) ([]byte, error) {
	requestURL := consts.WechatPayAPIServer + "/v3/bill/trade-bill?bill_date=" +
		url.QueryEscape(billDate) + "&bill_type=" + url.QueryEscape(billType)
	result, err := p.client.Get(ctx, requestURL)
	if err != nil {
		return nil, fmt.Errorf("申请交易账单失败: %w", err)
	}
	var bill tradeBillResp
	if err := json.NewDecoder(result.Response.Body).Decode(&bill); err != nil {
		return nil, fmt.Errorf("解析账单申请响应失败: %w", err)
	}
	if bill.DownloadURL == "" {
		return nil, fmt.Errorf("账单申请响应缺少 download_url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bill.DownloadURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := billHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载账单失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载账单失败: HTTP %d", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("账单压缩包解压失败: %w", err)
	}
	defer gz.Close()
	csv, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("读取账单内容失败: %w", err)
	}
	return csv, nil
}

func (p *sdkPayAPI) ParseNotify(req *http.Request) (*NotifyContent, error) {
	var content notifyTransactionContent
	event, err := p.notifyHandler.ParseNotifyRequest(context.Background(), req, &content)
	if err != nil {
		return nil, fmt.Errorf("解析支付回调失败: %w", err)
	}
	result := &NotifyContent{
		EventType:      event.EventType,
		OutTradeNo:     derefString(content.OutTradeNo),
		TransactionID:  derefString(content.TransactionID),
		TradeState:     derefString(content.TradeState),
		TradeStateDesc: derefString(content.TradeStateDesc),
		Attach:         derefString(content.Attach),
	}
	if content.Amount.Total != nil {
		result.AmountFen = *content.Amount.Total
	}
	if content.Amount.Refund != nil {
		result.RefundAmountFen = *content.Amount.Refund
	}
	if content.OutRefundNo != nil {
		result.OutRefundNo = derefString(content.OutRefundNo)
		result.RefundID = derefString(content.RefundID)
		result.RefundStatus = derefString(content.RefundStatus)
	}
	if t := derefString(content.SuccessTime); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			result.SuccessTime = parsed
			result.RefundSuccessTime = parsed
		}
	}
	return result, nil
}

func (p *sdkPayAPI) Refund(ctx context.Context, in RefundInput) (*RefundResult, error) {
	currency := in.Currency
	if currency == "" {
		currency = "CNY"
	}
	refund := in.RefundAmountFen
	total := in.TotalFen
	svc := refunddomestic.RefundsApiService{Client: p.client}
	resp, _, err := svc.Create(ctx, refunddomestic.CreateRequest{
		OutTradeNo:  core.String(in.OutTradeNo),
		OutRefundNo: core.String(in.OutRefundNo),
		Reason:      core.String(in.Reason),
		NotifyUrl:   core.String(in.NotifyURL),
		Amount: &refunddomestic.AmountReq{
			Refund:   &refund,
			Total:    &total,
			Currency: &currency,
		},
	})
	if err != nil {
		return nil, err
	}
	result := &RefundResult{
		RefundID:    derefString(resp.RefundId),
		OutRefundNo: derefString(resp.OutRefundNo),
		Status:      derefString((*string)(resp.Status)),
	}
	if resp.SuccessTime != nil {
		result.SuccessTime = resp.SuccessTime.Local()
	}
	return result, nil
}

func (p *sdkPayAPI) QueryRefund(ctx context.Context, outRefundNo string) (*RefundResult, error) {
	svc := refunddomestic.RefundsApiService{Client: p.client}
	resp, _, err := svc.QueryByOutRefundNo(ctx, refunddomestic.QueryByOutRefundNoRequest{
		OutRefundNo: core.String(outRefundNo),
	})
	if err != nil {
		return nil, err
	}
	result := &RefundResult{
		RefundID:    derefString(resp.RefundId),
		OutRefundNo: derefString(resp.OutRefundNo),
		Status:      derefString((*string)(resp.Status)),
	}
	if resp.SuccessTime != nil {
		result.SuccessTime = resp.SuccessTime.Local()
	}
	return result, nil
}

// --- 按配置缓存的客户端管理:指纹不变则复用,配置更新/删除后失效重建 ---

type payAPIEntry struct {
	fingerprint string
	api         PayAPI
}

var (
	payAPIMu      sync.Mutex
	payAPICache   = map[string]payAPIEntry{}
	payAPIFactory = func(cfg *PayConfig) (PayAPI, error) { return newSDKPayAPI(cfg) }
)

// SetPayAPIFactory 供测试替换客户端构建器。
func SetPayAPIFactory(f func(*PayConfig) (PayAPI, error)) {
	payAPIMu.Lock()
	defer payAPIMu.Unlock()
	payAPIFactory = f
	payAPICache = map[string]payAPIEntry{}
}

// InvalidatePayAPI 配置更新/删除后丢弃缓存的客户端。
func InvalidatePayAPI(configID string) {
	payAPIMu.Lock()
	defer payAPIMu.Unlock()
	delete(payAPICache, configID)
}

func payConfigFingerprint(cfg *PayConfig) string {
	h := sha256.Sum256([]byte(cfg.MchID + "\x00" + cfg.MchSerialNo + "\x00" + cfg.APIV3Key + "\x00" + cfg.PrivateKey))
	return hex.EncodeToString(h[:])
}

// GetPayAPI 取商户配置对应的客户端实例,懒加载并按指纹缓存。
func GetPayAPI(cfg *PayConfig) (PayAPI, error) {
	payAPIMu.Lock()
	defer payAPIMu.Unlock()
	fp := payConfigFingerprint(cfg)
	if entry, ok := payAPICache[cfg.ID]; ok && entry.fingerprint == fp {
		return entry.api, nil
	}
	api, err := payAPIFactory(cfg)
	if err != nil {
		return nil, err
	}
	payAPICache[cfg.ID] = payAPIEntry{fingerprint: fp, api: api}
	return api, nil
}
