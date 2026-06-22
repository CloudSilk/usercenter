// Package user 实现人类用户档案管理(User 实体 + CRUD)。
//
// 对应 REDESIGN.md 阶段0:从 model/user.go(含 User struct + 业务逻辑)迁入,
// 按 3 层拆分:domain(struct+校验)→ repo(gorm)→ service(业务编排)。
//
// 注意:鉴权主体抽象在 internal/principal,本包只管人类用户的档案数据,
// 不承担认证/授权职责(那些在 internal/auth、internal/permission)。
package user
