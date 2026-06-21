package alert

import (
	"context"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/patrickmn/go-cache"
)

// alertCache 用于告警计数的滑动窗口缓存(key 窗口 1 分钟,清理 2 分钟)
var alertCache = cache.New(time.Minute, 2*time.Minute)

const (
	// loginFailAlertThreshold 同一 IP 在 1 分钟窗口内登录失败达到该阈值时告警
	loginFailAlertThreshold = 3
	// authFailAlertThreshold 同一 IP 鉴权失败达到该阈值时告警
	authFailAlertThreshold = 10
)

// AlertLoginFailure 登录失败告警。基于 IP 的滑动窗口计数,首次达到阈值时告警(避免刷屏)。
func AlertLoginFailure(userName, ip string) {
	if ip == "" {
		return
	}
	count := incrAlertCount("login_fail:" + ip)
	if count == loginFailAlertThreshold {
		log.Errorf(context.Background(),
			"[SECURITY ALERT] 疑似暴力破解: userName=%s ip=%s 在1分钟内登录失败 %d 次",
			userName, ip, loginFailAlertThreshold)
		FireWebhook("brute_force_login", map[string]interface{}{
			"userName": userName, "ip": ip, "failures": loginFailAlertThreshold,
		})
	}
}

// AlertAuthFailure 鉴权失败告警。同一 IP 频繁 401/403 时告警(可能为接口扫描)。
func AlertAuthFailure(ip, url string) {
	if ip == "" {
		return
	}
	count := incrAlertCount("auth_fail:" + ip)
	if count == authFailAlertThreshold {
		log.Errorf(context.Background(),
			"[SECURITY ALERT] 疑似接口扫描/越权探测: ip=%s 在1分钟内鉴权失败 %d 次,最近 url=%s",
			ip, authFailAlertThreshold, url)
	}
}

// incrAlertCount 原子递增告警计数(key 不存在时初始化为 1)。
func incrAlertCount(key string) int {
	if v, err := alertCache.IncrementInt(key, 1); err == nil {
		return v
	}
	// key 不存在时 IncrementInt 返回 error,这里手动初始化
	alertCache.SetDefault(key, 1)
	return 1
}
