// Package web 通过 go:embed 将前端管理后台静态资源打包进二进制。
//
// 构建流程（React + Vite）：
//
//	cd web/admin-ui && npm install && npm run build
//
// go:embed 会将 admin-ui/dist 目录下所有文件递归嵌入，main.go 通过
// web.ReadFile("index.html") 等路径引用（已通过 fs.Sub 去掉前缀）。
package web

import (
	"embed"
	"io/fs"
)

//go:embed admin-ui/dist
var adminDist embed.FS

// ReadFile 从内嵌的 admin-ui/dist 中读取文件（路径不含 admin-ui/dist 前缀）。
func ReadFile(name string) ([]byte, error) {
	s, err := fs.Sub(adminDist, "admin-ui")
	if err != nil {
		return nil, err
	}
	sub, err := fs.Sub(s, "dist")
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(sub, name)
}

// DistFS 内嵌前端构建产物，对外路径以 "dist" 开头。
var DistFS fs.FS

func init() {
	s, err := fs.Sub(adminDist, "admin-ui")
	if err != nil {
		return
	}
	sub, err := fs.Sub(s, "dist")
	if err != nil {
		return
	}
	DistFS = sub
}
