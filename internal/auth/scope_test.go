package auth

import (
	"testing"
	"time"
)

func TestParseScope(t *testing.T) {
	scopes := ParseScope("read:order write:profile:*")
	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}
	if scopes[0].Action != "read" || scopes[0].Resource != "order" {
		t.Fatalf("unexpected first scope: %+v", scopes[0])
	}
	if scopes[1].Resource != "profile" || scopes[1].Constraint != "*" {
		t.Fatalf("unexpected second scope: %+v", scopes[1])
	}
}

func TestParseScope_Empty(t *testing.T) {
	scopes := ParseScope("")
	if len(scopes) != 0 {
		t.Fatalf("expected 0 scopes for empty input, got %d", len(scopes))
	}
}

func TestScopeMatches(t *testing.T) {
	sc := Scope{Action: "read", Resource: "order"}
	if !sc.Matches("read", "order") {
		t.Fatal("exact match should pass")
	}
	if sc.Matches("write", "order") {
		t.Fatal("wrong action should fail")
	}
}

func TestScopeMatches_Wildcard(t *testing.T) {
	sc := Scope{Action: "*", Resource: "*"}
	if !sc.Matches("read", "order") {
		t.Fatal("wildcard should match anything")
	}
	if !sc.Matches("delete", "user") {
		t.Fatal("wildcard should match anything")
	}
}

func TestScopeMatches_Admin(t *testing.T) {
	sc := Scope{Action: "admin", Resource: "order"}
	if !sc.Matches("read", "order") {
		t.Fatal("admin action should match any action")
	}
	if !sc.Matches("write", "order") {
		t.Fatal("admin action should match any action")
	}
}

func TestCheckScope(t *testing.T) {
	scopes := ParseScope("read:order write:profile")
	if !CheckScope(scopes, "read", "order") {
		t.Fatal("should allow read:order")
	}
	if !CheckScope(scopes, "write", "profile") {
		t.Fatal("should allow write:profile")
	}
	if CheckScope(scopes, "delete", "order") {
		t.Fatal("should deny delete:order (not in scope)")
	}
}

func TestCheckDelegation(t *testing.T) {
	chain := &DelegationChain{
		Links: []DelegationLink{
			{PrincipalID: "user-1", Kind: "human", Scope: "read:order write:order"},
			{PrincipalID: "agent-1", Kind: "agent", Scope: "read:order delete:order"},
		},
	}
	// read:order 在两跳都覆盖 → 授权
	if !CheckDelegation(chain, "read", "order") {
		t.Fatal("read:order should be allowed (covered by all links)")
	}
	// write:order 只在第一跳覆盖 → 拒绝（委派链取交集）
	if CheckDelegation(chain, "write", "order") {
		t.Fatal("write:order should be denied (not covered by all links)")
	}
	// delete:order 只在第二跳覆盖 → 拒绝
	if CheckDelegation(chain, "delete", "order") {
		t.Fatal("delete:order should be denied (not covered by all links)")
	}
}

func TestCheckDelegation_EmptyChain(t *testing.T) {
	if CheckDelegation(&DelegationChain{}, "read", "order") {
		t.Fatal("empty chain should deny")
	}
	if CheckDelegation(nil, "read", "order") {
		t.Fatal("nil chain should deny")
	}
}

func TestConsentRecord_TableName(t *testing.T) {
	if got := (ConsentRecord{}).TableName(); got != "oauth_consent" {
		t.Fatalf("expected table name 'oauth_consent', got %q", got)
	}
}

func TestDelegationLink_Kinds(t *testing.T) {
	link := DelegationLink{
		PrincipalID: "p1",
		Kind:        "human",
		Scope:       "read:order",
		GrantedAt:   time.Now(),
	}
	if link.Kind != "human" {
		t.Fatalf("expected kind 'human', got %s", link.Kind)
	}
}
