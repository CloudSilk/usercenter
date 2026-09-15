package audit

import (
	"testing"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	glebsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupQueryDB(t *testing.T) {
	t.Helper()
	gdb, err := gorm.Open(glebsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := gdb.AutoMigrate(&AuditLog{}); err != nil {
		t.Fatalf("migrate audit log: %v", err)
	}
	store.SetDB(db.NewDBClient(gdb, false))
	t.Cleanup(func() { store.SetDB(nil) })

	// 播种:两个租户、两类动作、两类主体
	rows := []AuditLog{
		{TenantID: "t1", RequestID: "req-1", UserID: "u1", UserName: "User1",
			PrincipalKind: 0, Action: "reset_password", TargetID: "target-1", IP: "1.1.1.1", Detail: "d1"},
		{TenantID: "t1", RequestID: "req-2", UserID: "u2", UserName: "User2",
			PrincipalKind: 2, Action: "pay_trade_bill_download", TargetID: "cfg-1", Detail: `{"billDate":"2026-09-12"}`},
		{TenantID: "t2", UserID: "u3", UserName: "User3",
			PrincipalKind: 1, Action: "reset_password", TargetID: "target-3"},
	}
	for i := range rows {
		if err := store.DB().Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}
}

func TestRecordAuditWithNilDBIsNoOp(t *testing.T) {
	RecordAuditWithContext(nil, "t", "r", "u", "n", 0, "a", "t", "ip", "d") // 不应 panic
	RecordAudit(nil, "u", "n", "a", "t", "ip", "d")
}

func TestRecordAuditConvenienceWrappers(t *testing.T) {
	setupQueryDB(t)
	defer store.SetDB(nil)

	RecordAudit(store.DB(), "u-a", "NameA", "action-a", "t-a", "1.2.3.4", "detail-a")
	RecordAuditWithKind(store.DB(), "u-svc", "Svc", 2, "action-svc", "t-svc", "", "detail-svc")

	var byKind0, byKind2 int64
	store.DB().Model(&AuditLog{}).Where("action = ?", "action-a").Count(&byKind0)
	store.DB().Model(&AuditLog{}).Where("action = ?", "action-svc").Count(&byKind2)
	if byKind0 != 1 || byKind2 != 1 {
		t.Fatalf("expected one row per action, got a=%d svc=%d", byKind0, byKind2)
	}
}

func TestQueryAuditLogsFiltersAndPaging(t *testing.T) {
	setupQueryDB(t)
	defer store.SetDB(nil)

	// 各过滤字段独立生效
	cases := []struct {
		name  string
		query AuditQuery
		want  int64
	}{
		{"tenant", AuditQuery{TenantID: "t1", PrincipalKind: -1}, 2},
		{"tenant t2", AuditQuery{TenantID: "t2", PrincipalKind: -1}, 1},
		{"request", AuditQuery{RequestID: "req-2", PrincipalKind: -1}, 1},
		{"user", AuditQuery{UserID: "u1", PrincipalKind: -1}, 1},
		{"action", AuditQuery{Action: "reset_password", PrincipalKind: -1}, 2},
		{"target", AuditQuery{TargetID: "target-1", PrincipalKind: -1}, 1},
		{"kind service", AuditQuery{PrincipalKind: 2}, 1},
		{"kind all", AuditQuery{PrincipalKind: -1}, 3},
		{"start past(all match)", AuditQuery{StartTime: 1, PrincipalKind: -1}, 3},
		{"end past", AuditQuery{EndTime: 1, PrincipalKind: -1}, 0},
	}
	for _, tc := range cases {
		list, total, err := QueryAuditLogs(&tc.query)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if total != tc.want || int64(len(list)) != tc.want {
			t.Fatalf("%s: expected %d, got total=%d list=%d", tc.name, tc.want, total, len(list))
		}
	}

	// 分页:pageSize=2 时第二页剩 1 条
	page1, total, err := QueryAuditLogs(&AuditQuery{PrincipalKind: -1, PageSize: 2, PageIndex: 1})
	if err != nil || total != 3 || len(page1) != 2 {
		t.Fatalf("page1: total=%d len=%d err=%v", total, len(page1), err)
	}
	page2, _, err := QueryAuditLogs(&AuditQuery{PrincipalKind: -1, PageSize: 2, PageIndex: 2})
	if err != nil || len(page2) != 1 {
		t.Fatalf("page2: len=%d err=%v", len(page2), err)
	}
	// 零值分页默认每页 10
	def, total, err := QueryAuditLogs(&AuditQuery{PrincipalKind: -1})
	if err != nil || total != 3 || len(def) != 3 {
		t.Fatalf("default paging: total=%d len=%d err=%v", total, len(def), err)
	}
}

func TestQueryAuditLogsOnEmptyDB(t *testing.T) {
	setupQueryDB(t)
	_ = store.DB().Where("1=1").Delete(&AuditLog{}).Error
	defer store.SetDB(nil)

	list, total, err := QueryAuditLogs(&AuditQuery{PrincipalKind: -1})
	if err != nil || total != 0 || len(list) != 0 {
		t.Fatalf("expected empty result, got total=%d len=%d err=%v", total, len(list), err)
	}
}
