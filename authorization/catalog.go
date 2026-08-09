// Package authorization exposes usercenter's native role, menu, API and
// Casbin initialization contract to embedded products.
package authorization

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	"gorm.io/gorm"
)

const authorizationCatalogIDMaxLength = 36

// AuthorizationCatalog is the native usercenter initialization contract used
// by embedded products. Every definition is reconciled into usercenter's own
// role, menu, API, tenant and Casbin tables; custom rows outside the catalog
// are preserved.
type AuthorizationCatalog struct {
	TenantID      string                     `json:"tenantID"`
	ProjectID     string                     `json:"projectID,omitempty"`
	DefaultRoleID string                     `json:"defaultRoleID,omitempty"`
	Roles         []AuthorizationRole        `json:"roles"`
	Menus         []AuthorizationMenu        `json:"menus"`
	APIs          []AuthorizationAPI         `json:"apis"`
	RoleGrants    []AuthorizationRoleGrant   `json:"roleGrants"`
	TenantGrants  []AuthorizationTenantGrant `json:"tenantGrants"`
	ABACPolicies  []AuthorizationABACPolicy  `json:"abacPolicies"`
}

type AuthorizationRole struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ParentID      string `json:"parentID,omitempty"`
	DefaultRouter string `json:"defaultRouter,omitempty"`
	Description   string `json:"description,omitempty"`
	Public        bool   `json:"public"`
}

type AuthorizationMenu struct {
	ID          string                  `json:"id"`
	ParentID    string                  `json:"parentID,omitempty"`
	Path        string                  `json:"path"`
	Name        string                  `json:"name"`
	Title       string                  `json:"title"`
	Icon        string                  `json:"icon,omitempty"`
	Component   string                  `json:"component,omitempty"`
	Level       uint32                  `json:"level"`
	Sort        int32                   `json:"sort"`
	Hidden      bool                    `json:"hidden"`
	Cache       bool                    `json:"cache"`
	DefaultMenu bool                    `json:"defaultMenu"`
	CloseTab    bool                    `json:"closeTab"`
	Functions   []AuthorizationFunction `json:"functions"`
}

type AuthorizationFunction struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Title  string   `json:"title"`
	Hidden bool     `json:"hidden"`
	APIIDs []string `json:"apiIDs"`
}

type AuthorizationAPI struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Method      string `json:"method"`
	Group       string `json:"group,omitempty"`
	Description string `json:"description,omitempty"`
	CheckAuth   bool   `json:"checkAuth"`
	CheckLogin  bool   `json:"checkLogin"`
}

type AuthorizationRoleGrant struct {
	RoleID    string   `json:"roleID"`
	MenuID    string   `json:"menuID"`
	Functions []string `json:"functions"`
	Show      bool     `json:"show"`
}

type AuthorizationTenantGrant struct {
	TenantID  string   `json:"tenantID"`
	MenuID    string   `json:"menuID"`
	Functions []string `json:"functions"`
}

type AuthorizationABACPolicy struct {
	TenantID  string `json:"tenantID"`
	RoleID    string `json:"roleID"`
	Resource  string `json:"resource"`
	Action    string `json:"action"`
	DataScope int32  `json:"dataScope"`
	Condition string `json:"condition,omitempty"`
	Priority  int32  `json:"priority"`
}

type AuthorizationCatalogSummary struct {
	RoleCount             int `json:"roleCount"`
	MenuCount             int `json:"menuCount"`
	FunctionCount         int `json:"functionCount"`
	APICount              int `json:"apiCount"`
	RoleGrantCount        int `json:"roleGrantCount"`
	TenantGrantCount      int `json:"tenantGrantCount"`
	ABACPolicyCount       int `json:"abacPolicyCount"`
	DefaultRoleUsersAdded int `json:"defaultRoleUsersAdded"`
	CasbinRuleCount       int `json:"casbinRuleCount"`
}

// Concise public aliases keep embedded-product catalog declarations readable
// while retaining the explicit Authorization* names inside documentation.
type Catalog = AuthorizationCatalog
type Role = AuthorizationRole
type Menu = AuthorizationMenu
type Function = AuthorizationFunction
type API = AuthorizationAPI
type RoleGrant = AuthorizationRoleGrant
type TenantGrant = AuthorizationTenantGrant
type ABACPolicy = AuthorizationABACPolicy
type Summary = AuthorizationCatalogSummary

