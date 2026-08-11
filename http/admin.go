package http

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/usercenter/internal/alert"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/pricing"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/usage"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// 管理端(admin)API:挂载在 /admin/api 下。
//
// 覆盖域:AI Provider / AI Key / Model Route / Usage(聚合+预算) /
// Session(列表+吊销) / Audit(分页查询) / Dashboard 统计 / 告警配置。
//
// 设计约束:
//   - AIKey 明文(APIKeyEnc)永不下发,仅返回 KeyHint。
//   - 写操作(Create/Update/Delete/Rotate/Revoke)一律记审计日志。
//   - 查询默认按当前登录用户的 tenantID 隔离,除非显式传 tenantID(超管视图)。
//   - 响应统一用 apipb.Code_* (20000/40000/50000),与 http/ 下其它 handler 一致。
//   - 列表/聚合使用领域包已封装的函数,避免 handler 直接拼裸 SQL 漂移表名。

// RegisterAdminRouter 注册管理面板 API。鉴权由调用方(中间件)负责,
// 此处只挂业务路由。所有路由前缀 /admin/api。
func RegisterAdminRouter(r *gin.Engine) {
	g := r.Group("/admin/api")

	registerAIProviderRoutes(g)
	registerAIKeyRoutes(g)
	registerModelRouteRoutes(g)
	registerUsageRoutes(g)
	registerSessionRoutes(g)
	registerLoginRecordRoutes(g)
	registerAuditRoutes(g)
	registerDashboardRoutes(g)
	registerPromptRoutes(g)
	registerOAuthClientRoutes(g)
	registerAuditStreamRoute(g)
	registerMFARoutes(g)
	registerSocialAdminRoutes(g)
	registerPricingRoutes(g)
	registerWebhookRoutes(g)
	RegisterAIGatewayAdminRoutes(g)
}

// ---------------------------------------------------------------------------
// AI Provider
// ---------------------------------------------------------------------------

type providerInfo struct {
	ID                string `json:"id"`
	TenantID          string `json:"tenantID"`
	Name              string `json:"name"`
	BaseURL           string `json:"baseURL"`
	AuthType          string `json:"authType"`
	Healthy           bool   `json:"healthy"`
	Description       string `json:"description"`
	IsMust            bool   `json:"isMust"`
	KeyCount          int64  `json:"keyCount"`
	RouteCount        int64  `json:"routeCount"`
	CanDelete         bool   `json:"canDelete"`
	DeleteBlockReason string `json:"deleteBlockReason"`
}

func providerToInfo(p *apikey.AIProvider, impact aiProviderImpact) *providerInfo {
	if p == nil {
		return nil
	}
	info := &providerInfo{
		ID: p.ID, TenantID: p.TenantID, Name: p.Name, BaseURL: p.BaseURL,
		AuthType: p.AuthType, Healthy: p.Healthy, Description: p.Description,
		IsMust: isSystemAIProvider(p), KeyCount: impact.KeyCount, RouteCount: impact.RouteCount,
		CanDelete: true,
	}
	switch {
	case info.IsMust:
		info.CanDelete = false
		info.DeleteBlockReason = "系统兜底服务商由启动流程维护"
	case impact.KeyCount > 0 || impact.RouteCount > 0:
		info.CanDelete = false
		info.DeleteBlockReason = fmt.Sprintf(
			"仍关联 %d 个 Key、%d 条模型路由",
			impact.KeyCount,
			impact.RouteCount,
		)
	}
	return info
}

