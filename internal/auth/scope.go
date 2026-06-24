package auth

// Agent 细粒度 Scope + 用户委派 consent(#10)

import (
	"context"
	"strings"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
)

// Scope OAuth2 scope 语义
// 格式:<resource>:<action> 或 <resource>:<action>:<constraint>
// 示例:read:order, write:profile, read:order:own_tenant_only

// ParseScope 解析 scope 字符串为结构化列表
func ParseScope(scopeStr string) []Scope {
	var scopes []Scope
	for _, s := range strings.Fields(scopeStr) {
		parts := strings.SplitN(s, ":", 3)
		sc := Scope{Raw: s}
		if len(parts) > 0 {
			sc.Action = parts[0]
		}
		if len(parts) > 1 {
			sc.Resource = parts[1]
		}
		if len(parts) > 2 {
			sc.Constraint = parts[2]
		}
		scopes = append(scopes, sc)
	}
	return scopes
}

// Scope 结构化 scope
type Scope struct {
	Raw        string // 原始字符串
	Action     string // read/write/delete/admin
	Resource   string // order/user/profile/agent/...
	Constraint string // 约束(如 own_tenant_only)
}

// Matches 检查请求是否在 scope 范围内
func (s Scope) Matches(action, resource string) bool {
	return (s.Action == action || s.Action == "admin" || s.Action == "*") &&
		(s.Resource == resource || s.Resource == "*")
}

// CheckScope 检查 scope 列表是否覆盖请求
func CheckScope(scopes []Scope, action, resource string) bool {
	for _, s := range scopes {
		if s.Matches(action, resource) {
			return true
		}
	}
	return false
}

// ConsentManager consent 管理(用户授权 Agent 的 scope)

// HasConsent 检查用户是否已授权某 Agent 的某 scope
// principalID = 授权人,clientID = Agent/应用
func HasConsent(principalID, clientID, scope string) bool {
	if store.DB() == nil {
		return true // DB 未就绪时放行（开发模式）
	}
	var rec ConsentRecord
	err := store.DB().Where(
		"principal_id = ? AND client_id = ? AND revoked = ? AND scope LIKE ?",
		principalID, clientID, false, "%"+scope+"%",
	).First(&rec).Error
	if err != nil {
		return false
	}
	// 检查过期（0 = 永不过期）
	if rec.ExpiresAt > 0 && time.Now().Unix() > rec.ExpiresAt {
		return false
	}
	return true
}

// GrantConsent 用户授权 consent（同一 principal+client 的旧记录先吊销再写入新记录）
func GrantConsent(principalID, clientID, scope string, expiresAt int64) error {
	if store.DB() == nil {
		return nil
	}
	// 吊销旧的同 principal+client 记录
	if err := store.DB().Model(&ConsentRecord{}).
		Where("principal_id = ? AND client_id = ? AND revoked = ?", principalID, clientID, false).
		Update("revoked", true).Error; err != nil {
		log.Errorf(context.Background(), "revoke old consent failed: %v", err)
	}
	rec := &ConsentRecord{
		PrincipalID: principalID,
		ClientID:    clientID,
		Scope:       scope,
		GrantedAt:   time.Now().Unix(),
		ExpiresAt:   expiresAt,
		Revoked:     false,
	}
	return store.DB().Create(rec).Error
}

// RevokeConsent 撤销 consent
func RevokeConsent(principalID, clientID string) error {
	if store.DB() == nil {
		return nil
	}
	return store.DB().Model(&ConsentRecord{}).
		Where("principal_id = ? AND client_id = ?", principalID, clientID).
		Update("revoked", true).Error
}

// ListConsents 列出用户授权过的所有 Agent/应用（未吊销）
func ListConsents(principalID string) ([]ConsentRecord, error) {
	if store.DB() == nil {
		return nil, nil
	}
	var list []ConsentRecord
	err := store.DB().Where("principal_id = ? AND revoked = ?", principalID, false).
		Order("granted_at desc").Find(&list).Error
	return list, err
}

// DelegationChain 委派链(user → agent → agent)
type DelegationChain struct {
	Links []DelegationLink
}

// DelegationLink 委派链中的一跳
type DelegationLink struct {
	PrincipalID string
	Kind        string // "human" / "agent"
	Scope       string
	GrantedAt   time.Time
}

// CheckDelegation 验证委派链(最弱权限原则:交集)
func CheckDelegation(chain *DelegationChain, action, resource string) bool {
	if chain == nil || len(chain.Links) == 0 {
		return false
	}
	// 委派链中每一跳的 scope 必须都覆盖请求(交集)
	for _, link := range chain.Links {
		scopes := ParseScope(link.Scope)
		if !CheckScope(scopes, action, resource) {
			return false // 任一跳不覆盖 = 委派链不授权
		}
	}
	return true
}
