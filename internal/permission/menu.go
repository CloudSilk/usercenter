package permission

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Menu struct {
	commonmodel.Model
	TenantID    string           `json:"tenantID" gorm:"size:36;index"`
	ProjectID   string           `json:"projectID" gorm:"index;size:36"`
	Level       uint32           `json:"level"`
	ParentID    string           `json:"parentID" gorm:"comment:父菜单ID"`
	Path        string           `json:"path" gorm:"size:200;comment:路由path"`
	Name        string           `json:"name" gorm:"size:100;comment:路由name"`
	Hidden      bool             `json:"hidden" gorm:"comment:是否在列表隐藏"`
	Component   string           `json:"component" gorm:"size:200;comment:对应前端文件路径"`
	Sort        int32            `json:"sort" gorm:"comment:排序标记"`
	Cache       bool             `json:"cache" gorm:"comment:是否缓存"`
	DefaultMenu bool             `json:"defaultMenu" gorm:"comment:是否是基础路由（开发中）"`
	Title       string           `json:"title" gorm:"size:100;comment:菜单名"`
	Icon        string           `json:"icon" gorm:"size:100;comment:菜单图标"`
	CloseTab    bool             `json:"closeTab" gorm:"comment:自动关闭tab"`
	IsMust      bool             `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
	Children    []*Menu          `json:"children" gorm:"-"`
	Parameters  []*MenuParameter `json:"parameters"`
	MenuFuncs   []*MenuFunc      `json:"menuFuncs"`
}

type MenuParameter struct {
	commonmodel.Model
	MenuID string `json:"menuID" gorm:"index"`
	Type   string `json:"type" gorm:"size:50;comment:地址栏携带参数为params还是query"`
	Key    string `json:"key" gorm:"size:100;comment:地址栏携带参数的key"`
	Value  string `json:"value" gorm:"size:200;comment:地址栏携带参数的值"`
}

type MenuFunc struct {
	commonmodel.Model
	MenuID       string        `json:"menuID" gorm:"index"`
	Name         string        `json:"name" gorm:"size:100;comment:功能名称"`
	Title        string        `json:"title" gorm:"size:100;comment:显示名称"`
	Hidden       bool          `json:"hidden" gorm:"comment:是否隐藏"`
	MenuFuncApis []MenuFuncApi `json:"menuFuncApis"`
}

type MenuFuncApi struct {
	commonmodel.Model
	MenuFuncID string `json:"menuFuncID" gorm:"index"`
	APIID      string `json:"apiID" gorm:"column:api_id"`
	API        *API   `json:"apiInfo"`
}

var (
	ErrInvalidMenu   = errors.New("invalid menu")
	ErrProtectedMenu = errors.New("system menu is protected")
	ErrMenuHasChild  = errors.New("menu still has child menus")
)

type MenuImpact struct {
	MenuID             string `json:"menuID"`
	DirectChildCount   int64  `json:"directChildCount"`
	FunctionCount      int64  `json:"functionCount"`
	APIBindingCount    int64  `json:"apiBindingCount"`
	ActiveRoleCount    int64  `json:"activeRoleCount"`
	AffectedUserCount  int64  `json:"affectedUserCount"`
	CanDelete          bool   `json:"canDelete"`
	DeleteBlockReason  string `json:"deleteBlockReason,omitempty"`
	HasStructuralRisk  bool   `json:"hasStructuralRisk"`
	HasPermissionScope bool   `json:"hasPermissionScope"`
}

func normalizeMenu(menu *Menu) error {
	if menu == nil {
		return fmt.Errorf("%w: request body is required", ErrInvalidMenu)
	}
	menu.ID = strings.TrimSpace(menu.ID)
	menu.TenantID = strings.TrimSpace(menu.TenantID)
	menu.ProjectID = strings.TrimSpace(menu.ProjectID)
	menu.ParentID = strings.TrimSpace(menu.ParentID)
	menu.Path = strings.TrimSpace(menu.Path)
	menu.Name = strings.TrimSpace(menu.Name)
	menu.Component = strings.TrimSpace(menu.Component)
	menu.Title = strings.TrimSpace(menu.Title)
	menu.Icon = strings.TrimSpace(menu.Icon)
	if menu.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidMenu)
	}
	if len(menu.Name) > 100 {
		return fmt.Errorf("%w: name cannot exceed 100 characters", ErrInvalidMenu)
	}
	if len(menu.Title) > 100 {
		return fmt.Errorf("%w: title cannot exceed 100 characters", ErrInvalidMenu)
	}
	if len(menu.Icon) > 100 {
		return fmt.Errorf("%w: icon cannot exceed 100 characters", ErrInvalidMenu)
	}
	if len(menu.Component) > 200 {
		return fmt.Errorf("%w: component cannot exceed 200 characters", ErrInvalidMenu)
	}
	if len(menu.Path) > 200 {
		return fmt.Errorf("%w: path cannot exceed 200 characters", ErrInvalidMenu)
	}
	if menu.Path != "" &&
		(!strings.HasPrefix(menu.Path, "/") || strings.ContainsAny(menu.Path, " \t\r\n?#")) {
		return fmt.Errorf("%w: path must start with / and cannot contain spaces, query strings or fragments", ErrInvalidMenu)
	}
	return nil
}

func menuIdentityExists(tx *gorm.DB, menu *Menu) (bool, error) {
	query := tx.Model(&Menu{}).
		Where("id <> ? AND tenant_id = ? AND project_id = ?", menu.ID, menu.TenantID, menu.ProjectID).
		Where("name = ?", menu.Name)
	if menu.Path != "" {
		query = tx.Model(&Menu{}).
			Where("id <> ? AND tenant_id = ? AND project_id = ?", menu.ID, menu.TenantID, menu.ProjectID).
			Where("name = ? OR path = ?", menu.Name, menu.Path)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func validateMenuParent(tx *gorm.DB, menuID, parentID, tenantID, projectID string) (uint32, error) {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return 0, nil
	}
	if parentID == menuID {
		return 0, fmt.Errorf("%w: a menu cannot be its own parent", ErrInvalidMenu)
	}
	parent := &Menu{}
	if err := tx.Where("id = ?", parentID).First(parent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("%w: parent menu does not exist", ErrInvalidMenu)
		}
		return 0, err
	}
	if parent.TenantID != tenantID || parent.ProjectID != projectID {
		return 0, fmt.Errorf("%w: parent menu must belong to the same tenant and project", ErrInvalidMenu)
	}
	visited := map[string]struct{}{parent.ID: {}}
	for ancestorID := strings.TrimSpace(parent.ParentID); ancestorID != ""; {
		if ancestorID == menuID {
			return 0, fmt.Errorf("%w: parent relationship cannot form a cycle", ErrInvalidMenu)
		}
		if _, exists := visited[ancestorID]; exists {
			return 0, fmt.Errorf("%w: existing parent relationship contains a cycle", ErrInvalidMenu)
		}
		visited[ancestorID] = struct{}{}
		ancestor := &Menu{}
		if err := tx.Select("id", "parent_id", "tenant_id", "project_id").
			Where("id = ?", ancestorID).
			First(ancestor).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, fmt.Errorf("%w: parent hierarchy contains a missing menu", ErrInvalidMenu)
			}
			return 0, err
		}
		if ancestor.TenantID != tenantID || ancestor.ProjectID != projectID {
			return 0, fmt.Errorf("%w: parent hierarchy crosses tenant or project boundaries", ErrInvalidMenu)
		}
		ancestorID = strings.TrimSpace(ancestor.ParentID)
	}
	return parent.Level + 1, nil
}

func updateMenuDescendantLevels(tx *gorm.DB, parentID string, parentLevel uint32, visited map[string]struct{}) error {
	var children []*Menu
	if err := tx.Select("id", "parent_id", "level").
		Where("parent_id = ?", parentID).
		Order("sort, title, name").
		Find(&children).Error; err != nil {
		return err
	}
	for _, child := range children {
		if _, exists := visited[child.ID]; exists {
			return fmt.Errorf("%w: menu hierarchy contains a cycle", ErrInvalidMenu)
		}
		visited[child.ID] = struct{}{}
		child.Level = parentLevel + 1
		if err := tx.Model(&Menu{}).Where("id = ?", child.ID).Update("level", child.Level).Error; err != nil {
			return err
		}
		if err := updateMenuDescendantLevels(tx, child.ID, child.Level, visited); err != nil {
			return err
		}
	}
	return nil
}

func menuRoleIDs(tx *gorm.DB, menuID string) ([]string, error) {
	var roleIDs []string
	err := tx.Model(&RoleMenu{}).
		Where("menu_id = ?", menuID).
		Distinct("role_id").
		Pluck("role_id", &roleIDs).Error
	return uniqueStrings(roleIDs), err
}

func revokeMenuUsers(tx *gorm.DB, roleIDs []string, reason string) ([]string, error) {
	roleIDs = uniqueStrings(roleIDs)
	if len(roleIDs) == 0 {
		return []string{}, nil
	}
	var userIDs []string
	if err := tx.Table("user_roles").
		Where("role_id IN ?", roleIDs).
		Distinct("user_id").
		Pluck("user_id", &userIDs).Error; err != nil {
		return nil, err
	}
	if len(userIDs) == 0 {
		return userIDs, nil
	}
	err := tx.Table("user_session").
		Where("principal_id IN ? AND revoked = ?", userIDs, false).
		Updates(map[string]interface{}{
			"revoked":        true,
			"revoked_reason": reason,
		}).Error
	return userIDs, err
}

func clearMenuTokenCache(userIDs []string) error {
	if token.DefaultTokenCache == nil {
		return nil
	}
	for _, userID := range uniqueStrings(userIDs) {
		if err := token.DefaultTokenCache.DelByUserID(userID); err != nil {
			return err
		}
	}
	return nil
}

func AddMenu(menu *Menu) error {
	if err := normalizeMenu(menu); err != nil {
		return err
	}
	return store.DB().Transaction(func(tx *gorm.DB) error {
		level, err := validateMenuParent(tx, menu.ID, menu.ParentID, menu.TenantID, menu.ProjectID)
		if err != nil {
			return err
		}
		menu.Level = level
		duplicate, err := menuIdentityExists(tx, menu)
		if err != nil {
			return err
		}
		if duplicate {
			return fmt.Errorf("%w: menu name or route path already exists in this tenant and project", ErrInvalidMenu)
		}
		return tx.Create(menu).Error
	})
}

func DeleteMenu(id string) (err error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: menu ID cannot be empty", ErrInvalidMenu)
	}
	var affectedUserIDs []string
	err = store.DB().Transaction(func(tx *gorm.DB) error {
		menu := &Menu{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("MenuFuncs.MenuFuncApis.API").
			Preload(clause.Associations).
			Where("id = ?", id).
			First(menu).Error; err != nil {
			return err
		}
		if menu.IsMust {
			return ErrProtectedMenu
		}
		var childCount int64
		if err := tx.Model(&Menu{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
			return err
		}
		if childCount > 0 {
			return fmt.Errorf("%w: move or delete %d child menus first", ErrMenuHasChild, childCount)
		}
		affectedRoleIDs, err := menuRoleIDs(tx, id)
		if err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&RoleMenu{}, "menu_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&MenuParameter{}, "menu_id = ?", id).Error; err != nil {
			return err
		}
		var menuFuncIDs []string
		for _, menuFunc := range menu.MenuFuncs {
			menuFuncIDs = append(menuFuncIDs, menuFunc.ID)
		}
		if len(menuFuncIDs) > 0 {
			err = tx.Unscoped().Delete(&MenuFuncApi{}, "menu_func_id in ?", menuFuncIDs).Error
			if err != nil {
				return err
			}
		}
		if err := tx.Unscoped().Delete(&MenuFunc{}, "menu_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&Menu{}, "id = ?", id).Error; err != nil {
			return err
		}
		if err := rebuildRoleAuthorizationPolicies(tx, affectedRoleIDs); err != nil {
			return err
		}
		if len(affectedRoleIDs) == 0 {
			return nil
		}
		affectedUserIDs, err = revokeMenuUsers(tx, affectedRoleIDs, "role menu deleted")
		return err
	})
	if err != nil {
		return err
	}
	if err := ReloadCasbinPolicy(); err != nil {
		return err
	}
	return clearMenuTokenCache(affectedUserIDs)
}

func syncMenuAssociations(tx *gorm.DB, current, incoming *Menu) error {
	if incoming.Parameters != nil {
		incomingIDs := make(map[string]struct{}, len(incoming.Parameters))
		for _, parameter := range incoming.Parameters {
			if parameter.ID != "" {
				incomingIDs[parameter.ID] = struct{}{}
			}
		}
		var deleteIDs []string
		for _, parameter := range current.Parameters {
			if _, keep := incomingIDs[parameter.ID]; !keep {
				deleteIDs = append(deleteIDs, parameter.ID)
			}
		}
		if len(deleteIDs) > 0 {
			if err := tx.Unscoped().Delete(&MenuParameter{}, "id IN ?", deleteIDs).Error; err != nil {
				return err
			}
		}
		for _, parameter := range incoming.Parameters {
			parameter.MenuID = incoming.ID
			if err := tx.Omit("created_at").Save(parameter).Error; err != nil {
				return err
			}
		}
	}
	if incoming.MenuFuncs == nil {
		return nil
	}
	incomingFunctionIDs := make(map[string]struct{}, len(incoming.MenuFuncs))
	for _, function := range incoming.MenuFuncs {
		if function.ID != "" {
			incomingFunctionIDs[function.ID] = struct{}{}
		}
	}
	var deleteFunctionIDs []string
	for _, function := range current.MenuFuncs {
		if _, keep := incomingFunctionIDs[function.ID]; !keep {
			deleteFunctionIDs = append(deleteFunctionIDs, function.ID)
		}
	}
	if len(deleteFunctionIDs) > 0 {
		if err := tx.Unscoped().Delete(&MenuFuncApi{}, "menu_func_id IN ?", deleteFunctionIDs).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&MenuFunc{}, "id IN ?", deleteFunctionIDs).Error; err != nil {
			return err
		}
	}
	currentByID := make(map[string]*MenuFunc, len(current.MenuFuncs))
	for _, function := range current.MenuFuncs {
		currentByID[function.ID] = function
	}
	for _, function := range incoming.MenuFuncs {
		function.MenuID = incoming.ID
		if err := tx.Omit("created_at", "MenuFuncApis").Save(function).Error; err != nil {
			return err
		}
		if function.MenuFuncApis == nil {
			continue
		}
		incomingLinkIDs := make(map[string]struct{}, len(function.MenuFuncApis))
		for _, link := range function.MenuFuncApis {
			if link.ID != "" {
				incomingLinkIDs[link.ID] = struct{}{}
			}
		}
		var deleteLinkIDs []string
		if existing := currentByID[function.ID]; existing != nil {
			for _, link := range existing.MenuFuncApis {
				if _, keep := incomingLinkIDs[link.ID]; !keep {
					deleteLinkIDs = append(deleteLinkIDs, link.ID)
				}
			}
		}
		if len(deleteLinkIDs) > 0 {
			if err := tx.Unscoped().Delete(&MenuFuncApi{}, "id IN ?", deleteLinkIDs).Error; err != nil {
				return err
			}
		}
		for i := range function.MenuFuncApis {
			function.MenuFuncApis[i].MenuFuncID = function.ID
			if err := tx.Omit("created_at", "API").Save(&function.MenuFuncApis[i]).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func UpdateMenu(menu *Menu) (err error) {
	if err := normalizeMenu(menu); err != nil {
		return err
	}
	if menu.ID == "" {
		return fmt.Errorf("%w: menu ID cannot be empty", ErrInvalidMenu)
	}
	var affectedUserIDs []string
	reloadPolicies := false
	err = store.DB().Transaction(func(tx *gorm.DB) error {
		current := &Menu{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("MenuFuncs.MenuFuncApis.API").
			Preload(clause.Associations).
			Where("id = ?", menu.ID).
			First(current).Error; err != nil {
			return err
		}
		menu.TenantID = current.TenantID
		menu.ProjectID = current.ProjectID
		menu.IsMust = current.IsMust
		level, err := validateMenuParent(tx, menu.ID, menu.ParentID, menu.TenantID, menu.ProjectID)
		if err != nil {
			return err
		}
		menu.Level = level
		structuralChanged := current.ParentID != menu.ParentID ||
			current.Path != menu.Path ||
			current.Name != menu.Name ||
			current.Hidden != menu.Hidden ||
			current.Component != menu.Component ||
			current.DefaultMenu != menu.DefaultMenu ||
			menu.Parameters != nil
		if current.IsMust && (structuralChanged || menu.MenuFuncs != nil) {
			return ErrProtectedMenu
		}
		duplicate, err := menuIdentityExists(tx, menu)
		if err != nil {
			return err
		}
		if duplicate {
			return fmt.Errorf("%w: menu name or route path already exists in this tenant and project", ErrInvalidMenu)
		}
		affectedRoleIDs, err := menuRoleIDs(tx, menu.ID)
		if err != nil {
			return err
		}
		if err := syncMenuAssociations(tx, current, menu); err != nil {
			return err
		}
		updates := map[string]interface{}{
			"level":        menu.Level,
			"parent_id":    menu.ParentID,
			"path":         menu.Path,
			"name":         menu.Name,
			"hidden":       menu.Hidden,
			"component":    menu.Component,
			"sort":         menu.Sort,
			"cache":        menu.Cache,
			"default_menu": menu.DefaultMenu,
			"title":        menu.Title,
			"icon":         menu.Icon,
			"close_tab":    menu.CloseTab,
		}
		if err := tx.Model(&Menu{}).Where("id = ?", menu.ID).Updates(updates).Error; err != nil {
			return err
		}
		if current.Level != menu.Level || current.ParentID != menu.ParentID {
			if err := updateMenuDescendantLevels(tx, menu.ID, menu.Level, map[string]struct{}{menu.ID: {}}); err != nil {
				return err
			}
		}
		reloadPolicies = menu.MenuFuncs != nil
		if reloadPolicies {
			if err := rebuildRoleAuthorizationPolicies(tx, affectedRoleIDs); err != nil {
				return err
			}
		}
		if structuralChanged || reloadPolicies {
			affectedUserIDs, err = revokeMenuUsers(tx, affectedRoleIDs, "menu updated")
		}
		return err
	})
	if err != nil {
		return err
	}
	if reloadPolicies {
		if err := ReloadCasbinPolicy(); err != nil {
			return err
		}
	}
	return clearMenuTokenCache(affectedUserIDs)
}

func GetMenuByID(id string) (*Menu, error) {
	menu := &Menu{}
	err := store.DB().
		Preload("MenuFuncs.MenuFuncApis.API").
		Preload(clause.Associations).
		Where("id = ?", strings.TrimSpace(id)).
		First(menu).Error
	return menu, err
}

func QueryMenu(req *apipb.QueryMenuRequest, resp *apipb.QueryMenuResponse, preload bool) {
	QueryMenus(req, resp, preload, "", nil)
}

func QueryMenus(
	req *apipb.QueryMenuRequest,
	resp *apipb.QueryMenuResponse,
	preload bool,
	keyword string,
	hidden *bool,
) {
	db := store.DB().Model(&Menu{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where(
			"name LIKE ? OR title LIKE ? OR path LIKE ? OR component LIKE ? OR icon LIKE ?",
			like, like, like, like, like,
		)
	}
	if req.Name != "" {
		db = db.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Path != "" {
		db = db.Where("path LIKE ?", "%"+req.Path+"%")
	}
	if req.Title != "" {
		db = db.Where("title LIKE ?", "%"+req.Title+"%")
	}
	if req.ParentID != "" {
		db = db.Where("`parent_id` = ?", req.ParentID)
	}
	if req.Level != 0 {
		db = db.Where("`level` = ?", req.Level)
	}
	if len(req.Ids) > 0 {
		db = db.Where("id in ?", req.Ids)
	}
	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	if hidden != nil {
		db = db.Where("hidden = ?", *hidden)
	}
	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`sort`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}
	var list []*Menu
	if preload {
		resp.Records, resp.Pages, err = store.Client().PageQueryWithPreload(db, req.PageSize, req.PageIndex, orderStr, []string{"MenuFuncs.MenuFuncApis", "Parameters", clause.Associations}, &list)
	} else {
		resp.Records, resp.Pages, err = store.Client().PageQuery(db, req.PageSize, req.PageIndex, orderStr, &list, nil)
	}
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.Data = MenusToPB(list)
	}
	resp.Total = resp.Records
}

func GetMenuImpact(id string) (*MenuImpact, error) {
	menu, err := GetMenuByID(id)
	if err != nil {
		return nil, err
	}
	impact := &MenuImpact{MenuID: menu.ID}
	db := store.DB()
	if err := db.Model(&Menu{}).
		Where("parent_id = ?", menu.ID).
		Count(&impact.DirectChildCount).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&MenuFunc{}).
		Where("menu_id = ?", menu.ID).
		Count(&impact.FunctionCount).Error; err != nil {
		return nil, err
	}
	if err := db.Table("menu_func_apis AS link").
		Joins("JOIN menu_funcs AS function ON function.id = link.menu_func_id AND function.deleted_at IS NULL").
		Where("function.menu_id = ? AND link.deleted_at IS NULL", menu.ID).
		Count(&impact.APIBindingCount).Error; err != nil {
		return nil, err
	}
	roleIDs, err := menuRoleIDs(db, menu.ID)
	if err != nil {
		return nil, err
	}
	var activeRoleIDs []string
	if len(roleIDs) > 0 {
		if err := db.Model(&Role{}).
			Where("id IN ? AND enable = ?", roleIDs, true).
			Pluck("id", &activeRoleIDs).Error; err != nil {
			return nil, err
		}
	}
	impact.ActiveRoleCount = int64(len(activeRoleIDs))
	if len(activeRoleIDs) > 0 {
		if err := db.Table("user_roles").
			Where("role_id IN ?", activeRoleIDs).
			Distinct("user_id").
			Count(&impact.AffectedUserCount).Error; err != nil {
			return nil, err
		}
	}
	impact.HasStructuralRisk = impact.DirectChildCount > 0
	impact.HasPermissionScope = impact.FunctionCount > 0 ||
		impact.APIBindingCount > 0 ||
		impact.ActiveRoleCount > 0 ||
		impact.AffectedUserCount > 0
	impact.CanDelete = !menu.IsMust && impact.DirectChildCount == 0
	if menu.IsMust {
		impact.DeleteBlockReason = "system menus cannot be deleted"
	} else if impact.DirectChildCount > 0 {
		impact.DeleteBlockReason = fmt.Sprintf(
			"move or delete %d child menus before deletion",
			impact.DirectChildCount,
		)
	}
	return impact, nil
}

func GetAllMenus(req *apipb.QueryMenuRequest) (menus []*Menu, err error) {
	db := store.DB()
	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	err = db.Find(&menus).Error
	return
}

func ExportAllMenus(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	db := store.DB().Model(&Menu{}).Preload("MenuFuncs.MenuFuncApis.API").Preload(clause.Associations)
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", req.IsMust)
	}
	var list []*Menu
	if err := db.Find(&list).Error; err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		buf, _ := json.Marshal(list)
		resp.Data = string(buf)
	}
}

// --- PB 转换(从 model/menu_convert.go 迁入)---

func PBToMenu(in *apipb.MenuInfo) *Menu {
	if in == nil {
		return nil
	}
	return &Menu{
		Model: commonmodel.Model{
			ID: in.Id,
		},
		ProjectID:   in.ProjectID,
		TenantID:    in.TenantID,
		Level:       in.Level,
		ParentID:    in.ParentID,
		Path:        in.Path,
		Name:        in.Name,
		Hidden:      in.Hidden,
		Component:   in.Component,
		Sort:        in.Sort,
		Cache:       in.Cache,
		DefaultMenu: in.DefaultMenu,
		Title:       in.Title,
		Icon:        in.Icon,
		CloseTab:    in.CloseTab,
		Parameters:  PBToMenuParameters(in.Parameters),
		MenuFuncs:   PBToMenuFuncs(in.MenuFuncs),
		IsMust:      in.IsMust,
	}
}

func MenuToPB(in *Menu) *apipb.MenuInfo {
	if in == nil {
		return nil
	}
	var children []*apipb.MenuInfo
	if len(in.Children) > 0 {
		children = MenusToPB(in.Children)
	}
	return &apipb.MenuInfo{
		Id:          in.ID,
		ProjectID:   in.ProjectID,
		TenantID:    in.TenantID,
		Level:       in.Level,
		ParentID:    in.ParentID,
		Path:        in.Path,
		Name:        in.Name,
		Hidden:      in.Hidden,
		Component:   in.Component,
		Sort:        in.Sort,
		Cache:       in.Cache,
		DefaultMenu: in.DefaultMenu,
		Title:       in.Title,
		Icon:        in.Icon,
		CloseTab:    in.CloseTab,
		Parameters:  MenuParametersToPB(in.Parameters),
		MenuFuncs:   MenuFuncsToPB(in.MenuFuncs),
		Children:    children,
		IsMust:      in.IsMust,
	}
}

func MenusToPB(in []*Menu) []*apipb.MenuInfo {
	var list []*apipb.MenuInfo
	for _, menu := range in {
		list = append(list, MenuToPB(menu))
	}
	return list
}

func PBToMenuParameters(params []*apipb.MenuParameter) []*MenuParameter {
	var list []*MenuParameter
	for _, param := range params {
		list = append(list, &MenuParameter{
			Model: commonmodel.Model{
				ID: param.Id,
			},
			MenuID: param.MenuID,
			Type:   param.Type,
			Key:    param.Key,
			Value:  param.Value,
		})
	}
	return list
}

func MenuParametersToPB(params []*MenuParameter) []*apipb.MenuParameter {
	var list []*apipb.MenuParameter
	for _, param := range params {
		list = append(list, &apipb.MenuParameter{
			Id:     param.ID,
			MenuID: param.MenuID,
			Type:   param.Type,
			Key:    param.Key,
			Value:  param.Value,
		})
	}
	return list
}

func PBToMenuFuncs(params []*apipb.MenuFunc) []*MenuFunc {
	var list []*MenuFunc
	for _, param := range params {
		list = append(list, &MenuFunc{
			Model: commonmodel.Model{
				ID: param.Id,
			},
			MenuID:       param.MenuID,
			Name:         param.Name,
			Title:        param.Title,
			Hidden:       param.Hidden,
			MenuFuncApis: PBToMenuFuncApis(param.MenuFuncApis),
		})
	}
	return list
}

func MenuFuncsToPB(params []*MenuFunc) []*apipb.MenuFunc {
	var list []*apipb.MenuFunc
	for _, param := range params {
		list = append(list, &apipb.MenuFunc{
			Id:           param.ID,
			MenuID:       param.MenuID,
			Name:         param.Name,
			Title:        param.Title,
			Hidden:       param.Hidden,
			MenuFuncApis: MenuFuncApisToPB(param.MenuFuncApis),
		})
	}
	return list
}

func PBToMenuFuncApis(params []*apipb.MenuFuncApi) []MenuFuncApi {
	var list []MenuFuncApi
	for _, param := range params {
		apiInfo := MenuFuncApi{
			Model: commonmodel.Model{
				ID: param.Id,
			},
			MenuFuncID: param.MenuFuncID,
			APIID:      param.ApiID,
		}
		if param.ApiInfo != nil {
			apiInfo.API = PBToAPI(param.ApiInfo)
		}
		list = append(list, apiInfo)
	}
	return list
}

func MenuFuncApisToPB(params []MenuFuncApi) []*apipb.MenuFuncApi {
	var list []*apipb.MenuFuncApi
	for _, param := range params {
		list = append(list, &apipb.MenuFuncApi{
			Id:         param.ID,
			MenuFuncID: param.MenuFuncID,
			ApiID:      param.APIID,
			ApiInfo:    APIToPB(param.API),
		})
	}
	return list
}
