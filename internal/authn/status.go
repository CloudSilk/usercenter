package authn

import (
	"errors"
	"fmt"
	"time"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	"gorm.io/gorm"
)

type humanStatus struct {
	ID            string
	TenantID      string
	Enable        bool
	LockedExpired int64
	Expired       int64
}

type tenantStatus struct {
	ID      string
	Enable  bool
	Expired time.Time
}

func validatePrincipalStatus(p principal.Principal, now time.Time) (int, error) {
	if p == nil {
		return model.TokenInvalid, errors.New("principal is missing")
	}
	if p.Kind() != principal.KindHuman {
		return model.Success, nil
	}

	database := store.DB()
	if database == nil {
		return model.InternalServerError, errors.New("database is not initialized")
	}

	account := humanStatus{}
	err := database.Model(&user.User{}).
		Select("id", "tenant_id", "enable", "locked_expired", "expired").
		Where("id = ?", p.Subject()).
		Take(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.TokenInvalid, errors.New("user does not exist")
	}
	if err != nil {
		return model.InternalServerError, fmt.Errorf("query user status: %w", err)
	}
	if account.TenantID != p.TenantID() {
		return model.TokenInvalid, errors.New("user tenant does not match token")
	}
	if !account.Enable {
		return model.UserDisabled, errors.New("user is disabled")
	}
	if account.LockedExpired > now.Unix() {
		return model.UserDisabled, errors.New("user is locked")
	}
	if account.Expired > 0 && account.Expired <= now.Unix() {
		return model.UserDisabled, errors.New("user is expired")
	}

	owner := tenantStatus{}
	err = database.Model(&tenant.Tenant{}).
		Select("id", "enable", "expired").
		Where("id = ?", account.TenantID).
		Take(&owner).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.TokenInvalid, errors.New("tenant does not exist")
	}
	if err != nil {
		return model.InternalServerError, fmt.Errorf("query tenant status: %w", err)
	}
	if !owner.Enable {
		return model.UserDisabled, errors.New("tenant is disabled")
	}
	if !owner.Expired.After(now) {
		return model.UserDisabled, errors.New("tenant is expired")
	}

	return model.Success, nil
}
