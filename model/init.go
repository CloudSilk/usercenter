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

var DefaultPwd = "ABC123def"

func SetDefaultPwd(defaultPwd string) {
	if defaultPwd != "" {
		DefaultPwd = defaultPwd
	}
}
