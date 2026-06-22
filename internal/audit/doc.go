// Package audit 实现结构化全量审计。
//
// 对应 REDESIGN.md Batch 1(最高优先级安全):
//   - 现状缺陷:model/audit.go 只有 5 个常量 + 零散埋点,Detail 字段 size:1000 非结构化
//   - 目标:认证(登录成功/失败/登出/锁定)、授权(越权拒绝/权限变更)、
//     管理(用户/角色/租户 CRUD)全覆盖;JSON 结构化(actor/target/ip/ua/
//     session_id/result/timestamp);落 WORM 存储
//   - actor 统一为 Principal(含类型),机器行为与人类行为同表可过滤(ADR-002)
//
// 当前为骨架包,model/audit.go 将在阶段0 迁入。
package audit
