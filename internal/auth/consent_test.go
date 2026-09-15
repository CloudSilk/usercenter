package auth

import (
	"os"
	"testing"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := gdb.AutoMigrate(&ConsentRecord{}); err != nil {
		panic(err)
	}
	store.SetDB(db.NewDBClient(gdb, false))
	code := m.Run()
	os.Exit(code)
}

// --- Consent 生命周期 ---

func TestGrantAndHasConsent(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("client_id = ?", "consent-agent").Delete(&ConsentRecord{}).Error
	})
	if err := GrantConsent("user-1", "agent-1", "read:order write:profile", 0); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if !HasConsent("user-1", "agent-1", "read:order") {
		t.Fatal("expected consent to be granted")
	}
	// 未授权的 scope
	if HasConsent("user-1", "agent-1", "delete:all") {
		t.Fatal("ungranted scope should not have consent")
	}
	// 重复 grant:旧记录吊销,新记录写入
	if err := GrantConsent("user-1", "agent-1", "read:order", 0); err != nil {
		t.Fatalf("re-grant: %v", err)
	}
	var count int64
	store.DB().Model(&ConsentRecord{}).
		Where("principal_id = ? AND client_id = ? AND revoked = ?", "user-1", "agent-1", false).
		Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 active consent after re-grant, got %d", count)
	}
}

func TestRevokeConsent(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("client_id = ?", "revoke-agent").Delete(&ConsentRecord{}).Error
	})
	if err := GrantConsent("user-2", "revoke-agent", "read:all", 0); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if !HasConsent("user-2", "revoke-agent", "read:all") {
		t.Fatal("consent should exist before revoke")
	}
	if err := RevokeConsent("user-2", "revoke-agent"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if HasConsent("user-2", "revoke-agent", "read:all") {
		t.Fatal("revoked consent should not have consent")
	}
	// 重复 revoke 幂等
	if err := RevokeConsent("user-2", "revoke-agent"); err != nil {
		t.Fatalf("repeated revoke: %v", err)
	}
}

func TestListConsents(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("client_id LIKE ?", "list-agent-%").Delete(&ConsentRecord{}).Error
	})
	if err := GrantConsent("user-list", "list-agent-1", "read:order", 0); err != nil {
		t.Fatalf("grant 1: %v", err)
	}
	if err := GrantConsent("user-list", "list-agent-2", "write:profile", 0); err != nil {
		t.Fatalf("grant 2: %v", err)
	}
	list, err := ListConsents("user-list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 consents, got %d", len(list))
	}
}

// --- CheckDelegation 委派链 ---

func TestCheckDelegationChain(t *testing.T) {
	// nil 链
	if CheckDelegation(nil, "read", "order") {
		t.Fatal("nil chain should deny")
	}
	// 空链
	empty := &DelegationChain{}
	if CheckDelegation(empty, "read", "order") {
		t.Fatal("empty chain should deny")
	}
	// 单跳
	chain := &DelegationChain{Links: []DelegationLink{
		{PrincipalID: "agent-1", Scope: "read:order"},
	}}
	if !CheckDelegation(chain, "read", "order") {
		t.Fatal("single hop should allow")
	}
	// 双跳:任一跳不覆盖则拒绝
	twoHop := &DelegationChain{Links: []DelegationLink{
		{PrincipalID: "hop-1", Scope: "read:order"},
		{PrincipalID: "hop-2", Scope: "write:profile"},
	}}
	if CheckDelegation(twoHop, "read", "order") {
		t.Fatal("second hop missing scope should deny")
	}
	// 双跳都覆盖
	goodChain := &DelegationChain{Links: []DelegationLink{
		{PrincipalID: "hop-1", Scope: "read:order write:profile"},
		{PrincipalID: "hop-2", Scope: "read:order"},
	}}
	if !CheckDelegation(goodChain, "read", "order") {
		t.Fatal("full chain should allow")
	}
}
