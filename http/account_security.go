package http

import (
	"fmt"
	"sort"
	"strings"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type accountSecurityRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type accountSecurityFactor struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Enable     bool   `json:"enable"`
	CreatedAt  int64  `json:"createdAt"`
	LastUsedAt int64  `json:"lastUsedAt"`
}

type accountSecuritySession struct {
	ID           string `json:"id"`
	DeviceType   int32  `json:"deviceType"`
	DeviceName   string `json:"deviceName"`
	IP           string `json:"ip"`
	Location     string `json:"location"`
	LastActiveAt int64  `json:"lastActiveAt"`
	CreatedAt    string `json:"createdAt"`
	Current      bool   `json:"current"`
}

type accountSecurityDataScope struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

type accountSecurityTenant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Current   bool   `json:"current"`
	RoleCount int    `json:"roleCount"`
}

type accountSecurityEvent struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Label     string `json:"label"`
	Detail    string `json:"detail,omitempty"`
	IP        string `json:"ip,omitempty"`
	CreatedAt int64  `json:"createdAt"`
}

type accountSecuritySummary struct {
	UserID            string                   `json:"userID"`
	TenantID          string                   `json:"tenantID"`
	TenantName        string                   `json:"tenantName"`
	UserName          string                   `json:"userName"`
	LoginIdentifier   string                   `json:"loginIdentifier"`
	LoginVerified     bool                     `json:"loginIdentifierVerified"`
	DisplayName       string                   `json:"displayName"`
	Avatar            string                   `json:"avatar,omitempty"`
	Email             string                   `json:"email,omitempty"`
	Mobile            string                   `json:"mobile,omitempty"`
	Roles             []accountSecurityRole    `json:"roles"`
	DataScope         accountSecurityDataScope `json:"dataScope"`
	AvailableTenants  []accountSecurityTenant  `json:"availableTenants"`
	MFAEnabled        bool                     `json:"mfaEnabled"`
	MFARecovery       string                   `json:"mfaRecovery"`
	MFAFactors        []accountSecurityFactor  `json:"mfaFactors"`
	Sessions          []accountSecuritySession `json:"sessions"`
	SecurityEvents    []accountSecurityEvent   `json:"securityEvents"`
	PasswordUpdatedAt int64                    `json:"passwordUpdatedAt"`
	CurrentSessionID  string                   `json:"currentSessionID,omitempty"`
}

