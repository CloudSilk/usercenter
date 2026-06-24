package http

import (
	"net/http"
	"strconv"

	"github.com/CloudSilk/usercenter/internal/aicache"
	"github.com/CloudSilk/usercenter/internal/conversation"
	"github.com/CloudSilk/usercenter/internal/gatewaylog"
	"github.com/CloudSilk/usercenter/internal/ratelimit"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

// AI 网关增强能力的管理端点：
//   - 对话会话管理（/admin/api/conversations）
//   - 请求日志查询（/admin/api/gateway-logs）
//   - 语义缓存管理（/admin/api/ai-cache）
//   - 限流配置（/admin/api/ratelimit）

// RegisterAIGatewayAdminRoutes 注册 AI 网关增强能力的管理路由。
func RegisterAIGatewayAdminRoutes(g *gin.RouterGroup) {
	// --- 对话会话 ---
	g.GET("/conversations", listConversations)
	g.GET("/conversations/:id", getConversationDetail)
	g.GET("/conversations/:id/messages", listConversationMessages)
	g.DELETE("/conversations/:id", deleteConversation)
	g.PUT("/conversations/:id/title", updateConversationTitle)
	g.POST("/conversations/:id/summarize", summarizeConversation)

	// --- 网关日志 ---
	g.GET("/gateway-logs", listGatewayLogs)
	g.GET("/gateway-logs/stats", gatewayLogStats)
	g.GET("/gateway-logs/:id", getGatewayLog)

	// --- 语义缓存 ---
	g.GET("/ai-cache/stats", cacheStats)
	g.DELETE("/ai-cache", clearCache)

	// --- 限流配置 ---
	g.GET("/ratelimit/stats", ratelimitStats)
}

// listConversations 列出对话会话
func listConversations(c *gin.Context) {
	tenantID := ucm.GetTenantID(c)
	principalID := c.Query("principalID")
	limit, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("pageIndex", "1"))
	if limit <= 0 {
		limit = 20
	}
	if offset > 0 {
		offset = (offset - 1) * limit
	}
	sessions, total, err := conversation.ListSessions(tenantID, principalID, limit, offset)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    apipb.Code_Success,
		"data":    sessions,
		"records": total,
	})
}

// getConversationDetail 获取会话详情
func getConversationDetail(c *gin.Context) {
	id := c.Param("id")
	s, err := conversation.GetSession(id)
	if err != nil || s == nil {
		writeBadRequest(c, errStr("会话不存在"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": s})
}

// listConversationMessages 列出会话消息
func listConversationMessages(c *gin.Context) {
	id := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	msgs, err := conversation.GetMessages(id, limit)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": msgs})
}

// deleteConversation 删除会话
func deleteConversation(c *gin.Context) {
	id := c.Param("id")
	if err := conversation.DeleteSession(id); err != nil {
		writeErr(c, err)
		return
	}
	writeOK(c, nil)
}

// updateConversationTitle 更新会话标题
func updateConversationTitle(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, err)
		return
	}
	if err := conversation.UpdateSessionTitle(id, req.Title); err != nil {
		writeErr(c, err)
		return
	}
	writeOK(c, nil)
}

// summarizeConversation 用 LLM 为会话自动生成标题
func summarizeConversation(c *gin.Context) {
	id := c.Param("id")
	summarizeSession(id)
	// 重新读取更新后的会话返回
	s, err := conversation.GetSession(id)
	if err != nil || s == nil {
		writeBadRequest(c, errStr("会话不存在"))
		return
	}
	writeOK(c, gin.H{"data": s})
}

// listGatewayLogs 列出网关日志
func listGatewayLogs(c *gin.Context) {
	tenantID := ucm.GetTenantID(c)
	startTime, _ := strconv.ParseInt(c.Query("startTime"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("endTime"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("pageIndex", "1"))
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset > 0 {
		offset = (offset - 1) * limit
	}
	logs, total, err := gatewaylog.Query(tenantID, startTime, endTime, limit, offset)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    apipb.Code_Success,
		"data":    logs,
		"records": total,
	})
}

// gatewayLogStats 网关日志统计
func gatewayLogStats(c *gin.Context) {
	tenantID := ucm.GetTenantID(c)
	startTime, _ := strconv.ParseInt(c.Query("startTime"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("endTime"), 10, 64)
	stats, err := gatewaylog.GetStats(tenantID, startTime, endTime)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": stats})
}

// getGatewayLog 获取单条网关日志
func getGatewayLog(c *gin.Context) {
	id := c.Param("id")
	log, err := gatewaylog.GetByID(id)
	if err != nil || log == nil {
		writeBadRequest(c, errStr("日志不存在"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": log})
}

// cacheStats 缓存统计
func cacheStats(c *gin.Context) {
	stats := aicache.Stats()
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": stats})
}

// clearCache 清空缓存
func clearCache(c *gin.Context) {
	aicache.Clear()
	writeOK(c, gin.H{"message": "缓存已清空"})
}

// ratelimitStats 限流统计
func ratelimitStats(c *gin.Context) {
	if ratelimit.Default == nil {
		c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": gin.H{"buckets": gin.H{}}})
		return
	}
	stats := ratelimit.Default.Stats()
	c.JSON(http.StatusOK, gin.H{"code": apipb.Code_Success, "data": stats})
}
