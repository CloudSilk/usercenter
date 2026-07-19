package http

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterSCIMRouterRequiresTokenAndMountsProvisioningRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	disabled := gin.New()
	RegisterSCIMRouter(disabled, "")
	require.Empty(t, disabled.Routes())

	enabled := gin.New()
	RegisterSCIMRouter(enabled, "test-scim-token")

	routes := enabled.Routes()
	require.NotEmpty(t, routes)
	for _, expected := range []struct {
		method string
		path   string
	}{
		{method: "GET", path: "/scim/v2/Users"},
		{method: "POST", path: "/scim/v2/Users"},
		{method: "PATCH", path: "/scim/v2/Users/:id"},
		{method: "GET", path: "/scim/v2/Groups"},
		{method: "POST", path: "/scim/v2/Groups"},
		{method: "PATCH", path: "/scim/v2/Groups/:id"},
		{method: "GET", path: "/scim/v2/ServiceProviderConfig"},
		{method: "GET", path: "/scim/v2/Schemas"},
	} {
		require.True(t, hasRoute(routes, expected.method, expected.path), "%s %s", expected.method, expected.path)
	}
}

func hasRoute(routes gin.RoutesInfo, method, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
