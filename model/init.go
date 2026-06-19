package model

import (
	"fmt"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/pkg/db/mysql"
	"github.com/CloudSilk/pkg/db/sqlite"
	"github.com/CloudSilk/usercenter/internal/store"
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
	)
}
