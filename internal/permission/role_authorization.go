package permission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/CloudSilk/pkg/constants"
	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrRoleAuthorizationRevisionConflict = errors.New("role authorization revision conflict")

type RoleAuthorizationSelection struct {
	MenuID string   `json:"menuID"`
	Show   bool     `json:"show"`
	Funcs  []string `json:"funcs"`
}

type RoleAuthorizationAPI struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description"`
	Enable      bool   `json:"enable"`
	CheckAuth   bool   `json:"checkAuth"`
}

type RoleAuthorizationFunction struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Title  string                 `json:"title"`
	Hidden bool                   `json:"hidden"`
	APIs   []RoleAuthorizationAPI `json:"apis"`
}

type RoleAuthorizationMenu struct {
	ID        string                      `json:"id"`
	ParentID  string                      `json:"parentID"`
	Title     string                      `json:"title"`
	Name      string                      `json:"name"`
	Path      string                      `json:"path"`
	Hidden    bool                        `json:"hidden"`
	Sort      int32                       `json:"sort"`
	Level     uint32                      `json:"level"`
	Functions []RoleAuthorizationFunction `json:"functions"`
}

type RoleAuthorizationPolicySource struct {
	MenuID        string `json:"menuID"`
	MenuTitle     string `json:"menuTitle"`
	FunctionName  string `json:"functionName"`
	FunctionTitle string `json:"functionTitle"`
	APIID         string `json:"apiID"`
}

type RoleAuthorizationPolicy struct {
	Path      string                          `json:"path"`
	Method    string                          `json:"method"`
	CheckAuth bool                            `json:"checkAuth"`
	Sources   []RoleAuthorizationPolicySource `json:"sources"`
}

type RoleAuthorizationSkippedAPI struct {
	APIID         string `json:"apiID"`
	Path          string `json:"path"`
	Method        string `json:"method"`
	MenuID        string `json:"menuID"`
	MenuTitle     string `json:"menuTitle"`
	FunctionName  string `json:"functionName"`
	FunctionTitle string `json:"functionTitle"`
	Reason        string `json:"reason"`
}

type RoleAuthorizationSummary struct {
	SelectedMenuCount       int `json:"selectedMenuCount"`
	VisibleMenuCount        int `json:"visibleMenuCount"`
	SelectedFunctionCount   int `json:"selectedFunctionCount"`
	PolicyCount             int `json:"policyCount"`
	SkippedDisabledAPICount int `json:"skippedDisabledAPICount"`
}

type RoleAuthorizationRole struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TenantID string `json:"tenantID"`
	Enable   bool   `json:"enable"`
	Public   bool   `json:"public"`
}

type RoleAuthorizationDetail struct {
	Role        RoleAuthorizationRole         `json:"role"`
	Revision    string                        `json:"revision"`
	Selections  []RoleAuthorizationSelection  `json:"selections"`
	Menus       []RoleAuthorizationMenu       `json:"menus"`
	Policies    []RoleAuthorizationPolicy     `json:"policies"`
	SkippedAPIs []RoleAuthorizationSkippedAPI `json:"skippedAPIs"`
	Summary     RoleAuthorizationSummary      `json:"summary"`
	Warnings    []string                      `json:"warnings"`
}

type RoleAuthorizationPreview struct {
	Role             RoleAuthorizationRole         `json:"role"`
	CurrentRevision  string                        `json:"currentRevision"`
	ProposedRevision string                        `json:"proposedRevision"`
	Selections       []RoleAuthorizationSelection  `json:"selections"`
	Policies         []RoleAuthorizationPolicy     `json:"policies"`
	SkippedAPIs      []RoleAuthorizationSkippedAPI `json:"skippedAPIs"`
	Summary          RoleAuthorizationSummary      `json:"summary"`
	Warnings         []string                      `json:"warnings"`
}

type RoleAuthorizationPublishResult struct {
	Revision        string                   `json:"revision"`
	Summary         RoleAuthorizationSummary `json:"summary"`
	SessionsRevoked int64                    `json:"sessionsRevoked"`
	AffectedUserIDs []string                 `json:"-"`
}

type tenantMenuAuthorization struct {
	MenuID string
	Funcs  string
}

type roleAuthorizationCandidate struct {
	menu         *Menu
	allowedFuncs map[string]struct{}
}

func roleAuthorizationRole(role *Role) RoleAuthorizationRole {
	return RoleAuthorizationRole{
		ID:       role.ID,
		Name:     role.Name,
		TenantID: role.TenantID,
		Enable:   role.Enable,
		Public:   role.Public,
	}
}

