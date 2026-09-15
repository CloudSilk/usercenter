package session

import (
	"testing"
	"time"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	glebsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	gdb, err := gorm.Open(glebsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := gdb.AutoMigrate(&Session{}, &LoginRecord{}); err != nil {
		panic(err)
	}
	store.SetDB(db.NewDBClient(gdb, false))
	m.Run()
}

func mkSession(principalID, sig string, revoked bool) *Session {
	return &Session{
		PrincipalID:  principalID,
		TenantID:     "t1",
		TokenSig:     sig,
		DeviceType:   0,
		DeviceName:   "Chrome",
		IP:           "1.2.3.4",
		LastActiveAt: 100,
		Revoked:      revoked,
	}
}

func TestSessionLifecycle(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("principal_id LIKE ?", "sess-life-%").Delete(&Session{}).Error
	})

	// 创建
	s := mkSession("sess-life-p1", "sig-life-1", false)
	if err := CreateSession(s); err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.LastActiveAt == 0 {
		t.Fatal("CreateSession should stamp last_active_at")
	}
	// 活跃列表可见,全部列表可见
	active, err := ListSessions("sess-life-p1")
	if err != nil || len(active) != 1 {
		t.Fatalf("active list: %v %d", err, len(active))
	}
	all, _ := ListAllSessions("sess-life-p1")
	if len(all) != 1 {
		t.Fatalf("all list: %d", len(all))
	}

	// 吊销(管理员,按 session ID)
	if err := RevokeSession(s.ID, "管理员吊销"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	revoked, _ := ListSessions("sess-life-p1")
	if len(revoked) != 0 {
		t.Fatalf("revoked session should leave active list, got %d", len(revoked))
	}
	all, _ = ListAllSessions("sess-life-p1")
	if len(all) != 1 || !all[0].Revoked || all[0].RevokedReason != "管理员吊销" {
		t.Fatalf("unexpected all list: %+v", all)
	}
}

func TestRevokeSessionForPrincipal(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("principal_id LIKE ?", "sess-self-%").Delete(&Session{}).Error
	})
	mine := mkSession("sess-self-p1", "sig-self-1", false)
	if err := CreateSession(mine); err != nil {
		t.Fatalf("create mine: %v", err)
	}
	theirs := mkSession("sess-self-p2", "sig-self-2", false)
	theirs.PrincipalID = "sess-self-p2"
	if err := CreateSession(theirs); err != nil {
		t.Fatalf("create theirs: %v", err)
	}

	// 主体不匹配:不吊销
	ok, err := RevokeSessionForPrincipal(mine.ID, "sess-self-p2", "x")
	if err != nil || ok {
		t.Fatalf("foreign principal should not revoke: ok=%v err=%v", ok, err)
	}
	// 匹配主体:吊销成功
	ok, err = RevokeSessionForPrincipal(mine.ID, "sess-self-p1", "自助下线")
	if err != nil || !ok {
		t.Fatalf("self revoke: ok=%v err=%v", ok, err)
	}
	got, _ := ListAllSessions("sess-self-p1")
	if len(got) != 1 || !got[0].Revoked {
		t.Fatalf("unexpected session: %+v", got)
	}
	_ = theirs
}

func TestRevokeByTokenSig(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("principal_id LIKE ?", "sess-sig-%").Delete(&Session{}).Error
	})
	s := mkSession("sess-sig-p", "sig-by-token", false)
	if err := CreateSession(s); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := RevokeByTokenSig("sig-by-token", "logout"); err != nil {
		t.Fatalf("revoke by sig: %v", err)
	}
	all, _ := ListAllSessions("sess-sig-p")
	if len(all) != 1 || !all[0].Revoked || all[0].RevokedReason != "logout" {
		t.Fatalf("unexpected session: %+v", all)
	}
}

func TestRotateTokenSignature(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("principal_id LIKE ?", "sess-rot-%").Delete(&Session{}).Error
	})
	s := mkSession("sess-rot-p", "sig-rot-old", false)
	if err := CreateSession(s); err != nil {
		t.Fatalf("create: %v", err)
	}

	// 主体/旧签名匹配:轮换成功
	ok, err := RotateTokenSignature(s.ID, "sess-rot-p", "sig-rot-old", "sig-rot-new")
	if err != nil || !ok {
		t.Fatalf("rotate: ok=%v err=%v", ok, err)
	}
	got, _ := ListAllSessions("sess-rot-p")
	if got[0].TokenSig != "sig-rot-new" {
		t.Fatalf("token sig not rotated: %+v", got[0])
	}

	// 旧签名已失效:再次轮换同签名失败
	ok, err = RotateTokenSignature(s.ID, "sess-rot-p", "sig-rot-old", "sig-rot-new2")
	if err != nil || ok {
		t.Fatalf("stale rotation should fail: ok=%v err=%v", ok, err)
	}
}

func TestCreateSessionWithoutStore(t *testing.T) {
	// 临时清空 store,验证显式错误而非 panic
	original := store.DB()
	store.SetDB(nil)
	defer func() {
		if original != nil {
			store.SetDB(db.NewDBClient(original, false))
		}
	}()

	err := CreateSession(mkSession("p", "sig", false))
	if err == nil || err.Error() != "session store is not initialized" {
		t.Fatalf("expected store init error, got %v", err)
	}
}

// --- 全量视图/吊销全部/活跃刷新/过期清理/异地检测 ---

