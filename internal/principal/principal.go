// Package principal 定义统一的鉴权主体(Principal)抽象。
//
// 这是 usercenter 重新设计的地基(ADR-002):传统 usercenter 的所有主体都是
// 人类 User,而 AI 时代需要把 AI Agent、机器服务账号(Service)也作为一等公民
// (NHI, Non-Human Identity)。Principal 接口让 Human/Agent/Service 三类主体
// 统一进入鉴权、计量、配额、审计体系。
//
// 设计原则(ADR-002):
//   - Principal interface 是加法,不破坏现有 User/Role/Casbin 任何一行
//   - 旧 token 解出的 *apipb.CurrentUser 通过 FromCurrentUser 单向适配为 Principal
//   - Agent 身份走独立签发路径(EncodeAgentPrincipal),不污染人类 token
//   - 适配器(FromCurrentUser)是过渡期债务,有 4 条 CI 退出门监控其归零(REDESIGN §4)
package principal

import (
	apipb "github.com/CloudSilk/usercenter/proto"
)

// Kind 鉴权主体类型
type Kind int32

const (
	KindUnknown Kind = 0
	KindHuman   Kind = 1 // 人类用户
	KindAgent   Kind = 2 // AI Agent(非人类身份 NHI)
	KindService Kind = 3 // 机器服务账号(M2M,如微服务/后台 Job/MCP Server)
)

// Principal 所有鉴权主体的统一抽象。
// 鉴权(Authenticate)、计量(UsageRecord)、配额(RateLimit)、审计(AuditLog)
// 都以 Principal 为统一主体模型,而非分散的 User/Agent/Service 各写一套。
type Principal interface {
	// Kind 返回主体类型(人/Agent/服务)
	Kind() Kind
	// Subject 返回主体唯一标识(userID / agentID / serviceID)
	Subject() string
	// TenantID 返回主体所属租户(租户隔离的强制 scope 依据)
	TenantID() string
	// Roles 返回主体的角色 ID 列表(供 RBAC 判定)
	Roles() []string
}

// HumanPrincipal 人类用户主体
type HumanPrincipal struct {
	userID   string
	tenantID string
	roles    []string
}

// NewHuman 构造人类用户主体
func NewHuman(userID, tenantID string, roles []string) *HumanPrincipal {
	return &HumanPrincipal{userID: userID, tenantID: tenantID, roles: roles}
}

func (h *HumanPrincipal) Kind() Kind       { return KindHuman }
func (h *HumanPrincipal) Subject() string  { return h.userID }
func (h *HumanPrincipal) TenantID() string { return h.tenantID }
func (h *HumanPrincipal) Roles() []string  { return h.roles }

// AgentPrincipal AI Agent 主体(非人类身份)。
// OwnerUserID 指向拥有该 Agent 的人类用户,用于委派(delegation)链追溯:
// "Agent A 代表用户 U 行动"。
type AgentPrincipal struct {
	agentID     string
	ownerUserID string
	tenantID    string
	roles       []string
}

// NewAgent 构造 AI Agent 主体
func NewAgent(agentID, ownerUserID, tenantID string, roles []string) *AgentPrincipal {
	return &AgentPrincipal{agentID: agentID, ownerUserID: ownerUserID, tenantID: tenantID, roles: roles}
}

// OwnerUserID 返回拥有该 Agent 的用户 ID(委派链追溯用)
func (a *AgentPrincipal) OwnerUserID() string { return a.ownerUserID }

func (a *AgentPrincipal) Kind() Kind       { return KindAgent }
func (a *AgentPrincipal) Subject() string  { return a.agentID }
func (a *AgentPrincipal) TenantID() string { return a.tenantID }
func (a *AgentPrincipal) Roles() []string  { return a.roles }

// ServicePrincipal 机器服务账号(M2M)。
// 用于微服务间调用、后台 Job、MCP Server 等,通过 client_credentials 颁发,
// 不走浏览器 JWT 流程。
type ServicePrincipal struct {
	serviceID string
	tenantID  string
	roles     []string
}

// NewService 构造机器服务账号主体
func NewService(serviceID, tenantID string, roles []string) *ServicePrincipal {
	return &ServicePrincipal{serviceID: serviceID, tenantID: tenantID, roles: roles}
}

func (s *ServicePrincipal) Kind() Kind       { return KindService }
func (s *ServicePrincipal) Subject() string  { return s.serviceID }
func (s *ServicePrincipal) TenantID() string { return s.tenantID }
func (s *ServicePrincipal) Roles() []string  { return s.roles }

// FromCurrentUser 从现有 *apipb.CurrentUser 适配出 Principal。
//
// 这是过渡期的单向适配器(REDESIGN §4 阶段1 的 PrincipalAdapter 雏形):
// 旧 token 解出 CurrentUser 后,转成 Principal 进入新鉴权链路。
// 单向数据流:只读旧模型产 Principal,禁止反向回写(避免循环适配)。
//
// 注意:本符号在 CI debt-check 闸门监控下(REDESIGN §4),目标是随 14 个
// Provider 迁移到 Principal 后归零并删除。
func FromCurrentUser(u *apipb.CurrentUser) Principal {
	return NewHuman(u.Id, u.TenantID, u.RoleIDs)
}
