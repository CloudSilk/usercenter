package pay

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// 小程序消息推送（https://developers.weixin.qq.com/miniprogram/dev/framework/server-ability/message-push.html）：
// URL 校验 signature = sha1(sort(token, timestamp, nonce))；
// 安全模式 msg_signature = sha1(sort(token, timestamp, nonce, Encrypt))，
// 密文 AES-256-CBC（key = Base64(EncodingAESKey+"="), IV = key[:16], PKCS#7 填充到 32 的倍数），
// 明文 FullStr = random(16B) + msg_len(4B 网络字节序) + msg + appid。

// PushSignature 消息推送签名：参数字典序排序后拼接做 sha1
func PushSignature(parts ...string) string {
	p := append([]string(nil), parts...)
	sort.Strings(p)
	h := sha1.Sum([]byte(strings.Join(p, "")))
	return hex.EncodeToString(h[:])
}

func eqSig(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

// PushCrypto 消息推送加解密
type PushCrypto struct {
	Token, AppID string
	key          []byte
}

// NewPushCrypto encodingAESKey 可为空（明文模式）
func NewPushCrypto(token, encodingAESKey, appID string) (*PushCrypto, error) {
	c := &PushCrypto{Token: token, AppID: appID}
	if encodingAESKey != "" {
		k, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
		if err != nil || len(k) != 32 {
			return nil, errors.New("xpay: EncodingAESKey 无效（应为 43 位）")
		}
		c.key = k
	}
	return c, nil
}

// Decrypt 解密 Encrypt 字段并校验 appid
func (c *PushCrypto) Decrypt(encrypt string) ([]byte, error) {
	if c.key == nil {
		return nil, errors.New("xpay: 未配置 EncodingAESKey，无法解密")
	}
	ct, err := base64.StdEncoding.DecodeString(encrypt)
	if err != nil || len(ct) == 0 || len(ct)%aes.BlockSize != 0 {
		return nil, errors.New("xpay: 密文格式错误")
	}
	blk, _ := aes.NewCipher(c.key)
	pt := make([]byte, len(ct))
	cipher.NewCBCDecrypter(blk, c.key[:16]).CryptBlocks(pt, ct)
	pad := int(pt[len(pt)-1])
	if pad < 1 || pad > 32 || pad > len(pt) {
		return nil, errors.New("xpay: 填充错误")
	}
	pt = pt[:len(pt)-pad]
	if len(pt) < 20 {
		return nil, errors.New("xpay: 明文过短")
	}
	n := int(binary.BigEndian.Uint32(pt[16:20]))
	if 20+n > len(pt) {
		return nil, errors.New("xpay: 消息长度错误")
	}
	msg, appid := pt[20:20+n], string(pt[20+n:])
	if c.AppID != "" && appid != c.AppID {
		return nil, fmt.Errorf("xpay: 推送 appid %q 与配置不符", appid)
	}
	return msg, nil
}

// Encrypt 加密回包；random 为空时随机生成（测试可固定）
func (c *PushCrypto) Encrypt(msg []byte, random []byte) (string, error) {
	if c.key == nil {
		return "", errors.New("xpay: 未配置 EncodingAESKey")
	}
	if len(random) != 16 {
		random = make([]byte, 16)
		io.ReadFull(rand.Reader, random)
	}
	var b bytes.Buffer
	b.Write(random)
	binary.Write(&b, binary.BigEndian, uint32(len(msg)))
	b.Write(msg)
	b.WriteString(c.AppID)
	pad := 32 - b.Len()%32
	b.Write(bytes.Repeat([]byte{byte(pad)}, pad))
	pt := b.Bytes()
	blk, _ := aes.NewCipher(c.key)
	ct := make([]byte, len(pt))
	cipher.NewCBCEncrypter(blk, c.key[:16]).CryptBlocks(ct, pt)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// PushQuery 推送请求 URL 上的参数
type PushQuery struct {
	Signature, Timestamp, Nonce, Echostr, EncryptType, MsgSignature string
}

// VerifyURL 服务器地址校验（GET）：验签通过返回 echostr
func (c *PushCrypto) VerifyURL(q PushQuery) (string, error) {
	if !eqSig(PushSignature(c.Token, q.Timestamp, q.Nonce), q.Signature) {
		return "", ErrBadSignature
	}
	return q.Echostr, nil
}

type pushEnvelope struct {
	XMLName    xml.Name `xml:"xml" json:"-"`
	ToUserName string   `xml:"ToUserName" json:"ToUserName"`
	Encrypt    string   `xml:"Encrypt" json:"Encrypt"`
}

// isXML 推送数据格式可选 XML 或 JSON
func isXML(b []byte) bool { return bytes.HasPrefix(bytes.TrimSpace(b), []byte("<")) }

// Open 验签并（安全/兼容模式下）解密推送包体，返回明文消息与其格式
func (c *PushCrypto) Open(q PushQuery, body []byte) (msg []byte, xmlFormat bool, err error) {
	xmlFormat = isXML(body)
	var env pushEnvelope
	if xmlFormat {
		err = xml.Unmarshal(body, &env)
	} else {
		err = json.Unmarshal(body, &env)
	}
	if err != nil {
		return nil, xmlFormat, fmt.Errorf("xpay: 推送包体解析失败：%w", err)
	}
	if env.Encrypt != "" {
		if !eqSig(PushSignature(c.Token, q.Timestamp, q.Nonce, env.Encrypt), q.MsgSignature) {
			return nil, xmlFormat, ErrBadSignature
		}
		msg, err = c.Decrypt(env.Encrypt)
		if err != nil {
			return nil, xmlFormat, fmt.Errorf("%w：%v", ErrBadSignature, err)
		}
		return msg, isXML(msg), nil
	}
	// 明文模式：不要使用未签名的包体，必须校验 signature
	if q.EncryptType == "aes" || !eqSig(PushSignature(c.Token, q.Timestamp, q.Nonce), q.Signature) {
		return nil, xmlFormat, ErrBadSignature
	}
	if c.key != nil && q.EncryptType != "" {
		return nil, xmlFormat, ErrBadSignature
	}
	return body, xmlFormat, nil
}

// XPayPushEvent 虚拟支付相关推送（字段名以官方文档「2.4 消息推送」为准）
type XPayPushEvent struct {
	ToUserName   string `xml:"ToUserName" json:"ToUserName"`
	FromUserName string `xml:"FromUserName" json:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime" json:"CreateTime"`
	MsgType      string `xml:"MsgType" json:"MsgType"`
	Event        string `xml:"Event" json:"Event"`
	OpenID       string `xml:"OpenId" json:"OpenId"`
	OutTradeNo   string `xml:"OutTradeNo" json:"OutTradeNo"`
	Env          int    `xml:"Env" json:"Env"`

	WeChatPayInfo *struct {
		MchOrderNo    string `xml:"MchOrderNo" json:"MchOrderNo"`
		TransactionID string `xml:"TransactionId" json:"TransactionId"`
		PaidTime      int64  `xml:"PaidTime" json:"PaidTime"`
	} `xml:"WeChatPayInfo" json:"WeChatPayInfo"`
	GoodsInfo *struct {
		ProductID   string `xml:"ProductId" json:"ProductId"`
		Quantity    int    `xml:"Quantity" json:"Quantity"`
		OrigPrice   int64  `xml:"OrigPrice" json:"OrigPrice"`
		ActualPrice int64  `xml:"ActualPrice" json:"ActualPrice"`
		Attach      string `xml:"Attach" json:"Attach"`
	} `xml:"GoodsInfo" json:"GoodsInfo"`
	CoinInfo *struct {
		Quantity    int    `xml:"Quantity" json:"Quantity"`
		OrigPrice   int64  `xml:"OrigPrice" json:"OrigPrice"`
		ActualPrice int64  `xml:"ActualPrice" json:"ActualPrice"`
		Attach      string `xml:"Attach" json:"Attach"`
	} `xml:"CoinInfo" json:"CoinInfo"`

	// xpay_refund_notify
	WxRefundID               string `xml:"WxRefundId" json:"WxRefundId"`
	MchRefundID              string `xml:"MchRefundId" json:"MchRefundId"`
	WxOrderID                string `xml:"WxOrderId" json:"WxOrderId"`
	MchOrderID               string `xml:"MchOrderId" json:"MchOrderId"`
	RefundFee                int64  `xml:"RefundFee" json:"RefundFee"`
	RetCode                  int    `xml:"RetCode" json:"RetCode"`
	RetMsg                   string `xml:"RetMsg" json:"RetMsg"`
	RefundSuccTimestamp      int64  `xml:"RefundSuccTimestamp" json:"RefundSuccTimestamp"`
	WxpayRefundTransactionID string `xml:"WxpayRefundTransactionId" json:"WxpayRefundTransactionId"`
	RetryTimes               int    `xml:"RetryTimes" json:"RetryTimes"`
}

// 虚拟支付推送事件
const (
	XPayEventGoodsDeliver = "xpay_goods_deliver_notify"
	XPayEventCoinPay      = "xpay_coin_pay_notify"
	XPayEventRefund       = "xpay_refund_notify"
)

// ParseXPayEvent 解析明文推送
func ParseXPayEvent(msg []byte, xmlFormat bool) (*XPayPushEvent, error) {
	var e XPayPushEvent
	var err error
	if xmlFormat {
		err = xml.Unmarshal(msg, &e)
	} else {
		err = json.Unmarshal(msg, &e)
	}
	return &e, err
}

// PushAck 推送回包：与推送格式一致的 ErrCode 方式（失败时平台最多重推 15 次）
func PushAck(xmlFormat bool, errCode int, errMsg string) (string, []byte) {
	if xmlFormat {
		return "application/xml", []byte(fmt.Sprintf("<xml><ErrCode>%d</ErrCode><ErrMsg><![CDATA[%s]]></ErrMsg></xml>", errCode, strings.ReplaceAll(errMsg, "]]>", "")))
	}
	b, _ := json.Marshal(map[string]any{"ErrCode": errCode, "ErrMsg": errMsg})
	return "application/json", b
}
