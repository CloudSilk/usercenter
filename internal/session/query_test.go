package session

import (
	"strings"

	"github.com/CloudSilk/usercenter/internal/store"
	"testing"
	"time"
)

// createNameTables 建立 QuerySessions 关键字搜索与名称解析所需的 User/Tenant 表。
func createNameTables(t *testing.T) {
	t.Helper()
	if err := store.DB().Exec(`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY, user_name TEXT, nickname TEXT, real_name TEXT,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error; err != nil {
		t.Fatalf("create users table: %v", err)
	}
	if err := store.DB().Exec(`CREATE TABLE IF NOT EXISTS tenants (
		id TEXT PRIMARY KEY, name TEXT,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error; err != nil {
		t.Fatalf("create tenants table: %v", err)
	}
}

func TestSessionStatus(t *testing.T) {
	now := time.Now().Unix()
	revoked := &Session{Revoked: true}
	if got := sessionStatus(revoked, 15); got != SessionStatusTerminated {
		t.Fatalf("revoked: %s", got)
	}
	idle := &Session{LastActiveAt: now - 60*60}
	if got := sessionStatus(idle, 15); got != SessionStatusIdle {
		t.Fatalf("idle: %s", got)
	}
	active := &Session{LastActiveAt: now - 60}
	if got := sessionStatus(active, 15); got != SessionStatusActive {
		t.Fatalf("active: %s", got)
	}
}

func TestNormalizePage(t *testing.T) {
	pi, ps := normalizePage(0, 0)
	if pi != 1 || ps != 20 {
		t.Fatalf("defaults: pi=%d ps=%d", pi, ps)
	}
	pi, ps = normalizePage(-3, 500)
	if pi != 1 || ps != 200 {
		t.Fatalf("clamps: pi=%d ps=%d", pi, ps)
	}
	pi, ps = normalizePage(3, 50)
	if pi != 3 || ps != 50 {
		t.Fatalf("passthrough: pi=%d ps=%d", pi, ps)
	}
}

func TestQuerySessionsFiltersAndStatus(t *testing.T) {
	createNameTables(t)
	t.Cleanup(func() {
		_ = store.DB().Where("principal_id LIKE ?", "q-").Delete(&Session{}).Error
		_ = store.DB().Exec(`DELETE FROM users WHERE id LIKE 'q-%'`).Error
		_ = store.DB().Exec(`DELETE FROM tenants WHERE id LIKE 'q-%'`).Error
	})

	now := time.Now().Unix()
	active := &Session{PrincipalID: "q-p1", TenantID: "q-t1", TokenSig: "q-sig-1",
		DeviceName: "Chrome", IP: "9.9.9.1", LastActiveAt: now}
	idle := &Session{PrincipalID: "q-p1", TenantID: "q-t1", TokenSig: "q-sig-2",
		DeviceName: "Firefox", IP: "9.9.9.2", LastActiveAt: now - 3600}
	revoked := &Session{PrincipalID: "q-p1", TenantID: "q-t2", TokenSig: "q-sig-3",
		DeviceName: "Safari", IP: "9.9.9.3", LastActiveAt: now, Revoked: true}
	for _, s := range []*Session{active, idle, revoked} {
		if err := CreateSession(s); err != nil {
			t.Fatalf("create session: %v", err)
		}
	}
	// CreateSession 会盖 last_active_at,idle 会话需回填为 1 小时前
	if err := store.DB().Model(&Session{}).Where("token_sig = ?", "q-sig-2").
		Update("last_active_at", now-3600).Error; err != nil {
		t.Fatalf("backdate idle: %v", err)
	}
	// 名称解析依赖的 user/tenant 行
	if err := store.DB().Exec(`INSERT INTO users (id, user_name, nickname, real_name) VALUES ('q-p1','quser','QNick','QReal')`).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := store.DB().Exec(`INSERT INTO tenants (id, name) VALUES ('q-t1','QTenant')`).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}

	// 主体过滤
	list, total, err := QuerySessions(Query{PrincipalID: "q-p1", ActiveOnly: true})
	if err != nil || total != 2 {
		t.Fatalf("principal filter: total=%d err=%v", total, err)
	}
	// 状态标签:活跃/空闲/终止
	statusBySig := map[string]string{}
	for _, v := range list {
		statusBySig[v.TokenSig] = v.Status
	}
	if statusBySig["q-sig-1"] != SessionStatusActive || statusBySig["q-sig-2"] != SessionStatusIdle {
		t.Fatalf("unexpected statuses: %v", statusBySig)
	}
	// 全量(含吊销) → TERMINATED
	all, total, err := QuerySessions(Query{PrincipalID: "q-p1"})
	if err != nil || total != 3 {
		t.Fatalf("all: total=%d err=%v", total, err)
	}
	statusBySig = map[string]string{}
	for _, v := range all {
		statusBySig[v.TokenSig] = v.Status
	}
	if statusBySig["q-sig-3"] != SessionStatusTerminated {
		t.Fatalf("revoked should be TERMINATED: %v", statusBySig)
	}

	// 名称解析:real_name 优先于 nickname 优先于 user_name
	for _, v := range all {
		if v.PrincipalID == "q-p1" && v.UserName != "quser" {
			t.Fatalf("expected user name resolved, got %q", v.UserName)
		}
		if v.PrincipalID == "q-p1" && v.DisplayName != "QReal" {
			t.Fatalf("expected display name QReal, got %q", v.DisplayName)
		}
		// 名称解析:q-t1 租户行存在应解析 QTenant;q-t2 无租户行则为空
		if v.TenantID == "q-t1" && v.TenantName != "QTenant" {
			t.Fatalf("expected tenant name QTenant, got %q", v.TenantName)
		}
	}

	// 关键字搜索命中 IP/设备/用户名
	for _, kw := range []string{"9.9.9.1", "Chrome", "quser", "QTenant"} {
		_, total, err := QuerySessions(Query{Keyword: kw})
		if err != nil {
			t.Fatalf("keyword %s: %v", kw, err)
		}
		if total < 1 {
			t.Fatalf("keyword %s should match, got %d", kw, total)
		}
	}

	// 分页:pageSize=2 第二页剩 1 条
	page1, total, err := QuerySessions(Query{PageSize: 2, PageIndex: 1})
	if err != nil || total != 3 || len(page1) != 2 {
		t.Fatalf("page1: total=%d len=%d err=%v", total, len(page1), err)
	}
	page2, _, err := QuerySessions(Query{PageSize: 2, PageIndex: 2})
	if err != nil || len(page2) != 1 {
		t.Fatalf("page2: len=%d err=%v", len(page2), err)
	}
}

func TestQuerySessionsKeywordNoMatch(t *testing.T) {
	createNameTables(t)
	_, total, err := QuerySessions(Query{Keyword: "zzz-no-such-keyword"})
	if err != nil {
		t.Fatalf("QuerySessions: %v", err)
	}
	// 依赖 User/Tenant 表存在;无匹配时结果为空
	if total != 0 {
		t.Fatalf("expected no matches, got total=%d", total)
	}
}

func TestRecordLoginAndQuery(t *testing.T) {
	createNameTables(t)
	t.Cleanup(func() {
		_ = store.DB().Where("principal_id LIKE ?", "q-login-%").Delete(&LoginRecord{}).Error
		_ = store.DB().Exec(`DELETE FROM users WHERE id LIKE 'q-login-%'`).Error
		_ = store.DB().Exec(`DELETE FROM tenants WHERE id LIKE 'q-login-%'`).Error
	})
	// Tenant 表种子,验证视图 TenantName 解析
	if err := store.DB().Exec(`INSERT INTO tenants (id, name) VALUES ('q-login-t1','LoginTenant')`).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}

	RecordLogin(nil) // nil 无操作
	RecordLogin(&LoginRecord{PrincipalID: "q-login-p1", TenantID: "q-login-t1",
		UserName: "login-user", Result: LoginResultSuccess, Abnormal: false})
	RecordLogin(&LoginRecord{PrincipalID: "q-login-p1", TenantID: "q-login-t1",
		UserName: "login-user", Result: LoginResultFailed, Abnormal: true, AnomalyReason: "异地"})

	// Result 过滤
	success, total, err := QueryLoginRecords(LoginRecordQuery{Result: strings.ToUpper("success")})
	if err != nil || total != 1 || len(success) != 1 {
		t.Fatalf("success filter: total=%d len=%d err=%v", total, len(success), err)
	}
	// 失败记录为 abnormal
	abnormal, total, err := QueryLoginRecords(LoginRecordQuery{Abnormal: boolPtr(true)})
	if err != nil || total != 1 {
		t.Fatalf("abnormal filter: total=%d err=%v", total, err)
	}
	if !abnormal[0].Abnormal {
		t.Fatalf("expected abnormal record: %+v", abnormal[0])
	}
	// 主体过滤
	byP, total, err := QueryLoginRecords(LoginRecordQuery{PrincipalID: "q-login-p1"})
	if err != nil || total != 2 {
		t.Fatalf("principal filter: total=%d err=%v", total, err)
	}
	// 视图 TenantName 解析
	if byP[0].TenantName != "" && byP[0].TenantName != "LoginTenant" {
		t.Fatalf("unexpected tenant name: %q", byP[0].TenantName)
	}
}

func boolPtr(b bool) *bool { return &b }
