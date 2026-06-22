// Package principal 定义统一的鉴权主体(Principal)抽象。
package principal

import apipb "github.com/CloudSilk/usercenter/proto"

type Kind int32

const (
	KindUnknown Kind = 0
	KindHuman   Kind = 1
	KindAgent   Kind = 2
	KindService Kind = 3
)

// Principal 所有鉴权主体的统一抽象。
type Principal interface {
	Kind() Kind
	Subject() string
	TenantID() string
	Roles() []string
	DisplayName() string
}

// HumanPrincipal 人类用户主体
type HumanPrincipal struct {
	userID   string
	tenantID string
	roles    []string
}

func NewHuman(userID, tenantID string, roles []string) *HumanPrincipal {
	return &HumanPrincipal{userID: userID, tenantID: tenantID, roles: roles}
}

func (h *HumanPrincipal) Kind() Kind        { return KindHuman }
func (h *HumanPrincipal) Subject() string   { return h.userID }
func (h *HumanPrincipal) TenantID() string  { return h.tenantID }
func (h *HumanPrincipal) Roles() []string   { return h.roles }
func (h *HumanPrincipal) DisplayName() string { return h.userID }

// AgentPrincipal AI Agent 主体
type AgentPrincipal struct {
	agentID     string
	ownerUserID string
	tenantID    string
	roles       []string
}

func NewAgent(agentID, ownerUserID, tenantID string, roles []string) *AgentPrincipal {
	return &AgentPrincipal{agentID: agentID, ownerUserID: ownerUserID, tenantID: tenantID, roles: roles}
}

func (a *AgentPrincipal) OwnerUserID() string  { return a.ownerUserID }
func (a *AgentPrincipal) Kind() Kind           { return KindAgent }
func (a *AgentPrincipal) Subject() string      { return a.agentID }
func (a *AgentPrincipal) TenantID() string     { return a.tenantID }
func (a *AgentPrincipal) Roles() []string      { return a.roles }
func (a *AgentPrincipal) DisplayName() string  { return "agent:" + a.agentID }

// ServicePrincipal 机器服务账号
type ServicePrincipal struct {
	serviceID string
	tenantID  string
	roles     []string
}

func NewService(serviceID, tenantID string, roles []string) *ServicePrincipal {
	return &ServicePrincipal{serviceID: serviceID, tenantID: tenantID, roles: roles}
}

func (s *ServicePrincipal) Kind() Kind          { return KindService }
func (s *ServicePrincipal) Subject() string     { return s.serviceID }
func (s *ServicePrincipal) TenantID() string    { return s.tenantID }
func (s *ServicePrincipal) Roles() []string     { return s.roles }
func (s *ServicePrincipal) DisplayName() string { return "service:" + s.serviceID }

// FromTokenAndUser 从 CurrentUser 构造 Principal。
// Agent type=1 → NewAgent, Service type=2 → NewService, 默认 → NewHuman。
// token 参数保留但当前不使用(Agent claims 已在 CurrentUser.Type 中编码);
// 需要深度解 Agent claims(ownerUserID 等)时,调用方应在 middleware 层解码后传参。
func FromTokenAndUser(_ string, u *apipb.CurrentUser) Principal {
	if u == nil {
		return nil
	}
	switch u.Type {
	case 1:
		return NewAgent(u.Id, "", u.TenantID, u.RoleIDs)
	case 2:
		return NewService(u.Id, u.TenantID, u.RoleIDs)
	default:
		return NewHuman(u.Id, u.TenantID, u.RoleIDs)
	}
}
