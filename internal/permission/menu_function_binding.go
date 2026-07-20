package permission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrMenuFunctionRevisionConflict = errors.New("menu function binding revision conflict")

type MenuFunctionBindingInput struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Title  string   `json:"title"`
	Hidden bool     `json:"hidden"`
	APIIDs []string `json:"apiIDs"`
}

type MenuFunctionBindingAPI struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description"`
	Group       string `json:"group"`
	Enable      bool   `json:"enable"`
	CheckAuth   bool   `json:"checkAuth"`
	CheckLogin  bool   `json:"checkLogin"`
}

type MenuFunctionBindingFunction struct {
	ID                string                   `json:"id"`
	Name              string                   `json:"name"`
	Title             string                   `json:"title"`
	Hidden            bool                     `json:"hidden"`
	SelectedRoleCount int                      `json:"selectedRoleCount"`
	APIs              []MenuFunctionBindingAPI `json:"apis"`
}

type MenuFunctionRoleImpact struct {
	RoleID               string   `json:"roleID"`
	RoleName             string   `json:"roleName"`
	RoleEnabled          bool     `json:"roleEnabled"`
	SelectedFunctions    []string `json:"selectedFunctions"`
	StaleFunctions       []string `json:"staleFunctions"`
	GeneratedPolicyCount int      `json:"generatedPolicyCount"`
}

type MenuFunctionBindingSummary struct {
	FunctionCount           int   `json:"functionCount"`
	APIBindingCount         int   `json:"apiBindingCount"`
	DisabledAPIBindingCount int   `json:"disabledAPIBindingCount"`
	RoleCount               int   `json:"roleCount"`
	ActiveRoleCount         int   `json:"activeRoleCount"`
	GeneratedPolicyCount    int   `json:"generatedPolicyCount"`
	AffectedUserCount       int64 `json:"affectedUserCount"`
}

type MenuFunctionBindingDetail struct {
	MenuID    string                        `json:"menuID"`
	MenuTitle string                        `json:"menuTitle"`
	MenuName  string                        `json:"menuName"`
	MenuPath  string                        `json:"menuPath"`
	TenantID  string                        `json:"tenantID"`
	ProjectID string                        `json:"projectID"`
	Protected bool                          `json:"protected"`
	Revision  string                        `json:"revision"`
	Functions []MenuFunctionBindingFunction `json:"functions"`
	Roles     []MenuFunctionRoleImpact      `json:"roles"`
	Summary   MenuFunctionBindingSummary    `json:"summary"`
}

type MenuFunctionBindingUpdate struct {
	MenuID       string                     `json:"menuID"`
	BaseRevision string                     `json:"baseRevision"`
	Functions    []MenuFunctionBindingInput `json:"functions"`
}

type MenuFunctionBindingUpdateResult struct {
	Revision        string                     `json:"revision"`
	Summary         MenuFunctionBindingSummary `json:"summary"`
	SessionsRevoked int64                      `json:"sessionsRevoked"`
	AffectedUserIDs []string                   `json:"-"`
}

type menuFunctionRevisionItem struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Title  string   `json:"title"`
	Hidden bool     `json:"hidden"`
	APIIDs []string `json:"apiIDs"`
}

