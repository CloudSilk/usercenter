package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// 审计事件实时流（SSE）。
//
// GET /admin/api/audit/stream?access_token=<jwt>
//
// 服务端推送新产生的审计事件给管理后台「实时审计大屏」。单向 server→client，
// 浏览器用原生 EventSource 订阅（EventSource 不能设 Authorization 头，故 token 走 query）。
// 订阅者通道满时丢弃（见 audit.bus）。此端点需在鉴权中间件中放行（自身校验 token）。

var sseSubscriberGauge = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "usercenter_audit_sse_subscribers",
	Help: "当前活跃的审计 SSE 订阅者数量",
})

func init() {
	prometheus.MustRegister(sseSubscriberGauge)
}

// ssePingInterval SSE 心跳间隔，防止反向代理/负载均衡器因空闲超时切断连接。
const ssePingInterval = 15 * time.Second

func registerAuditStreamRoute(g *gin.RouterGroup) {
	g.GET("/audit/stream", auditStream)
}

func auditStream(c *gin.Context) {
	// 自校验 token（query），EventSource 无法带 Authorization 头
	if t := c.Query("access_token"); t != "" {
		if cu, err := token.DecodeToken(t); err != nil || cu == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}
	} else if c.GetHeader("Authorization") == "" {
		// 既无 query token 也无 header（中间件已放行此路径），拒绝匿名
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	// 初始心跳，确认连接建立
	_, _ = c.Writer.Write([]byte(": connected\n\n"))
	flusher.Flush()

	events, unsub := audit.Subscribe()
	defer unsub()

	sseSubscriberGauge.Inc()
	defer sseSubscriberGauge.Dec()

	ticker := time.NewTicker(ssePingInterval)
	defer ticker.Stop()

	notify := c.Request.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-ticker.C:
			// 定期发送 SSE 注释帧作为心跳（EventSource 会忽略以 ':' 开头的行）
			_, _ = c.Writer.Write([]byte(": ping\n\n"))
			flusher.Flush()
		case al, ok := <-events:
			if !ok {
				return
			}
			b, _ := json.Marshal(al)
			_, _ = c.Writer.Write([]byte("data: "))
			_, _ = c.Writer.Write(b)
			_, _ = c.Writer.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}

