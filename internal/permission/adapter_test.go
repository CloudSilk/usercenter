package permission

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func newTestAdapter(t *testing.T) *Adapter {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	if err := gdb.AutoMigrate(&CasbinRule{}); err != nil {
		t.Fatalf("migrate casbin_rule: %v", err)
	}
	a, err := NewAdapterByDBWithCustomTable(gdb, &CasbinRule{})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	return a
}

func TestAdapter_AddAndLoadPolicy(t *testing.T) {
	a := newTestAdapter(t)
	if err := a.AddPolicy("p", "p", []string{"role1", "/api/foo", "GET", "allow"}); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}
	if err := a.AddPolicy("p", "p", []string{"role1", "/api/bar", "POST", "deny"}); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}

	var rules []CasbinRule
	a.db.Find(&rules)
	assert.Len(t, rules, 2)
}

func TestAdapter_RemovePolicy(t *testing.T) {
	a := newTestAdapter(t)
	_ = a.AddPolicy("p", "p", []string{"role1", "/api/foo", "GET", "allow"})
	_ = a.AddPolicy("p", "p", []string{"role1", "/api/bar", "POST", "deny"})

	if err := a.RemovePolicy("p", "p", []string{"role1", "/api/foo", "GET", "allow"}); err != nil {
		t.Fatalf("RemovePolicy: %v", err)
	}

	var rules []CasbinRule
	a.db.Find(&rules)
	assert.Len(t, rules, 1)
	if rules[0].Path != "/api/bar" {
		t.Fatalf("expected remaining rule for /api/bar, got %s", rules[0].Path)
	}
}

func TestAdapter_RemoveFilteredPolicy(t *testing.T) {
	a := newTestAdapter(t)
	_ = a.AddPolicy("p", "p", []string{"role1", "/api/foo", "GET", "allow"})
	_ = a.AddPolicy("p", "p", []string{"role1", "/api/bar", "POST", "deny"})
	_ = a.AddPolicy("p", "p", []string{"role2", "/api/foo", "GET", "allow"})

	if err := a.RemoveFilteredPolicy("p", "p", 0, "role1"); err != nil {
		t.Fatalf("RemoveFilteredPolicy: %v", err)
	}

	var rules []CasbinRule
	a.db.Find(&rules)
	assert.Len(t, rules, 1)
	if rules[0].RoleID != "role2" {
		t.Fatalf("expected remaining rule for role2, got %s", rules[0].RoleID)
	}
}

func TestAdapter_LoadPolicy(t *testing.T) {
	a := newTestAdapter(t)
	_ = a.AddPolicy("p", "p", []string{"r1", "/a", "GET", "allow"})

	var rules []CasbinRule
	a.db.Find(&rules)
	assert.Len(t, rules, 1)
	if rules[0].RoleID != "r1" || rules[0].Path != "/a" || rules[0].Method != "GET" {
		t.Fatalf("rule mismatch: %+v", rules[0])
	}
}

func TestAdapter_NewAdapterByDBWithCustomTable_EmptyTable(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = gdb.AutoMigrate(&CasbinRule{})
	a, err := NewAdapterByDBWithCustomTable(gdb, &CasbinRule{})
	assert.NoError(t, err)
	assert.NotNil(t, a)
}
