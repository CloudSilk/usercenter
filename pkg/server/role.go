package server

import (
	"errors"
	"strings"

	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TenantProjectRole is the stable public view of a tenant-owned role.
// Code is the machine identifier stored by UserCenter; DisplayName is the
// product-facing label stored in the native role description field.
type TenantProjectRole struct {
	ID            string `json:"id"`
	TenantID      string `json:"tenantID"`
	ProjectID     string `json:"projectID"`
	Code          string `json:"code"`
	DisplayName   string `json:"displayName"`
	DefaultRouter string `json:"defaultRouter"`
	Enabled       bool   `json:"enabled"`
	Deletable     bool   `json:"deletable"`
}

// CreateTenantProjectRoleRequest deliberately exposes only tenant-safe role
// fields. Public/system/mandatory flags and cross-tenant parent relations are
// not accepted through the embedded-product facade.
type CreateTenantProjectRoleRequest struct {
	TenantID      string
	ProjectID     string
	Code          string
	DisplayName   string
	DefaultRouter string
}

// RoleAuthorizationSelection and the related result types are aliases of
// UserCenter's native authorization contract. Embedded products can publish
// their menu/function selections without writing role_menus or Casbin tables.
type RoleAuthorizationSelection = permission.RoleAuthorizationSelection
type RoleAuthorizationDetail = permission.RoleAuthorizationDetail
type RoleAuthorizationPublishResult = permission.RoleAuthorizationPublishResult
type RoleAuthorizationABACPolicy = permission.RoleAuthorizationABACPolicy

func validateTenantProjectRoleFields(tenantID, projectID, code, displayName string) error {
	if strings.TrimSpace(tenantID) == "" {
		return errors.New("tenant ID cannot be empty")
	}
	if strings.TrimSpace(projectID) == "" {
		return errors.New("project ID cannot be empty")
	}
	if strings.TrimSpace(code) == "" {
		return errors.New("role code cannot be empty")
	}
	if strings.TrimSpace(displayName) == "" {
		return errors.New("role display name cannot be empty")
	}
	if len([]rune(code)) > 100 {
		return errors.New("role code is too long")
	}
	if len([]rune(displayName)) > 200 {
		return errors.New("role display name is too long")
	}
	return nil
}

func tenantProjectRoleView(role *permission.Role) TenantProjectRole {
	return TenantProjectRole{
		ID: role.ID, TenantID: role.TenantID, ProjectID: role.ProjectID,
		Code: role.Name, DisplayName: role.Description,
		DefaultRouter: role.DefaultRouter, Enabled: role.Enable,
		Deletable: role.CanDel && !role.IsMust,
	}
}

// CreateTenantProjectRole creates a native UserCenter role within one tenant
// and project. Role limits remain the responsibility of the embedding product;
// passing zero to UserCenter's limit callback means unlimited.
func CreateTenantProjectRole(request CreateTenantProjectRoleRequest) (TenantProjectRole, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	request.Code = strings.TrimSpace(request.Code)
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.DefaultRouter = strings.TrimSpace(request.DefaultRouter)
	if err := validateTenantProjectRoleFields(request.TenantID, request.ProjectID, request.Code, request.DisplayName); err != nil {
		return TenantProjectRole{}, err
	}
	var count int64
	if err := DB().Model(&permission.Role{}).
		Where("tenant_id = ? AND project_id = ? AND name = ?", request.TenantID, request.ProjectID, request.Code).
		Count(&count).Error; err != nil {
		return TenantProjectRole{}, err
	}
	if count > 0 {
		return TenantProjectRole{}, errors.New("role code already exists in tenant project")
	}
	role := &permission.Role{
		TenantID: request.TenantID, ProjectID: request.ProjectID,
		Name: request.Code, Description: request.DisplayName,
		DefaultRouter: request.DefaultRouter,
	}
	role.ID = uuid.NewString()
	if err := permission.CreateRole(role, func(string) (bool, int32, error) { return false, 0, nil }); err != nil {
		return TenantProjectRole{}, err
	}
	return tenantProjectRoleView(role), nil
}

// ListTenantProjectRoles returns only roles owned by the exact tenant and
// project pair. Public and other-project roles are intentionally excluded.
func ListTenantProjectRoles(tenantID, projectID string) ([]TenantProjectRole, error) {
	tenantID = strings.TrimSpace(tenantID)
	projectID = strings.TrimSpace(projectID)
	if tenantID == "" || projectID == "" {
		return nil, errors.New("tenant ID and project ID are required")
	}
	var roles []*permission.Role
	if err := DB().Where("tenant_id = ? AND project_id = ?", tenantID, projectID).
		Order("name ASC, id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	result := make([]TenantProjectRole, 0, len(roles))
	for _, role := range roles {
		result = append(result, tenantProjectRoleView(role))
	}
	return result, nil
}

// RoleBelongsToTenantProject reports whether an enabled role belongs to the
// exact tenant/project boundary. Embedded products must use this stricter
// check before accepting a client-supplied role ID; tenant ownership alone is
// insufficient when one tenant hosts roles for multiple products.
func RoleBelongsToTenantProject(roleID, tenantID, projectID string) bool {
	roleID = strings.TrimSpace(roleID)
	tenantID = strings.TrimSpace(tenantID)
	projectID = strings.TrimSpace(projectID)
	if roleID == "" || tenantID == "" || projectID == "" {
		return false
	}
	var count int64
	if err := DB().Model(&permission.Role{}).
		Where("id = ? AND tenant_id = ? AND project_id = ? AND enable = ?", roleID, tenantID, projectID, true).
		Count(&count).Error; err != nil {
		return false
	}
	return count == 1
}

// RenameTenantProjectRole changes only the product-facing display label. The
// machine code, tenant/project ownership and authorization selections remain
// immutable through this facade.
func RenameTenantProjectRole(tenantID, projectID, roleID, displayName string) (TenantProjectRole, error) {
	tenantID = strings.TrimSpace(tenantID)
	projectID = strings.TrimSpace(projectID)
	roleID = strings.TrimSpace(roleID)
	displayName = strings.TrimSpace(displayName)
	if roleID == "" || displayName == "" {
		return TenantProjectRole{}, errors.New("role ID and display name are required")
	}
	if len([]rune(displayName)) > 200 {
		return TenantProjectRole{}, errors.New("role display name is too long")
	}
	role, err := permission.GetRoleByID(roleID)
	if err != nil {
		return TenantProjectRole{}, err
	}
	if role.TenantID != tenantID || role.ProjectID != projectID || role.Public {
		return TenantProjectRole{}, gorm.ErrRecordNotFound
	}
	role.Description = displayName
	role.RoleMenus = nil
	if err := permission.UpdateRole(role); err != nil {
		return TenantProjectRole{}, err
	}
	return tenantProjectRoleView(role), nil
}

// DeleteTenantProjectRole validates the exact ownership boundary before using
// UserCenter's native deletion workflow. Assigned and parent roles remain
// protected by the native implementation.
func DeleteTenantProjectRole(tenantID, projectID, roleID string) error {
	tenantID = strings.TrimSpace(tenantID)
	projectID = strings.TrimSpace(projectID)
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return errors.New("role ID cannot be empty")
	}
	role, err := permission.GetRoleByID(roleID)
	if err != nil {
		return err
	}
	if role.TenantID != tenantID || role.ProjectID != projectID || role.Public {
		return gorm.ErrRecordNotFound
	}
	return permission.DeleteRole(roleID)
}

func GetRoleAuthorization(roleID string) (*RoleAuthorizationDetail, error) {
	return permission.GetRoleAuthorization(roleID)
}

func PublishRoleAuthorization(roleID, baseRevision string, selections []RoleAuthorizationSelection) (*RoleAuthorizationPublishResult, error) {
	return permission.PublishRoleAuthorization(roleID, baseRevision, selections)
}

func PublishRoleAuthorizationWithABAC(roleID, baseRevision string, selections []RoleAuthorizationSelection, policy RoleAuthorizationABACPolicy) (*RoleAuthorizationPublishResult, error) {
	return permission.PublishRoleAuthorizationWithABAC(roleID, baseRevision, selections, policy)
}

func IsRoleAuthorizationRevisionConflict(err error) bool {
	return errors.Is(err, permission.ErrRoleAuthorizationRevisionConflict)
}
