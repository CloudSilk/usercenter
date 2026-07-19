package permission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/alert"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Role struct {
	commonmodel.Model
	TenantID      string      `gorm:"index;size:36"`
	ProjectID     string      `gorm:"index;size:36"`
	Name          string      `json:"name" gorm:"size:100;comment:角色名"`
	ParentID      string      `json:"parentID" gorm:"comment:父角色ID"`
	Children      []*Role     `json:"children" gorm:"-"`
	RoleMenus     []*RoleMenu `json:"roleMenus"`
	DefaultRouter string      `json:"defaultRouter" gorm:"size:100;comment:默认菜单;default:dashboard"`
	Description   string      `json:"description" gorm:"size:200;"`
	CanDel        bool        `json:"canDel" gorm:"default:1"`
	Public        bool        `gorm:"comment:是否是公共角色;default:0"`
	IsMust        bool        `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
	Enable        bool        `json:"enable" gorm:"index;default:1;comment:角色是否启用"`
}

type RoleMenu struct {
	commonmodel.Model
	RoleID string `json:"roleID" gorm:"index;comment:角色ID"`
	MenuID string `json:"menuID" gorm:"index;comment:菜单ID"`
	Funcs  string `json:"funcs" gorm:"size:500;comment:功能名称,多个以逗号隔开"`
	Show   bool
	Menu   *Menu `json:"menu"`
}

func (r *RoleMenu) GetMenuID() string  { return r.MenuID }
func (r *RoleMenu) GetFuncs() []string { return strings.Split(r.Funcs, ",") }
func (r *RoleMenu) GetShow() bool      { return r.Show }

type RoleResponse struct {
	Role Role `json:"role"`
}
type RoleCopyResponse struct {
	Role      Role   `json:"role"`
	OldRoleID string `json:"oldRoleID"`
}

func CreateRole(newRole *Role, tenantCountFn func(string) (bool, int32, error)) error {
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		if err := validateRoleParent(tx, newRole.ID, newRole.ParentID, newRole.TenantID, newRole.Public); err != nil {
			return err
		}
		count, err := statisticRoleCount(tx, newRole.TenantID)
		if err != nil {
			return err
		}
		expired, tenantRoleCount, err := tenantCountFn(newRole.TenantID)
		if err != nil {
			return err
		}
		if expired {
			return fmt.Errorf("账号使用期限已过，你可以联系管理员!")
		}
		if tenantRoleCount > 0 && tenantRoleCount <= int32(count) {
			return fmt.Errorf("只能创建 %d 个角色", tenantRoleCount)
		}
		newRole.CanDel = true
		newRole.Enable = true
		duplication, err := store.Client().CreateWithCheckDuplicationWithDB(tx, newRole, "id = ?", newRole.ID)
		if err != nil {
			return err
		}
		if duplication {
			return errors.New("存在相同角色id")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := updateRoleAuth(newRole.ID); err != nil {
		log.Error(context.Background(), err)
	}
	return nil
}

func updateRoleAuth(id string) error {
	roleDetail, err := GetFullRoleByID(id)
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if !roleDetail.Enable {
		_, err = ClearCasbin(0, id)
		return err
	}
	roleID := roleDetail.ID
	var newRules = make(map[string]*CasbinRule)
	for _, m := range roleDetail.RoleMenus {
		if m.Menu == nil {
			continue
		}
		funcs := strings.Split(m.Funcs, ",")
		for _, fn := range m.Menu.MenuFuncs {
			flag := false
			for _, f := range funcs {
				if fn.Name == f {
					flag = true
					break
				}
			}
			if !flag {
				continue
			}
			for _, api := range fn.MenuFuncApis {
				if api.API == nil || !api.API.Enable {
					continue
				}
				checkAuth := "true"
				if !api.API.CheckAuth {
					checkAuth = "false"
				}
				key := fmt.Sprintf("p-%v-%v-%v-%v", roleID, api.API.Path, api.API.Method, checkAuth)
				if _, ok := newRules[key]; ok {
					continue
				}
				newRules[key] = &CasbinRule{
					Ptype: "p", RoleID: roleID, Path: api.API.Path, Method: api.API.Method, CheckAuth: checkAuth,
				}
			}
		}
	}
	var list []*CasbinRule
	for _, r := range newRules {
		list = append(list, r)
	}
	_, err = ClearCasbin(0, roleID)
	if err != nil {
		log.Errorf(context.Background(), "ClearCasbin error: %v", err)
	}
	if len(list) > 0 {
		err = UpdateCasbin(roleID, list)
		if err != nil {
			return err
		}
	}
	return nil
}

func CopyRole(copyInfo RoleCopyResponse) (*Role, error) {
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		duplication, err := store.Client().CheckDuplication(tx.Model(&Role{}), "id = ?", copyInfo.Role.ID)
		if err != nil {
			return err
		}
		if duplication {
			return errors.New("存在相同角色id")
		}
		copyInfo.Role.Children = []*Role{}
		menus, err := GetMenuRole(copyInfo.OldRoleID)
		if err != nil {
			return err
		}
		var roleMenus []*RoleMenu
		for _, v := range menus {
			roleMenus = append(roleMenus, &RoleMenu{MenuID: v.MenuID, RoleID: copyInfo.Role.ID, Funcs: v.Funcs, Show: v.Show})
		}
		copyInfo.Role.RoleMenus = roleMenus
		err = store.DB().Create(&copyInfo.Role).Error
		if err != nil {
			return err
		}
		rules := GetPolicyPathByRoleID(copyInfo.OldRoleID)
		for i := range rules {
			rules[i].RoleID = copyInfo.Role.ID
		}
		return UpdateCasbin(copyInfo.Role.ID, rules)
	})
	return &copyInfo.Role, err
}

func UpdateRole(newRole *Role) error {
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		oldRole := &Role{}
		err := tx.Preload("RoleMenus").Preload(clause.Associations).Where("id = ?", newRole.ID).First(oldRole).Error
		if err != nil {
			return err
		}
		if err := validateRoleParent(tx, newRole.ID, newRole.ParentID, newRole.TenantID, newRole.Public); err != nil {
			return err
		}
		if newRole.RoleMenus != nil {
			var deleteRoleMenu []string
			for _, oldRM := range oldRole.RoleMenus {
				flag := false
				for _, newRM := range newRole.RoleMenus {
					if newRM.ID == oldRM.ID {
						flag = true
					}
				}
				if !flag {
					deleteRoleMenu = append(deleteRoleMenu, oldRM.ID)
				}
			}
			if len(deleteRoleMenu) > 0 {
				err = tx.Unscoped().Delete(&RoleMenu{}, "id in ?", deleteRoleMenu).Error
				if err != nil {
					return err
				}
			}
			for _, m := range newRole.RoleMenus {
				m.RoleID = newRole.ID
				err = tx.Omit("created_at").Save(m).Error
				if err != nil {
					return err
				}
			}
		}
		if newRole.TenantID == "" {
			err = tx.Exec("update roles set tenant_id=NULL,name=?,parent_id=?,description=?,default_router=?,`public`=?,updated_at=? where id=?", newRole.Name, newRole.ParentID, newRole.Description, newRole.DefaultRouter, newRole.Public, time.Now(), newRole.ID).Error
		} else {
			err = tx.Exec("update roles set tenant_id=?,name=?,parent_id=?,description=?,default_router=?,`public`=?,updated_at=? where id=?", newRole.TenantID, newRole.Name, newRole.ParentID, newRole.Description, newRole.DefaultRouter, newRole.Public, time.Now(), newRole.ID).Error
		}
		return err
	})
	if err != nil {
		return err
	}
	if err := updateRoleAuth(newRole.ID); err != nil {
		log.Errorf(context.Background(), "更新角色权限失败:%v", err)
	}
	alert.FireEvent("role.updated", map[string]interface{}{
		"id": newRole.ID, "name": newRole.Name, "tenantID": newRole.TenantID,
	})
	return nil
}

func validateRoleParent(tx *gorm.DB, roleID, parentID, tenantID string, public bool) error {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return nil
	}
	if parentID == roleID {
		return errors.New("角色不能将自己设为父角色")
	}
	parent := &Role{}
	if err := tx.Where("id = ?", parentID).First(parent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("父角色不存在")
		}
		return err
	}
	if public {
		if !parent.Public {
			return errors.New("公共角色只能继承公共角色")
		}
		return nil
	}
	if !parent.Public && parent.TenantID != tenantID {
		return errors.New("父角色不属于当前租户范围")
	}
	visited := map[string]struct{}{parent.ID: {}}
	for ancestorID := strings.TrimSpace(parent.ParentID); ancestorID != ""; {
		if ancestorID == roleID {
			return errors.New("父角色关系不能形成循环")
		}
		if _, exists := visited[ancestorID]; exists {
			return errors.New("父角色关系中已存在循环")
		}
		visited[ancestorID] = struct{}{}
		ancestor := &Role{}
		if err := tx.Select("id", "parent_id").Where("id = ?", ancestorID).First(ancestor).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.New("父角色层级中存在无效角色")
			}
			return err
		}
		ancestorID = strings.TrimSpace(ancestor.ParentID)
	}
	return nil
}

// SetRoleEnabled changes the native role state, revokes every affected login
// context and synchronizes Casbin policies. System roles cannot be disabled.
func SetRoleEnabled(roleID string, enable bool) error {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return errors.New("角色 ID 不能为空")
	}
	var affectedUserIDs []string
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		role := &Role{}
		if err := tx.Where("id = ?", roleID).First(role).Error; err != nil {
			return err
		}
		if role.Enable == enable {
			return nil
		}
		if !enable && (role.IsMust || !role.CanDel) {
			return errors.New("系统角色不允许停用")
		}
		if err := tx.Model(&Role{}).Where("id = ?", roleID).Update("enable", enable).Error; err != nil {
			return err
		}
		if err := tx.Table("user_roles").
			Where("role_id = ?", roleID).
			Distinct("user_id").
			Pluck("user_id", &affectedUserIDs).Error; err != nil {
			return err
		}
		if len(affectedUserIDs) > 0 {
			if err := tx.Table("user_session").
				Where("principal_id IN ? AND revoked = ?", affectedUserIDs, false).
				Updates(map[string]interface{}{
					"revoked":        true,
					"revoked_reason": "role state updated",
				}).Error; err != nil {
				return err
			}
			if token.DefaultTokenCache != nil {
				for _, userID := range affectedUserIDs {
					if err := token.DefaultTokenCache.DelByUserID(userID); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := updateRoleAuth(roleID); err != nil {
		return err
	}
	alert.FireEvent("role.updated", map[string]interface{}{
		"id": roleID, "enable": enable,
	})
	return nil
}

func DeleteRole(roleID string) (err error) {
	return store.DB().Transaction(func(tx *gorm.DB) error {
		// 解耦:直接查 user_roles 表,不依赖 UserRole struct
		var userRoleCount int64
		if err := tx.Table("user_roles").Where("role_id = ?", roleID).Count(&userRoleCount).Error; err != nil {
			return err
		}
		if userRoleCount > 0 {
			return errors.New("此角色有用户正在使用禁止删除")
		}
		duplication, err := store.Client().CheckDuplication(tx.Model(&Role{}), "parent_id = ?", roleID)
		if err != nil {
			return err
		}
		if duplication {
			return errors.New("此角色存在子角色不允许删除")
		}
		oldRole, err := GetRoleByID(roleID)
		if err != nil {
			return err
		}
		if !oldRole.CanDel {
			return errors.New("此角色不允许删除")
		}
		err = tx.Unscoped().Delete(&RoleMenu{}, "role_id=?", roleID).Error
		if err != nil {
			return err
		}
		err = tx.Unscoped().Delete(&Role{}, "id=?", roleID).Error
		if err != nil {
			return err
		}
		ClearCasbin(0, fmt.Sprint(roleID))
		return err
	})
}

func QueryRole(req *apipb.QueryRoleRequest, resp *apipb.QueryRoleResponse, preload bool) {
	db := store.DB().Model(&Role{})
	if req.Name != "" {
		db = db.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if len(req.Ids) > 0 {
		db = db.Where("id in ?", req.Ids)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`name`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}
	var list []*Role
	if preload {
		resp.Records, resp.Pages, err = store.Client().PageQueryWithPreload(db, req.PageSize, req.PageIndex, orderStr, []string{"RoleMenus", clause.Associations}, &list)
	} else {
		resp.Records, resp.Pages, err = store.Client().PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
	}
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = RolesToPB(list)
	}
}

func GetRoleByID(id string) (*Role, error) {
	role := &Role{}
	err := store.DB().Preload("RoleMenus").Preload(clause.Associations).Where("id = ?", id).First(role).Error
	return role, err
}

func GetFullRoleByID(id string) (*Role, error) {
	role := &Role{}
	err := store.DB().Preload("RoleMenus.Menu.MenuFuncs.MenuFuncApis.API").Preload(clause.Associations).Where("id = ?", id).First(role).Error
	return role, err
}

func GetAllRole(tenantID string, containerCommon bool) (roles []*Role, err error) {
	db := store.DB()
	if containerCommon {
		db = db.Where("tenant_id = ? OR `public` = ?", tenantID, true)
	} else if tenantID != "" {
		db = db.Where("tenant_id=?", tenantID)
	}
	err = db.Find(&roles).Error
	return
}

func StatisticRoleCount(tenantID string) (int64, error) {
	return statisticRoleCount(store.DB(), tenantID)
}

func statisticRoleCount(db *gorm.DB, tenantID string) (int64, error) {
	db = db.Model(&Role{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	var count int64
	err := db.Count(&count).Error
	return count, err
}

func ExportAllRoles(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := store.DB().Model(&Role{}).Preload("RoleMenus.Menu.MenuFuncs.MenuFuncApis.API").Preload(clause.Associations)
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	var list []*Role
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

// --- menu tree helpers(调 internal/menu)---

func getBaseChildrenList(m *Menu, treeMap map[string][]*Menu) (err error) {
	m.Children = treeMap[m.ID]
	for i := 0; i < len(m.Children); i++ {
		getBaseChildrenList(m.Children[i], treeMap)
	}
	return err
}

func getBaseMenuTreeMap() (treeMap map[string][]*Menu, err error) {
	var allMenus []*Menu
	treeMap = make(map[string][]*Menu)
	err = store.DB().Order("sort").Preload("MenuFuncs").Find(&allMenus).Error
	for _, v := range allMenus {
		treeMap[v.ParentID] = append(treeMap[v.ParentID], v)
	}
	return treeMap, err
}

func GetBaseMenuTree() (menus []*Menu, err error) {
	treeMap, err := getBaseMenuTreeMap()
	menus = treeMap[""]
	for i := 0; i < len(menus); i++ {
		getBaseChildrenList(menus[i], treeMap)
	}
	return menus, err
}

func GetMenuRole(roleID string) (menus []RoleMenu, err error) {
	err = store.DB().Where("role_id = ? ", roleID).Order("sort").Find(&menus).Error
	return menus, err
}

// --- role PB 转换(从 model/convert.go 迁入)---

func PBToRole(in *apipb.RoleInfo) *Role {
	if in == nil {
		return nil
	}
	return &Role{
		Model:         commonmodel.Model{ID: in.Id},
		TenantID:      in.TenantID,
		ProjectID:     in.ProjectID,
		Name:          in.Name,
		ParentID:      in.ParentID,
		DefaultRouter: in.DefaultRouter,
		Description:   in.Description,
		CanDel:        in.CanDel,
		RoleMenus:     PBToRoleMenus(in.RoleMenus),
		Public:        in.Public,
		IsMust:        in.IsMust,
		Enable:        in.Enable,
	}
}

func RoleToPB(in *Role) *apipb.RoleInfo {
	if in == nil {
		return nil
	}
	var children []*apipb.RoleInfo
	if len(in.Children) > 0 {
		children = RolesToPB(in.Children)
	}
	role := &apipb.RoleInfo{
		Id:            in.ID,
		TenantID:      in.TenantID,
		ProjectID:     in.ProjectID,
		Name:          in.Name,
		ParentID:      in.ParentID,
		DefaultRouter: in.DefaultRouter,
		Description:   in.Description,
		CanDel:        in.CanDel,
		RoleMenus:     RoleMenusToPB(in.RoleMenus),
		Children:      children,
		Public:        in.Public,
		IsMust:        in.IsMust,
		Enable:        in.Enable,
	}
	return role
}

func RolesToPB(in []*Role) []*apipb.RoleInfo {
	var list []*apipb.RoleInfo
	for _, role := range in {
		list = append(list, RoleToPB(role))
	}
	return list
}

func PBToRoleMenus(roleMenus []*apipb.RoleMenu) []*RoleMenu {
	var list []*RoleMenu
	for _, rm := range roleMenus {
		list = append(list, &RoleMenu{
			Model:  commonmodel.Model{ID: rm.Id},
			RoleID: rm.RoleID,
			MenuID: rm.MenuID,
			Funcs:  rm.Funcs,
			Show:   rm.Show,
			Menu:   PBToMenu(rm.Menu),
		})
	}
	return list
}

func RoleMenusToPB(roleMenus []*RoleMenu) []*apipb.RoleMenu {
	var list []*apipb.RoleMenu
	for _, rm := range roleMenus {
		list = append(list, &apipb.RoleMenu{
			Id:     rm.ID,
			RoleID: rm.RoleID,
			MenuID: rm.MenuID,
			Funcs:  rm.Funcs,
			Show:   rm.Show,
			Menu:   MenuToPB(rm.Menu),
		})
	}
	return list
}