// ApplyAuthorizationCatalog validates and idempotently reconciles a complete
// authorization catalog. Existing system rows are healed by stable ID (or
// natural key for data created before stable IDs were introduced), while
// custom rows and additional administrator grants are retained.
func Apply(catalog AuthorizationCatalog) (AuthorizationCatalogSummary, error) {
	catalog = normalizeAuthorizationCatalog(catalog)
	if err := validateAuthorizationCatalog(catalog); err != nil {
		return AuthorizationCatalogSummary{}, err
	}
	db := store.DB()
	if db == nil {
		return AuthorizationCatalogSummary{}, errors.New("usercenter store is not initialized")
	}

	summary := AuthorizationCatalogSummary{
		RoleCount:        len(catalog.Roles),
		MenuCount:        len(catalog.Menus),
		APICount:         len(catalog.APIs),
		RoleGrantCount:   len(catalog.RoleGrants),
		TenantGrantCount: len(catalog.TenantGrants),
		ABACPolicyCount:  len(catalog.ABACPolicies),
	}
	for _, menu := range catalog.Menus {
		summary.FunctionCount += len(menu.Functions)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		roleIDs := make(map[string]string, len(catalog.Roles))
		for _, definition := range catalog.Roles {
			id, err := upsertCatalogRole(tx, catalog, definition)
			if err != nil {
				return err
			}
			roleIDs[definition.ID] = id
		}

		menuIDs := make(map[string]string, len(catalog.Menus))
		for _, definition := range catalog.Menus {
			id, err := upsertCatalogMenu(tx, catalog, definition)
			if err != nil {
				return err
			}
			menuIDs[definition.ID] = id
		}

		apiIDs := make(map[string]string, len(catalog.APIs))
		for _, definition := range catalog.APIs {
			id, err := upsertCatalogAPI(tx, catalog, definition)
			if err != nil {
				return err
			}
			apiIDs[definition.ID] = id
		}

		functionIDs := make(map[string]string, summary.FunctionCount)
		functionDefinitions := make(map[string]AuthorizationFunction, summary.FunctionCount)
		for _, menuDefinition := range catalog.Menus {
			menuID := menuIDs[menuDefinition.ID]
			for _, definition := range menuDefinition.Functions {
				id, err := upsertCatalogFunction(tx, menuID, definition)
				if err != nil {
					return err
				}
				functionIDs[definition.ID] = id
				functionDefinitions[definition.ID] = definition
				for _, catalogAPIID := range definition.APIIDs {
					if err := ensureCatalogFunctionAPI(tx, id, apiIDs[catalogAPIID]); err != nil {
						return err
					}
				}
			}
		}

		for _, grant := range catalog.RoleGrants {
			if err := ensureCatalogRoleGrant(
				tx,
				roleIDs[grant.RoleID],
				menuIDs[grant.MenuID],
				grant.Functions,
				grant.Show,
			); err != nil {
				return err
			}
		}
		for _, grant := range catalog.TenantGrants {
			if err := ensureCatalogTenantGrant(
				tx,
				grant.TenantID,
				menuIDs[grant.MenuID],
				grant.Functions,
			); err != nil {
				return err
			}
		}
		for _, policy := range catalog.ABACPolicies {
			if err := ensureCatalogABACPolicy(tx, policy, roleIDs[policy.RoleID]); err != nil {
				return err
			}
		}

		casbinRules, err := ensureCatalogCasbinRules(
			tx,
			catalog,
			roleIDs,
			menuIDs,
			functionIDs,
			functionDefinitions,
			apiIDs,
		)
		if err != nil {
			return err
		}
		summary.CasbinRuleCount = casbinRules

		if catalog.DefaultRoleID != "" {
			added, err := backfillCatalogDefaultRole(
				tx,
				catalog.TenantID,
				roleIDs[catalog.DefaultRoleID],
			)
			if err != nil {
				return err
			}
			summary.DefaultRoleUsersAdded = added
		}
		return nil
	})
	if err != nil {
		return AuthorizationCatalogSummary{}, err
	}

	// Rebuild login-only/public projections from the reconciled API inventory,
	// then load both those rules and the role policies into the live enforcer.
	permission.UpdateNotCheckAuthRule()
	permission.UpdateNotCheckLoginRule()
	if err := permission.ReloadCasbinPolicy(); err != nil {
		return AuthorizationCatalogSummary{}, fmt.Errorf("reload Casbin policy: %w", err)
	}
	return summary, nil
}

