package http

import (
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
}
