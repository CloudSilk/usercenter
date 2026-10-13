package pay

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
)

// NotifyHandler 业务处理已验签的回调：返回错误时渠道会重试
type NotifyHandler func(ctx context.Context, p Provider, n *Notify) error

// GinNotify 回调入口：验签解密 → 业务处理 → 按渠道要求回包（实现 Acker 的渠道用其格式，其余 2xx / JSON 错误）。
// 虚拟支付的消息推送地址需同时注册 GET（URL 校验）与 POST，见 MountNotify。
func GinNotify(get func() (Provider, error), handle NotifyHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := get()
		if err != nil || p == nil {
			c.JSON(404, gin.H{"code": "FAIL", "message": "渠道未启用"})
			return
		}
		n, err := p.ParseNotify(c.Request.Context(), c.Request)
		if err == nil && n != nil && n.Kind != "verify" && handle != nil {
			err = handle(c.Request.Context(), p, n)
		}
		if a, ok := p.(Acker); ok {
			st, ct, b := a.Ack(n, err)
			c.Data(st, ct, b)
			return
		}
		switch {
		case errors.Is(err, ErrBadSignature):
			c.JSON(401, gin.H{"code": "FAIL", "message": err.Error()})
		case err != nil:
			c.JSON(500, gin.H{"code": "FAIL", "message": err.Error()})
		default:
			c.Status(204)
		}
	}
}

// MountNotify 在 path 上同时注册 GET 与 POST（消息推送 URL 校验 + 推送）
func MountNotify(r gin.IRoutes, path string, get func() (Provider, error), handle NotifyHandler) {
	h := GinNotify(get, handle)
	r.GET(path, h)
	r.POST(path, h)
}
