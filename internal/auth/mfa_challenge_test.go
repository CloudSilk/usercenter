package auth

import (
	"testing"

	"github.com/CloudSilk/usercenter/internal/store"
)

// TestMFAChallengeGuardsWithoutDB 验证 store.DB() 未初始化时,
// MFA 因子查询与校验安全降级为 false(不 panic)。
func TestMFAChallengeGuardsWithoutDB(t *testing.T) {
	store.SetDB(nil)
	t.Cleanup(func() { store.SetDB(nil) })

	if HasEnabledMFA("p-any") {
		t.Fatal("HasEnabledMFA should be false without DB")
	}
	if VerifyMFACode("p-any", "123456") {
		t.Fatal("VerifyMFACode should be false without DB")
	}
	// challenge 令牌是内存缓存,不依赖 DB,仍可正常签发与消费
	tok, err := IssueMFAChallenge("p-any")
	if err != nil {
		t.Fatalf("issue challenge: %v", err)
	}
	if id, ok := ConsumeMFAChallenge(tok); !ok || id != "p-any" {
		t.Fatalf("consume without DB: id=%q ok=%v", id, ok)
	}
}
