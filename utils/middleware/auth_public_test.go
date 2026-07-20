package middleware

import "testing"

func TestPublicAuthenticationPaths(t *testing.T) {
	public := []string{
		"/api/core/auth/login/options",
		"/api/core/auth/user/login",
		"/api/core/auth/user/mfa/verify",
		"/api/core/auth/user/phone/status",
		"/api/core/auth/user/phone/code",
		"/api/core/auth/user/phone/login",
		"/api/social/providers",
		"/api/oauth/github/login",
		"/api/oauth/github/callback",
		"/api/wechat/notify/example",
		"/api/wechat/mini/login",
		"/api/wechat/mini/register/check",
		"/api/wechat/connect/qrconnect",
		"/api/wechat/web/login",
		"/api/wechat/qrcode",
		"/api/wechat/qrcode/result",
	}
	for _, path := range public {
		if !isPublicAuthPath(path) {
			t.Errorf("expected public path: %s", path)
		}
	}

	protected := []string{
		"/api/core/auth/user/profile",
		"/api/core/wechat/config/query",
		"/api/wechat/mini/phone/bind",
		"/api/core/auth/role/authorization",
	}
	for _, path := range protected {
		if isPublicAuthPath(path) {
			t.Errorf("expected protected path: %s", path)
		}
	}
}