func normalizeAuthorizationCatalog(catalog AuthorizationCatalog) AuthorizationCatalog {
	catalog.TenantID = strings.TrimSpace(catalog.TenantID)
	catalog.ProjectID = strings.TrimSpace(catalog.ProjectID)
	catalog.DefaultRoleID = strings.TrimSpace(catalog.DefaultRoleID)
	for i := range catalog.Roles {
		role := &catalog.Roles[i]
		role.ID = strings.TrimSpace(role.ID)
		role.Name = strings.TrimSpace(role.Name)
		role.ParentID = strings.TrimSpace(role.ParentID)
		role.DefaultRouter = strings.TrimSpace(role.DefaultRouter)
		role.Description = strings.TrimSpace(role.Description)
	}
	for i := range catalog.Menus {
		menu := &catalog.Menus[i]
		menu.ID = strings.TrimSpace(menu.ID)
		menu.ParentID = strings.TrimSpace(menu.ParentID)
		menu.Path = strings.TrimSpace(menu.Path)
		menu.Name = strings.TrimSpace(menu.Name)
		menu.Title = strings.TrimSpace(menu.Title)
		menu.Icon = strings.TrimSpace(menu.Icon)
		menu.Component = strings.TrimSpace(menu.Component)
		for j := range menu.Functions {
			function := &menu.Functions[j]
			function.ID = strings.TrimSpace(function.ID)
			function.Name = strings.TrimSpace(function.Name)
			function.Title = strings.TrimSpace(function.Title)
			function.APIIDs = normalizeStringSet(function.APIIDs)
		}
	}
	for i := range catalog.APIs {
		api := &catalog.APIs[i]
		api.ID = strings.TrimSpace(api.ID)
		api.Path = strings.TrimSpace(api.Path)
		api.Method = strings.ToUpper(strings.TrimSpace(api.Method))
		api.Group = strings.TrimSpace(api.Group)
		api.Description = strings.TrimSpace(api.Description)
	}
	for i := range catalog.RoleGrants {
		grant := &catalog.RoleGrants[i]
		grant.RoleID = strings.TrimSpace(grant.RoleID)
		grant.MenuID = strings.TrimSpace(grant.MenuID)
		grant.Functions = normalizeStringSet(grant.Functions)
	}
	for i := range catalog.TenantGrants {
		grant := &catalog.TenantGrants[i]
		grant.TenantID = strings.TrimSpace(grant.TenantID)
		if grant.TenantID == "" {
			grant.TenantID = catalog.TenantID
		}
		grant.MenuID = strings.TrimSpace(grant.MenuID)
		grant.Functions = normalizeStringSet(grant.Functions)
	}
	for i := range catalog.ABACPolicies {
		policy := &catalog.ABACPolicies[i]
		policy.TenantID = strings.TrimSpace(policy.TenantID)
		if policy.TenantID == "" {
			policy.TenantID = catalog.TenantID
		}
		policy.RoleID = strings.TrimSpace(policy.RoleID)
		policy.Resource = strings.TrimSpace(policy.Resource)
		policy.Action = strings.TrimSpace(policy.Action)
		policy.Condition = strings.TrimSpace(policy.Condition)
	}
	return catalog
}

