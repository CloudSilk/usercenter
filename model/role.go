package model

import (
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type Role = permission.Role
type RoleMenu = permission.RoleMenu
type RoleResponse = permission.RoleResponse
type RoleCopyResponse = permission.RoleCopyResponse

func CreateRole(r *Role) error       { return permission.CreateRole(r, getTenantRoleCount) }
func UpdateRole(r *Role) error       { return permission.UpdateRole(r) }
func DeleteRole(roleID string) error { return permission.DeleteRole(roleID) }
func QueryRole(req *apipb.QueryRoleRequest, resp *apipb.QueryRoleResponse, preload bool) {
	permission.QueryRole(req, resp, preload)
}
func GetAllRole(tenantID string, containerCommon bool) ([]*Role, error) {
	return permission.GetAllRole(tenantID, containerCommon)
}
func GetRoleByID(id string) (*Role, error)          { return permission.GetRoleByID(id) }
func GetFullRoleByID(id string) (*Role, error)      { return permission.GetFullRoleByID(id) }
func CopyRole(info RoleCopyResponse) (*Role, error) { return permission.CopyRole(info) }
func StatisticRoleCount(tenantID string) (int64, error) {
	return permission.StatisticRoleCount(tenantID)
}
func ExportAllRoles(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	permission.ExportAllRoles(req, resp)
}
func GetAuthorizedMenuTree(tenantID string) ([]*permission.Menu, int64, error) {
	if tenantID != "" {
		t, err := GetTenantByID(tenantID)
		if err != nil {
			return nil, 0, err
		}
		authMenus := t.GetAuthorizedMenu()
		result, err := permission.GetAuthorizedMenu(store.DB(), authMenus, false)
		return result, 0, err
	}
	menus, err := permission.GetBaseMenuTree()
	return menus, 0, err
}
func GetBaseMenuTree() ([]*permission.Menu, error)  { return permission.GetBaseMenuTree() }
func GetMenuRole(roleID string) ([]RoleMenu, error) { return permission.GetMenuRole(roleID) }

// role PB 委托
func PBToRole(in *apipb.RoleInfo) *Role              { return permission.PBToRole(in) }
func RoleToPB(in *Role) *apipb.RoleInfo              { return permission.RoleToPB(in) }
func RolesToPB(in []*Role) []*apipb.RoleInfo         { return permission.RolesToPB(in) }
func PBToRoleMenus(in []*apipb.RoleMenu) []*RoleMenu { return permission.PBToRoleMenus(in) }
func RoleMenusToPB(in []*RoleMenu) []*apipb.RoleMenu { return permission.RoleMenusToPB(in) }
