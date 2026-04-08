package wechat

import (
	"context"

	"github.com/CloudSilk/pkg/utils/log"
	ucmodel "github.com/CloudSilk/usercenter/model"
	wchat "github.com/silenceper/wechat/v2"
	"github.com/silenceper/wechat/v2/cache"
	"github.com/silenceper/wechat/v2/miniprogram"
	miniConfig "github.com/silenceper/wechat/v2/miniprogram/config"
)

var (
	miniPrograms = make(map[string]*MiniProgramConfig)
)

type MiniProgramConfig struct {
	MiniProgram   *miniprogram.MiniProgram
	MiniAppConfig *ucmodel.WechatConfig
}

func InitWechat() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf(context.Background(), "鍒濆鍖栧井淇￠厤缃烦杩?%v", r)
		}
	}()
	list, err := ucmodel.GetAllWechatConfigs()
	if err != nil {
		log.Errorf(context.Background(), "初始化微信配置失败:%v", err)
		return
	}
	for _, miniApp := range list {
		if miniApp.AppType == 1 {
			wc := wchat.NewWechat()
			miniPrograms[miniApp.AppName] = &MiniProgramConfig{
				MiniProgram: wc.GetMiniProgram(&miniConfig.Config{
					AppID:     miniApp.AppID,
					AppSecret: miniApp.Secret,
					Cache:     cache.NewMemory(),
				}),
				MiniAppConfig: miniApp,
			}
		} else if miniApp.AppType == 4 || miniApp.AppType == 2 {
			wechatOpenPlatformWebs[miniApp.AppName] = NewWechatOpenPlatformWeb(miniApp)
		}

	}
}

func GetMiniProgram(app string) *MiniProgramConfig {
	return miniPrograms[app]
}
