package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"dubbo.apache.org/dubbo-go/v3/config"
	_ "dubbo.apache.org/dubbo-go/v3/imports"
	"github.com/CloudSilk/pkg/constants"
	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/pkg/db/mysql"
	"github.com/CloudSilk/pkg/db/sqlite"
	"github.com/CloudSilk/pkg/utils"
	ucconfig "github.com/CloudSilk/usercenter/config"
	"github.com/CloudSilk/usercenter/docs"
	"github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/model"
	"github.com/CloudSilk/usercenter/model/token"
	"github.com/CloudSilk/usercenter/provider"
	"github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
)

// gin-swagger middleware
// swagger embed files
func main() {
	config.SetProviderService(&provider.UserProvider{})
	config.SetProviderService(&provider.TenantProvider{})
	config.SetProviderService(&provider.RoleProvider{})
	config.SetProviderService(&provider.MenuProvider{})
	config.SetProviderService(&provider.APIProvider{})
	config.SetProviderService(&provider.IdentityProvider{})
	config.SetProviderService(&provider.FormComponentProvider{})
	config.SetProviderService(&provider.ProjectProvider{})
	config.SetProviderService(&provider.DictionariesProvider{})
	config.SetProviderService(&provider.LanguageProvider{})
	config.SetProviderService(&provider.SystemConfigProvider{})
	config.SetProviderService(&provider.WebSiteProvider{})
	config.SetProviderService(&provider.WechatConfigProvider{})
	config.SetProviderService(&provider.WechatProvider{})
	if err := config.Load(); err != nil {
		panic(err)
	}
	configCenter := config.GetRootConfig().ConfigCenter
	nacosAddr := configCenter.Address
	list := strings.Split(nacosAddr, ":")
	port, err := strconv.ParseUint(list[1], 10, 64)
	if err != nil {
		panic(err)
	}
	ucconfig.Init(configCenter.Namespace, list[0], port, configCenter.Username, configCenter.Password)

	var dbClient db.DBClientInterface
	if ucconfig.DefaultConfig.DBType == "sqlite" {
		dbClient = sqlite.NewSqlite2("", "", ucconfig.DefaultConfig.Sqlite, "usercenter", ucconfig.DefaultConfig.Debug)
	} else {
		dbClient = mysql.NewMysql(ucconfig.DefaultConfig.Mysql, ucconfig.DefaultConfig.Debug)
	}

	model.InitDB(dbClient, true)
	token.InitTokenCache(ucconfig.DefaultConfig.Token.Key, ucconfig.DefaultConfig.Token.RedisAddr, ucconfig.DefaultConfig.Token.RedisName, ucconfig.DefaultConfig.Token.RedisPwd, ucconfig.DefaultConfig.Token.Expired)
	constants.SetPlatformTenantID(ucconfig.DefaultConfig.PlatformTenantID)
	constants.SetSuperAdminRoleID(ucconfig.DefaultConfig.SuperAdminRoleID)
	constants.SetDefaultRoleID(ucconfig.DefaultConfig.DefaultRoleID)
	constants.SetEnabelTenant(ucconfig.DefaultConfig.EnableTenant)
	model.SetDefaultPwd(ucconfig.DefaultConfig.DefaultPwd)
	fmt.Println("started server")
	Start(GetPort("ATALI_PORT", 48080))
}

// 从环境变量中获取端口号
func GetPort(envName string, defaultPort int) int {
	port := defaultPort
	if os.Getenv(envName) != "" {
		var err error
		port, err = strconv.Atoi(os.Getenv(envName))
		if err != nil {
			fmt.Println("get port from env failed, err:", err)
			port = defaultPort
		}
	}
	return port
}

func Start(port int) {
	// programatically set swagger info
	docs.SwaggerInfo.Title = "UserCenter API"
	docs.SwaggerInfo.Description = "This is a UserCenter server."
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Schemes = []string{"http", "https"}

	r := gin.Default()
	r.Use(middleware.AuthRequired)
	r.Use(utils.Cors())
	http.RegisterAuthRouter(r)
	r.GET("/swagger/usercenter/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.Run(fmt.Sprintf(":%d", port))
}