func normalizeAuthorizationSelections(input []RoleAuthorizationSelection) ([]RoleAuthorizationSelection, error) {
	seenMenus := make(map[string]struct{}, len(input))
	selections := make([]RoleAuthorizationSelection, 0, len(input))
	for _, item := range input {
		item.MenuID = strings.TrimSpace(item.MenuID)
		if item.MenuID == "" {
			return nil, errors.New("menu ID cannot be empty")
		}
		if _, exists := seenMenus[item.MenuID]; exists {
			return nil, fmt.Errorf("menu %q is selected more than once", item.MenuID)
		}
		seenMenus[item.MenuID] = struct{}{}

		funcSet := make(map[string]struct{}, len(item.Funcs))
		for _, functionName := range item.Funcs {
			functionName = strings.TrimSpace(functionName)
			if functionName != "" {
				funcSet[functionName] = struct{}{}
			}
		}
		item.Funcs = make([]string, 0, len(funcSet))
		for functionName := range funcSet {
			item.Funcs = append(item.Funcs, functionName)
		}
		sort.Strings(item.Funcs)
		if !item.Show && len(item.Funcs) == 0 {
			continue
		}
		selections = append(selections, item)
	}
	sort.Slice(selections, func(i, j int) bool {
		return selections[i].MenuID < selections[j].MenuID
	})
	return selections, nil
}

func selectionsFromRoleMenus(roleMenus []*RoleMenu) ([]RoleAuthorizationSelection, error) {
	input := make([]RoleAuthorizationSelection, 0, len(roleMenus))
	for _, roleMenu := range roleMenus {
		if roleMenu == nil {
			continue
		}
		input = append(input, RoleAuthorizationSelection{
			MenuID: roleMenu.MenuID,
			Show:   roleMenu.Show,
			Funcs:  strings.Split(roleMenu.Funcs, ","),
		})
	}
	return normalizeAuthorizationSelections(input)
}

