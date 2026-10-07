package permission

import (
	"testing"

	"github.com/CloudSilk/pkg/constants"
	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
)

func TestConfiguredSuperAdminAndCachedPolicy(t *testing.T) {
	previousRole, previousEnforcer := constants.SuperAdminRoleID, enforcer
	t.Cleanup(func() { constants.SuperAdminRoleID, enforcer = previousRole, previousEnforcer; InvalidateAuthCache() })
	constants.SuperAdminRoleID = ""
	m, err := casbinmodel.NewModelFromString(rbacModel)
	if err != nil {
		t.Fatal(err)
	}
	enforcer, err = casbin.NewEnforcer(m)
	if err != nil {
		t.Fatal(err)
	}
	enforcer.AddFunction("ParamsMatch", ParamsMatchFunc)
	enforcer.AddFunction("IsSuperAdmin", isConfiguredSuperAdmin)
	if _, err := enforcer.AddPolicy("author", "/api/v1/*", "GET", "true"); err != nil {
		t.Fatal(err)
	}
	check := func(role, path, method string, want bool) {
		t.Helper()
		got, err := EnforceCached(role, path, method)
		if err != nil || got != want {
			t.Fatalf("role=%q path=%q method=%q got=%v want=%v err=%v", role, path, method, got, want, err)
		}
	}
	check("1", "/admin/api/ai-providers", "POST", true)
	check("super_admin", "/admin/api/ai-providers", "POST", false)
	// Initialize constants after enforcer construction; cached old decisions
	// must not preserve ID 1 privilege or deny the configured role.
	constants.SuperAdminRoleID = "super_admin"
	check("1", "/admin/api/ai-providers", "POST", false)
	check("super_admin", "/admin/api/ai-providers", "POST", true)
	check("author", "/api/v1/novels", "GET", true)
	check("author", "/admin/api/ai-providers", "POST", false)
	check("-1", "/api/v1/novels", "GET", false)
	check("0", "/api/v1/novels", "GET", false)
	constants.SuperAdminRoleID = "tenant-admin-uuid"
	check("super_admin", "/admin/api/ai-providers", "POST", false)
	check("tenant-admin-uuid", "/admin/api/ai-providers", "POST", true)
}
