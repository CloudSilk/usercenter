package token

import (
	"testing"

	"github.com/CloudSilk/usercenter/internal/principal"
	apipb "github.com/CloudSilk/usercenter/proto"
)

// TestPrincipalShadowConsistency 验证 FromCurrentUser 提取的 Roles
// 与 CurrentUser.RoleIDs 一致(影子双跑的核心不变量)
func TestPrincipalShadowConsistency(t *testing.T) {
	cu := &apipb.CurrentUser{
		Id:       "u1",
		UserName: "alice",
		TenantID: "t1",
		RoleIDs:  []string{"role-a", "role-b", "role-c"},
	}

	p := principal.FromCurrentUser(cu)
	if p.Kind() != principal.KindHuman {
		t.Fatalf("expected KindHuman, got %v", p.Kind())
	}
	if p.Subject() != "u1" {
		t.Fatalf("expected subject u1, got %s", p.Subject())
	}
	if p.TenantID() != "t1" {
		t.Fatalf("expected tenant t1, got %s", p.TenantID())
	}
	if len(p.Roles()) != 3 {
		t.Fatalf("expected 3 roles, got %d", len(p.Roles()))
	}
	// 影子双跑不变量:Principal.Roles() == CurrentUser.RoleIDs
	for i, r := range cu.RoleIDs {
		if p.Roles()[i] != r {
			t.Fatalf("role mismatch at %d: principal=%s current=%s", i, p.Roles()[i], r)
		}
	}
}

// TestAgentPrincipalNotHuman 验证 Agent Principal 的 Kind != Human
func TestAgentPrincipalNotHuman(t *testing.T) {
	a := principal.NewAgent("agent-1", "owner-1", "t1", []string{"r1"})
	if a.Kind() == principal.KindHuman {
		t.Fatal("agent should not be KindHuman")
	}
	if a.Kind() != principal.KindAgent {
		t.Fatalf("expected KindAgent, got %v", a.Kind())
	}
	// Agent 的 OwnerUserID 用于委派链追溯
	if a.OwnerUserID() != "owner-1" {
		t.Fatalf("expected owner owner-1, got %s", a.OwnerUserID())
	}
}