func validateAuthorizationCatalog(catalog AuthorizationCatalog) error {
	if catalog.TenantID == "" {
		return errors.New("authorization catalog tenant ID is required")
	}
	if err := validateAuthorizationCatalogID("authorization catalog tenant ID", catalog.TenantID); err != nil {
		return err
	}
	if catalog.ProjectID != "" {
		if err := validateAuthorizationCatalogID("authorization catalog project ID", catalog.ProjectID); err != nil {
			return err
		}
	}
	if catalog.DefaultRoleID != "" {
		if err := validateAuthorizationCatalogID("authorization catalog default role ID", catalog.DefaultRoleID); err != nil {
			return err
		}
	}
	roleIDs := make(map[string]struct{}, len(catalog.Roles))
	for _, role := range catalog.Roles {
		if role.ID == "" || role.Name == "" {
			return errors.New("authorization catalog role ID and name are required")
		}
		if err := validateAuthorizationCatalogID("authorization role ID", role.ID); err != nil {
			return err
		}
		if role.ParentID != "" {
			if err := validateAuthorizationCatalogID("authorization role parent ID", role.ParentID); err != nil {
				return err
			}
		}
		if _, exists := roleIDs[role.ID]; exists {
			return fmt.Errorf("duplicate authorization role ID %q", role.ID)
		}
		roleIDs[role.ID] = struct{}{}
	}
	for _, role := range catalog.Roles {
		if role.ParentID != "" {
			if _, exists := roleIDs[role.ParentID]; !exists {
				return fmt.Errorf("role %q references unknown parent %q", role.ID, role.ParentID)
			}
		}
	}
	if catalog.DefaultRoleID != "" {
		if _, exists := roleIDs[catalog.DefaultRoleID]; !exists {
			return fmt.Errorf("default role %q is not defined", catalog.DefaultRoleID)
		}
	}

	apiIDs := make(map[string]struct{}, len(catalog.APIs))
	apiNaturalKeys := make(map[string]struct{}, len(catalog.APIs))
	for _, api := range catalog.APIs {
		if api.ID == "" || api.Path == "" || api.Method == "" {
			return errors.New("authorization API ID, path and method are required")
		}
		if err := validateAuthorizationCatalogID("authorization API ID", api.ID); err != nil {
			return err
		}
		if !strings.HasPrefix(api.Path, "/") {
			return fmt.Errorf("authorization API %q path must start with /", api.ID)
		}
		if !validHTTPMethod(api.Method) {
			return fmt.Errorf("authorization API %q has unsupported method %q", api.ID, api.Method)
		}
		if _, exists := apiIDs[api.ID]; exists {
			return fmt.Errorf("duplicate authorization API ID %q", api.ID)
		}
		apiIDs[api.ID] = struct{}{}
		key := api.Method + "\x00" + api.Path
		if _, exists := apiNaturalKeys[key]; exists {
			return fmt.Errorf("duplicate authorization API %s %s", api.Method, api.Path)
		}
		apiNaturalKeys[key] = struct{}{}
	}

	menuIDs := make(map[string]struct{}, len(catalog.Menus))
	menuFunctionNames := make(map[string]map[string]struct{}, len(catalog.Menus))
	functionIDs := make(map[string]struct{})
	for _, menu := range catalog.Menus {
		if menu.ID == "" || menu.Name == "" || menu.Title == "" {
			return errors.New("authorization menu ID, name and title are required")
		}
		if err := validateAuthorizationCatalogID("authorization menu ID", menu.ID); err != nil {
			return err
		}
		if menu.ParentID != "" {
			if err := validateAuthorizationCatalogID("authorization menu parent ID", menu.ParentID); err != nil {
				return err
			}
		}
		if _, exists := menuIDs[menu.ID]; exists {
			return fmt.Errorf("duplicate authorization menu ID %q", menu.ID)
		}
		menuIDs[menu.ID] = struct{}{}
		names := make(map[string]struct{}, len(menu.Functions))
		for _, function := range menu.Functions {
			if function.ID == "" || function.Name == "" || function.Title == "" {
				return fmt.Errorf("menu %q has a function without ID, name or title", menu.ID)
			}
			if err := validateAuthorizationCatalogID("authorization function ID", function.ID); err != nil {
				return err
			}
			if _, exists := functionIDs[function.ID]; exists {
				return fmt.Errorf("duplicate authorization function ID %q", function.ID)
			}
			if _, exists := names[function.Name]; exists {
				return fmt.Errorf("menu %q has duplicate function name %q", menu.ID, function.Name)
			}
			functionIDs[function.ID] = struct{}{}
			names[function.Name] = struct{}{}
			for _, apiID := range function.APIIDs {
				if _, exists := apiIDs[apiID]; !exists {
					return fmt.Errorf("function %q references unknown API %q", function.ID, apiID)
				}
			}
		}
		menuFunctionNames[menu.ID] = names
	}
	for _, menu := range catalog.Menus {
		if menu.ParentID != "" {
			if _, exists := menuIDs[menu.ParentID]; !exists {
				return fmt.Errorf("menu %q references unknown parent %q", menu.ID, menu.ParentID)
			}
		}
	}
	for _, grant := range catalog.RoleGrants {
		if _, exists := roleIDs[grant.RoleID]; !exists {
			return fmt.Errorf("role grant references unknown role %q", grant.RoleID)
		}
		names, exists := menuFunctionNames[grant.MenuID]
		if !exists {
			return fmt.Errorf("role grant references unknown menu %q", grant.MenuID)
		}
		for _, function := range grant.Functions {
			if _, exists := names[function]; !exists {
				return fmt.Errorf("role grant for menu %q references unknown function %q", grant.MenuID, function)
			}
		}
	}
	for _, grant := range catalog.TenantGrants {
		names, exists := menuFunctionNames[grant.MenuID]
		if !exists {
			return fmt.Errorf("tenant grant references unknown menu %q", grant.MenuID)
		}
		for _, function := range grant.Functions {
			if _, exists := names[function]; !exists {
				return fmt.Errorf("tenant grant for menu %q references unknown function %q", grant.MenuID, function)
			}
		}
	}
	for _, policy := range catalog.ABACPolicies {
		if _, exists := roleIDs[policy.RoleID]; !exists {
			return fmt.Errorf("ABAC policy references unknown role %q", policy.RoleID)
		}
		if policy.Resource == "" || policy.Action == "" {
			return errors.New("ABAC policy resource and action are required")
		}
		if !validDataScope(policy.DataScope) {
			return fmt.Errorf("ABAC policy has unsupported data scope %d", policy.DataScope)
		}
	}
	return nil
}

