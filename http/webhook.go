package http

import (
	"github.com/CloudSilk/usercenter/internal/webhook"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// registerWebhookRoutes 注册 Webhook 订阅 CRUD 路由。
func registerWebhookRoutes(g *gin.RouterGroup) {
	w := g.Group("/webhooks")

	w.GET("", func(c *gin.Context) {
		tenantID := effectiveTenantID(c)
		limit := queryLimit(c, 20, 100)
		offset := 0
		// 支持 pageIndex/pageSize 分页参数（1-based）。
		pageIndex := parseUnix(c.Query("pageIndex"))
		pageSize := parseUnix(c.Query("pageSize"))
		if pageSize > 0 {
			limit = int(pageSize)
		}
		if pageIndex > 0 && pageSize > 0 {
			offset = int((pageIndex - 1) * pageSize)
		}

		list, total, err := webhook.ListSubs(tenantID, limit, offset)
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": list, "total": total})
	})

	w.POST("", func(c *gin.Context) {
		var req webhook.Subscription
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		if req.TenantID == "" {
			req.TenantID = ucm.GetTenantID(c)
		}
		id, err := webhook.CreateSub(&req)
		if err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "webhook_sub_add", id, req.Name)
		writeOK(c, gin.H{"data": id})
	})

	w.PUT("/:id", func(c *gin.Context) {
		var req webhook.Subscription
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		req.ID = c.Param("id")
		if err := webhook.UpdateSub(&req); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "webhook_sub_update", req.ID, req.Name)
		writeOK(c, nil)
	})

	w.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := webhook.DeleteSub(id); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "webhook_sub_delete", id, "")
		writeOK(c, nil)
	})
}
