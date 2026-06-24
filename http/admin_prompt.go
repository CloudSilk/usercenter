package http

import (
	"strconv"

	"github.com/CloudSilk/usercenter/internal/prompt"
	"github.com/gin-gonic/gin"
)

// 管理端 Prompt 模板 API（/admin/api/prompts）。
// 配合 AI 网关：模板集中管理 + 变量渲染，调用方渲染后打 /v1/chat/completions。

func registerPromptRoutes(g *gin.RouterGroup) {
	p := g.Group("/prompts")

	p.GET("", func(c *gin.Context) {
		list, err := prompt.List(effectiveTenantID(c), c.Query("category"))
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": list})
	})

	p.POST("", func(c *gin.Context) {
		var req prompt.PromptTemplate
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		if req.TenantID == "" {
			req.TenantID = effectiveTenantID(c)
		}
		id, err := prompt.Create(&req)
		if err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "prompt_add", id, req.Name)
		writeOK(c, gin.H{"data": id})
	})

	p.PUT("/:id", func(c *gin.Context) {
		var req prompt.PromptTemplate
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, err)
			return
		}
		req.ID = c.Param("id")
		if err := prompt.Update(&req); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "prompt_update", req.ID, req.Name)
		writeOK(c, nil)
	})

	p.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := prompt.Delete(id); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "prompt_delete", id, "")
		writeOK(c, nil)
	})

	// 渲染：POST /admin/api/prompts/:id/render  body: {"vars":{"k":"v"}}
	p.POST("/:id/render", func(c *gin.Context) {
		tpl, err := prompt.GetByID(c.Param("id"))
		if err != nil {
			writeErr(c, err)
			return
		}
		var body struct {
			Vars map[string]string `json:"vars"`
		}
		_ = c.ShouldBindJSON(&body)
		writeOK(c, gin.H{"data": gin.H{
			"rendered":  prompt.Render(tpl.Content, body.Vars),
			"variables": prompt.ExtractVariables(tpl.Content),
			"model":     tpl.ModelAlias,
		}})
	})

	// 版本历史：GET /admin/api/prompts/:id/versions
	p.GET("/:id/versions", func(c *gin.Context) {
		list, err := prompt.ListVersions(c.Param("id"))
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": list})
	})

	// 查看指定版本：GET /admin/api/prompts/:id/versions/:version
	p.GET("/:id/versions/:version", func(c *gin.Context) {
		version, err := strconv.Atoi(c.Param("version"))
		if err != nil {
			writeBadRequest(c, errStr("版本号无效"))
			return
		}
		v, err := prompt.GetVersion(c.Param("id"), version)
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": v})
	})

	// 回滚：POST /admin/api/prompts/:id/rollback  body: {"version":3}
	p.POST("/:id/rollback", func(c *gin.Context) {
		var body struct {
			Version int `json:"version"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			writeBadRequest(c, err)
			return
		}
		if body.Version <= 0 {
			writeBadRequest(c, errStr("版本号必须大于 0"))
			return
		}
		if err := prompt.Rollback(c.Param("id"), body.Version); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "prompt_rollback", c.Param("id"), "v"+strconv.Itoa(body.Version))
		writeOK(c, gin.H{"message": "已回滚到版本 v" + strconv.Itoa(body.Version)})
	})
}
