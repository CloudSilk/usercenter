package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/pkg/utils/log"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Role struct {
	model.Model
	TenantID      string      `gorm:"index;size:36"`
	ProjectID     string      `gorm:"index;size:36"`
	Name          string      `json:"name" gorm:"size:100;comment:角色名"`
	ParentID      string      `json:"parentID" gorm:"comment:父角色ID"`
	Children      []*Role     `json:"children" gorm:"-"`
	RoleMenus     []*RoleMenu `json:"roleMenus"`
	DefaultRouter string      `json:"defaultRouter" gorm:"size:100;comment:默认菜单;default:dashboard"`
	Description   string      `json:"description" gorm:"size:200;"`
	CanDel        bool        `json:"canDel" gorm:"default:1"`
	Tenant        *Tenant     `json:"tenant"`
	Public        bool        `gorm:"comment:是否是公共角色;default:0"`
	IsMust        bool        `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

type RoleMenu struct {
	model.Model
	RoleID string `json:"roleID" gorm:"index;comment:角色ID"`
	MenuID string `json:"menuID" gorm:"index;comment:菜单ID"`
	Funcs  string `json:"funcs" gorm:"size:500;comment:功能名称,多个以逗号隔开"`
	Show   bool
	Menu   *Menu `json:"menu"`
}

func (r *RoleMenu) GetMenuID() string {
	return r.MenuID
}
func (r *RoleMenu) GetFuncs() []string {
	return strings.Split(r.Funcs, ",")
}
func (r *RoleMenu) GetShow() bool {
	return r.Show
}

func CreateRole(newRole *Role) error {
	err := dbClient.DB().Transaction(func(tx *gorm.DB) error {
		count, err := statisticRoleCount(tx, newRole.TenantID)
		if err != nil {
			return err
		}

		expired, tenantRoleCount, err := getTenantRoleCount(tx, newRole.TenantID)
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
		duplication, err := dbClient.CreateWithCheckDuplicationWithDB(tx, newRole, "id = ?", newRole.ID)
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
	err = updateRoleAuth(newRole.ID)
	if err != nil {
		fmt.Println(err)
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
	roleID := roleDetail.ID
	var newRules = make(map[string]*CasbinRule)
	for _, menu := range roleDetail.RoleMenus {
		funcs := strings.Split(menu.Funcs, ",")
		for _, fn := range menu.Menu.MenuFuncs {
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
				_, ok := newRules[key]
				if ok {
					continue
				}
				newRules[key] = &CasbinRule{
					Ptype:     "p",
					RoleID:    roleID,
					Path:      api.API.Path,
					Method:    api.API.Method,
					CheckAuth: checkAuth,
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
		fmt.Println("ClearCasbin error:", err)
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
	err := dbClient.DB().Transaction(func(tx *gorm.DB) error {
		duplication, err := dbClient.CheckDuplication(tx.Model(&Role{}), "id = ?", copyInfo.Role.ID)
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
			roleMenus = append(roleMenus, &RoleMenu{
				MenuID: v.MenuID,
				RoleID: copyInfo.Role.ID,
				Funcs:  v.Funcs,
				Show:   v.Show,
			})
		}
		copyInfo.Role.RoleMenus = roleMenus

		err = dbClient.DB().Create(&copyInfo.Role).Error
		if err != nil {
			return err
		}

		roleID := copyInfo.Role.ID
		rules := GetPolicyPathByRoleID(copyInfo.OldRoleID)
		for i := range rules {
			rules[i].RoleID = roleID
		}
		return UpdateCasbin(roleID, rules)

	})

	return &copyInfo.Role, err
}

func UpdateRole(newRole *Role) error {
	err := dbClient.DB().Transaction(func(tx *gorm.DB) error {
		oldRole := &Role{}
		err := tx.Preload("RoleMenus").Preload(clause.Associations).Where("id = ?", newRole.ID).First(oldRole).Error
		if err != nil {
			return err
		}
		var deleteRoleMenu []string
		for _, oldRoleMenu := range oldRole.RoleMenus {
			flag := false
			for _, newRoleMenu := range newRole.RoleMenus {
				if newRoleMenu.ID == oldRoleMenu.ID {
					flag = true
				}
			}
			if !flag {
				deleteRoleMenu = append(deleteRoleMenu, oldRoleMenu.ID)
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
			// if m.Show {
			// 	fmt.Println("===========>", m.MenuID)
			// }
			err = tx.Omit("created_at").Save(m).Error
			if err != nil {
				return err
			}
		}
		if newRole.TenantID == "" {
			err = tx.Exec("update roles set tenant_id=NULL,name=?,parent_id=?,description=?,default_router=?,`public`=?,updated_at=? where id=?", newRole.Name, newRole.ParentID, newRole.Description, newRole.DefaultRouter, newRole.Public, time.Now(), newRole.ID).Error
		} else {
			err = tx.Exec("update roles set tenant_id=?,name=?,parent_id=?,description=?,default_router=?,`public`=?,updated_at=? where id=?", newRole.TenantID, newRole.Name, newRole.ParentID, newRole.Description, newRole.DefaultRouter, newRole.Public, time.Now(), newRole.ID).Error
		}

		if err != nil {
			return err
		}

		return err
	})
	if err != nil {
		return err
	}
	err = updateRoleAuth(newRole.ID)
	if err != nil {
		log.Errorf(context.Background(), "更新角色权限失败:%v", err)
	}
	return nil
}

func DeleteRole(roleID string) (err error) {
	return dbClient.DB().Transaction(func(tx *gorm.DB) error {
		duplication, err := dbClient.CheckDuplication(tx.Model(&UserRole{}), "role_id = ?", roleID)
		if err != nil {
			return err
		}
		if duplication {
			return errors.New("此角色有用户正在使用禁止删除")
		}

		duplication, err = dbClient.CheckDuplication(tx.Model(&Role{}), "parent_id = ?", roleID)
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
	db := dbClient.DB().Model(&Role{})

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
		resp.Records, resp.Pages, err = dbClient.PageQueryWithPreload(db, req.PageSize, req.PageIndex, orderStr, []string{"RoleMenus", clause.Associations}, &list)
	} else {
		resp.Records, resp.Pages, err = dbClient.PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
	}
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = RolesToPB(list)
	}
}

func findChildrenRole(authority *Role) (err error) {
	err = dbClient.DB().Preload("RoleMenus").Where("parent_id = ?", authority.ID).Find(&authority.Children).Error
	if len(authority.Children) > 0 {
		for k := range authority.Children {
			err = findChildrenRole(authority.Children[k])
		}
	}
	return err
}

func getMenuTreeMap(roleID string) (treeMap map[string][]*Menu, err error) {
	var allMenus []*Menu
	treeMap = make(map[string][]*Menu)
	err = dbClient.DB().Where("role_id = ?", roleID).Order("sort").Preload("Parameters").Find(&allMenus).Error
	for _, v := range allMenus {
		treeMap[v.ParentID] = append(treeMap[v.ParentID], v)
	}
	return treeMap, err
}

func GetMenuTreeByRoleID(roleID string) (menus []*Menu, err error) {
	menuTree, err := getMenuTreeMap(roleID)
	menus = menuTree[""]
	for i := 0; i < len(menus); i++ {
		err = getChildrenList(menus[i], menuTree)
	}
	return menus, err
}

func getChildrenList(menu *Menu, treeMap map[string][]*Menu) (err error) {
	menu.Children = treeMap[menu.ID]
	for i := 0; i < len(menu.Children); i++ {
		err = getChildrenList(menu.Children[i], treeMap)
	}
	return err
}

func GetAuthorizedMenuTree(tenantID string) (list []*Menu, total int64, err error) {
	if tenantID != "" {
		t, err := GetTenantByID(tenantID)
		if err != nil {
			return nil, 0, err
		}
		authiruzedMenus := t.getAuthorizedMenu()
		result, err := GetAuthorizedMenu(dbClient.DB(), authiruzedMenus, false)
		return result, 0, err
	}
	var menuList []*Menu
	treeMap, err := getBaseMenuTreeMap()
	menuList = treeMap[""]
	for i := 0; i < len(menuList); i++ {
		err = getBaseChildrenList(menuList[i], treeMap)
	}
	return menuList, total, err
}

func getBaseChildrenList(menu *Menu, treeMap map[string][]*Menu) (err error) {
	menu.Children = treeMap[menu.ID]
	for i := 0; i < len(menu.Children); i++ {
		err = getBaseChildrenList(menu.Children[i], treeMap)
	}
	return err
}

func getBaseMenuTreeMap() (treeMap map[string][]*Menu, err error) {
	var allMenus []*Menu
	treeMap = make(map[string][]*Menu)
	err = dbClient.DB().Order("sort").Preload("MenuFuncs").Find(&allMenus).Error
	for _, v := range allMenus {
		treeMap[v.ParentID] = append(treeMap[v.ParentID], v)
	}
	return treeMap, err
}

func GetBaseMenuTree() (menus []*Menu, err error) {
	treeMap, err := getBaseMenuTreeMap()
	menus = treeMap[""]
	for i := 0; i < len(menus); i++ {
		err = getBaseChildrenList(menus[i], treeMap)
	}
	return menus, err
}

func GetMenuRole(roleID string) (menus []RoleMenu, err error) {
	err = dbClient.DB().Where("role_id = ? ", roleID).Order("sort").Find(&menus).Error
	return menus, err
}

type RoleResponse struct {
	Role Role `json:"role"`
}

type RoleCopyResponse struct {
	Role      Role   `json:"role"`
	OldRoleID string `json:"oldRoleID"`
}

func GetRoleByID(id string) (*Role, error) {
	role := &Role{}
	err := dbClient.DB().Preload("RoleMenus").Preload(clause.Associations).Where("id = ?", id).First(role).Error
	return role, err
}

func GetFullRoleByID(id string) (*Role, error) {
	role := &Role{}
	err := dbClient.DB().Preload("RoleMenus.Menu.MenuFuncs.MenuFuncApis.API").Preload(clause.Associations).Where("id = ?", id).First(role).Error
	return role, err
}

// GetAllRole 获取所有用户
// containerCommon true-包含公共角色 false-不包含公共角色
// 公共角色定义：不设置租户的角色
func GetAllRole(tenantID string, containerCommon bool) (roles []*Role, err error) {
	db := dbClient.DB()

	if containerCommon {
		db = db.Or("tenant_id=? or `public`=?", tenantID, containerCommon)
	} else if tenantID != "" {
		db = db.Where("tenant_id=?", tenantID)
	}

	err = db.Unscoped().Find(&roles).Error
	return
}

func StatisticRoleCount(tenantID string) (int64, error) {
	return statisticRoleCount(dbClient.DB(), tenantID)
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
	db := dbClient.DB().Model(&Role{}).Preload("RoleMenus.Menu.MenuFuncs.MenuFuncApis.API").Preload(clause.Associations)

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
