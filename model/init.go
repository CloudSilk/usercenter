package model

import (
	"fmt"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/pkg/db/mysql"
	"github.com/CloudSilk/pkg/db/sqlite"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/usage"
)

var dbClient db.DBClientInterface

func Init(connStr string, debug bool) {
	dbClient = mysql.NewMysql(connStr, debug)
	store.SetDB(dbClient)
	initDB(debug)
}

func InitSqlite(database string, debug bool) {
	dbClient = sqlite.NewSqlite2("", "", database, "", debug)
	store.SetDB(dbClient)
	initDB(debug)
}

func InitDB(client db.DBClientInterface, debug bool) {
	dbClient = client
	store.SetDB(dbClient)
	initDB(debug)
}

func initDB(debug bool) {
	if debug {
		fmt.Println(AutoMigrate())
	}
	InitCasbin()
	updateNotCheckAuthRule()
	updateNotCheckLoginRule()
}

func AutoMigrate() error {
	return dbClient.DB().AutoMigrate(&CasbinRule{}, &API{}, &Menu{}, &MenuParameter{}, &MenuFunc{},
		&MenuFuncApi{}, &Role{}, &RoleMenu{}, &User{}, &UserRole{}, &UserWechatOpenIDMap{}, &APP{},
		&APPProp{}, &Tenant{}, &TenantMenu{}, &TenantCertificate{}, &FormComponent{}, &FormComponentResource{},
		&Project{}, &ProjectFormComponent{},
		&Dictionaries{}, &Language{}, &SystemConfig{}, &WebSite{}, &WechatConfig{},
		&AuditLog{},
		// REDESIGN 新增域表：AI Key/路由、用量计量、会话、ABAC、MFA、OAuth。
		// 此前这些表不在迁移清单内，全新部署的库访问对应功能会报 Table doesn't exist。
		&apikey.AIProvider{}, &apikey.AIKey{}, &apikey.ModelRoute{},
		&usage.UsageRecord{}, &usage.UsageBudget{},
		&session.Session{},
		&permission.ABACPolicy{},
		&auth.MFAFactor{}, &auth.RefreshToken{}, &auth.OAuthClient{}, &auth.ConsentRecord{},
	)
}
