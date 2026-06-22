package model

import (
	"github.com/CloudSilk/usercenter/internal/alert"
)

// 委托 internal/alert(REDESIGN §4 阶段0)。
// 告警逻辑已迁入 internal/alert,此处保留薄委托,向后兼容现有 model.Alert* 调用。

// AlertLoginFailure 登录失败告警
func AlertLoginFailure(userName, ip string) {
	alert.AlertLoginFailure(userName, ip)
}

// AlertAuthFailure 鉴权失败告警
func AlertAuthFailure(ip, url string) {
	alert.AlertAuthFailure(ip, url)
}
