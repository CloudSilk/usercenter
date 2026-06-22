// Package web 通过 go:embed 将前端管理后台静态资源打包进二进制。
//
// 目前仅内嵌单页 admin.html（Vue 3 + Element Plus CDN 实现），由 main.go
// 挂载到 /web/admin 路由对外提供。新增静态资源时在此处扩展 embed 指令与
// 对应的 http.Handler。
package web

import _ "embed"

// AdminHTML 内嵌管理后台单页应用。
//go:embed admin.html
var AdminHTML []byte

// AdminFileName 为对外暴露的文件名，供 main.go 组装响应头用。
const AdminFileName = "admin.html"
