package token

import (
	"testing"
)

func TestAgentTokenRoundtrip(t *testing.T) {
	InitTokenCache("test-agent-secret", "", "", "", 120)

	// 签发 Agent token
	tokenStr, err := EncodeAgentPrincipal("agent-001", "owner-user-1", "tenant-1", []string{"role-a", "role-b"})
	if err != nil {
		t.Fatalf("EncodeAgentPrincipal: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("expected non-empty token")
	}

	// 解码验证主体类型
	currentUser, err := DecodeToken(tokenStr)
	if err != nil {
		t.Fatalf("DecodeToken: %v", err)
	}
	if !IsAgentToken(currentUser) {
		t.Fatalf("expected agent token (type=%d), got type=%d", PrincipalTypeAgent, currentUser.Type)
	}

	// 解析 Agent 身份
	ac, err := DecodeAgentPrincipal(tokenStr)
	if err != nil {
		t.Fatalf("DecodeAgentPrincipal: %v", err)
	}
	if ac.AgentID != "agent-001" {
		t.Fatalf("expected agentID=agent-001, got %s", ac.AgentID)
	}
	if ac.OwnerUserID != "owner-user-1" {
		t.Fatalf("expected ownerUserID=owner-user-1, got %s", ac.OwnerUserID)
	}
	if ac.TenantID != "tenant-1" {
		t.Fatalf("expected tenantID=tenant-1, got %s", ac.TenantID)
	}
	if len(ac.RoleIDs) != 2 || ac.RoleIDs[0] != "role-a" {
		t.Fatalf("expected roleIDs=[role-a,role-b], got %v", ac.RoleIDs)
	}
}

func TestHumanTokenIsNotAgent(t *testing.T) {
	// 人类 token type=0(默认),不应被识别为 Agent
	// (EncodeToken 默认 type 来自 user.Type,通常为 0)
	// IsAgentToken(nil) 不应 panic
	if IsAgentToken(nil) {
		t.Fatal("nil should not be agent")
	}
}
