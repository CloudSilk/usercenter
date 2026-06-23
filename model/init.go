package model

import (
	"fmt"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/pkg/db/mysql"
	"github.com/CloudSilk/pkg/db/sqlite"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/identity"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/pricing"
	"github.com/CloudSilk/usercenter/internal/prompt"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/usage"
)

var dbClient db.DBClientInterface

func Init(connStr string, runMigration bool) {
	dbClient = mysql.NewMysql(connStr, true)
	store.SetDB(dbClient)
	initDB(runMigration)
}

func InitSqlite(database string, runMigration bool) {
	dbClient = sqlite.NewSqlite2("", "", database, "", true)
	store.SetDB(dbClient)
	initDB(runMigration)
}

func InitDB(client db.DBClientInterface, runMigration bool) {
	dbClient = client
	store.SetDB(dbClient)
	initDB(runMigration)
}

func initDB(runMigration bool) {
	if runMigration {
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
		// REDESIGN 新增域表：AI Key/路由、用量计量、会话、ABAC、MFA、OAuth、Prompt 模板。
		// 此前这些表不在迁移清单内，全新部署的库访问对应功能会报 Table doesn't exist。
		&apikey.AIProvider{}, &apikey.AIKey{}, &apikey.ModelRoute{},
		&usage.UsageRecord{}, &usage.UsageBudget{},
		&session.Session{},
		&permission.ABACPolicy{},
		&auth.MFAFactor{}, &auth.RefreshToken{}, &auth.OAuthClient{}, &auth.ConsentRecord{},
		&prompt.PromptTemplate{},
		&identity.UserExternalIdentity{},
		&pricing.ModelPrice{},
	)
}
