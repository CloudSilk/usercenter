package apikeyauth

import (
	"errors"
	"strings"
	"testing"

	pkgdb "github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCreateValidateAndDeleteKey(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open("file:api-key-lifecycle?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	store.SetDB(pkgdb.NewDBClient(db, false))
	if err := db.AutoMigrate(&APIKeyAuth{}); err != nil {
		t.Fatalf("migrate API key: %v", err)
	}

	key := &APIKeyAuth{
		TenantID:    "platform",
		PrincipalID: "runner-1",
		Name:        "Runner 1",
		Roles:       "ideasprint_runner",
	}
	plaintext, err := CreateKey(key)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if !strings.HasPrefix(plaintext, "uc_") || key.KeyHash == plaintext {
		t.Fatalf("plaintext/hash contract violated: %#v", key)
	}
	if key.KeyPrefix != plaintext[:prefixDisplay] {
		t.Fatalf("key prefix = %q, want %q", key.KeyPrefix, plaintext[:prefixDisplay])
	}

	validated, err := ValidateKey(plaintext)
	if err != nil {
		t.Fatalf("validate key: %v", err)
	}
	if validated.ID != key.ID || validated.PrincipalID != "runner-1" {
		t.Fatalf("validated key = %#v", validated)
	}

	if err := DeleteKey(key.ID); err != nil {
		t.Fatalf("delete key: %v", err)
	}
	if _, err := ValidateKey(plaintext); err == nil {
		t.Fatal("deleted key still validates")
	}
	if err := DeleteKey(key.ID); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("deleting an already deleted key error = %v, want ErrKeyNotFound", err)
	}
}
