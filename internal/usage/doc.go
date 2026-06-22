// Package usage 实现 Token/成本计量中心。
//
// 对应 REDESIGN.md Batch 2(AI 差异化核心):
//   - UsageRecord 实体:一次 LLM 调用一条(principalID/providerID/model/
//     promptTokens/completionTokens/cost/requestID/时间戳)
//   - 按 principal/tenant/model/department/time-window 实时聚合(不能等月结,
//     Agent 配额秒级生效)
//   - principalID 含 principalType 旁注字段消歧(ADR-002:避免 userID 语义二义)
//
// ADR-001:本包作为领域包实现,可远程化接口。UsageRecord 写入若挤占主库 I/O,
// 触发拆独立服务(参见 REDESIGN ADR-001 演进触发条件)。
package usage
