package authn

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
)

// AuthorizeBackgroundUser is for trusted, in-process job runners only. The host
// supplies IDs from an already authorized durable task, never from HTTP headers.
// Re-read status and roles on every call so queued work cannot retain revoked
// privileges. No browser session, provider key or reusable token is issued.
func AuthorizeBackgroundUser(ctx context.Context, userID, tenantID, method, path string) (principal.Principal, *apipb.CurrentUser, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(tenantID) == "" {
		return nil, nil, errors.New("background AI task requires a user and tenant")
	}
	p := principal.NewHuman(userID, tenantID, nil)
	if code, err := validatePrincipalStatus(p, time.Now()); code != model.Success {
		return nil, nil, err
	}
	var account user.User
	if err := store.DB().WithContext(ctx).Preload("UserRoles").Where("id = ? AND tenant_id = ?", userID, tenantID).First(&account).Error; err != nil {
		return nil, nil, err
	}
	roles := account.GetEnabledRoleIDs()
	allowed, err := permission.EnforceCached("0", path, method)
	if err != nil {
		return nil, nil, err
	}
	for _, role := range roles {
		if allowed {
			break
		}
		allowed, err = permission.EnforceCached(role, path, method)
		if err != nil {
			return nil, nil, err
		}
	}
	if !allowed {
		return nil, nil, errors.New("background AI user is not authorized")
	}
	current := &apipb.CurrentUser{Id: userID, TenantID: tenantID, UserName: account.UserName, RoleIDs: roles}
	return principal.FromTokenAndUser("", current), current, nil
}
