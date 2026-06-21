package http

import (
	"strconv"
	"time"

	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/usage"
	"github.com/CloudSilk/usercenter/internal/user"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus 指标暴露（REDESIGN Wave3 可观测性）。
//
// /metrics 暴露：
//   - HTTP 请求量/延迟（按 method/route/status）
//   - AI 网关调用量、token、成本、错误
//   - 域计数：用户/角色/会话/今日 token（GaugeFunc，采集时查库）
//
// 告警与审计实时流见 alert / audit 包及 /admin/api/audit/stream。

var (
	metricHTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "usercenter_http_requests_total", Help: "HTTP 请求总数",
	}, []string{"method", "route", "status"})
	metricHTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "usercenter_http_request_duration_seconds", Help: "HTTP 请求延迟(秒)",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	metricAIRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "usercenter_aigateway_requests_total", Help: "AI 网关调用总数",
	}, []string{"model", "result"})
	metricAITokens = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "usercenter_aigateway_tokens_total", Help: "AI 网关 token 用量",
	}, []string{"model", "kind"}) // kind: prompt / completion
	metricAICost = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "usercenter_aigateway_cost_usd_total", Help: "AI 网关累计成本(USD)",
	}, []string{"model"})
	metricAIQuotaExceeded = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "usercenter_aigateway_quota_exceeded_total", Help: "AI 网关超额拒绝次数",
	})
)

func init() {
	prometheus.MustRegister(metricHTTPRequests, metricHTTPDuration,
		metricAIRequests, metricAITokens, metricAICost, metricAIQuotaExceeded,
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "usercenter_users_total", Help: "用户总数",
		}, gaugeUserCount),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "usercenter_roles_total", Help: "角色总数",
		}, gaugeRoleCount),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "usercenter_sessions_active", Help: "活跃会话数",
		}, gaugeActiveSessions),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "usercenter_tokens_today", Help: "今日 token 总用量",
		}, gaugeTodayTokens),
	)
}

// MetricsMiddleware 记录每条 HTTP 请求的量与延迟。
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		metricHTTPRequests.WithLabelValues(c.Request.Method, route, status).Inc()
		metricHTTPDuration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}

// observeAIGatewayCall 由 AI 网关在调用结束后上报指标。
func observeAIGatewayCall(model string, prompt, comp int64, cost float64, success bool) {
	res := "success"
	if !success {
		res = "error"
	}
	metricAIRequests.WithLabelValues(model, res).Inc()
	metricAITokens.WithLabelValues(model, "prompt").Add(float64(prompt))
	metricAITokens.WithLabelValues(model, "completion").Add(float64(comp))
	if cost > 0 {
		metricAICost.WithLabelValues(model).Add(cost)
	}
}

func observeAIQuotaExceeded() {
	metricAIQuotaExceeded.Inc()
}

// RegisterMetricsRouter 挂载 /metrics（需在 AuthRequired 中放行）。
func RegisterMetricsRouter(r *gin.Engine) {
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// --- 域计数 GaugeFunc ---

func gaugeUserCount() float64 {
	if store.DB() == nil {
		return 0
	}
	var n int64
	_ = store.DB().Model(&user.User{}).Count(&n).Error
	return float64(n)
}

func gaugeRoleCount() float64 {
	if store.DB() == nil {
		return 0
	}
	var n int64
	_ = store.DB().Model(&permission.Role{}).Count(&n).Error
	return float64(n)
}

func gaugeActiveSessions() float64 {
	if store.DB() == nil {
		return 0
	}
	var n int64
	_ = store.DB().Model(&session.Session{}).Where("revoked = ?", false).Count(&n).Error
	return float64(n)
}

func gaugeTodayTokens() float64 {
	if store.DB() == nil {
		return 0
	}
	var n int64
	_ = store.DB().Model(&usage.UsageRecord{}).
		Where("created_at >= ?", time.Now().Truncate(24*time.Hour).Unix()).
		Select("COALESCE(SUM(total_tokens),0)").Scan(&n).Error
	return float64(n)
}
