package principal_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CloudSilk/usercenter/internal/principal"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

// 验证 Principal 注入 gin.Context 后，handler 能通过 GetPrincipal 获取身份
func TestPrincipalInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("Principal", principal.NewHuman("u1", "t1", []string{"admin"}))
		c.Set("User", &apipb.CurrentUser{Id: "u1", TenantID: "t1", UserName: "alice"})
		c.Next()
	})
	r.GET("/test", func(c *gin.Context) {
		p, ok := getPrincipalFromContext(c)
		if !ok {
			t.Error("expected Principal in context")
			c.JSON(500, "no principal")
			return
		}
		if p.Kind() != principal.KindHuman {
			t.Errorf("expected KindHuman, got %v", p.Kind())
		}
		if p.Subject() != "u1" {
			t.Errorf("expected subject u1, got %s", p.Subject())
		}
		c.JSON(200, gin.H{"subject": p.Subject(), "kind": p.Kind()})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// 验证 Agent Principal 能被正确识别(IsAgentRequest 场景)
func TestAgentPrincipalInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("Principal", principal.NewAgent("agent-1", "owner-1", "t1", []string{"ai-reader"}))
		c.Next()
	})
	r.GET("/agent-api", func(c *gin.Context) {
		p, ok := getPrincipalFromContext(c)
		if !ok || p.Kind() != principal.KindAgent {
			c.JSON(403, "not agent")
			return
		}
		c.JSON(200, gin.H{"agentID": p.Subject(), "owner": p.(*principal.AgentPrincipal).OwnerUserID()})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/agent-api", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("Agent should access agent-api, got %d", w.Code)
	}
}

// getPrincipalFromContext 模拟 middleware.GetPrincipal(避免 import 循环)
func getPrincipalFromContext(c *gin.Context) (principal.Principal, bool) {
	obj, exists := c.Get("Principal")
	if !exists {
		return nil, false
	}
	p, ok := obj.(principal.Principal)
	return p, ok
}
