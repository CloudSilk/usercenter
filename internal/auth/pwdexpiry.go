package auth

import "time"

var defaultPwdMaxAge int64 = 0 // 0 = never expire

// SetPwdMaxAge 设置密码过期天数。0=永不过期。
func SetPwdMaxAge(days int) {
	defaultPwdMaxAge = int64(days) * 86400
}

// IsPwdExpired 检查密码是否已过期。
// passwordUpdatedAt 是密码最后修改的 unix 时间戳。
// 当 defaultPwdMaxAge <= 0 或 passwordUpdatedAt <= 0 时返回 false（不过期）。
func IsPwdExpired(passwordUpdatedAt int64) bool {
	if defaultPwdMaxAge <= 0 || passwordUpdatedAt <= 0 {
		return false
	}
	return time.Now().Unix()-passwordUpdatedAt > defaultPwdMaxAge
}
