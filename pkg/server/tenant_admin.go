package server

import (
	"errors"
	"strings"

	ucpermission "github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	ucuser "github.com/CloudSilk/usercenter/internal/user"
	"gorm.io/gorm"
)

const tenantAdministratorRoleCode = "TENANT_ADMIN"

type TenantAdministrator struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenantId"`
	ProjectID string `json:"projectId"`
	UserName  string `json:"userName"`
	Nickname  string `json:"nickname"`
	Mobile    string `json:"mobile"`
	Email     string `json:"email"`
	Enabled   bool   `json:"enabled"`
	RoleID    string `json:"roleId"`
}

type CreateTenantAdministratorRequest struct {
	TenantID  string
	ProjectID string
	UserName  string
	Nickname  string
	Mobile    string
	Email     string
	Password  string
}

func CreateTenantAdministrator(request CreateTenantAdministratorRequest) (TenantAdministrator, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	request.UserName = strings.TrimSpace(request.UserName)
	request.Nickname = strings.TrimSpace(request.Nickname)
	if request.TenantID == "" || request.ProjectID == "" || request.UserName == "" || request.Nickname == "" || request.Password == "" {
		return TenantAdministrator{}, errors.New("tenant, project, username, nickname and password are required")
	}
	if _, err := GetTenant(request.TenantID); err != nil {
		return TenantAdministrator{}, err
	}

	role, err := ensureTenantAdministratorRole(request.TenantID, request.ProjectID)
	if err != nil {
		return TenantAdministrator{}, err
	}
	account := &ucuser.User{
		ProjectID: request.ProjectID, UserName: request.UserName, Nickname: request.Nickname,
		Password: request.Password, Mobile: strings.TrimSpace(request.Mobile), Email: strings.TrimSpace(request.Email),
		Enable: true, ForceChangePwd: true,
	}
	account.TenantID = request.TenantID
	if err := ucuser.CreateUser(account, false); err != nil {
		return TenantAdministrator{}, err
	}
	if _, err := ucuser.UpdateUserRoles(account.ID, []string{role.ID}); err != nil {
		_ = ucuser.DeleteUser(account.ID)
		return TenantAdministrator{}, err
	}
	return tenantAdministratorView(account, role.ID), nil
}

func ListTenantAdministrators(tenantID, projectID string) ([]TenantAdministrator, error) {
	tenantID = strings.TrimSpace(tenantID)
	projectID = strings.TrimSpace(projectID)
	var accounts []ucuser.User
	if err := store.DB().
		Joins("JOIN user_roles ON user_roles.user_id = users.id AND user_roles.deleted_at IS NULL").
		Joins("JOIN roles ON roles.id = user_roles.role_id AND roles.deleted_at IS NULL").
		Where("users.tenant_id = ? AND users.project_id = ? AND roles.tenant_id = ? AND roles.project_id = ? AND roles.name = ?", tenantID, projectID, tenantID, projectID, tenantAdministratorRoleCode).
		Order("users.created_at").Find(&accounts).Error; err != nil {
		return nil, err
	}
	role, err := tenantAdministratorRole(tenantID, projectID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	roleID := ""
	if err == nil {
		roleID = role.ID
	}
	result := make([]TenantAdministrator, 0, len(accounts))
	for index := range accounts {
		result = append(result, tenantAdministratorView(&accounts[index], roleID))
	}
	return result, nil
}

func CountTenantProjectUsers(tenantID, projectID string) (int64, error) {
	tenantID = strings.TrimSpace(tenantID)
	projectID = strings.TrimSpace(projectID)
	if tenantID == "" || projectID == "" {
		return 0, errors.New("tenant and project are required")
	}
	var count int64
	if err := store.DB().Model(&ucuser.User{}).
		Where("tenant_id = ? AND project_id = ?", tenantID, projectID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func SetTenantAdministratorEnabled(tenantID, projectID, userID string, enabled bool) error {
	if _, err := getTenantAdministrator(tenantID, projectID, userID); err != nil {
		return err
	}
	return ucuser.EnableUser(userID, enabled)
}

func ResetTenantAdministratorPassword(tenantID, projectID, userID, password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("password is required")
	}
	if _, err := getTenantAdministrator(tenantID, projectID, userID); err != nil {
		return err
	}
	return ucuser.ResetPwd(userID, password)
}

func ensureTenantAdministratorRole(tenantID, projectID string) (TenantProjectRole, error) {
	role, err := tenantAdministratorRole(tenantID, projectID)
	if err == nil {
		return role, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return TenantProjectRole{}, err
	}
	return CreateTenantProjectRole(CreateTenantProjectRoleRequest{
		TenantID: tenantID, ProjectID: projectID, Code: tenantAdministratorRoleCode,
		DisplayName: "租户管理员", DefaultRouter: "/platform",
	})
}

func tenantAdministratorRole(tenantID, projectID string) (TenantProjectRole, error) {
	var role ucpermission.Role
	if err := store.DB().Where("tenant_id = ? AND project_id = ? AND name = ?", tenantID, projectID, tenantAdministratorRoleCode).First(&role).Error; err != nil {
		return TenantProjectRole{}, err
	}
	return tenantProjectRoleView(&role), nil
}

func getTenantAdministrator(tenantID, projectID, userID string) (*ucuser.User, error) {
	var account ucuser.User
	if err := store.DB().
		Joins("JOIN user_roles ON user_roles.user_id = users.id AND user_roles.deleted_at IS NULL").
		Joins("JOIN roles ON roles.id = user_roles.role_id AND roles.deleted_at IS NULL").
		Where("users.id = ? AND users.tenant_id = ? AND users.project_id = ? AND roles.tenant_id = ? AND roles.project_id = ? AND roles.name = ?", userID, tenantID, projectID, tenantID, projectID, tenantAdministratorRoleCode).
		First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func tenantAdministratorView(account *ucuser.User, roleID string) TenantAdministrator {
	return TenantAdministrator{
		ID: account.ID, TenantID: account.TenantID, ProjectID: account.ProjectID,
		UserName: account.UserName, Nickname: account.Nickname, Mobile: account.Mobile,
		Email: account.Email, Enabled: account.Enable, RoleID: roleID,
	}
}