// AccountSecuritySummary returns the current human user's own identity and
// security state. It deliberately exposes an allowlist instead of serializing
// the User, MFAFactor, or Session persistence models.
func AccountSecuritySummary(c *gin.Context) {
	principalID := strings.TrimSpace(ucm.GetUserID(c))
	if principalID == "" {
		writeBadRequest(c, fmt.Errorf("当前用户身份缺失"))
		return
	}

	account, err := user.GetUserById(principalID)
	if err != nil {
		writeErr(c, err)
		return
	}
	currentSessionID := ""
	currentTenantID := account.TenantID
	if ok, current := ucm.GetUser(c); ok && current != nil {
		currentSessionID = strings.TrimSpace(current.SessionID)
		if value := strings.TrimSpace(current.TenantID); value != "" {
			currentTenantID = value
		}
	}

	var factors []*auth.MFAFactor
	if err := store.DB().Where("principal_id = ?", principalID).Order("created_at desc").Find(&factors).Error; err != nil {
		writeErr(c, err)
		return
	}
	activeSessions, err := session.ListSessions(principalID)
	if err != nil {
		writeErr(c, err)
		return
	}

	summary := accountSecuritySummary{
		UserID: account.ID, TenantID: currentTenantID, TenantName: currentTenantID,
		UserName: account.UserName, LoginIdentifier: account.UserName, LoginVerified: true,
		DisplayName: accountDisplayName(account), Avatar: account.Avatar,
		Email: account.Email, Mobile: account.Mobile, Roles: make([]accountSecurityRole, 0),
		DataScope: accountDataScope(account, currentTenantID), AvailableTenants: accountTenants(account, currentTenantID),
		MFAFactors:        make([]accountSecurityFactor, 0, len(factors)),
		Sessions:          make([]accountSecuritySession, 0, len(activeSessions)),
		SecurityEvents:    accountRecentSecurityEvents(principalID),
		PasswordUpdatedAt: account.PasswordUpdatedAt,
		CurrentSessionID:  currentSessionID,
	}
	if account.Tenant != nil && account.Tenant.ID == currentTenantID && strings.TrimSpace(account.Tenant.Name) != "" {
		summary.TenantName = strings.TrimSpace(account.Tenant.Name)
	} else if record, tenantErr := tenant.GetTenantByID(currentTenantID); tenantErr == nil && strings.TrimSpace(record.Name) != "" {
		summary.TenantName = strings.TrimSpace(record.Name)
	}
	for _, assignment := range account.UserRoles {
		if assignment == nil || assignment.Role == nil || !assignment.Role.Enable ||
			(!assignment.Role.Public && assignment.Role.TenantID != currentTenantID) {
			continue
		}
		summary.Roles = append(summary.Roles, accountSecurityRole{ID: assignment.Role.ID, Name: assignment.Role.Name})
	}
	sort.Slice(summary.Roles, func(i, j int) bool { return summary.Roles[i].Name < summary.Roles[j].Name })
	for _, factor := range factors {
		if factor == nil {
			continue
		}
		summary.MFAFactors = append(summary.MFAFactors, accountSecurityFactor{
			ID: factor.ID, Type: factor.Type, Name: factor.Name, Enable: factor.Enable,
			CreatedAt: factor.CreatedAt, LastUsedAt: factor.LastUsedAt,
		})
		if factor.Enable {
			summary.MFAEnabled = true
		}
	}
	enabledFactorCount := 0
	for _, factor := range summary.MFAFactors {
		if factor.Enable {
			enabledFactorCount++
		}
	}
	switch {
	case enabledFactorCount > 1:
		summary.MFARecovery = "可使用其他已绑定验证器恢复"
	case enabledFactorCount == 1:
		summary.MFARecovery = "联系租户管理员重置验证器"
	default:
		summary.MFARecovery = "启用后建议绑定备用验证器"
	}
	for _, item := range activeSessions {
		if item == nil {
			continue
		}
		summary.Sessions = append(summary.Sessions, accountSecuritySession{
			ID: item.ID, DeviceType: item.DeviceType, DeviceName: item.DeviceName,
			IP: item.IP, Location: item.Location, LastActiveAt: item.LastActiveAt,
			CreatedAt: item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"), Current: item.ID == currentSessionID,
		})
	}
	writeOK(c, gin.H{"data": summary})
}

// RevokeOwnSession lets a user disable one of their own devices without
// granting access to the administrator session API.
func RevokeOwnSession(c *gin.Context) {
	principalID := strings.TrimSpace(ucm.GetUserID(c))
	sessionID := strings.TrimSpace(c.Param("id"))
	if principalID == "" || sessionID == "" {
		writeBadRequest(c, fmt.Errorf("设备会话参数不完整"))
		return
	}
	var target session.Session
	if err := store.DB().Where("id = ? AND principal_id = ? AND revoked = ?", sessionID, principalID, false).First(&target).Error; err != nil {
		writeBadRequest(c, fmt.Errorf("设备会话不存在或无权操作"))
		return
	}
	currentSessionID := ""
	if ok, current := ucm.GetUser(c); ok && current != nil {
		currentSessionID = strings.TrimSpace(current.SessionID)
	}
	if sessionID != currentSessionID && !requireAccountReauth(c, "revoke_session:"+sessionID) {
		return
	}
	revoked, err := session.RevokeSessionForPrincipal(sessionID, principalID, "self_revoked")
	if err != nil {
		writeErr(c, err)
		return
	}
	if !revoked {
		writeBadRequest(c, fmt.Errorf("设备会话不存在或无权操作"))
		return
	}
	recordAudit(c, "session_self_revoke", sessionID, "self_revoked")
	writeOK(c, nil)
}

// RevokeAllOwnSessions revokes every active session, including the caller.
// A single-use step-up proof is always required because the action signs the
// account out everywhere and may interrupt active production work.
func RevokeAllOwnSessions(c *gin.Context) {
	principalID := strings.TrimSpace(ucm.GetUserID(c))
	if principalID == "" || !requireAccountReauth(c, "revoke_all_sessions") {
		return
	}
	count, err := session.RevokeAllByPrincipal(principalID, "", "self_revoke_all")
	if err != nil {
		writeErr(c, err)
		return
	}
	recordAudit(c, "session_self_revoke_all", principalID, fmt.Sprintf("revoked=%d", count))
	writeOK(c, gin.H{"data": gin.H{"revoked": count}})
}

