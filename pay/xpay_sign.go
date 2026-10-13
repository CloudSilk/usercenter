package pay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// 小程序虚拟支付签名（https://developers.weixin.qq.com/miniprogram/dev/platform-capabilities/business-capabilities/virtual-payment.html 「签名详解」）

// XPayClientURI 基础库 wx.requestVirtualPayment 的支付签名 uri 固定值
const XPayClientURI = "requestVirtualPayment"

// XPayPaySig 支付签名：to_hex(hmac_sha256(appKey, uri + '&' + signData))。
// 前端调起时 uri = "requestVirtualPayment"、signData 为基础库的 signData 字符串；
// 服务端 /xpay/* 接口时 uri 为不带 query 的接口路径、signData 为实际发送的 POST body。
// appKey 需与 env 对应：env=0 现网 AppKey，env=1 沙箱 AppKey。
func XPayPaySig(appKey, uri, signData string) string {
	h := hmac.New(sha256.New, []byte(appKey))
	h.Write([]byte(uri + "&" + signData))
	return hex.EncodeToString(h.Sum(nil))
}

// XPayUserSignature 用户态签名：to_hex(hmac_sha256(sessionKey, signData))
func XPayUserSignature(sessionKey, signData string) string {
	h := hmac.New(sha256.New, []byte(sessionKey))
	h.Write([]byte(signData))
	return hex.EncodeToString(h.Sum(nil))
}

// XPaySignData wx.requestVirtualPayment 的 signData 字段（字段名以官方文档为准）
type XPaySignData struct {
	OfferID              string `json:"offerId"`
	BuyQuantity          int    `json:"buyQuantity"`
	Env                  int    `json:"env"`
	CurrencyType         string `json:"currencyType"`
	ProductID            string `json:"productId,omitempty"`            // 仅 mode=short_series_goods 必填
	GoodsPrice           int64  `json:"goodsPrice,omitempty"`           // 道具单价（分），仅 short_series_goods 必填
	ActivitySellingPrice int64  `json:"activitySellingPrice,omitempty"` // 优惠价（分），须与 goodsPrice 一起传
	OutTradeNo           string `json:"outTradeNo"`
	Attach               string `json:"attach"`
}

// XPay 调起模式
const (
	XPayModeGoods = "short_series_goods" // 道具直购
	XPayModeCoin  = "short_series_coin"  // 代币充值
)

// XPayClientParams 返回给小程序、可直接展开传入 wx.requestVirtualPayment 的参数
type XPayClientParams struct {
	SignData  string `json:"signData"`
	Mode      string `json:"mode"`
	PaySig    string `json:"paySig"`
	Signature string `json:"signature"`
}

// BuildXPayClientParams 序列化 signData 并计算 paySig 与用户态签名。
// 参与签名的 signData 与下发给前端的字符串完全一致（前端不得重新序列化）。
func BuildXPayClientParams(appKey, sessionKey, mode string, d XPaySignData) (*XPayClientParams, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	sd := string(b)
	return &XPayClientParams{SignData: sd, Mode: mode, PaySig: XPayPaySig(appKey, XPayClientURI, sd), Signature: XPayUserSignature(sessionKey, sd)}, nil
}
