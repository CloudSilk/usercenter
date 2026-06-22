// Package apikey 实现 AI Capability Key 管理与多 Provider 路由。
//
// 对应 REDESIGN.md Batch 2(AI 差异化核心):
//   - AIProvider 实体(OpenAI/Anthropic/本地 vLLM/TTS/ImageGen):baseURL、鉴权、健康状态
//   - AIKey 实体:属 Provider+Tenant,支持主从/故障转移池、轮转、429 自动 cooldown、
//     **明文 key 必须 AES-GCM 加密存储**(可逆,区别于密码的 scrypt 不可逆)
//   - ModelRoute 策略:request → model alias → 选用哪个 provider 的哪个 key,
//     支持按租户/成本/延迟/区域路由
//
// ADR-001:本包作为领域包实现,对外 API 从第一天设计成"可远程化"(纯函数+context),
// 失效信号触发时拎出去成独立服务是换 transport 不是重写。
package apikey
