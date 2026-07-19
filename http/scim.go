package http

import (
	"strings"

	"github.com/CloudSilk/usercenter/internal/scim"
	"github.com/gin-gonic/gin"
)

// RegisterSCIMRouter mounts the SCIM 2.0 provisioning endpoints for embedded
// consumers. An empty token keeps the endpoints disabled.
func RegisterSCIMRouter(r *gin.Engine, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	scim.RegisterSCIMRouter(r, token)
}
