// Package http — 对宿主应用暴露 DB 注入入口。
//
// usercenter 的 internal/store 无法被外部仓库直接导入(internal 可见性),
// 宿主 main.go 须经由本包在 RegisterAuthRouter 之前完成 DB 注入,
// 否则权限生效路由(accesseffect.EnsureAPIResources)会因 nil 句柄 panic。
package http

import (
	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/store"
)

// SetDB 注入宿主应用的 DB 客户端(main.go 启动时调用,须在任何
// usercenter internal 包使用之前)。
func SetDB(client db.DBClientInterface) {
	store.SetDB(client)
}