func menuFunctionRevision(functions []*MenuFunc) string {
	items := make([]menuFunctionRevisionItem, 0, len(functions))
	for _, function := range functions {
		if function == nil {
			continue
		}
		apiIDs := make([]string, 0, len(function.MenuFuncApis))
		for _, link := range function.MenuFuncApis {
			if apiID := strings.TrimSpace(link.APIID); apiID != "" {
				apiIDs = append(apiIDs, apiID)
			}
		}
		sort.Strings(apiIDs)
		items = append(items, menuFunctionRevisionItem{
			ID:     function.ID,
			Name:   function.Name,
			Title:  function.Title,
			Hidden: function.Hidden,
			APIIDs: apiIDs,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID != items[j].ID {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
	canonical, _ := json.Marshal(items)
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func normalizeMenuFunctionInputs(input []MenuFunctionBindingInput) ([]MenuFunctionBindingInput, error) {
	result := make([]MenuFunctionBindingInput, 0, len(input))
	seenIDs := make(map[string]struct{}, len(input))
	seenNames := make(map[string]struct{}, len(input))
	for index, function := range input {
		function.ID = strings.TrimSpace(function.ID)
		function.Name = strings.TrimSpace(function.Name)
		function.Title = strings.TrimSpace(function.Title)
		if function.Name == "" {
			return nil, fmt.Errorf("%w: function %d name is required", ErrInvalidMenu, index+1)
		}
		if strings.ContainsAny(function.Name, ", \t\r\n") {
			return nil, fmt.Errorf("%w: function name %q cannot contain commas or spaces", ErrInvalidMenu, function.Name)
		}
		if len(function.Name) > 100 {
			return nil, fmt.Errorf("%w: function name cannot exceed 100 characters", ErrInvalidMenu)
		}
		if function.Title == "" {
			return nil, fmt.Errorf("%w: function %q title is required", ErrInvalidMenu, function.Name)
		}
		if len(function.Title) > 100 {
			return nil, fmt.Errorf("%w: function title cannot exceed 100 characters", ErrInvalidMenu)
		}
		if function.ID != "" {
			if _, exists := seenIDs[function.ID]; exists {
				return nil, fmt.Errorf("%w: function ID %q is duplicated", ErrInvalidMenu, function.ID)
			}
			seenIDs[function.ID] = struct{}{}
		}
		if _, exists := seenNames[function.Name]; exists {
			return nil, fmt.Errorf("%w: function name %q is duplicated", ErrInvalidMenu, function.Name)
		}
		seenNames[function.Name] = struct{}{}
		function.APIIDs = uniqueStrings(function.APIIDs)
		sort.Strings(function.APIIDs)
		result = append(result, function)
	}
	return result, nil
}

func loadMenuFunctionBindingMenu(db *gorm.DB, menuID string, lock bool) (*Menu, error) {
	query := db.
		Preload("MenuFuncs.MenuFuncApis.API").
		Where("id = ?", strings.TrimSpace(menuID))
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	menu := &Menu{}
	if err := query.First(menu).Error; err != nil {
		return nil, err
	}
	return menu, nil
}

func selectedFunctionNames(value string) []string {
	names := uniqueStrings(strings.Split(value, ","))
	sort.Strings(names)
	return names
}

func menuFunctionPolicyCount(role *Role, selected []string, byName map[string]*MenuFunc) int {
	if role == nil || !role.Enable {
		return 0
	}
	keys := make(map[string]struct{})
	for _, name := range selected {
		function := byName[name]
		if function == nil {
			continue
		}
		for _, link := range function.MenuFuncApis {
			if link.API == nil || !link.API.Enable {
				continue
			}
			key := strings.Join([]string{
				link.API.Path,
				link.API.Method,
				fmt.Sprintf("%t", link.API.CheckAuth),
			}, "\x00")
			keys[key] = struct{}{}
		}
	}
	return len(keys)
}

func buildMenuFunctionBindingDetail(db *gorm.DB, menu *Menu) (*MenuFunctionBindingDetail, error) {
	var roleMenus []*RoleMenu
	if err := db.Where("menu_id = ?", menu.ID).Find(&roleMenus).Error; err != nil {
		return nil, err
	}
	roleIDs := make([]string, 0, len(roleMenus))
	for _, roleMenu := range roleMenus {
		roleIDs = append(roleIDs, roleMenu.RoleID)
	}
	var roles []*Role
	if len(roleIDs) > 0 {
		if err := db.Where("id IN ?", uniqueStrings(roleIDs)).Find(&roles).Error; err != nil {
			return nil, err
		}
	}
	roleByID := make(map[string]*Role, len(roles))
	for _, role := range roles {
		roleByID[role.ID] = role
	}

	byName := make(map[string]*MenuFunc, len(menu.MenuFuncs))
	selectedRoleCounts := make(map[string]int, len(menu.MenuFuncs))
	for _, function := range menu.MenuFuncs {
		byName[function.Name] = function
	}
	roleImpacts := make([]MenuFunctionRoleImpact, 0, len(roleMenus))
	activeRoleIDs := make([]string, 0, len(roleMenus))
	generatedPolicyCount := 0
	for _, roleMenu := range roleMenus {
		role := roleByID[roleMenu.RoleID]
		if role == nil {
			continue
		}
		selected := selectedFunctionNames(roleMenu.Funcs)
		valid := make([]string, 0, len(selected))
		stale := make([]string, 0)
		for _, name := range selected {
			if byName[name] == nil {
				stale = append(stale, name)
				continue
			}
			valid = append(valid, name)
			selectedRoleCounts[name]++
		}
		policyCount := menuFunctionPolicyCount(role, valid, byName)
		generatedPolicyCount += policyCount
		if role.Enable {
			activeRoleIDs = append(activeRoleIDs, role.ID)
		}
		roleImpacts = append(roleImpacts, MenuFunctionRoleImpact{
			RoleID:               role.ID,
			RoleName:             role.Name,
			RoleEnabled:          role.Enable,
			SelectedFunctions:    valid,
			StaleFunctions:       stale,
			GeneratedPolicyCount: policyCount,
		})
	}
	sort.Slice(roleImpacts, func(i, j int) bool {
		if roleImpacts[i].RoleName != roleImpacts[j].RoleName {
			return roleImpacts[i].RoleName < roleImpacts[j].RoleName
		}
		return roleImpacts[i].RoleID < roleImpacts[j].RoleID
	})

	functions := make([]MenuFunctionBindingFunction, 0, len(menu.MenuFuncs))
	summary := MenuFunctionBindingSummary{
		FunctionCount:        len(menu.MenuFuncs),
		RoleCount:            len(roleImpacts),
		ActiveRoleCount:      len(uniqueStrings(activeRoleIDs)),
		GeneratedPolicyCount: generatedPolicyCount,
	}
	for _, function := range menu.MenuFuncs {
		apis := make([]MenuFunctionBindingAPI, 0, len(function.MenuFuncApis))
		for _, link := range function.MenuFuncApis {
			if link.API == nil {
				continue
			}
			apis = append(apis, MenuFunctionBindingAPI{
				ID:          link.API.ID,
				Path:        link.API.Path,
				Method:      link.API.Method,
				Description: link.API.Description,
				Group:       link.API.Group,
				Enable:      link.API.Enable,
				CheckAuth:   link.API.CheckAuth,
				CheckLogin:  link.API.CheckLogin,
			})
			summary.APIBindingCount++
			if !link.API.Enable {
				summary.DisabledAPIBindingCount++
			}
		}
		sort.Slice(apis, func(i, j int) bool {
			if apis[i].Path != apis[j].Path {
				return apis[i].Path < apis[j].Path
			}
			if apis[i].Method != apis[j].Method {
				return apis[i].Method < apis[j].Method
			}
			return apis[i].ID < apis[j].ID
		})
		functions = append(functions, MenuFunctionBindingFunction{
			ID:                function.ID,
			Name:              function.Name,
			Title:             function.Title,
			Hidden:            function.Hidden,
			SelectedRoleCount: selectedRoleCounts[function.Name],
			APIs:              apis,
		})
	}
	sort.Slice(functions, func(i, j int) bool {
		if functions[i].Title != functions[j].Title {
			return functions[i].Title < functions[j].Title
		}
		if functions[i].Name != functions[j].Name {
			return functions[i].Name < functions[j].Name
		}
		return functions[i].ID < functions[j].ID
	})

	if len(roleIDs) > 0 {
		if err := db.Table("user_roles").
			Where("role_id IN ?", uniqueStrings(roleIDs)).
			Distinct("user_id").
			Count(&summary.AffectedUserCount).Error; err != nil {
			return nil, err
		}
	}
	return &MenuFunctionBindingDetail{
		MenuID:    menu.ID,
		MenuTitle: menu.Title,
		MenuName:  menu.Name,
		MenuPath:  menu.Path,
		TenantID:  menu.TenantID,
		ProjectID: menu.ProjectID,
		Protected: menu.IsMust,
		Revision:  menuFunctionRevision(menu.MenuFuncs),
		Functions: functions,
		Roles:     roleImpacts,
		Summary:   summary,
	}, nil
}

func GetMenuFunctionBindings(menuID string) (*MenuFunctionBindingDetail, error) {
	menuID = strings.TrimSpace(menuID)
	if menuID == "" {
		return nil, fmt.Errorf("%w: menu ID cannot be empty", ErrInvalidMenu)
	}
	menu, err := loadMenuFunctionBindingMenu(store.DB(), menuID, false)
	if err != nil {
		return nil, err
	}
	return buildMenuFunctionBindingDetail(store.DB(), menu)
}

func validateMenuFunctionAPIs(tx *gorm.DB, menu *Menu, functions []MenuFunctionBindingInput) (map[string]*API, error) {
	apiIDs := make([]string, 0)
	for _, function := range functions {
		apiIDs = append(apiIDs, function.APIIDs...)
	}
	apiIDs = uniqueStrings(apiIDs)
	if len(apiIDs) == 0 {
		return map[string]*API{}, nil
	}
	var apis []*API
	if err := tx.Where("id IN ?", apiIDs).Find(&apis).Error; err != nil {
		return nil, err
	}
	if len(apis) != len(apiIDs) {
		return nil, fmt.Errorf("%w: one or more API resources do not exist", ErrInvalidMenu)
	}
	byID := make(map[string]*API, len(apis))
	for _, api := range apis {
		if api.TenantID != menu.TenantID || api.ProjectID != menu.ProjectID {
			return nil, fmt.Errorf(
				"%w: API %q must belong to the same tenant and project as the menu",
				ErrInvalidMenu,
				api.ID,
			)
		}
		byID[api.ID] = api
	}
	return byID, nil
}

func pruneMenuFunctionSelections(tx *gorm.DB, menuID string, validNames map[string]struct{}) error {
	var roleMenus []*RoleMenu
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("menu_id = ?", menuID).
		Find(&roleMenus).Error; err != nil {
		return err
	}
	for _, roleMenu := range roleMenus {
		selected := selectedFunctionNames(roleMenu.Funcs)
		kept := make([]string, 0, len(selected))
		for _, name := range selected {
			if _, valid := validNames[name]; valid {
				kept = append(kept, name)
			}
		}
		if len(kept) == 0 && !roleMenu.Show {
			if err := tx.Unscoped().Delete(roleMenu).Error; err != nil {
				return err
			}
			continue
		}
		if err := tx.Model(&RoleMenu{}).
			Where("id = ?", roleMenu.ID).
			Update("funcs", strings.Join(kept, ",")).Error; err != nil {
			return err
		}
	}

	type tenantMenuSelection struct {
		ID    string
		Funcs string
	}
	var tenantMenus []tenantMenuSelection
	if err := tx.Table("tenant_menus").
		Select("id", "funcs").
		Where("menu_id = ? AND deleted_at IS NULL", menuID).
		Scan(&tenantMenus).Error; err != nil {
		return err
	}
	for _, tenantMenu := range tenantMenus {
		kept := make([]string, 0)
		for _, name := range selectedFunctionNames(tenantMenu.Funcs) {
			if _, valid := validNames[name]; valid {
				kept = append(kept, name)
			}
		}
		if err := tx.Table("tenant_menus").
			Where("id = ?", tenantMenu.ID).
			Update("funcs", strings.Join(kept, ",")).Error; err != nil {
			return err
		}
	}
	return nil
}

func ReplaceMenuFunctionBindings(input MenuFunctionBindingUpdate) (*MenuFunctionBindingUpdateResult, error) {
	input.MenuID = strings.TrimSpace(input.MenuID)
	input.BaseRevision = strings.TrimSpace(input.BaseRevision)
	if input.MenuID == "" {
		return nil, fmt.Errorf("%w: menu ID cannot be empty", ErrInvalidMenu)
	}
	if input.BaseRevision == "" {
		return nil, fmt.Errorf("%w: base revision cannot be empty", ErrInvalidMenu)
	}
	functions, err := normalizeMenuFunctionInputs(input.Functions)
	if err != nil {
		return nil, err
	}

	result := &MenuFunctionBindingUpdateResult{}
	reloadPolicies := false
	err = store.DB().Transaction(func(tx *gorm.DB) error {
		menu, err := loadMenuFunctionBindingMenu(tx, input.MenuID, true)
		if err != nil {
			return err
		}
		if menu.IsMust {
			return ErrProtectedMenu
		}
		currentRevision := menuFunctionRevision(menu.MenuFuncs)
		if currentRevision != input.BaseRevision {
			return fmt.Errorf(
				"%w: current revision is %s",
				ErrMenuFunctionRevisionConflict,
				currentRevision,
			)
		}

		currentByID := make(map[string]*MenuFunc, len(menu.MenuFuncs))
		for _, function := range menu.MenuFuncs {
			currentByID[function.ID] = function
		}
		for _, function := range functions {
			if function.ID != "" && currentByID[function.ID] == nil {
				return fmt.Errorf(
					"%w: function %q does not belong to menu %q",
					ErrInvalidMenu,
					function.ID,
					menu.ID,
				)
			}
		}
		if _, err := validateMenuFunctionAPIs(tx, menu, functions); err != nil {
			return err
		}

		affectedRoleIDs, err := menuRoleIDs(tx, menu.ID)
		if err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(
			&MenuFuncApi{},
			"menu_func_id IN (SELECT id FROM menu_funcs WHERE menu_id = ?)",
			menu.ID,
		).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&MenuFunc{}, "menu_id = ?", menu.ID).Error; err != nil {
			return err
		}

		validNames := make(map[string]struct{}, len(functions))
		for _, item := range functions {
			function := &MenuFunc{
				Model:  commonmodel.Model{ID: item.ID},
				MenuID: menu.ID,
				Name:   item.Name,
				Title:  item.Title,
				Hidden: item.Hidden,
			}
			if err := tx.Create(function).Error; err != nil {
				return err
			}
			validNames[function.Name] = struct{}{}
			for _, apiID := range item.APIIDs {
				link := &MenuFuncApi{MenuFuncID: function.ID, APIID: apiID}
				if err := tx.Create(link).Error; err != nil {
					return err
				}
			}
		}
		if err := pruneMenuFunctionSelections(tx, menu.ID, validNames); err != nil {
			return err
		}
		if err := rebuildRoleAuthorizationPolicies(tx, affectedRoleIDs); err != nil {
			return err
		}
		reloadPolicies = len(affectedRoleIDs) > 0

		if len(affectedRoleIDs) > 0 {
			var userIDs []string
			if err := tx.Table("user_roles").
				Where("role_id IN ?", affectedRoleIDs).
				Distinct("user_id").
				Pluck("user_id", &userIDs).Error; err != nil {
				return err
			}
			result.AffectedUserIDs = uniqueStrings(userIDs)
			if len(result.AffectedUserIDs) > 0 {
				sessionUpdate := tx.Table("user_session").
					Where("principal_id IN ? AND revoked = ?", result.AffectedUserIDs, false).
					Updates(map[string]interface{}{
						"revoked":        true,
						"revoked_reason": "menu function bindings updated",
					})
				if sessionUpdate.Error != nil {
					return sessionUpdate.Error
				}
				result.SessionsRevoked = sessionUpdate.RowsAffected
			}
		}

		updated, err := loadMenuFunctionBindingMenu(tx, menu.ID, false)
		if err != nil {
			return err
		}
		detail, err := buildMenuFunctionBindingDetail(tx, updated)
		if err != nil {
			return err
		}
		result.Revision = detail.Revision
		result.Summary = detail.Summary
		return nil
	})
	if err != nil {
		return nil, err
	}
	if reloadPolicies {
		if err := ReloadCasbinPolicy(); err != nil {
			return nil, err
		}
	}
	if err := clearMenuTokenCache(result.AffectedUserIDs); err != nil {
		return nil, err
	}
	return result, nil
}
