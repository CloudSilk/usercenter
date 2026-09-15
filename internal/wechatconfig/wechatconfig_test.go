package wechatconfig

import (
	"testing"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	glebsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	gdb, err := gorm.Open(glebsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := gdb.AutoMigrate(&WechatConfig{}); err != nil {
		panic(err)
	}
	store.SetDB(db.NewDBClient(gdb, false))
	m.Run()
}

// TestWechatConfigCRUD 覆盖创建/更新/删除/按ID查询全生命周期。
func TestWechatConfigCRUD(t *testing.T) {
	id, err := CreateWechatConfig(&WechatConfig{
		AppID: "wx-crud", AppName: "crud-app", DisplayName: "CRUD 应用",
		Secret: "sec", TenantID: "t-crud", AppType: 1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := GetWechatConfigByID(id)
	if err != nil || got.AppName != "crud-app" || got.Secret != "sec" {
		t.Fatalf("get by id: %+v err=%v", got, err)
	}

	// 更新
	got.DisplayName = "CRUD 应用(改)"
	if err := UpdateWechatConfig(got); err != nil {
		t.Fatalf("update: %v", err)
	}
	again, _ := GetWechatConfigByID(id)
	if again.DisplayName != "CRUD 应用(改)" {
		t.Fatalf("update not persisted: %+v", again)
	}

	// 删除后不可见
	if err := DeleteWechatConfig(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := GetWechatConfigByID(id); err == nil {
		t.Fatal("deleted config should be gone")
	}
}

// TestQueryWechatConfig 覆盖租户/类型/名称过滤与 IsMust 分支。
func TestQueryWechatConfig(t *testing.T) {
	seed := []*WechatConfig{
		{AppID: "wx-q1", AppName: "query-mini-1", TenantID: "t-q", AppType: 1, IsMust: true},
		{AppID: "wx-q2", AppName: "query-mini-2", TenantID: "t-q", AppType: 2},
		{AppID: "wx-q3", AppName: "query-web", TenantID: "t-q2", AppType: 4},
	}
	for _, s := range seed {
		if _, err := CreateWechatConfig(s); err != nil {
			t.Fatalf("seed %s: %v", s.AppName, err)
		}
	}
	t.Cleanup(func() {
		_ = store.DB().Where("app_name LIKE ?", "query-%").Delete(&WechatConfig{}).Error
	})

	cases := []struct {
		name    string
		req     func() *apipb.QueryWechatConfigRequest
		wantLen int
	}{
		{"by tenant", func() *apipb.QueryWechatConfigRequest {
			return &apipb.QueryWechatConfigRequest{TenantID: "t-q", PageSize: 10, PageIndex: 1}
		}, 2},
		{"by app type", func() *apipb.QueryWechatConfigRequest {
			return &apipb.QueryWechatConfigRequest{AppType: 4, PageSize: 10, PageIndex: 1}
		}, 1},
		{"by name", func() *apipb.QueryWechatConfigRequest {
			return &apipb.QueryWechatConfigRequest{AppName: "query-web", PageSize: 10, PageIndex: 1}
		}, 1},
	}
	for _, tc := range cases {
		resp := &apipb.QueryWechatConfigResponse{}
		QueryWechatConfig(tc.req(), resp, false)
		if int(resp.Records) != tc.wantLen {
			t.Fatalf("%s: expected %d, got %d", tc.name, tc.wantLen, resp.Records)
		}
	}
}
