package authn

import (
	"testing"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/principal"
	usersession "github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAuthenticateRequiredPrincipalPreservesAgentDelegation(t *testing.T) {
	token.InitTokenCache("authn-agent-secret", "", "", "", 120)
	encoded, err := token.EncodeAgentPrincipal("agent-1", "owner-1", "tenant-1", []string{"role-1"})
	if err != nil {
		t.Fatalf("encode agent token: %v", err)
	}
	database, err := gorm.Open(sqlite.Open("file:authn-agent-delegation?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open session database: %v", err)
	}
	if err := database.AutoMigrate(&usersession.Session{}); err != nil {
		t.Fatalf("migrate sessions: %v", err)
	}
	previous := store.Client()
	store.SetDB(db.NewDBClient(database, false))
	defer store.SetDB(previous)

	authenticated, _, code, err := AuthenticateRequiredPrincipal(encoded)
	if code != model.Success || err != nil {
		t.Fatalf("authenticate agent: code=%d err=%v", code, err)
	}
	agent, ok := authenticated.(*principal.AgentPrincipal)
	if !ok || agent.OwnerUserID() != "owner-1" || agent.Subject() != "agent-1" || agent.TenantID() != "tenant-1" {
		t.Fatalf("agent delegation was not preserved: %#v", authenticated)
	}
}
