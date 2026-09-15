package auth

import (
	"testing"
	"time"
)

func TestSetPwdMaxAgeZeroMeansNeverExpire(t *testing.T) {
	SetPwdMaxAge(0)
	defer SetPwdMaxAge(0)
	if IsPwdExpired(time.Now().AddDate(0, 0, -3650).Unix()) {
		t.Fatal("max age 0 should never expire")
	}
}

func TestIsPwdExpired(t *testing.T) {
	SetPwdMaxAge(30) // 30 天
	defer SetPwdMaxAge(0)

	now := time.Now().Unix()
	if IsPwdExpired(now - 29*86400) {
		t.Fatal("29 days old should not be expired")
	}
	if !IsPwdExpired(now - 31*86400) {
		t.Fatal("31 days old should be expired")
	}
	// 非法时间戳(<=0)不过期
	if IsPwdExpired(0) || IsPwdExpired(-100) {
		t.Fatal("invalid timestamp should not be expired")
	}
}
