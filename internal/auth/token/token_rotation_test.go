package token

import (
	"testing"

	apipb "github.com/CloudSilk/usercenter/proto"
)

func TestRotateTokenReplacesActiveCacheEntry(t *testing.T) {
	InitTokenCache("test-token-rotation-secret", "", "", "", 120)
	current := &apipb.CurrentUser{
		Id: "rotation-user", TenantID: "rotation-tenant", SessionID: "rotation-session",
	}
	oldToken, err := EncodeToken(current)
	if err != nil {
		t.Fatalf("encode old token: %v", err)
	}

	newToken, err := RotateToken(current, oldToken)
	if err != nil {
		t.Fatalf("rotate token: %v", err)
	}
	if newToken == oldToken {
		t.Fatal("rotated token must have a unique signature")
	}
	if active, err := DefaultTokenCache.Exists(current.Id, oldToken); err != nil || active {
		t.Fatalf("old token must be inactive: active=%v err=%v", active, err)
	}
	if active, err := DefaultTokenCache.Exists(current.Id, newToken); err != nil || !active {
		t.Fatalf("new token must be active: active=%v err=%v", active, err)
	}
	decoded, err := DecodeToken(newToken)
	if err != nil || decoded.SessionID != current.SessionID {
		t.Fatalf("rotated token must preserve session: user=%#v err=%v", decoded, err)
	}
}
