// Package tenant 实现租户的行级强制隔离。
//
// 对应 REDESIGN.md Batch 1(最高优先级安全修复):
//   - 现状缺陷:model/user.go:493 Login 查询不带 tenant_id,跨租户越权靠前端传参
//   - 目标:租户上下文从 token 注入到 DB 层,所有查询 GORM scope 强制追加
//     tenant_id = ctx.tenantID;超管/平台账号显式 bypass;写入同样
//   - 原则:默认安全,显式放行
//
// 当前为骨架包,租户逻辑(model/tenant)将在阶段0 迁入。
package tenant