func validateAuthorizationCatalogID(label, value string) error {
	length := utf8.RuneCountInString(value)
	if length <= authorizationCatalogIDMaxLength {
		return nil
	}
	return fmt.Errorf(
		"%s %q exceeds the %d-character database limit (got %d)",
		label,
		value,
		authorizationCatalogIDMaxLength,
		length,
	)
}

func upsertCatalogRole(tx *gorm.DB, catalog AuthorizationCatalog, definition AuthorizationRole) (string, error) {
	var row permission.Role
	err := tx.Unscoped().Where("id = ?", definition.ID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = tx.Unscoped().
			Where("tenant_id = ? AND project_id = ? AND name = ?", catalog.TenantID, catalog.ProjectID, definition.Name).
			First(&row).Error
	}
	values := map[string]any{
		"tenant_id":      catalog.TenantID,
		"project_id":     catalog.ProjectID,
		"name":           definition.Name,
		"parent_id":      definition.ParentID,
		"default_router": definition.DefaultRouter,
		"description":    definition.Description,
		"can_del":        false,
		"public":         definition.Public,
		"is_must":        true,
		"enable":         true,
		"deleted_at":     nil,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = permission.Role{
			Model:         commonmodel.Model{ID: definition.ID},
			TenantID:      catalog.TenantID,
			ProjectID:     catalog.ProjectID,
			Name:          definition.Name,
			ParentID:      definition.ParentID,
			DefaultRouter: definition.DefaultRouter,
			Description:   definition.Description,
			CanDel:        false,
			Public:        definition.Public,
			IsMust:        true,
			Enable:        true,
		}
		if err := tx.Create(&row).Error; err != nil {
			return "", fmt.Errorf("create role %q: %w", definition.ID, err)
		}
		return row.ID, nil
	}
	if err != nil {
		return "", fmt.Errorf("load role %q: %w", definition.ID, err)
	}
	if err := tx.Unscoped().Model(&row).UpdateColumns(values).Error; err != nil {
		return "", fmt.Errorf("update role %q: %w", definition.ID, err)
	}
	return row.ID, nil
}

