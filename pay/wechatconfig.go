package pay

import (
	"fmt"

	"github.com/CloudSilk/usercenter/internal/wechatconfig"
)

// MiniAppCredentials 从 usercenter「微信配置」（按 appName）读取小程序 AppID 与 AppSecret，
// 供 XPayConfig.AppID/AppSecret 复用登录所用的同一份配置。
func MiniAppCredentials(appName string) (appID, appSecret string, err error) {
	c, err := wechatconfig.GetWechatConfigByAppName(appName)
	if err != nil || c == nil {
		return "", "", fmt.Errorf("pay: 未找到微信配置 %q", appName)
	}
	return c.AppID, c.Secret, nil
}