func TestRevokeAllByPrincipal(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("principal_id LIKE ?", "revoke-all-%").Delete(&Session{}).Error
	})
	p := "revoke-all-p"
	keep := mkSession(p, "revoke-all-keep", false)
	other := mkSession(p, "revoke-all-other", false)
	if err := CreateSession(keep); err != nil {
		t.Fatalf("create keep: %v", err)
	}
	if err := CreateSession(other); err != nil {
		t.Fatalf("create other: %v", err)
	}

	// exceptSessionID 保护当前会话不被吊销
	count, err := RevokeAllByPrincipal(p, keep.ID, "all devices 登出")
	if err != nil || count != 1 {
		t.Fatalf("expected 1 revoked (other), got %d err=%v", count, err)
	}
	got, _ := ListAllSessions(p)
	bySig := map[string]*Session{}
	for _, s := range got {
		bySig[s.TokenSig] = s
	}
	if !bySig["revoke-all-other"].Revoked {
		t.Fatal("other session should be revoked")
	}
	if bySig["revoke-all-keep"].Revoked {
		t.Fatal("kept session should stay active")
	}
}

func TestUpdateActivityAndIsRevoked(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("principal_id LIKE ?", "ua-%").Delete(&Session{}).Error
	})
	old := time.Now().Add(-time.Hour).Unix()
	s := &Session{PrincipalID: "ua-p", TokenSig: "ua-sig-1", LastActiveAt: old}
	if err := CreateSession(s); err != nil {
		t.Fatalf("create: %v", err)
	}
	// 回填旧活跃时间
	if err := store.DB().Model(&Session{}).Where("id = ?", s.ID).
		Update("last_active_at", old).Error; err != nil {
		t.Fatalf("backdate: %v", err)
	}

	UpdateActivity("ua-sig-1")
	var got Session
	if err := store.DB().First(&got, "id = ?", s.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.LastActiveAt <= old {
		t.Fatalf("activity should be refreshed: %d <= %d", got.LastActiveAt, old)
	}

	// IsRevoked:活跃为 false,吊销后为 true
	if IsRevoked("ua-sig-1") {
		t.Fatal("active session should not be revoked")
	}
	if err := store.DB().Model(&Session{}).Where("id = ?", s.ID).
		Update("revoked", true).Error; err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if !IsRevoked("ua-sig-1") {
		t.Fatal("revoked session should report true")
	}
	// 未知签名
	if IsRevoked("no-such-sig") {
		t.Fatal("unknown sig should not be revoked")
	}
}

func TestCleanExpiredSessions(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("principal_id LIKE ?", "clean-%").Delete(&Session{}).Error
	})
	// 旧会话(3 小时前活跃)
	stale := mkSession("clean-p", "clean-stale", false)
	if err := CreateSession(stale); err != nil {
		t.Fatalf("create stale: %v", err)
	}
	if err := store.DB().Model(&Session{}).Where("id = ?", stale.ID).
		Update("last_active_at", time.Now().Add(-4*time.Hour).Unix()).Error; err != nil {
		t.Fatalf("backdate: %v", err)
	}
	// 新会话(刚刚活跃)
	fresh := mkSession("clean-p", "clean-fresh", false)
	if err := CreateSession(fresh); err != nil {
		t.Fatalf("create fresh: %v", err)
	}

	count, err := CleanExpiredSessions(2) // 清理 2 小时前的
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 cleaned, got %d", count)
	}
	var freshCount int64
	store.DB().Model(&Session{}).Where("token_sig = ?", fresh.TokenSig).Count(&freshCount)
	if freshCount != 1 {
		t.Fatal("fresh session should remain")
	}
}

func TestDetectAnomaly(t *testing.T) {
	t.Cleanup(func() {
		_ = store.DB().Where("principal_id LIKE ?", "anomaly-%").Delete(&Session{}).Error
	})
	now := time.Now().Unix()
	seed := func(sig, ip string, lastActive int64) {
		t.Helper()
		s := &Session{PrincipalID: "anomaly-p", TokenSig: sig, IP: ip,
			LastActiveAt: lastActive, TenantID: "t"}
		if err := store.DB().Create(s).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// 三个会话 IP 依次变化,活跃时间递减:最近一次为 9.9.9.3
	seed("anomaly-1", "1.1.1.1", now-3600)
	seed("anomaly-2", "2.2.2.2", now-150)
	seed("anomaly-3", "3.3.3.3", now-90)

	// 同 IP(与最近一次一致):无异常
	anomaly, lastIP := DetectAnomaly("anomaly-p", "3.3.3.3", 60)
	if anomaly || lastIP != "3.3.3.3" {
		t.Fatalf("same IP should not anomaly: anomaly=%v lastIP=%q", anomaly, lastIP)
	}
	// 换 IP:窗口内有活跃会话 → 异常,返回上次 IP
	anomaly, lastIP = DetectAnomaly("anomaly-p", "9.9.9.9", 60)
	if !anomaly || lastIP != "3.3.3.3" {
		t.Fatalf("expected anomaly with lastIP 3.3.3.3, got anomaly=%v lastIP=%q", anomaly, lastIP)
	}
	// 窗口 1 分钟:会话在 90 秒前,窗口外 → 无异常
	anomaly, lastIP = DetectAnomaly("anomaly-p", "9.9.9.9", 1)
	if anomaly || lastIP != "" {
		t.Fatalf("expected no anomaly outside window, got anomaly=%v lastIP=%q", anomaly, lastIP)
	}
}

func TestRecordLoginNilAndStoreNil(t *testing.T) {
	RecordLogin(nil) // nil 记录无操作不 panic

	original := store.DB()
	store.SetDB(nil)
	RecordLogin(&LoginRecord{PrincipalID: "x"}) // store 未初始化无操作
	if original != nil {
		store.SetDB(db.NewDBClient(original, false))
	}
}
