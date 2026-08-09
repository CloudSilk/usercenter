package authn

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestValidatePrincipalStatus(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		principal principal.Principal
		prepare   func(*gorm.DB)
		wantCode  int
		wantErr   string
	}{
		{
			name:      "active human",
			principal: principal.NewHuman("user-1", "tenant-1", nil),
			wantCode:  model.Success,
		},
		{
			name:      "missing user",
			principal: principal.NewHuman("missing", "tenant-1", nil),
			wantCode:  model.TokenInvalid,
			wantErr:   "user does not exist",
		},
		{
			name:      "tenant mismatch",
			principal: principal.NewHuman("user-1", "tenant-2", nil),
			wantCode:  model.TokenInvalid,
			wantErr:   "user tenant does not match token",
		},
		{
			name:      "disabled user",
			principal: principal.NewHuman("user-1", "tenant-1", nil),
			prepare: func(database *gorm.DB) {
				database.Model(&user.User{}).Where("id = ?", "user-1").Update("enable", false)
			},
			wantCode: model.UserDisabled,
			wantErr:  "user is disabled",
		},
		{
			name:      "locked user",
			principal: principal.NewHuman("user-1", "tenant-1", nil),
			prepare: func(database *gorm.DB) {
				database.Model(&user.User{}).Where("id = ?", "user-1").Update("locked_expired", now.Add(time.Minute).Unix())
			},
			wantCode: model.UserDisabled,
			wantErr:  "user is locked",
		},
		{
			name:      "expired user",
			principal: principal.NewHuman("user-1", "tenant-1", nil),
			prepare: func(database *gorm.DB) {
				database.Model(&user.User{}).Where("id = ?", "user-1").Update("expired", now.Add(-time.Minute).Unix())
			},
			wantCode: model.UserDisabled,
			wantErr:  "user is expired",
		},
		{
			name:      "missing tenant",
			principal: principal.NewHuman("user-1", "tenant-1", nil),
			prepare: func(database *gorm.DB) {
				database.Where("id = ?", "tenant-1").Delete(&tenant.Tenant{})
			},
			wantCode: model.TokenInvalid,
			wantErr:  "tenant does not exist",
		},
		{
			name:      "disabled tenant",
			principal: principal.NewHuman("user-1", "tenant-1", nil),
			prepare: func(database *gorm.DB) {
				database.Model(&tenant.Tenant{}).Where("id = ?", "tenant-1").Update("enable", false)
			},
			wantCode: model.UserDisabled,
			wantErr:  "tenant is disabled",
		},
		{
			name:      "expired tenant",
			principal: principal.NewHuman("user-1", "tenant-1", nil),
			prepare: func(database *gorm.DB) {
				database.Model(&tenant.Tenant{}).Where("id = ?", "tenant-1").Update("expired", now.Add(-time.Minute))
			},
			wantCode: model.UserDisabled,
			wantErr:  "tenant is expired",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database := newStatusTestDB(t, strings.ReplaceAll(test.name, " ", "-"), now)
			if test.prepare != nil {
				test.prepare(database)
			}

			code, err := validatePrincipalStatus(test.principal, now)
			if code != test.wantCode {
				t.Fatalf("expected code %d, got %d (err=%v)", test.wantCode, code, err)
			}
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("expected error %q, got %v", test.wantErr, err)
			}
		})
	}
}

func TestValidatePrincipalStatusDoesNotTreatMachinesAsUsers(t *testing.T) {
	previous := store.Client()
	store.SetDB(nil)
	defer store.SetDB(previous)
	now := time.Now()
	for _, machine := range []principal.Principal{
		principal.NewAgent("agent-1", "owner-1", "tenant-1", nil),
		principal.NewService("service-1", "tenant-1", nil),
	} {
		if code, err := validatePrincipalStatus(machine, now); code != model.Success || err != nil {
			t.Fatalf("machine principal %v must bypass human status lookup: code=%d err=%v", machine.Kind(), code, err)
		}
	}
}

func newStatusTestDB(t *testing.T, name string, now time.Time) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:authn-status-%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&tenant.Tenant{}, &user.User{}); err != nil {
		t.Fatalf("migrate status models: %v", err)
	}
	store.SetDB(db.NewDBClient(database, false))

	if err := database.Create(&tenant.Tenant{
		Model:   model.Model{ID: "tenant-1"},
		Name:    "Tenant 1",
		Enable:  true,
		Expired: now.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := database.Create(&user.User{
		TenantModel: model.TenantModel{
			Model:    model.Model{ID: "user-1"},
			TenantID: "tenant-1",
		},
		UserName: "alice",
		Nickname: "Alice",
		Enable:   true,
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return database
}
