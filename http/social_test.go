package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSocialState_SingleUse(t *testing.T) {
	c := newSocialStateCache()
	c.set("state-1", "https://app.front/cb")
	got, ok := c.take("state-1")
	if !ok || got != "https://app.front/cb" {
		t.Fatalf("首次 take 应返回 redirect，got %q ok=%v", got, ok)
	}
	// 重放应失败（CSRF 防护）
	if _, ok := c.take("state-1"); ok {
		t.Fatal("state 不应可重放")
	}
}

func TestSocialProviders_Map(t *testing.T) {
	g, ok := socialProviders["github"]
	if !ok {
		t.Fatal("缺 github provider")
	}
	sub, email, name, avatar := g.Map(map[string]interface{}{
		"id": 12345, "login": "alice", "name": "Alice", "avatar_url": "http://x/a.png",
	})
	if sub != "12345" || email != "" || name != "Alice" || avatar != "http://x/a.png" {
		t.Fatalf("github map 错误: sub=%s email=%s name=%s avatar=%s", sub, email, name, avatar)
	}
	// name 缺失时回退到 login
	_, _, name2, _ := g.Map(map[string]interface{}{"id": 1, "login": "bob"})
	if name2 != "bob" {
		t.Fatalf("name 应回退到 login，got %q", name2)
	}

	gg := socialProviders["google"]
	sub2, email2, name2b, av2 := gg.Map(map[string]interface{}{
		"id": "g-1", "email": "u@x.com", "name": "U", "picture": "http://p.png",
	})
	if sub2 != "g-1" || email2 != "u@x.com" || name2b != "U" || av2 != "http://p.png" {
		t.Fatalf("google map 错误: %+v", []string{sub2, email2, name2b, av2})
	}
}

func TestSocialLoginRedirect_ConfiguredVsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 未配置 → 400
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/github/login", nil)
	c.Params = gin.Params{{Key: "provider", Value: "github"}}
	socialLoginRedirect(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未配置应 400，got %d", w.Code)
	}

	// 配置后 → 302 到 github 授权 URL
	SetSocialLogins([]SocialLoginConfig{{Provider: "github", ClientID: "cid", ClientSecret: "sec", RedirectURI: "http://host/api/oauth/github/callback"}})
	defer SetSocialLogins(nil)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/github/login?redirect=https://app/cb", nil)
	c2.Params = gin.Params{{Key: "provider", Value: "github"}}
	socialLoginRedirect(c2)
	if w2.Code != http.StatusFound {
		t.Fatalf("配置后应 302，got %d", w2.Code)
	}
	loc := w2.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://github.com/login/oauth/authorize") ||
		!strings.Contains(loc, "client_id=cid") || !strings.Contains(loc, "state=") {
		t.Fatalf("重定向 URL 异常: %s", loc)
	}
}

func TestRandToken_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tk := randToken()
		if seen[tk] {
			t.Fatal("token 重复")
		}
		seen[tk] = true
	}
}
