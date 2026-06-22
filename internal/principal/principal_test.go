package principal

import (
	"testing"

	apipb "github.com/CloudSilk/usercenter/proto"
)

func TestKinds(t *testing.T) {
	cases := []struct {
		name string
		p    Principal
		want Kind
	}{
		{"human", NewHuman("u1", "t1", nil), KindHuman},
		{"agent", NewAgent("a1", "u1", "t1", nil), KindAgent},
		{"service", NewService("s1", "t1", nil), KindService},
	}
	for _, c := range cases {
		if c.p.Kind() != c.want {
			t.Errorf("%s: %T.Kind() = %v, want %v", c.name, c.p, c.p.Kind(), c.want)
		}
	}
}

func TestAgentOwner(t *testing.T) {
	a := NewAgent("a1", "owner-u1", "t1", []string{"r1"})
	if a.OwnerUserID() != "owner-u1" {
		t.Fatalf("expected owner owner-u1, got %s", a.OwnerUserID())
	}
	// 委派链追溯:Agent 主体本身 Subject 是 agentID,Owner 才是背后的人
	if a.Subject() != "a1" {
		t.Fatalf("expected subject a1, got %s", a.Subject())
	}
}

func TestFromCurrentUser(t *testing.T) {
	u := &apipb.CurrentUser{Id: "u1", TenantID: "t1", RoleIDs: []string{"r1", "r2"}}
	p := FromTokenAndUser("", u)
	if p.Kind() != KindHuman {
		t.Fatalf("expected KindHuman, got %v", p.Kind())
	}
	if p.Subject() != "u1" || p.TenantID() != "t1" {
		t.Fatalf("unexpected subject/tenant: %s/%s", p.Subject(), p.TenantID())
	}
	if len(p.Roles()) != 2 || p.Roles()[0] != "r1" {
		t.Fatalf("unexpected roles: %v", p.Roles())
	}
}