func registerAIProviderRoutes(g *gin.RouterGroup) {
	p := g.Group("/ai-providers")

	p.GET("", func(c *gin.Context) {
		tenantID := effectiveTenantID(c)
		list, err := apikey.GetAllProviders(tenantID)
		if err != nil {
			writeErr(c, err)
			return
		}
		out := make([]*providerInfo, 0, len(list))
		for _, it := range list {
			impact, err := aiProviderImpactForTenant(tenantID, it.ID)
			if err != nil {
				writeErr(c, err)
				return
			}
			out = append(out, providerToInfo(it, impact))
		}
		writeOK(c, gin.H{"data": out})
	})

	p.POST("", func(c *gin.Context) {
		var req apikey.AIProvider
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		id, err := createAIProviderForTenant(effectiveTenantID(c), req)
		if err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_provider_add", id, req.Name)
		writeOK(c, gin.H{"data": id})
	})

	p.PUT("/:id", func(c *gin.Context) {
		var req apikey.AIProvider
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		id := c.Param("id")
		if err := updateAIProviderForTenant(effectiveTenantID(c), id, req); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_provider_update", id, req.Name)
		writeOK(c, nil)
	})

	p.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := deleteAIProviderForTenant(effectiveTenantID(c), id); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_provider_delete", id, "")
		writeOK(c, nil)
	})
}

// ---------------------------------------------------------------------------
// AI Key(明文永不下发)
// ---------------------------------------------------------------------------

type keyInfo struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenantID"`
	ProviderID  string `json:"providerID"`
	Name        string `json:"name"`
	KeyHint     string `json:"keyHint"`
	Priority    int32  `json:"priority"`
	Enable      bool   `json:"enable"`
	CooldownEnd int64  `json:"cooldownEnd"`
	Last429     int64  `json:"last429"`
	IsMust      bool   `json:"isMust"`
}

func keyToInfo(k *apikey.AIKey) *keyInfo {
	if k == nil {
		return nil
	}
	return &keyInfo{
		ID: k.ID, TenantID: k.TenantID, ProviderID: k.ProviderID, Name: k.Name,
		KeyHint: k.KeyHint, Priority: k.Priority, Enable: k.Enable,
		CooldownEnd: k.CooldownEnd, Last429: k.Last429, IsMust: k.IsMust,
	}
}

type createKeyRequest struct {
	TenantID   string `json:"tenantID"`
	ProviderID string `json:"providerID" binding:"required"`
	Name       string `json:"name" binding:"required"`
	APIKey     string `json:"apiKey" binding:"required"` // 明文,服务端 AES-GCM 加密存储
	Priority   int32  `json:"priority"`
	Enable     bool   `json:"enable"`
}

type updateKeyRequest struct {
	ProviderID string `json:"providerID"`
	Name       string `json:"name"`
	Priority   int32  `json:"priority"`
	Enable     bool   `json:"enable"`
}

type rotateKeyRequest struct {
	NewAPIKey string `json:"newAPIKey" binding:"required"`
}

func registerAIKeyRoutes(g *gin.RouterGroup) {
	k := g.Group("/ai-keys")

	k.GET("", func(c *gin.Context) {
		providerID := c.Query("providerID")
		if providerID == "" {
			writeBadRequest(c, errStr("providerID required"))
			return
		}
		tenantID := effectiveTenantID(c)
		if _, err := findAIProviderForTenant(tenantID, providerID); err != nil {
			writeErr(c, err)
			return
		}
		list, err := apikey.GetKeysByProvider(providerID, tenantID)
		if err != nil {
			writeErr(c, err)
			return
		}
		out := make([]*keyInfo, 0, len(list))
		for _, it := range list {
			out = append(out, keyToInfo(it))
		}
		writeOK(c, gin.H{"data": out})
	})

	k.POST("", func(c *gin.Context) {
		var req createKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		id, err := createAIKeyForTenant(
			effectiveTenantID(c),
			req.ProviderID,
			req.Name,
			req.APIKey,
			req.Priority,
			req.Enable,
		)
		if err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_key_add", id, req.Name)
		writeOK(c, gin.H{"data": id})
	})

	k.PUT("/:id", func(c *gin.Context) {
		var req updateKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		id := c.Param("id")
		if err := updateAIKeyForTenant(
			effectiveTenantID(c),
			id,
			req.ProviderID,
			req.Name,
			req.Priority,
			req.Enable,
		); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_key_update", id, req.Name)
		writeOK(c, nil)
	})

	k.POST("/:id/rotate", func(c *gin.Context) {
		var req rotateKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		id := c.Param("id")
		if err := rotateAIKeyForTenant(effectiveTenantID(c), id, req.NewAPIKey); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_key_rotate", id, "")
		writeOK(c, nil)
	})

	k.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := deleteAIKeyForTenant(effectiveTenantID(c), id); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_key_delete", id, "")
		writeOK(c, nil)
	})

	// 一键实测：用该 Key 向服务商发一个 max_tokens=1 的最小补全，验证 Key 可用性。
	// 返回 {success, statusCode, latencyMs, model, error}。
	k.POST("/:id/test", testAIKey)
}

