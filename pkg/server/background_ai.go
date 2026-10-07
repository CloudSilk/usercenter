package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/authn"
	"github.com/gin-gonic/gin"
)

// BackgroundAIPrincipalResolver obtains ownership from an authorized durable
// job in the host. Never resolve identity from caller-controlled HTTP headers.
type BackgroundAIPrincipalResolver func(context.Context) (userID, tenantID string)

// NewBackgroundAITransport embeds the same UserCenter model routing, encrypted
// keys, quotas and usage logging used by /v1. It buffers responses for background
// jobs and exposes only chat completions and model listing. It is not an HTTP
// authentication middleware and must never be mounted on a network listener.
// The host must initialize UserCenter and register its AI gateway before use.
func NewBackgroundAITransport(resolve BackgroundAIPrincipalResolver) http.RoundTripper {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if resolve == nil {
			c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"message": "background identity resolver is missing"}})
			return
		}
		userID, tenantID := resolve(c.Request.Context())
		p, current, err := authn.AuthorizeBackgroundUser(c.Request.Context(), userID, tenantID, c.Request.Method, c.Request.URL.Path)
		if err != nil {
			c.AbortWithStatusJSON(403, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		c.Set("Principal", p)
		c.Set("User", current)
	})
	r.POST("/v1/chat/completions", userhttp.ChatCompletions)
	r.GET("/v1/models", userhttp.ListModels)
	return &backgroundAITransport{handler: r}
}

type backgroundAITransport struct{ handler http.Handler }

func (t *backgroundAITransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("background AI request is nil")
	}
	if req.Body != nil {
		defer req.Body.Close()
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	// No general admin/business dispatch, even for a super-admin job owner.
	if req.URL.Path != "/v1/chat/completions" && req.URL.Path != "/v1/models" {
		return nil, fmt.Errorf("unsupported background AI endpoint")
	}
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, req)
	response := recorder.Result()
	response.Request = req
	return response, nil
}
