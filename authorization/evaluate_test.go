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

func TestEvaluateDataScopesReturnsPolicyUnionAndTenantFallback(t *testing.T) {
	gdb := setupAuthorizationCatalogTestDB(t)
	require.NoError(t, gdb.Create(&[]permission.ABACPolicy{
		{
			TenantID: "tenant-a", RoleID: "role-project", Resource: "/api/v1/work-orders/:id",
			Action: http.MethodGet, DataScope: DataScopeProject, Priority: 200, Enable: true,
		},
		{
			TenantID: "tenant-a", RoleID: "role-self", Resource: "/api/v1/work-orders/:id",
			Action: http.MethodGet, DataScope: DataScopeSelf, Priority: 100, Enable: true,
		},
		{
			TenantID: "tenant-b", RoleID: "role-project", Resource: "/api/v1/work-orders/:id",
			Action: http.MethodGet, DataScope: DataScopeAll, Priority: 999, Enable: true,
		},
	}).Error)

	decision, err := EvaluateDataScopes(
		[]string{"role-self", "role-missing", "role-project", "role-self"},
		"tenant-a",
		"/api/v1/work-orders/:id",
		http.MethodGet,
	)
	require.NoError(t, err)
	require.Equal(t, []DataScopeRule{
		{RoleID: "role-missing", DataScope: DataScopeTenant, Source: "tenant_fallback"},
		{RoleID: "role-project", DataScope: DataScopeProject, Priority: 200, Source: "abac_policy"},
		{RoleID: "role-self", DataScope: DataScopeSelf, Priority: 100, Source: "abac_policy"},
	}, decision.Rules)

	fallback, err := EvaluateDataScopes(nil, "tenant-a", "/api/v1/work-orders", http.MethodGet)
	require.NoError(t, err)
	require.Equal(t, tenantFallback(""), fallback)

	superAdmin, err := EvaluateDataScopes([]string{"1"}, "tenant-a", "/api/v1/work-orders", http.MethodDelete)
	require.NoError(t, err)
	require.Equal(t, DataScopeAll, superAdmin.Rules[0].DataScope)
}

func TestEvaluateDataScopesUsesGlobalPublicRolePolicy(t *testing.T) {
	gdb := setupAuthorizationCatalogTestDB(t)
	require.NoError(t, gdb.Create(&permission.ABACPolicy{
		TenantID: "*", RoleID: "public-employee", Resource: "/api/v1/self",
		Action: http.MethodGet, DataScope: DataScopeSelf, Priority: 100, Enable: true,
	}).Error)

	decision, err := EvaluateDataScopes([]string{"public-employee"}, "tenant-a", "/api/v1/self", http.MethodGet)
	require.NoError(t, err)
	require.Equal(t, []DataScopeRule{{
		RoleID: "public-employee", DataScope: DataScopeSelf, Priority: 100, Source: "abac_policy",
	}}, decision.Rules)
}

func TestEvaluateDataScopesRejectsInvalidStoredScope(t *testing.T) {
	gdb := setupAuthorizationCatalogTestDB(t)
	require.NoError(t, gdb.Create(&permission.ABACPolicy{
		TenantID: "tenant-a", RoleID: "role-invalid", Resource: "/api/v1/assets", Action: http.MethodGet,
		DataScope: 99, Priority: 100, Enable: true,
	}).Error)

	_, err := EvaluateDataScopes([]string{"role-invalid"}, "tenant-a", "/api/v1/assets", http.MethodGet)
	require.ErrorContains(t, err, "data scope 99 is invalid")
}
