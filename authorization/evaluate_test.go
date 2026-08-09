package authorization

import (
	"net/http"
	"testing"

	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/stretchr/testify/require"
)

func TestEvaluateRolesUsesPublicAuthenticatedAndRolePolicies(t *testing.T) {
	setupAuthorizationCatalogTestDB(t)
	permission.InitCasbin()

	catalog := testAuthorizationCatalog()
	catalog.APIs = append(catalog.APIs,
		API{ID: "api-public", Path: "/runtime/public", Method: http.MethodGet, CheckLogin: false},
		API{ID: "api-authenticated", Path: "/runtime/self", Method: http.MethodGet, CheckLogin: true},
	)
	_, err := Apply(catalog)
	require.NoError(t, err)

	publicDecision, err := EvaluateRoles(nil, "/runtime/public", http.MethodGet)
	require.NoError(t, err)
	require.True(t, publicDecision.Allow)
	require.Equal(t, "public_policy", publicDecision.Reason)

	authenticatedDecision, err := EvaluateRoles(nil, "/runtime/self", http.MethodGet)
	require.NoError(t, err)
	require.True(t, authenticatedDecision.Allow)
	require.Equal(t, "authenticated_policy", authenticatedDecision.Reason)

	roleDecision, err := EvaluateRoles([]string{"2"}, "/api/core/auth/user/query", http.MethodGet)
	require.NoError(t, err)
	require.True(t, roleDecision.Allow)
	require.Equal(t, []string{"2"}, roleDecision.MatchedRoleIDs)

	deniedDecision, err := EvaluateRoles([]string{"2"}, "/api/core/auth/user/query", http.MethodPost)
	require.NoError(t, err)
	require.False(t, deniedDecision.Allow)
	require.Equal(t, "no_matching_policy", deniedDecision.Reason)
}

func TestEvaluateRolesRejectsInvalidResources(t *testing.T) {
	for _, test := range []struct {
		path   string
		method string
	}{
		{path: "relative", method: http.MethodGet},
		{path: "/with?query=true", method: http.MethodGet},
		{path: "/resource", method: "TRACE"},
	} {
		_, err := EvaluateRoles(nil, test.path, test.method)
		require.Error(t, err)
	}
}
