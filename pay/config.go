package pay

import (
	"context"
	"fmt"
	"strings"
)

// Config 按配置选择渠道
type Config struct {
	// Provider：jsapi（别名 wechat）/ virtual（别名 xpay）/ mock
	Provider   string
	JSAPI      WechatConfig
	XPay       XPayConfig
	MockSecret string
}

// Normalize 规范化渠道名：jsapi / virtual / mock
func Normalize(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "jsapi", "wechat", "wechatpay":
		return "jsapi"
	case "virtual", "xpay", "virtual-payment":
		return "virtual"
	case "mock", "":
		return "mock"
	}
	return strings.ToLower(name)
}

// New 构造配置选择的渠道
func New(ctx context.Context, c Config) (Provider, error) {
	switch Normalize(c.Provider) {
	case "jsapi":
		return NewWechat(ctx, c.JSAPI)
	case "virtual":
		return NewXPay(c.XPay)
	case "mock":
		if c.MockSecret == "" {
			c.MockSecret = "mock-secret"
		}
		return NewMock(c.MockSecret), nil
	}
	return nil, fmt.Errorf("pay: 未知支付渠道 %q（可选 jsapi / virtual / mock）", c.Provider)
}
