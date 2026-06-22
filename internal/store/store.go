// Package store 提供全局共享的 DB 访问点,解耦 internal 领域包对 model 包
// 私有 dbClient 的依赖。
//
// 背景(REDESIGN §4 阶段0):model 包持有私有全局 dbClient,所有领域函数直接
// 调 dbClient.DB()。这阻碍领域逻辑迁入 internal/(internal 包不应反向依赖 model)。
// store 作为 internal 包共享的 DB 访问层,由 main.go 启动时注入。
//
// 使用:
//
//	main.go:
//	  store.SetDB(dbClient)        // 须在任何 internal 包使用前
//
//	internal 领域包:
//	  db := store.DB()             // 获取 *gorm.DB
//	  store.Client().PageQuery(...) // 需要 PageQuery 等高级方法时
package store

import (
	"github.com/CloudSilk/pkg/db"
	"gorm.io/gorm"
)

// dbClient 全局 DB 客户端,由 main.go 启动时注入
var dbClient db.DBClientInterface

// SetDB 注入 DB 客户端(main.go 启动时调用,须在任何 internal 包使用前)
func SetDB(client db.DBClientInterface) { dbClient = client }

// DB 返回 *gorm.DB;未初始化返回 nil(调用方应判空)
func DB() *gorm.DB {
	if dbClient == nil {
		return nil
	}
	return dbClient.DB()
}

// Client 返回原始 DBClientInterface(需要 PageQuery / CreateWithCheckDuplication
// 等高级方法时使用)
func Client() db.DBClientInterface { return dbClient }