func roleAuthorizationRevision(selections []RoleAuthorizationSelection) string {
	canonical, _ := json.Marshal(selections)
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func loadCurrentAuthorizationSelections(db *gorm.DB, roleID string) ([]RoleAuthorizationSelection, error) {
	var roleMenus []*RoleMenu
	if err := db.Where("role_id = ?", roleID).Find(&roleMenus).Error; err != nil {
		return nil, err
	}
	return selectionsFromRoleMenus(roleMenus)
}

func loadRoleAuthorizationCandidates(db *gorm.DB, role *Role) ([]RoleAuthorizationMenu, map[string]*roleAuthorizationCandidate, error) {
	allowedByMenu := make(map[string]map[string]struct{})
	baseAuthorization := role.Public || role.TenantID == "" || role.TenantID == constants.PlatformTenantID
	if !baseAuthorization {
		var tenantMenus []tenantMenuAuthorization
		err := db.Table("tenant_menus").
			Select("menu_id", "funcs").
			Where("tenant_id = ? AND deleted_at IS NULL", role.TenantID).
			Scan(&tenantMenus).Error
		if err != nil {
			return nil, nil, err
		}
		for _, tenantMenu := range tenantMenus {
			allowedFuncs := allowedByMenu[tenantMenu.MenuID]
			if allowedFuncs == nil {
				allowedFuncs = make(map[string]struct{})
				allowedByMenu[tenantMenu.MenuID] = allowedFuncs
			}
			for _, functionName := range strings.Split(tenantMenu.Funcs, ",") {
				if functionName = strings.TrimSpace(functionName); functionName != "" {
					allowedFuncs[functionName] = struct{}{}
				}
			}
		}
	}

	var menus []*Menu
	query := db.Order("sort ASC, id ASC").Preload("MenuFuncs.MenuFuncApis.API")
	if !baseAuthorization {
		menuIDs := make([]string, 0, len(allowedByMenu))
		for menuID := range allowedByMenu {
			menuIDs = append(menuIDs, menuID)
		}
		if len(menuIDs) == 0 {
			return []RoleAuthorizationMenu{}, map[string]*roleAuthorizationCandidate{}, nil
		}
		query = query.Where("id IN ?", menuIDs)
	}
	if err := query.Find(&menus).Error; err != nil {
		return nil, nil, err
	}

	result := make([]RoleAuthorizationMenu, 0, len(menus))
	candidateMap := make(map[string]*roleAuthorizationCandidate, len(menus))
	for _, menu := range menus {
		allowedFuncs := make(map[string]struct{})
		functions := make([]RoleAuthorizationFunction, 0, len(menu.MenuFuncs))
		for _, function := range menu.MenuFuncs {
			if !baseAuthorization {
				if _, allowed := allowedByMenu[menu.ID][function.Name]; !allowed {
					continue
				}
			}
			allowedFuncs[function.Name] = struct{}{}
			apis := make([]RoleAuthorizationAPI, 0, len(function.MenuFuncApis))
			for _, link := range function.MenuFuncApis {
				if link.API == nil {
					continue
				}
				apis = append(apis, RoleAuthorizationAPI{
					ID:          link.API.ID,
					Path:        link.API.Path,
					Method:      link.API.Method,
					Description: link.API.Description,
					Enable:      link.API.Enable,
					CheckAuth:   link.API.CheckAuth,
				})
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
			functions = append(functions, RoleAuthorizationFunction{
				ID:     function.ID,
				Name:   function.Name,
				Title:  function.Title,
				Hidden: function.Hidden,
				APIs:   apis,
			})
		}
		sort.Slice(functions, func(i, j int) bool {
			if functions[i].Title != functions[j].Title {
				return functions[i].Title < functions[j].Title
			}
			return functions[i].Name < functions[j].Name
		})
		result = append(result, RoleAuthorizationMenu{
			ID:        menu.ID,
			ParentID:  menu.ParentID,
			Title:     menu.Title,
			Name:      menu.Name,
			Path:      menu.Path,
			Hidden:    menu.Hidden,
			Sort:      menu.Sort,
			Level:     menu.Level,
			Functions: functions,
		})
		candidateMap[menu.ID] = &roleAuthorizationCandidate{menu: menu, allowedFuncs: allowedFuncs}
	}
	return result, candidateMap, nil
}

func buildRoleAuthorizationPreview(db *gorm.DB, role *Role, input []RoleAuthorizationSelection, strict bool) (*RoleAuthorizationPreview, []RoleAuthorizationMenu, error) {
	selections, err := normalizeAuthorizationSelections(input)
	if err != nil {
		return nil, nil, err
	}
	menus, candidates, err := loadRoleAuthorizationCandidates(db, role)
	if err != nil {
		return nil, nil, err
	}
	selectedByMenu := make(map[string]RoleAuthorizationSelection, len(selections))
	warnings := make([]string, 0, 4)
	for _, selection := range selections {
		candidate := candidates[selection.MenuID]
		if candidate == nil {
			message := fmt.Sprintf("menu %q is outside the role tenant authorization boundary", selection.MenuID)
			if strict {
				return nil, nil, errors.New(message)
			}
			warnings = append(warnings, message)
			continue
		}
		for _, functionName := range selection.Funcs {
			if _, allowed := candidate.allowedFuncs[functionName]; !allowed {
				message := fmt.Sprintf("function %q does not belong to authorized menu %q", functionName, selection.MenuID)
				if strict {
					return nil, nil, errors.New(message)
				}
				warnings = append(warnings, message)
			}
		}
		selectedByMenu[selection.MenuID] = selection
	}
	for _, selection := range selections {
		if !selection.Show {
			continue
		}
		candidate := candidates[selection.MenuID]
		if candidate == nil {
			continue
		}
		for parentID := strings.TrimSpace(candidate.menu.ParentID); parentID != ""; {
			parent := candidates[parentID]
			if parent == nil {
				message := fmt.Sprintf("visible menu %q has an ancestor outside the role tenant authorization boundary", selection.MenuID)
				if strict {
					return nil, nil, errors.New(message)
				}
				warnings = append(warnings, message)
				break
			}
			parentSelection, selected := selectedByMenu[parentID]
			if !selected || !parentSelection.Show {
				message := fmt.Sprintf("visible menu %q requires visible ancestor menu %q", selection.MenuID, parentID)
				if strict {
					return nil, nil, errors.New(message)
				}
				warnings = append(warnings, message)
				break
			}
			parentID = strings.TrimSpace(parent.menu.ParentID)
		}
	}

	type policyAccumulator struct {
		policy  RoleAuthorizationPolicy
		sources map[string]struct{}
	}
	policyByKey := make(map[string]*policyAccumulator)
	skipped := make([]RoleAuthorizationSkippedAPI, 0)
	skippedKeys := make(map[string]struct{})
	selectedFunctionCount := 0
	visibleMenuCount := 0
	for _, selection := range selections {
		if selection.Show {
			visibleMenuCount++
		}
		selectedFunctionCount += len(selection.Funcs)
		selectedFunctions := make(map[string]struct{}, len(selection.Funcs))
		for _, functionName := range selection.Funcs {
			selectedFunctions[functionName] = struct{}{}
		}
		candidate := candidates[selection.MenuID]
		if candidate == nil {
			continue
		}
		for _, function := range candidate.menu.MenuFuncs {
			if _, selected := selectedFunctions[function.Name]; !selected {
				continue
			}
			for _, link := range function.MenuFuncApis {
				if link.API == nil {
					continue
				}
				source := RoleAuthorizationPolicySource{
					MenuID:        candidate.menu.ID,
					MenuTitle:     candidate.menu.Title,
					FunctionName:  function.Name,
					FunctionTitle: function.Title,
					APIID:         link.API.ID,
				}
				if !link.API.Enable {
					skippedKey := strings.Join([]string{candidate.menu.ID, function.Name, link.API.ID}, "\x00")
					if _, exists := skippedKeys[skippedKey]; !exists {
						skippedKeys[skippedKey] = struct{}{}
						skipped = append(skipped, RoleAuthorizationSkippedAPI{
							APIID:         link.API.ID,
							Path:          link.API.Path,
							Method:        link.API.Method,
							MenuID:        candidate.menu.ID,
							MenuTitle:     candidate.menu.Title,
							FunctionName:  function.Name,
							FunctionTitle: function.Title,
							Reason:        "API is disabled and will not be published",
						})
					}
					continue
				}
				key := strings.Join([]string{
					link.API.Path,
					link.API.Method,
					fmt.Sprintf("%t", link.API.CheckAuth),
				}, "\x00")
				accumulator := policyByKey[key]
				if accumulator == nil {
					accumulator = &policyAccumulator{
						policy: RoleAuthorizationPolicy{
							Path:      link.API.Path,
							Method:    link.API.Method,
							CheckAuth: link.API.CheckAuth,
						},
						sources: make(map[string]struct{}),
					}
					policyByKey[key] = accumulator
				}
				sourceKey := strings.Join([]string{source.MenuID, source.FunctionName, source.APIID}, "\x00")
				if _, exists := accumulator.sources[sourceKey]; !exists {
					accumulator.sources[sourceKey] = struct{}{}
					accumulator.policy.Sources = append(accumulator.policy.Sources, source)
				}
			}
		}
	}

	policies := make([]RoleAuthorizationPolicy, 0, len(policyByKey))
	for _, accumulator := range policyByKey {
		sort.Slice(accumulator.policy.Sources, func(i, j int) bool {
			left, right := accumulator.policy.Sources[i], accumulator.policy.Sources[j]
			if left.MenuID != right.MenuID {
				return left.MenuID < right.MenuID
			}
			if left.FunctionName != right.FunctionName {
				return left.FunctionName < right.FunctionName
			}
			return left.APIID < right.APIID
		})
		policies = append(policies, accumulator.policy)
	}
	sort.Slice(policies, func(i, j int) bool {
		if policies[i].Path != policies[j].Path {
			return policies[i].Path < policies[j].Path
		}
		if policies[i].Method != policies[j].Method {
			return policies[i].Method < policies[j].Method
		}
		return !policies[i].CheckAuth && policies[j].CheckAuth
	})
	sort.Slice(skipped, func(i, j int) bool {
		if skipped[i].Path != skipped[j].Path {
			return skipped[i].Path < skipped[j].Path
		}
		if skipped[i].Method != skipped[j].Method {
			return skipped[i].Method < skipped[j].Method
		}
		return skipped[i].APIID < skipped[j].APIID
	})

	currentSelections, err := loadCurrentAuthorizationSelections(db, role.ID)
	if err != nil {
		return nil, nil, err
	}
	if len(skipped) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d disabled API bindings will not be published", len(skipped)))
	}
	if !role.Enable {
		warnings = append(warnings, "the role is disabled; selections will be saved without active Casbin policies")
	}
	return &RoleAuthorizationPreview{
		Role:             roleAuthorizationRole(role),
		CurrentRevision:  roleAuthorizationRevision(currentSelections),
		ProposedRevision: roleAuthorizationRevision(selections),
		Selections:       selections,
		Policies:         policies,
		SkippedAPIs:      skipped,
		Summary: RoleAuthorizationSummary{
			SelectedMenuCount:       len(selections),
			VisibleMenuCount:        visibleMenuCount,
			SelectedFunctionCount:   selectedFunctionCount,
			PolicyCount:             len(policies),
			SkippedDisabledAPICount: len(skipped),
		},
		Warnings: warnings,
	}, menus, nil
}

func GetRoleAuthorization(roleID string) (*RoleAuthorizationDetail, error) {
	role := &Role{}
	if err := store.DB().Where("id = ?", strings.TrimSpace(roleID)).First(role).Error; err != nil {
		return nil, err
	}
	selections, err := loadCurrentAuthorizationSelections(store.DB(), role.ID)
	if err != nil {
		return nil, err
	}
	preview, menus, err := buildRoleAuthorizationPreview(store.DB(), role, selections, false)
	if err != nil {
		return nil, err
	}
	return &RoleAuthorizationDetail{
		Role:        preview.Role,
		Revision:    preview.CurrentRevision,
		Selections:  preview.Selections,
		Menus:       menus,
		Policies:    preview.Policies,
		SkippedAPIs: preview.SkippedAPIs,
		Summary:     preview.Summary,
		Warnings:    preview.Warnings,
	}, nil
}

func PreviewRoleAuthorization(roleID string, selections []RoleAuthorizationSelection) (*RoleAuthorizationPreview, error) {
	role := &Role{}
	if err := store.DB().Where("id = ?", strings.TrimSpace(roleID)).First(role).Error; err != nil {
		return nil, err
	}
	preview, _, err := buildRoleAuthorizationPreview(store.DB(), role, selections, true)
	return preview, err
}

func replaceRoleAuthorizationPolicies(tx *gorm.DB, role *Role, policies []RoleAuthorizationPolicy) error {
	if err := tx.Where("ptype = ? AND v0 = ?", "p", role.ID).Delete(&CasbinRule{}).Error; err != nil {
		return err
	}
	if !role.Enable || len(policies) == 0 {
		return nil
	}
	rules := make([]*CasbinRule, 0, len(policies))
	for _, policy := range policies {
		checkAuth := "false"
		if policy.CheckAuth {
			checkAuth = "true"
		}
		rules = append(rules, &CasbinRule{
			Ptype:     "p",
			RoleID:    role.ID,
			Path:      policy.Path,
			Method:    policy.Method,
			CheckAuth: checkAuth,
		})
	}
	return tx.Create(&rules).Error
}

func PublishRoleAuthorization(roleID, baseRevision string, selections []RoleAuthorizationSelection) (*RoleAuthorizationPublishResult, error) {
	roleID = strings.TrimSpace(roleID)
	baseRevision = strings.TrimSpace(baseRevision)
	if roleID == "" {
		return nil, errors.New("role ID cannot be empty")
	}
	if baseRevision == "" {
		return nil, errors.New("base revision cannot be empty")
	}

	result := &RoleAuthorizationPublishResult{}
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		role := &Role{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", roleID).First(role).Error; err != nil {
			return err
		}
		currentSelections, err := loadCurrentAuthorizationSelections(tx, role.ID)
		if err != nil {
			return err
		}
		if currentRevision := roleAuthorizationRevision(currentSelections); currentRevision != baseRevision {
			return fmt.Errorf("%w: current revision is %s", ErrRoleAuthorizationRevisionConflict, currentRevision)
		}
		preview, _, err := buildRoleAuthorizationPreview(tx, role, selections, true)
		if err != nil {
			return err
		}

		if err := tx.Unscoped().Where("role_id = ?", role.ID).Delete(&RoleMenu{}).Error; err != nil {
			return err
		}
		for _, selection := range preview.Selections {
			roleMenu := &RoleMenu{
				Model:  commonmodel.Model{},
				RoleID: role.ID,
				MenuID: selection.MenuID,
				Funcs:  strings.Join(selection.Funcs, ","),
				Show:   selection.Show,
			}
			if err := tx.Create(roleMenu).Error; err != nil {
				return err
			}
		}

		if err := replaceRoleAuthorizationPolicies(tx, role, preview.Policies); err != nil {
			return err
		}

		if err := tx.Table("user_roles").
			Where("role_id = ?", role.ID).
			Distinct("user_id").
			Pluck("user_id", &result.AffectedUserIDs).Error; err != nil {
			return err
		}
		if len(result.AffectedUserIDs) > 0 {
			sessionUpdate := tx.Table("user_session").
				Where("principal_id IN ? AND revoked = ?", result.AffectedUserIDs, false).
				Updates(map[string]interface{}{
					"revoked":        true,
					"revoked_reason": "role authorization published",
				})
			if sessionUpdate.Error != nil {
				return sessionUpdate.Error
			}
			result.SessionsRevoked = sessionUpdate.RowsAffected
		}
		result.Revision = preview.ProposedRevision
		result.Summary = preview.Summary
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := ReloadCasbinPolicy(); err != nil {
		return nil, err
	}
	if token.DefaultTokenCache != nil {
		for _, userID := range result.AffectedUserIDs {
			if err := token.DefaultTokenCache.DelByUserID(userID); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
