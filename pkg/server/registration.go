package server

import (
	"errors"
	"fmt"
	"strings"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/auth"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TenantAdminRegistration is the narrow cross-product contract for creating a
// tenant and its first administrator. Product-specific onboarding data stays
// in the embedding product and can be written in the same database transaction.
type TenantAdminRegistration struct {
	TenantID      string
	TenantName    string
	Contact       string
	CellPhone     string
	Address       string
	AdminUserName string
	AdminPassword string
	AdminNickname string
	AdminEmail    string
	AdminMobile   string
	RoleID        string
	ProjectID     string
}

type TenantAdminRegistrationResult struct {
	TenantID string
	UserID   string
	RoleID   string
}

// RegisterTenantAdminInTransaction writes only UserCenter-owned tenant,
// principal and role-assignment rows through the caller-owned transaction.
// The function never commits: an embedding product can atomically roll these
// rows back when its own onboarding profile fails.
func RegisterTenantAdminInTransaction(tx *gorm.DB, request TenantAdminRegistration) (TenantAdminRegistrationResult, error) {
	if tx == nil {
		return TenantAdminRegistrationResult{}, errors.New("registration transaction is required")
	}
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.TenantName = strings.TrimSpace(request.TenantName)
	request.AdminUserName = strings.ToLower(strings.TrimSpace(request.AdminUserName))
	request.AdminNickname = strings.TrimSpace(request.AdminNickname)
	request.AdminEmail = strings.TrimSpace(request.AdminEmail)
	request.AdminMobile = strings.TrimSpace(request.AdminMobile)
	request.RoleID = strings.TrimSpace(request.RoleID)
	if request.TenantID == "" {
		request.TenantID = uuid.NewString()
	}
	if request.TenantName == "" || request.AdminUserName == "" || request.AdminPassword == "" || request.RoleID == "" {
		return TenantAdminRegistrationResult{}, errors.New("tenant name, administrator username, password and role are required")
	}
	if !auth.ValidPasswdStrength(request.AdminPassword) {
		return TenantAdminRegistrationResult{}, errors.New("administrator password does not meet UserCenter strength requirements")
	}
	if request.AdminNickname == "" {
		request.AdminNickname = request.AdminUserName
	}

	var duplicateCount int64
	if err := tx.Unscoped().Model(&tenant.Tenant{}).
		Where("id = ? OR name = ?", request.TenantID, request.TenantName).
		Count(&duplicateCount).Error; err != nil {
		return TenantAdminRegistrationResult{}, fmt.Errorf("check tenant uniqueness: %w", err)
	}
	if duplicateCount > 0 {
		return TenantAdminRegistrationResult{}, errors.New("tenant ID or name already exists")
	}
	userQuery := tx.Unscoped().Model(&user.User{}).Where("user_name = ?", request.AdminUserName)
	if request.AdminMobile != "" {
		userQuery = userQuery.Or("mobile = ?", request.AdminMobile)
	}
	if err := userQuery.Count(&duplicateCount).Error; err != nil {
		return TenantAdminRegistrationResult{}, fmt.Errorf("check administrator uniqueness: %w", err)
	}
	if duplicateCount > 0 {
		return TenantAdminRegistrationResult{}, errors.New("administrator username or mobile already exists")
	}

	var assignedRole permission.Role
	if err := tx.Where("id = ? AND enable = ? AND (public = ? OR tenant_id = ?)", request.RoleID, true, true, request.TenantID).
		First(&assignedRole).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TenantAdminRegistrationResult{}, errors.New("administrator role is unavailable for the new tenant")
		}
		return TenantAdminRegistrationResult{}, fmt.Errorf("load administrator role: %w", err)
	}

	hashedPassword, err := auth.EncryptedPassword(request.AdminPassword)
	if err != nil {
		return TenantAdminRegistrationResult{}, fmt.Errorf("hash administrator password: %w", err)
	}
	tenantRecord := tenant.Tenant{
		Model: commonmodel.Model{ID: request.TenantID},
		Name:  request.TenantName, Contact: strings.TrimSpace(request.Contact),
		CellPhone: strings.TrimSpace(request.CellPhone), Address: strings.TrimSpace(request.Address),
		Enable: true, Expired: time.Now().UTC().AddDate(10, 0, 0),
	}
	if err := tx.Create(&tenantRecord).Error; err != nil {
		return TenantAdminRegistrationResult{}, fmt.Errorf("create tenant: %w", err)
	}
	administrator := user.User{
		TenantModel: commonmodel.TenantModel{TenantID: request.TenantID},
		ProjectID:   strings.TrimSpace(request.ProjectID), UserName: request.AdminUserName,
		Password: hashedPassword, PasswordUpdatedAt: time.Now().UTC().Unix(),
		Nickname: request.AdminNickname, Email: request.AdminEmail, Mobile: request.AdminMobile,
		Enable: true, CanDel: false, ForceChangePwd: false,
	}
	if err := tx.Create(&administrator).Error; err != nil {
		return TenantAdminRegistrationResult{}, fmt.Errorf("create tenant administrator: %w", err)
	}
	if err := tx.Create(&user.UserRole{UserID: administrator.ID, RoleID: assignedRole.ID}).Error; err != nil {
		return TenantAdminRegistrationResult{}, fmt.Errorf("assign tenant administrator role: %w", err)
	}
	return TenantAdminRegistrationResult{TenantID: tenantRecord.ID, UserID: administrator.ID, RoleID: assignedRole.ID}, nil
}
