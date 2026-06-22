// Package auth 实现认证体系:OIDC / Passkey / MFA / Token(JWT + JWKS + 轮换)。
//
// 对应 REDESIGN.md:
//   - ADR-002:Principal 抽象的认证侧落地(Agent 独立签发 EncodeAgentPrincipal)
//   - Batch 2:Access+Refresh 双 Token、标准 OIDC 端点、MFA Step-up、WebAuthn
//
// 当前为骨架包,认证逻辑(model/token、model/auth、model/password)将在
// 阶段0 从 model/ 迁入此包。
package auth
