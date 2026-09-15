package session

import (
	"testing"

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