func upsertCatalogMenu(tx *gorm.DB, catalog AuthorizationCatalog, definition AuthorizationMenu) (string, error) {
	var row permission.Menu
	err := tx.Unscoped().Where("id = ?", definition.ID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = tx.Unscoped().
			Where("tenant_id = ? AND project_id = ? AND name = ?", catalog.TenantID, catalog.ProjectID, definition.Name).
			First(&row).Error
	}
	values := map[string]any{
		"tenant_id":    catalog.TenantID,
		"project_id":   catalog.ProjectID,
		"level":        definition.Level,
		"parent_id":    definition.ParentID,
		"path":         definition.Path,
		"name":         definition.Name,
		"hidden":       definition.Hidden,
		"component":    definition.Component,
		"sort":         definition.Sort,
		"cache":        definition.Cache,
		"default_menu": definition.DefaultMenu,
		"title":        definition.Title,
		"icon":         definition.Icon,
		"close_tab":    definition.CloseTab,
		"is_must":      true,
		"deleted_at":   nil,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = permission.Menu{
			Model:       commonmodel.Model{ID: definition.ID},
			TenantID:    catalog.TenantID,
			ProjectID:   catalog.ProjectID,
			Level:       definition.Level,
			ParentID:    definition.ParentID,
			Path:        definition.Path,
			Name:        definition.Name,
			Hidden:      definition.Hidden,
			Component:   definition.Component,
			Sort:        definition.Sort,
			Cache:       definition.Cache,
			DefaultMenu: definition.DefaultMenu,
			Title:       definition.Title,
			Icon:        definition.Icon,
			CloseTab:    definition.CloseTab,
			IsMust:      true,
		}
		if err := tx.Create(&row).Error; err != nil {
			return "", fmt.Errorf("create menu %q: %w", definition.ID, err)
		}
		return row.ID, nil
	}
	if err != nil {
		return "", fmt.Errorf("load menu %q: %w", definition.ID, err)
	}
	if err := tx.Unscoped().Model(&row).UpdateColumns(values).Error; err != nil {
		return "", fmt.Errorf("update menu %q: %w", definition.ID, err)
	}
	return row.ID, nil
}

func upsertCatalogAPI(tx *gorm.DB, catalog AuthorizationCatalog, definition AuthorizationAPI) (string, error) {
	var row permission.API
	err := tx.Unscoped().Where("id = ?", definition.ID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = tx.Unscoped().
			Where(
				"tenant_id = ? AND project_id = ? AND path = ? AND method = ?",
				catalog.TenantID,
				catalog.ProjectID,
				definition.Path,
				definition.Method,
			).
			First(&row).Error
	}
	values := map[string]any{
		"tenant_id":   catalog.TenantID,
		"project_id":  catalog.ProjectID,
		"path":        definition.Path,
		"group":       definition.Group,
		"method":      definition.Method,
		"description": definition.Description,
		"enable":      true,
		"check_auth":  definition.CheckAuth,
		"check_login": definition.CheckLogin,
		"is_must":     true,
		"deleted_at":  nil,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = permission.API{
			Model:       commonmodel.Model{ID: definition.ID},
			TenantID:    catalog.TenantID,
			ProjectID:   catalog.ProjectID,
			Path:        definition.Path,
			Group:       definition.Group,
			Method:      definition.Method,
			Description: definition.Description,
			Enable:      true,
			CheckAuth:   definition.CheckAuth,
			CheckLogin:  definition.CheckLogin,
			IsMust:      true,
		}
		if err := tx.Create(&row).Error; err != nil {
			return "", fmt.Errorf("create API %q: %w", definition.ID, err)
		}
		return row.ID, nil
	}
	if err != nil {
		return "", fmt.Errorf("load API %q: %w", definition.ID, err)
	}
	if err := tx.Unscoped().Model(&row).UpdateColumns(values).Error; err != nil {
		return "", fmt.Errorf("update API %q: %w", definition.ID, err)
	}
	return row.ID, nil
}

func upsertCatalogFunction(tx *gorm.DB, menuID string, definition AuthorizationFunction) (string, error) {
	var row permission.MenuFunc
	err := tx.Unscoped().Where("id = ?", definition.ID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = tx.Unscoped().Where("menu_id = ? AND name = ?", menuID, definition.Name).First(&row).Error
	}
	values := map[string]any{
		"menu_id":    menuID,
		"name":       definition.Name,
		"title":      definition.Title,
		"hidden":     definition.Hidden,
		"deleted_at": nil,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = permission.MenuFunc{
			Model:  commonmodel.Model{ID: definition.ID},
			MenuID: menuID,
			Name:   definition.Name,
			Title:  definition.Title,
			Hidden: definition.Hidden,
		}
		if err := tx.Create(&row).Error; err != nil {
			return "", fmt.Errorf("create function %q: %w", definition.ID, err)
		}
		return row.ID, nil
	}
	if err != nil {
		return "", fmt.Errorf("load function %q: %w", definition.ID, err)
	}
	if err := tx.Unscoped().Model(&row).UpdateColumns(values).Error; err != nil {
		return "", fmt.Errorf("update function %q: %w", definition.ID, err)
	}
	return row.ID, nil
}