func testAIKey(c *gin.Context) {
	id := c.Param("id")
	key, provider, err := findAIKeyForTenant(effectiveTenantID(c), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	plaintext, err := apikey.GetDecryptedKey(id)
	if err != nil {
		writeErr(c, err)
		return
	}
	model := c.Query("model")
	if model == "" {
		model = "gpt-3.5-turbo"
	}
	body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}],"max_tokens":1}`, model)
	target := strings.TrimRight(provider.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		writeErr(c, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	switch strings.ToLower(provider.AuthType) {
	case "bearer", "":
		req.Header.Set("Authorization", "Bearer "+plaintext)
	case "header", "apikey":
		req.Header.Set("Authorization", plaintext)
	case "query":
		query := req.URL.Query()
		query.Set("key", plaintext)
		req.URL.RawQuery = query.Encode()
	default:
		req.Header.Set("Authorization", "Bearer "+plaintext)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		writeOK(c, gin.H{"data": gin.H{"success": false, "latencyMs": latency, "error": err.Error()}})
		return
	}
	defer resp.Body.Close()
	success := resp.StatusCode < 400
	errCode := ""
	if resp.StatusCode == http.StatusTooManyRequests {
		apikey.MarkCooldown(key.ID, 5*time.Minute)
		errCode = "429 (Key 已冷却)"
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	recordAudit(c, "ai_key_test", id, fmt.Sprintf("status=%d success=%v", resp.StatusCode, success))
	writeOK(c, gin.H{"data": gin.H{
		"success": success, "statusCode": resp.StatusCode, "latencyMs": latency,
		"model": model, "error": errCode, "snippet": string(b),
	}})
}

// ---------------------------------------------------------------------------
// Model Route
// ---------------------------------------------------------------------------

func registerModelRouteRoutes(g *gin.RouterGroup) {
	rt := g.Group("/model-routes")

	rt.GET("", func(c *gin.Context) {
		list, err := apikey.GetRoutes(effectiveTenantID(c))
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": list})
	})

	rt.POST("", func(c *gin.Context) {
		var req apikey.ModelRoute
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		if req.TenantID == "" {
			req.TenantID = ucm.GetTenantID(c)
		}
		id, err := createAIRouteForTenant(effectiveTenantID(c), req)
		if err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_route_add", id, req.ModelAlias)
		writeOK(c, gin.H{"data": id})
	})

	rt.PUT("/:id", func(c *gin.Context) {
		var req apikey.ModelRoute
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		id := c.Param("id")
		if err := updateAIRouteForTenant(effectiveTenantID(c), id, req); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_route_update", id, req.ModelAlias)
		writeOK(c, nil)
	})

	rt.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := deleteAIRouteForTenant(effectiveTenantID(c), id); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_route_delete", id, "")
		writeOK(c, nil)
	})
}

// ---------------------------------------------------------------------------
// Usage(只读聚合 + 预算 CRUD)
// ---------------------------------------------------------------------------

func registerUsageRoutes(g *gin.RouterGroup) {
	u := g.Group("/usage")

	u.GET("/summary", func(c *gin.Context) {
		start, end := queryTimeRange(c)
		s, err := usage.GetUsageSummary(effectiveTenantID(c), c.Query("principalID"), start, end)
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": s})
	})

	u.GET("/by-model", func(c *gin.Context) {
		start, end := queryTimeRange(c)
		data, err := usage.GetUsageByModel(effectiveTenantID(c), start, end)
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": data})
	})

	u.GET("/by-principal", func(c *gin.Context) {
		start, end := queryTimeRange(c)
		data, err := usage.GetUsageByPrincipal(effectiveTenantID(c), start, end)
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": data})
	})

	// 最近用量明细(分租户),limit 上限 500。
	u.GET("/recent", func(c *gin.Context) {
		limit := queryLimit(c, 50, 500)
		tenantID := effectiveTenantID(c)
		db := store.DB().Model(&usage.UsageRecord{})
		if tenantID != "" {
			db = db.Where("tenant_id = ?", tenantID)
		}
		var records []*usage.UsageRecord
		if err := db.Order("created_at desc").Limit(limit).Find(&records).Error; err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": records})
	})

	// 预算 CRUD。
	b := u.Group("/budgets")
	b.GET("", func(c *gin.Context) {
		tenantID := effectiveTenantID(c)
		var budgets []*usage.UsageBudget
		db := store.DB()
		if tenantID != "" {
			db = db.Where("tenant_id = ?", tenantID)
		}
		if err := db.Order("updated_at desc").Find(&budgets).Error; err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": budgets})
	})
	b.POST("", func(c *gin.Context) {
		var req usage.UsageBudget
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		if req.TenantID == "" {
			req.TenantID = ucm.GetTenantID(c)
		}
		id, err := usage.CreateBudget(&req)
		if err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_budget_add", id, req.ModelName)
		writeOK(c, gin.H{"data": id})
	})
	b.PUT("/:id", func(c *gin.Context) {
		var req usage.UsageBudget
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		req.ID = c.Param("id")
		if err := usage.UpdateBudget(&req); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_budget_update", req.ID, req.ModelName)
		writeOK(c, nil)
	})
	b.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := usage.DeleteBudget(id); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "ai_budget_delete", id, "")
		writeOK(c, nil)
	})
}

// ---------------------------------------------------------------------------
// Session(列表 + 吊销 + 清理)
// ---------------------------------------------------------------------------

func registerSessionRoutes(g *gin.RouterGroup) {
	s := g.Group("/sessions")

	// ?active=1 仅活跃会话;默认含已吊销。principalID 为空时返回平台级分页列表。
	s.GET("", func(c *gin.Context) {
		pageIndex := parseUnix(c.Query("pageIndex"))
		pageSize := parseUnix(c.Query("pageSize"))
		list, total, err := session.QuerySessions(session.Query{
			PrincipalID: c.Query("principalID"),
			TenantID:    c.Query("tenantID"),
			Keyword:     c.Query("keyword"),
			ActiveOnly:  c.Query("active") == "1",
			PageIndex:   pageIndex,
			PageSize:    pageSize,
			IdleMinutes: int(parseUnix(c.Query("idleMinutes"))),
		})
		if err != nil {
			writeErr(c, err)
			return
		}
		pageIndex, pageSize = normalizeAdminPage(pageIndex, pageSize)
		writeOK(c, gin.H{"data": list, "total": total, "pageIndex": pageIndex, "pageSize": pageSize})
	})

	s.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := session.RevokeSession(id, "admin_revoked"); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "session_revoke", id, "admin_revoked")
		writeOK(c, nil)
	})

	// 吊销主体全部会话(下线所有设备)。
	s.POST("/revoke-all", func(c *gin.Context) {
		var req struct {
			PrincipalID     string `json:"principalID" binding:"required"`
			ExceptSessionID string `json:"exceptSessionID"`
			Reason          string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		count, err := session.RevokeAllByPrincipal(req.PrincipalID, req.ExceptSessionID, req.Reason)
		if err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "session_revoke_all", req.PrincipalID, req.Reason)
		writeOK(c, gin.H{"data": gin.H{"count": count}})
	})

	// 清理过期会话。?maxAgeHours=720(默认 30 天)。
	s.POST("/clean", func(c *gin.Context) {
		maxAge := queryLimit(c, 720, 0) // 上限 0 = 不封顶
		count, err := session.CleanExpiredSessions(maxAge)
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": gin.H{"count": count}})
	})
}

func registerLoginRecordRoutes(g *gin.RouterGroup) {
	g.GET("/login-records", func(c *gin.Context) {
		pageIndex := parseUnix(c.Query("pageIndex"))
		pageSize := parseUnix(c.Query("pageSize"))
		query := session.LoginRecordQuery{
			PrincipalID: c.Query("principalID"),
			TenantID:    c.Query("tenantID"),
			Keyword:     c.Query("keyword"),
			Result:      c.Query("result"),
			PageIndex:   pageIndex,
			PageSize:    pageSize,
		}
		if raw := c.Query("abnormal"); raw != "" {
			abnormal, err := strconv.ParseBool(raw)
			if err != nil {
				writeBadRequest(c, errStr("abnormal must be true or false"))
				return
			}
			query.Abnormal = &abnormal
		}
		list, total, err := session.QueryLoginRecords(query)
		if err != nil {
			writeErr(c, err)
			return
		}
		pageIndex, pageSize = normalizeAdminPage(pageIndex, pageSize)
		writeOK(c, gin.H{"data": list, "total": total, "pageIndex": pageIndex, "pageSize": pageSize})
	})
}

// ---------------------------------------------------------------------------
// Audit(分页查询)
// ---------------------------------------------------------------------------

func registerAuditRoutes(g *gin.RouterGroup) {
	g.GET("/audit-logs", func(c *gin.Context) {
		q := &audit.AuditQuery{
			TenantID:  c.Query("tenantID"),
			RequestID: c.Query("requestID"),
			UserID:    c.Query("userID"),
			Action:    c.Query("action"),
			TargetID:  c.Query("targetID"),
			StartTime: parseUnix(c.Query("startTime")),
			EndTime:   parseUnix(c.Query("endTime")),
			PageIndex: parseUnix(c.Query("pageIndex")),
			PageSize:  parseUnix(c.Query("pageSize")),
		}
		if v := c.Query("principalKind"); v != "" {
			if kind, err := strconv.Atoi(v); err == nil {
				q.PrincipalKind = int32(kind)
			}
		} else {
			q.PrincipalKind = -1 // 不过滤
		}

		list, total, err := audit.QueryAuditLogs(q)
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": list, "total": total})
	})
}

// ---------------------------------------------------------------------------
// Dashboard 统计 + 告警配置
// ---------------------------------------------------------------------------

func registerDashboardRoutes(g *gin.RouterGroup) {
	g.GET("/stats", func(c *gin.Context) {
		var userCount, roleCount, sessionCount, todayTokens int64

		// 用模型结构体 Count,让 GORM 自行解析表名,避免硬编码漂移。
		_ = store.DB().Model(&user.User{}).Count(&userCount).Error
		_ = store.DB().Model(&permission.Role{}).Count(&roleCount).Error
		_ = store.DB().Model(&session.Session{}).Where("revoked = ?", false).Count(&sessionCount).Error
		_ = store.DB().Model(&usage.UsageRecord{}).
			Where("created_at >= ?", localDayStart(time.Now())).
			Select("COALESCE(SUM(total_tokens), 0)").Scan(&todayTokens).Error

		writeOK(c, gin.H{"data": gin.H{
			"userCount":    userCount,
			"roleCount":    roleCount,
			"sessionCount": sessionCount,
			"todayTokens":  todayTokens,
		}})
	})

	// 告警阈值配置(当前为静态默认,后续可接 systemconfig 动态化)。
	g.GET("/alerts/status", func(c *gin.Context) {
		writeOK(c, gin.H{"data": gin.H{
			"loginFailThreshold": 3,
			"authFailThreshold":  10,
			"windowMinutes":      1,
		}})
	})

	// 告警 Webhook 通道配置：读取/设置全局告警推送 URL（Slack/钉钉/飞书/自建平台）。
	// 配置在启动时从 alertWebhookURL 注入；此处允许管理端运行时调整，空 URL 即禁用推送。
	g.GET("/alerts/webhook", func(c *gin.Context) {
		writeOK(c, gin.H{"data": gin.H{"url": alert.WebhookURL()}})
	})
	g.PUT("/alerts/webhook", func(c *gin.Context) {
		var req struct {
			URL string `json:"url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		alert.SetWebhookURL(strings.TrimSpace(req.URL))
		recordAudit(c, "alert_webhook_update", "", req.URL)
		writeOK(c, gin.H{"data": gin.H{"url": alert.WebhookURL()}})
	})
}

