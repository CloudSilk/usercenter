package model

import (
	"fmt"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/pkg/db/mysql"
	"github.com/CloudSilk/pkg/db/sqlite"
	"github.com/CloudSilk/usercenter/internal/store"
)

var dbClient db.DBClientInterface

// Init Init
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
	roles, err := GetAllRole("", true)
	if err != nil {
		panic(err)
	}
	_ = roles
	updateNotCheckAuthRule()
	updateNotCheckLoginRule()
}

// AutoMigrate migrates model tables.
func AutoMigrate() error {
	return dbClient.DB().AutoMigrate(&CasbinRule{}, &API{}, &Menu{}, &MenuParameter{}, &MenuFunc{},
		&MenuFuncApi{}, &Role{}, &RoleMenu{}, &User{}, &UserRole{}, &UserWechatOpenIDMap{}, &APP{},
		&APPProp{}, &Tenant{}, &TenantMenu{}, &TenantCertificate{}, &FormComponent{}, &FormComponentResource{},
		&Project{}, &ProjectFormComponent{},
		&Dictionaries{}, &Language{}, &SystemConfig{}, &WebSite{}, &WechatConfig{},
		&AuditLog{},
	)
}

var DefaultPwd = ""

var (
	loginLockMaxErrCount int32 = 5
	loginLockLockMinutes int   = 15
)

func SetDefaultPwd(defaultPwd string) {
	if defaultPwd != "" {
		DefaultPwd = defaultPwd
	}
}

func SetLoginLock(maxErrCount int, lockMinutes int) {
	if maxErrCount > 0 {
		loginLockMaxErrCount = int32(maxErrCount)
	}
	if lockMinutes > 0 {
		loginLockLockMinutes = lockMinutes
	}
}