// SwitchOwnTenant issues a new tenant-scoped token only when the user has an
// enabled role assignment in the target tenant. The old session is revoked
// after the replacement manageable session has been persisted.
func SwitchOwnTenant(c *gin.Context) {
	principalID := strings.TrimSpace(ucm.GetUserID(c))
	var req struct {
		TenantID string `json:"tenantID" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, fmt.Errorf("请选择要切换的租户"))
		return
	}
	req.TenantID = strings.TrimSpace(req.TenantID)
	if principalID == "" || !requireAccountReauth(c, "switch_tenant:"+req.TenantID) {
		return
	}
	account, err := user.GetUserById(principalID)
	if err != nil {
		writeErr(c, err)
		return
	}
	ok, current := ucm.GetUser(c)
	if !ok || current == nil {
		writeBadRequest(c, fmt.Errorf("当前登录状态无效"))
		return
	}
	var target accountSecurityTenant
	allowed := false
	for _, item := range accountTenants(account, current.TenantID) {
		if item.ID == req.TenantID {
			target, allowed = item, true
			break
		}
	}
	if !allowed {
		writeBadRequest(c, fmt.Errorf("当前账户未加入该租户"))
		return
	}
	newSessionID := uuid.NewString()
	replacement := &apipb.CurrentUser{
		Id: account.ID, UserName: account.UserName, Gender: account.Gender,
		RoleIDs: account.GetEnabledRoleIDsForTenant(req.TenantID), TenantID: req.TenantID,
		Nickname: account.Nickname, Avatar: account.Avatar, SessionID: newSessionID,
		DeviceType: current.DeviceType, ClientIP: current.ClientIP,
	}
	issuedToken, err := token.EncodeToken(replacement)
	if err != nil {
		writeErr(c, err)
		return
	}
	newSession := &session.Session{
		Model: commonmodel.Model{ID: newSessionID}, PrincipalID: account.ID, TenantID: req.TenantID,
		TokenSig: token.GetTokenSignature(issuedToken), DeviceType: current.DeviceType,
		IP: current.ClientIP, LastActiveAt: time.Now().Unix(),
	}
	if current.SessionID != "" {
		var previous session.Session
		if err := store.DB().First(&previous, "id = ? AND principal_id = ?", current.SessionID, account.ID).Error; err == nil {
			newSession.DeviceName = previous.DeviceName
			newSession.UserAgent = previous.UserAgent
			newSession.Location = previous.Location
			if newSession.IP == "" {
				newSession.IP = previous.IP
			}
		}
	}
	if err := session.CreateSession(newSession); err != nil {
		if token.DefaultTokenCache != nil {
			_ = token.DefaultTokenCache.Del(account.ID, issuedToken)
		}
		writeErr(c, err)
		return
	}
	if current.SessionID != "" {
		if err := session.RevokeSession(current.SessionID, "tenant_switched"); err != nil {
			_ = session.RevokeSession(newSession.ID, "tenant_switch_failed")
			if token.DefaultTokenCache != nil {
				_ = token.DefaultTokenCache.Del(account.ID, issuedToken)
			}
			writeErr(c, err)
			return
		}
	}
	recordAudit(c, "tenant_self_switch", req.TenantID, current.TenantID+" -> "+req.TenantID)
	writeOK(c, gin.H{"data": gin.H{"token": issuedToken, "tenant": target}})
}

func accountDisplayName(account user.User) string {
	for _, value := range []string{account.RealName, account.ChineseName, account.Nickname, account.UserName} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "当前用户"
}

func accountTenants(account user.User, currentTenantID string) []accountSecurityTenant {
	roleCounts := map[string]int{account.TenantID: 0}
	for _, assignment := range account.UserRoles {
		if assignment == nil || assignment.Role == nil || !assignment.Role.Enable || assignment.Role.Public {
			continue
		}
		tenantID := strings.TrimSpace(assignment.Role.TenantID)
		if tenantID != "" {
			roleCounts[tenantID]++
		}
	}
	result := make([]accountSecurityTenant, 0, len(roleCounts))
	for tenantID, roleCount := range roleCounts {
		name := tenantID
		if record, err := tenant.GetTenantByID(tenantID); err == nil && strings.TrimSpace(record.Name) != "" {
			name = strings.TrimSpace(record.Name)
		}
		result = append(result, accountSecurityTenant{ID: tenantID, Name: name, Current: tenantID == currentTenantID, RoleCount: roleCount})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Current != result[j].Current {
			return result[i].Current
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func accountDataScope(account user.User, currentTenantID string) accountSecurityDataScope {
	roleIDs := account.GetEnabledRoleIDsForTenant(currentTenantID)
	if len(roleIDs) == 0 {
		return accountSecurityDataScope{Code: "SELF", Label: "仅本人", Detail: "当前没有生效角色，数据访问限制为本人相关记录"}
	}
	var policies []permission.ABACPolicy
	if err := store.DB().Where("enable = ? AND role_id IN ? AND tenant_id IN (?, '')", true, roleIDs, currentTenantID).
		Order("priority desc").Find(&policies).Error; err != nil || len(policies) == 0 {
		return accountSecurityDataScope{Code: "TENANT", Label: "本租户", Detail: "默认按当前租户隔离，并继续受角色与资源权限约束"}
	}
	seen := make(map[int32]struct{})
	for _, policy := range policies {
		seen[policy.DataScope] = struct{}{}
	}
	if len(seen) > 1 {
		return accountSecurityDataScope{Code: "MIXED", Label: "按资源授权", Detail: "不同业务资源按角色策略应用不同数据范围"}
	}
	for scope := range seen {
		return accountDataScopeLabel(permission.DataScope(scope))
	}
	return accountSecurityDataScope{Code: "TENANT", Label: "本租户", Detail: "按当前租户隔离"}
}

func accountDataScopeLabel(scope permission.DataScope) accountSecurityDataScope {
	switch scope {
	case permission.DataScopeAll:
		return accountSecurityDataScope{Code: "ALL", Label: "全部数据", Detail: "角色策略允许访问全部授权资源数据"}
	case permission.DataScopeDept:
		return accountSecurityDataScope{Code: "DEPARTMENT", Label: "本部门", Detail: "角色策略限制为当前部门数据"}
	case permission.DataScopeSelf:
		return accountSecurityDataScope{Code: "SELF", Label: "仅本人", Detail: "角色策略限制为本人创建或负责的数据"}
	case permission.DataScopeCustom:
		return accountSecurityDataScope{Code: "CUSTOM", Label: "自定义范围", Detail: "按 UserCenter 自定义条件过滤数据"}
	default:
		return accountSecurityDataScope{Code: "TENANT", Label: "本租户", Detail: "角色策略限制为当前租户数据"}
	}
}

func accountRecentSecurityEvents(principalID string) []accountSecurityEvent {
	logs, _, err := audit.QueryAuditLogs(&audit.AuditQuery{UserID: principalID, PrincipalKind: -1, PageIndex: 1, PageSize: 5})
	if err != nil {
		return []accountSecurityEvent{}
	}
	result := make([]accountSecurityEvent, 0, len(logs))
	for _, item := range logs {
		if item == nil {
			continue
		}
		result = append(result, accountSecurityEvent{
			ID: item.ID, Action: item.Action, Label: accountSecurityEventLabel(item.Action),
			Detail: item.Detail, IP: item.IP, CreatedAt: item.CreatedAt.Unix(),
		})
	}
	return result
}

func accountSecurityEventLabel(action string) string {
	switch action {
	case audit.AuditActionChangePwd:
		return "密码已修改"
	case "mfa_totp_enroll":
		return "已绑定多因素验证"
	case "mfa_unbind":
		return "已关闭多因素验证"
	case "session_self_revoke":
		return "登录设备已下线"
	case "session_self_revoke_all":
		return "全部设备已退出"
	case "tenant_self_switch":
		return "已切换租户"
	case "account_reverified":
		return "已重新验证身份"
	default:
		return strings.ReplaceAll(action, "_", " ")
	}
}
