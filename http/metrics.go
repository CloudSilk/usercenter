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
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus 指标暴露（REDESIGN Wave3 可观测性）。
//
// /metrics 暴露：
//   - Go runtime / 进程指标（GoCollector / ProcessCollector）
//   - HTTP 请求量/延迟（按 method/route/status）
//   - AI 网关调用量、token、成本、错误
//   - 域计数：用户/角色/会话/今日 token（后台 goroutine 定时刷新，避免采集时阻塞查库）
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

	// 域计数 Gauge：由后台 goroutine 周期刷新，取代旧的 GaugeFunc（GaugeFunc 在每次
	// /metrics 采集时同步查库，高 QPS 下会放大 DB 压力并拖慢采集）。
	gaugeUsers       = prometheus.NewGauge(prometheus.GaugeOpts{Name: "usercenter_users_total", Help: "用户总数"})
	gaugeRoles       = prometheus.NewGauge(prometheus.GaugeOpts{Name: "usercenter_roles_total", Help: "角色总数"})
	gaugeSessions    = prometheus.NewGauge(prometheus.GaugeOpts{Name: "usercenter_sessions_active", Help: "活跃会话数"})
	gaugeTodayTokens = prometheus.NewGauge(prometheus.GaugeOpts{Name: "usercenter_tokens_today", Help: "今日 token 总用量"})
)

func init() {
	prometheus.MustRegister(
		// Go runtime 指标（GC、goroutine、memstats、sched 等）
		collectors.NewGoCollector(),
		// 进程级指标（RSS、open fds、启动时间等）
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		metricHTTPRequests, metricHTTPDuration,
		metricAIRequests, metricAITokens, metricAICost, metricAIQuotaExceeded,
		gaugeUsers, gaugeRoles, gaugeSessions, gaugeTodayTokens,
	)
	// 后台 goroutine 周期刷新域计数。
	go refreshDomainMetrics(30 * time.Second)
}

// refreshDomainMetrics 每 interval 刷新一次域计数到对应 Gauge，由 init() 启动。
// 单条查询失败只跳过该指标，不影响其它指标与采集。
func refreshDomainMetrics(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		d := store.DB()
		if d == nil {
			continue
		}
		var n int64
		if err := d.Model(&user.User{}).Count(&n).Error; err == nil {
			gaugeUsers.Set(float64(n))
		}
		if err := d.Model(&permission.Role{}).Count(&n).Error; err == nil {
			gaugeRoles.Set(float64(n))
		}
		if err := d.Model(&session.Session{}).Where("revoked = ?", false).Count(&n).Error; err == nil {
			gaugeSessions.Set(float64(n))
		}
		if err := d.Model(&usage.UsageRecord{}).
			Where("created_at >= ?", time.Now().Truncate(24*time.Hour).Unix()).
			Select("COALESCE(SUM(total_tokens),0)").Scan(&n).Error; err == nil {
			gaugeTodayTokens.Set(float64(n))
		}
	}
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