func ensureCatalogFunctionAPI(tx *gorm.DB, functionID, apiID string) error {
	var row permission.MenuFuncApi
	err := tx.Unscoped().
		Where("menu_func_id = ? AND api_id = ?", functionID, apiID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := tx.Create(&permission.MenuFuncApi{MenuFuncID: functionID, APIID: apiID}).Error; err != nil {
			return fmt.Errorf("bind function %q to API %q: %w", functionID, apiID, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("load function/API binding: %w", err)
	}
	if row.DeletedAt.Valid {
		if err := tx.Unscoped().Model(&row).UpdateColumn("deleted_at", nil).Error; err != nil {
			return fmt.Errorf("restore function/API binding: %w", err)
		}
	}
	return nil
}

func ensureCatalogRoleGrant(tx *gorm.DB, roleID, menuID string, functions []string, show bool) error {
	var row permission.RoleMenu
	err := tx.Unscoped().Where("role_id = ? AND menu_id = ?", roleID, menuID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = permission.RoleMenu{RoleID: roleID, MenuID: menuID, Funcs: strings.Join(functions, ","), Show: show}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("create role/menu grant: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("load role/menu grant: %w", err)
	}
	functions = unionCSV(row.Funcs, functions)
	return tx.Unscoped().Model(&row).UpdateColumns(map[string]any{
		"funcs":      strings.Join(functions, ","),
		"show":       row.Show || show,
		"deleted_at": nil,
	}).Error
}

func ensureCatalogTenantGrant(tx *gorm.DB, tenantID, menuID string, functions []string) error {
	var row tenant.TenantMenu
	err := tx.Unscoped().Where("tenant_id = ? AND menu_id = ?", tenantID, menuID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = tenant.TenantMenu{TenantID: tenantID, MenuID: menuID, Funcs: strings.Join(functions, ",")}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("create tenant/menu grant: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("load tenant/menu grant: %w", err)
	}
	functions = unionCSV(row.Funcs, functions)
	return tx.Unscoped().Model(&row).UpdateColumns(map[string]any{
		"funcs":      strings.Join(functions, ","),
		"deleted_at": nil,
	}).Error
}

func ensureCatalogABACPolicy(tx *gorm.DB, definition AuthorizationABACPolicy, roleID string) error {
	var row permission.ABACPolicy
	err := tx.Where(
		"tenant_id = ? AND role_id = ? AND resource = ? AND action = ?",
		definition.TenantID,
		roleID,
		definition.Resource,
		definition.Action,
	).First(&row).Error
	values := map[string]any{
		"data_scope": definition.DataScope,
		"condition":  definition.Condition,
		"enable":     true,
		"priority":   definition.Priority,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = permission.ABACPolicy{
			TenantID:  definition.TenantID,
			RoleID:    roleID,
			Resource:  definition.Resource,
			Action:    definition.Action,
			DataScope: definition.DataScope,
			Condition: definition.Condition,
			Enable:    true,
			Priority:  definition.Priority,
		}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("create ABAC policy: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("load ABAC policy: %w", err)
	}
	return tx.Model(&row).Updates(values).Error
}

func ensureCatalogCasbinRules(
	tx *gorm.DB,
	catalog AuthorizationCatalog,
	roleIDs map[string]string,
	menuIDs map[string]string,
	functionIDs map[string]string,
	functionDefinitions map[string]AuthorizationFunction,
	apiIDs map[string]string,
) (int, error) {
	type apiProjection struct {
		Path      string
		Method    string
		CheckAuth bool
	}
	apiProjectionByID := make(map[string]apiProjection, len(catalog.APIs))
	for _, definition := range catalog.APIs {
		apiProjectionByID[apiIDs[definition.ID]] = apiProjection{
			Path: definition.Path, Method: definition.Method, CheckAuth: definition.CheckAuth,
		}
	}
	functionByMenuAndName := make(map[string]string)
	for _, menu := range catalog.Menus {
		for _, function := range menu.Functions {
			functionByMenuAndName[menuIDs[menu.ID]+"\x00"+function.Name] = functionIDs[function.ID]
		}
	}
	count := 0
	for _, grant := range catalog.RoleGrants {
		roleID := roleIDs[grant.RoleID]
		menuID := menuIDs[grant.MenuID]
		for _, name := range grant.Functions {
			functionID := functionByMenuAndName[menuID+"\x00"+name]
			definition, found := functionDefinitionsByResolvedID(functionDefinitions, functionIDs, functionID)
			if !found {
				continue
			}
			for _, catalogAPIID := range definition.APIIDs {
				projection := apiProjectionByID[apiIDs[catalogAPIID]]
				rule := permission.CasbinRule{
					Ptype:     "p",
					RoleID:    roleID,
					Path:      projection.Path,
					Method:    projection.Method,
					CheckAuth: fmt.Sprintf("%t", projection.CheckAuth),
				}
				result := tx.Where(
					"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ? AND v4 = '' AND v5 = ''",
					rule.Ptype,
					rule.RoleID,
					rule.Path,
					rule.Method,
					rule.CheckAuth,
				).FirstOrCreate(&rule)
				if result.Error != nil {
					return 0, fmt.Errorf(
						"create Casbin rule %s %s for role %s: %w",
						rule.Method,
						rule.Path,
						roleID,
						result.Error,
					)
				}
				count++
			}
		}
	}
	return count, nil
}

func functionDefinitionsByResolvedID(
	definitions map[string]AuthorizationFunction,
	resolvedIDs map[string]string,
	target string,
) (AuthorizationFunction, bool) {
	for catalogID, resolvedID := range resolvedIDs {
		if resolvedID == target {
			definition, found := definitions[catalogID]
			return definition, found
		}
	}
	return AuthorizationFunction{}, false
}

func backfillCatalogDefaultRole(tx *gorm.DB, tenantID, roleID string) (int, error) {
	var users []*user.User
	subquery := tx.Model(&user.UserRole{}).Select("user_id")
	if err := tx.Where("tenant_id = ? AND id NOT IN (?)", tenantID, subquery).Find(&users).Error; err != nil {
		return 0, fmt.Errorf("load users without roles: %w", err)
	}
	added := 0
	for _, target := range users {
		created, err := ensureCatalogUserRole(tx, target.ID, roleID)
		if err != nil {
			return 0, err
		}
		if created {
			added++
		}
	}
	return added, nil
}

// EnsureUserRole idempotently assigns one native usercenter role after
// validating that the user and role share a tenant and that the role is active.
func EnsureUserRole(userID, roleID string) error {
	db := store.DB()
	if db == nil {
		return errors.New("usercenter store is not initialized")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		_, err := ensureCatalogUserRole(tx, strings.TrimSpace(userID), strings.TrimSpace(roleID))
		return err
	})
}

func ensureCatalogUserRole(tx *gorm.DB, userID, roleID string) (bool, error) {
	if userID == "" || roleID == "" {
		return false, errors.New("user ID and role ID are required")
	}
	var target user.User
	if err := tx.Where("id = ?", userID).First(&target).Error; err != nil {
		return false, fmt.Errorf("load user %q: %w", userID, err)
	}
	var role permission.Role
	if err := tx.Where("id = ? AND enable = ?", roleID, true).First(&role).Error; err != nil {
		return false, fmt.Errorf("load role %q: %w", roleID, err)
	}
	if role.TenantID != "" && role.TenantID != target.TenantID {
		return false, fmt.Errorf("role %q does not belong to user tenant", roleID)
	}
	var link user.UserRole
	err := tx.Unscoped().Where("user_id = ? AND role_id = ?", userID, roleID).First(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := tx.Create(&user.UserRole{UserID: userID, RoleID: roleID}).Error; err != nil {
			return false, fmt.Errorf("assign role %q to user %q: %w", roleID, userID, err)
		}
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("load user/role link: %w", err)
	}
	if link.DeletedAt.Valid {
		if err := tx.Unscoped().Model(&link).UpdateColumn("deleted_at", nil).Error; err != nil {
			return false, fmt.Errorf("restore user/role link: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func normalizeStringSet(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func unionCSV(existing string, required []string) []string {
	values := strings.Split(existing, ",")
	values = append(values, required...)
	return normalizeStringSet(values)
}

func validHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}
