package http

import (
	"fmt"
	"sort"
	"strings"

	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
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

type accountSecuritySummary struct {
	UserID           string                   `json:"userID"`
	TenantID         string                   `json:"tenantID"`
	TenantName       string                   `json:"tenantName"`
	UserName         string                   `json:"userName"`
	DisplayName      string                   `json:"displayName"`
	Avatar           string                   `json:"avatar,omitempty"`
	Email            string                   `json:"email,omitempty"`
	Mobile           string                   `json:"mobile,omitempty"`
	Roles            []accountSecurityRole    `json:"roles"`
	MFAEnabled       bool                     `json:"mfaEnabled"`
	MFAFactors       []accountSecurityFactor  `json:"mfaFactors"`
	Sessions         []accountSecuritySession `json:"sessions"`
	CurrentSessionID string                   `json:"currentSessionID,omitempty"`
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
	if ok, current := ucm.GetUser(c); ok && current != nil {
		currentSessionID = strings.TrimSpace(current.SessionID)
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
		UserID: account.ID, TenantID: account.TenantID, TenantName: account.TenantID,
		UserName: account.UserName, DisplayName: accountDisplayName(account), Avatar: account.Avatar,
		Email: account.Email, Mobile: account.Mobile, Roles: make([]accountSecurityRole, 0),
		MFAFactors:       make([]accountSecurityFactor, 0, len(factors)),
		Sessions:         make([]accountSecuritySession, 0, len(activeSessions)),
		CurrentSessionID: currentSessionID,
	}
	if account.Tenant != nil && strings.TrimSpace(account.Tenant.Name) != "" {
		summary.TenantName = strings.TrimSpace(account.Tenant.Name)
	}
	for _, assignment := range account.UserRoles {
		if assignment == nil || assignment.Role == nil || !assignment.Role.Enable {
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

func accountDisplayName(account user.User) string {
	for _, value := range []string{account.RealName, account.ChineseName, account.Nickname, account.UserName} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "当前用户"
}
