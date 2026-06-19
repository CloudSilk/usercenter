// Package session 实现会话列表、单会话吊销与风险信号。
//
// 对应 REDESIGN.md Batch 3(安全深化):
//   - /sessions 列表(设备/IP/最近活跃/地理位置)+ 单会话吊销
//   - 新登录通知 + 异地登录提醒
//   - "this device" vs "all devices" 两种登出
//   - Zero Trust 连续验证:基于实时信号(IP 漂移/设备指纹/异常事件)算风险分
//
// 当前为骨架包,会话逻辑(model/token/token_cache.go 的 Session 部分)将在
// 阶段0/1 迁入。
package session
