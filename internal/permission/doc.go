// Package permission 实现授权体系:RBAC + ABAC + DataScope + CAS(Tool 权限)。
//
// 对应 REDESIGN.md:
//   - Batch 1/2:从纯 RBAC(roleID→URL)演进到 RBAC+ABAC+数据范围
//   - ADR-001 安全红线:ABAC 资源参数判定必须在能拿到完整 tool call 的位置,
//     不下沉到只有工具粒度的远端策略服务
//
// 当前为骨架包,授权逻辑(model/casbin_rule、model/adapter、model/auth 的
// Enforce 部分)将在阶段0 迁入此包。
package permission
