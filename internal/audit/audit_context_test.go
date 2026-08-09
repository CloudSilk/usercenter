package audit

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRecordAuditWithContextPersistsTenantAndRequestID(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&AuditLog{}); err != nil {
		t.Fatalf("migrate audit log: %v", err)
	}

	RecordAuditWithContext(
		database,
		"tenant-a",
		"request-a",
		"user-a",
		"User A",
		2,
		"update_order",
		"order-a",
		"127.0.0.1",
		"status=RELEASED",
	)

	var record AuditLog
	if err := database.First(&record).Error; err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if record.TenantID != "tenant-a" {
		t.Fatalf("tenant ID = %q, want tenant-a", record.TenantID)
	}
	if record.RequestID != "request-a" {
		t.Fatalf("request ID = %q, want request-a", record.RequestID)
	}
}
