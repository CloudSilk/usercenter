package wechat

import (
	"context"
	"sync"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/wechatconfig"
	wchat "github.com/silenceper/wechat/v2"
	"github.com/silenceper/wechat/v2/cache"
	"github.com/silenceper/wechat/v2/miniprogram"
	miniConfig "github.com/silenceper/wechat/v2/miniprogram/config"
)

var (
	miniPrograms = make(map[string]*MiniProgramConfig)
	miniMu       sync.RWMutex
)

type MiniProgramConfig struct {
	MiniProgram   *miniprogram.MiniProgram
	MiniAppConfig *wechatconfig.WechatConfig
}

func InitWechat() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf(context.Background(), "鍒濆鍖栧井淇￠厤缃烦杩?%v", r)
		}
	}()
	list, err := wechatconfig.GetAllWechatConfigs()
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
	miniMu.RLock()
	defer miniMu.RUnlock()
	return miniPrograms[app]
}

// ReloadMiniPrograms 热重载小程序配置：清空后重新从数据库装载。
// 供嵌入式宿主在配置增删改后调用（并发安全）。
func ReloadMiniPrograms() {
	miniMu.Lock()
	defer miniMu.Unlock()
	for k := range miniPrograms {
		delete(miniPrograms, k)
	}
	InitWechat()
}
