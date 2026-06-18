package model

import (
	"fmt"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/pkg/db/mysql"
	"github.com/CloudSilk/pkg/db/sqlite"
)

var dbClient db.DBClientInterface

// Init Init
func Init(connStr string, debug bool) {
	dbClient = mysql.NewMysql(connStr, debug)
	initDB(debug)
}

func InitSqlite(database string, debug bool) {
	dbClient = sqlite.NewSqlite2("", "", database, "", debug)
	initDB(debug)
}

func InitDB(client db.DBClientInterface, debug bool) {
	dbClient = client
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
	for _, role := range roles {
		updateRoleAuth(role.ID)
	}
	updateNotCheckAuthRule()
	updateNotCheckLoginRule()
}

// AutoMigrate 自动生成表
func AutoMigrate() error {
	return dbClient.DB().AutoMigrate(&CasbinRule{}, &API{}, &Menu{}, &MenuParameter{}, &MenuFunc{},
		&MenuFuncApi{}, &Role{}, &RoleMenu{}, &User{}, &UserRole{}, &UserWechatOpenIDMap{}, &APP{},
		&APPProp{}, &Tenant{}, &TenantMenu{}, &TenantCertificate{}, &FormComponent{}, &FormComponentResource{},
		&Project{}, &ProjectFormComponent{},
		&Dictionaries{}, &Language{}, &SystemConfig{}, &WebSite{}, &WechatConfig{},
	)
}

var DefaultPwd = ""

// 登录失败锁定参数（默认值，可由 SetLoginLock 覆盖）
var (
	loginLockMaxErrCount int32 = 5
	loginLockLockMinutes int   = 15
)

func SetDefaultPwd(defaultPwd string) {
	if defaultPwd != "" {
		DefaultPwd = defaultPwd
	}
}

// SetLoginLock 设置登录失败锁定参数：最大连续失败次数与锁定时长（分钟）
func SetLoginLock(maxErrCount int, lockMinutes int) {
	if maxErrCount > 0 {
		loginLockMaxErrCount = int32(maxErrCount)
	}
	if lockMinutes > 0 {
		loginLockLockMinutes = lockMinutes
	}
}
