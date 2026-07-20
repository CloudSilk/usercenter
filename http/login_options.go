package http

import (
	"net/http"
	"sort"

	"github.com/CloudSilk/usercenter/internal/user"
	"github.com/CloudSilk/usercenter/internal/wechatconfig"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

type loginWechatApp struct {
	AppName     string `json:"appName"`
	AppID       string `json:"appID"`
	DisplayName string `json:"displayName"`
	AppType     int32  `json:"appType,omitempty"`
}

type loginOptions struct {
	Phone struct {
		Enabled bool `json:"enabled"`
	} `json:"phone"`
	SocialProviders []string        `json:"socialProviders"`
	Wechat          loginWechatApps `json:"wechat"`
}

type loginWechatApps struct {
	MiniApps []loginWechatApp `json:"miniApps"`
	WebApps  []loginWechatApp `json:"webApps"`
}

// GetLoginOptions exposes only the non-secret login capabilities that a
// client can render before authentication.
func GetLoginOptions(c *gin.Context) {
	options := loginOptions{
		SocialProviders: GetSocialProviders(),
		Wechat: loginWechatApps{
			MiniApps: make([]loginWechatApp, 0),
			WebApps:  make([]loginWechatApp, 0),
		},
	}
	options.Phone.Enabled = user.PhoneAuthEnabled()
	sort.Strings(options.SocialProviders)

	configs, err := wechatconfig.GetAllWechatConfigs()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    apipb.Code_InternalServerError,
			"message": err.Error(),
		})
		return
	}
	for _, config := range configs {
		if config == nil || config.AppName == "" || config.AppID == "" {
			continue
		}
		item := loginWechatApp{
			AppName:     config.AppName,
			AppID:       config.AppID,
			DisplayName: config.DisplayName,
			AppType:     config.AppType,
		}
		switch config.AppType {
		case 1:
			item.AppType = 0
			options.Wechat.MiniApps = append(options.Wechat.MiniApps, item)
		case 2, 4:
			options.Wechat.WebApps = append(options.Wechat.WebApps, item)
		}
	}
	sort.Slice(options.Wechat.MiniApps, func(i, j int) bool {
		return options.Wechat.MiniApps[i].AppName < options.Wechat.MiniApps[j].AppName
	})
	sort.Slice(options.Wechat.WebApps, func(i, j int) bool {
		return options.Wechat.WebApps[i].AppName < options.Wechat.WebApps[j].AppName
	})

	c.JSON(http.StatusOK, gin.H{
		"code": apipb.Code_Success,
		"data": options,
	})
}

func RegisterLoginOptionsRouter(r *gin.Engine) {
	r.GET("/api/core/auth/login/options", GetLoginOptions)
}