// ---------------------------------------------------------------------------
// 响应/工具
// ---------------------------------------------------------------------------

func writeOK(c *gin.Context, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	data["code"] = int32(apipb.Code_Success)
	c.JSON(http.StatusOK, data)
}

func writeErr(c *gin.Context, err error) {
	c.JSON(http.StatusOK, &apipb.CommonResponse{Code: apipb.Code_InternalServerError, Message: err.Error()})
}

func writeBadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusOK, &apipb.CommonResponse{Code: apipb.Code_BadRequest, Message: err.Error()})
}

func errStr(s string) error { return &simpleError{msg: s} }

func normalizeAdminPage(pageIndex, pageSize int64) (int64, int64) {
	if pageIndex <= 0 {
		pageIndex = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return pageIndex, pageSize
}

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }

// recordAudit 记录一条管理端审计日志。失败只记日志,不影响业务响应。
func recordAudit(c *gin.Context, action, targetID, detail string) {
	audit.RecordAuditWithContext(
		store.DB(),
		ucm.GetTenantID(c),
		ucm.TraceID(c),
		ucm.GetUserID(c),
		currentUserName(c),
		int32(ucm.GetPrincipalKind(c)),
		action,
		targetID,
		c.ClientIP(),
		detail,
	)
}

// currentUserName 取当前登录用户名(优先 nickname),用于审计。
func currentUserName(c *gin.Context) string {
	if ok, u := ucm.GetUser(c); ok && u != nil {
		if u.Nickname != "" {
			return u.Nickname
		}
		return u.Id
	}
	return ucm.GetUserID(c)
}

