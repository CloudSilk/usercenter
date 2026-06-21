package http

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/gin-gonic/gin"
)

// OAuth Client 管理（/admin/api/oauth-clients）。
// Secret 仅在创建/轮转时返回明文一次，DB 中存 bcrypt 哈希。

func registerOAuthClientRoutes(g *gin.RouterGroup) {
	oc := g.Group("/oauth-clients")
	oc.GET("", listOAuthClients)
	oc.POST("", createOAuthClient)
	oc.PUT("/:id", updateOAuthClient)
	oc.POST("/:id/rotate-secret", rotateOAuthClientSecret)
	oc.DELETE("/:id", deleteOAuthClient)
}

func listOAuthClients(c *gin.Context) {
	var list []*auth.OAuthClient
	// Secret 字段 json:"-"，序列化时自动隐藏明文
	if err := store.DB().Find(&list).Error; err != nil {
		writeErr(c, err)
		return
	}
	writeOK(c, gin.H{"data": list})
}

type createClientReq struct {
	ID           string `json:"id"`
	Name         string `json:"name" binding:"required"`
	RedirectURIs string `json:"redirectURIs"`
	GrantTypes   string `json:"grantTypes"`
	Scopes       string `json:"scopes"`
}

func createOAuthClient(c *gin.Context) {
	var req createClientReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, err)
		return
	}
	id := req.ID
	if id == "" {
		id = "client-" + randSuffix()
	}
	plain := genClientSecret()
	hashed, err := hashClientSecret(plain)
	if err != nil {
		writeErr(c, err)
		return
	}
	client := &auth.OAuthClient{
		ID: id, Secret: hashed, Name: req.Name, RedirectURIs: req.RedirectURIs,
		GrantTypes: req.GrantTypes, Scopes: req.Scopes, Enable: true,
	}
	if err := store.DB().Create(client).Error; err != nil {
		writeErr(c, err)
		return
	}
	recordAudit(c, "oauth_client_add", id, req.Name)
	// 仅此一次返回明文 secret
	writeOK(c, gin.H{"data": gin.H{"id": id, "secret": plain, "name": req.Name,
		"redirectURIs": req.RedirectURIs, "grantTypes": req.GrantTypes, "scopes": req.Scopes}})
}

func updateOAuthClient(c *gin.Context) {
	var req struct {
		Name         string `json:"name"`
		RedirectURIs string `json:"redirectURIs"`
		GrantTypes   string `json:"grantTypes"`
		Scopes       string `json:"scopes"`
		Enable       *bool  `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, err)
		return
	}
	id := c.Param("id")
	updates := map[string]interface{}{
		"name": req.Name, "redirect_uris": req.RedirectURIs,
		"grant_types": req.GrantTypes, "scopes": req.Scopes,
	}
	if req.Enable != nil {
		updates["enable"] = *req.Enable
	}
	if err := store.DB().Model(&auth.OAuthClient{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		writeErr(c, err)
		return
	}
	recordAudit(c, "oauth_client_update", id, req.Name)
	writeOK(c, nil)
}

func rotateOAuthClientSecret(c *gin.Context) {
	id := c.Param("id")
	plain := genClientSecret()
	hashed, err := hashClientSecret(plain)
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := store.DB().Model(&auth.OAuthClient{}).Where("id = ?", id).Update("secret", hashed).Error; err != nil {
		writeErr(c, err)
		return
	}
	recordAudit(c, "oauth_client_rotate", id, "")
	writeOK(c, gin.H{"data": gin.H{"secret": plain}})
}

func deleteOAuthClient(c *gin.Context) {
	id := c.Param("id")
	if err := store.DB().Delete(&auth.OAuthClient{}, "id = ?", id).Error; err != nil {
		writeErr(c, err)
		return
	}
	recordAudit(c, "oauth_client_delete", id, "")
	writeOK(c, nil)
}

// randSuffix 生成 8 字节随机后缀用于 client_id。
func randSuffix() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
