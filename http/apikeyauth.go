package http

import (
	"net/http"
	"strings"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/apikeyauth"
	userm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
)

type apiKeyInfo struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenantID"`
	PrincipalID string `json:"principalID"`
	Name        string `json:"name"`
	KeyPrefix   string `json:"keyPrefix"`
	Roles       string `json:"roles"`
	Enable      bool   `json:"enable"`
	LastUsedAt  int64  `json:"lastUsedAt"`
}

func apiKeyToInfo(k *apikeyauth.APIKeyAuth) *apiKeyInfo {
	if k == nil {
		return nil
	}
	return &apiKeyInfo{
		ID:          k.ID,
		TenantID:    k.TenantID,
		PrincipalID: k.PrincipalID,
		Name:        k.Name,
		KeyPrefix:   k.KeyPrefix,
		Roles:       k.Roles,
		Enable:      k.Enable,
		LastUsedAt:  k.LastUsedAt,
	}
}

func registerAPIKeyAuthRoutes(g *gin.RouterGroup) {
	kg := g.Group("/api-keys")

	kg.GET("", func(c *gin.Context) {
		list, err := apikeyauth.GetKeys()
		if err != nil {
			writeErr(c, err)
			return
		}
		out := make([]*apiKeyInfo, 0, len(list))
		for i := range list {
			out = append(out, apiKeyToInfo(&list[i]))
		}
		c.JSON(http.StatusOK, gin.H{
			"code": model.Success,
			"data": out,
		})
	})

	kg.POST("", func(c *gin.Context) {
		var req struct {
			TenantID    string `json:"tenantID"`
			PrincipalID string `json:"principalID" binding:"required"`
			Name        string `json:"name" binding:"required"`
			Roles       string `json:"roles"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, &model.CommonResponse{
				Code:    model.BadRequest,
				Message: err.Error(),
			})
			return
		}
		if req.TenantID == "" {
			req.TenantID = userm.GetTenantID(c)
		}
		// Normalize roles: trim spaces, remove empty entries
		roleList := make([]string, 0)
		for _, r := range strings.Split(req.Roles, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				roleList = append(roleList, r)
			}
		}
		k := &apikeyauth.APIKeyAuth{
			TenantID:    req.TenantID,
			PrincipalID: req.PrincipalID,
			Name:        req.Name,
			Roles:       strings.Join(roleList, ","),
		}
		plaintext, err := apikeyauth.CreateKey(k)
		if err != nil {
			writeErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": model.Success,
			"data": gin.H{
				"id":        k.ID,
				"plaintext": plaintext,
			},
		})
	})

	kg.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := apikeyauth.DeleteKey(id); err != nil {
			writeErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": model.Success,
			"data": nil,
		})
	})
}