// effectiveTenantID allows an explicit tenant override only for the platform
// tenant. Ordinary tenant administrators are always scoped to their token.
func effectiveTenantID(c *gin.Context) string {
	current := strings.TrimSpace(ucm.GetTenantID(c))
	if current == constants.PlatformTenantID {
		if requested := strings.TrimSpace(c.Query("tenantID")); requested != "" {
			return requested
		}
	}
	return current
}

// queryTimeRange 解析 startTime/endTime(unix 秒,0 表示不限定)。
func queryTimeRange(c *gin.Context) (start, end int64) {
	return parseUnix(c.Query("startTime")), parseUnix(c.Query("endTime"))
}

// queryLimit 解析正整数 limit,默认 def;max>0 时封顶 max,max=0 不封顶。
func queryLimit(c *gin.Context, def, max int) int {
	v, err := strconv.Atoi(c.Query("limit"))
	if err != nil || v <= 0 {
		return def
	}
	if max > 0 && v > max {
		return max
	}
	return v
}

// parseUnix 解析 unix 秒字符串,空/失败返回 0。
func parseUnix(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// ---------------------------------------------------------------------------
// Pricing CRUD (模型计价表)
// ---------------------------------------------------------------------------

func registerPricingRoutes(g *gin.RouterGroup) {
	p := g.Group("/pricing")

	p.GET("", func(c *gin.Context) {
		tenantID := effectiveTenantID(c)
		list, err := pricing.ListPrices(tenantID)
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": list})
	})

	p.POST("", func(c *gin.Context) {
		var req pricing.ModelPrice
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		if req.TenantID == "" {
			req.TenantID = ucm.GetTenantID(c)
		}
		id, err := pricing.CreatePrice(&req)
		if err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "pricing_add", id, req.ModelName)
		writeOK(c, gin.H{"data": id})
	})

	p.PUT("/:id", func(c *gin.Context) {
		var req pricing.ModelPrice
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		req.ID = c.Param("id")
		if err := pricing.UpdatePrice(&req); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "pricing_update", req.ID, req.ModelName)
		writeOK(c, nil)
	})

	p.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := pricing.DeletePrice(id); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "pricing_delete", id, "")
		writeOK(c, nil)
	})
}
