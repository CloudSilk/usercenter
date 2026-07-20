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
	"github.com/CloudSilk/usercenter/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrTenantMenuAuthorizationForbidden        = errors.New("tenant menu authorization target is outside the actor scope")
	ErrTenantMenuAuthorizationRevisionConflict = errors.New("tenant menu authorization revision conflict")
	ErrTenantMenuAuthorizationSystemBaseline   = errors.New("platform tenant uses the unrestricted system menu baseline")
)

type TenantMenuAuthorizationSelection struct {
	MenuID string   `json:"menuID"`
	Funcs  []string `json:"funcs"`
}

type TenantMenuAuthorizationTarget struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Enable                  bool   `json:"enable"`
	IsMust                  bool   `json:"isMust"`
	SystemBaseline          bool   `json:"systemBaseline"`
	AuthorizedMenuCount     int    `json:"authorizedMenuCount"`
	AuthorizedFunctionCount int    `json:"authorizedFunctionCount"`
	RoleCount               int64  `json:"roleCount"`
	AssignedUserCount       int64  `json:"assignedUserCount"`
}

type TenantMenuAuthorizationTargetList struct {
	ActorTenantID        string                          `json:"actorTenantID"`
	PlatformTenantID     string                          `json:"platformTenantID"`
	CanManageCrossTenant bool                            `json:"canManageCrossTenant"`
	Targets              []TenantMenuAuthorizationTarget `json:"targets"`
}

type TenantMenuAuthorizationRemovedFunction struct {
	MenuID        string `json:"menuID"`
	MenuTitle     string `json:"menuTitle"`
	FunctionName  string `json:"functionName"`
	FunctionTitle string `json:"functionTitle"`
}

type TenantMenuAuthorizationRoleImpact struct {
	RoleID                string                                   `json:"roleID"`
	RoleName              string                                   `json:"roleName"`
	RoleEnabled           bool                                     `json:"roleEnabled"`
	Changed               bool                                     `json:"changed"`
	CurrentMenuCount      int                                      `json:"currentMenuCount"`
	CurrentFunctionCount  int                                      `json:"currentFunctionCount"`
	RetainedMenuCount     int                                      `json:"retainedMenuCount"`
	RetainedFunctionCount int                                      `json:"retainedFunctionCount"`
	RemovedMenuIDs        []string                                 `json:"removedMenuIDs"`
	RemovedFunctions      []TenantMenuAuthorizationRemovedFunction `json:"removedFunctions"`
	GeneratedPolicyCount  int                                      `json:"generatedPolicyCount"`
	AssignedUserCount     int64                                    `json:"assignedUserCount"`
}

type TenantMenuAuthorizationSummary struct {
	CandidateMenuCount    int   `json:"candidateMenuCount"`
	SelectedMenuCount     int   `json:"selectedMenuCount"`
	SelectedFunctionCount int   `json:"selectedFunctionCount"`
	RoleCount             int   `json:"roleCount"`
	AffectedRoleCount     int   `json:"affectedRoleCount"`
	ActiveRoleCount       int   `json:"activeRoleCount"`
	AffectedUserCount     int64 `json:"affectedUserCount"`
	GeneratedPolicyCount  int   `json:"generatedPolicyCount"`
	PrunedMenuCount       int   `json:"prunedMenuCount"`
	PrunedFunctionCount   int   `json:"prunedFunctionCount"`
}

type TenantMenuAuthorizationDetail struct {
	Tenant         TenantMenuAuthorizationTarget       `json:"tenant"`
	Revision       string                              `json:"revision"`
	Selections     []TenantMenuAuthorizationSelection  `json:"selections"`
	Menus          []RoleAuthorizationMenu             `json:"menus"`
	Roles          []TenantMenuAuthorizationRoleImpact `json:"roles"`
	Summary        TenantMenuAuthorizationSummary      `json:"summary"`
	Warnings       []string                            `json:"warnings"`
	SystemBaseline bool                                `json:"systemBaseline"`
}

type TenantMenuAuthorizationPreview struct {
	Tenant           TenantMenuAuthorizationTarget       `json:"tenant"`
	CurrentRevision  string                              `json:"currentRevision"`
	ProposedRevision string                              `json:"proposedRevision"`
	Selections       []TenantMenuAuthorizationSelection  `json:"selections"`
	Roles            []TenantMenuAuthorizationRoleImpact `json:"roles"`
	Summary          TenantMenuAuthorizationSummary      `json:"summary"`
	Warnings         []string                            `json:"warnings"`
}

type TenantMenuAuthorizationPublishResult struct {
	Revision        string                         `json:"revision"`
	Summary         TenantMenuAuthorizationSummary `json:"summary"`
	SessionsRevoked int64                          `json:"sessionsRevoked"`
	AffectedUserIDs []string                       `json:"-"`
}

type tenantAuthorizationRecord struct {
	ID     string
	Name   string
	Enable bool
	IsMust bool
}

type tenantMenuAuthorizationGrant struct {
	commonmodel.Model
	TenantID string
	MenuID   string
	Funcs    string
}

func (tenantMenuAuthorizationGrant) TableName() string {
	return "tenant_menus"
}

type tenantMenuAuthorizationCandidate struct {
	menu         *Menu
	allowedFuncs map[string]struct{}
	funcByName   map[string]*MenuFunc
}

type tenantMenuAuthorizationRoleProjection struct {
	role      *Role
	current   []RoleAuthorizationSelection
	projected []RoleAuthorizationSelection
	changed   bool
}

func loadTenantAuthorizationRecord(db *gorm.DB, tenantID string, lock bool) (*tenantAuthorizationRecord, error) {
	query := db.Table("tenants").
		Select("id", "name", "enable", "is_must").
		Where("id = ? AND deleted_at IS NULL", strings.TrimSpace(tenantID))
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	target := &tenantAuthorizationRecord{}
	if err := query.Take(target).Error; err != nil {
		return nil, err
	}
	return target, nil
}

func authorizeTenantMenuTarget(actorTenantID, targetTenantID string) error {
	actorTenantID = strings.TrimSpace(actorTenantID)
	targetTenantID = strings.TrimSpace(targetTenantID)
	if actorTenantID == "" || targetTenantID == "" {
		return ErrTenantMenuAuthorizationForbidden
	}
	if actorTenantID != constants.PlatformTenantID && actorTenantID != targetTenantID {
		return ErrTenantMenuAuthorizationForbidden
	}
	return nil
}

func tenantAuthorizationTarget(record *tenantAuthorizationRecord) TenantMenuAuthorizationTarget {
	return TenantMenuAuthorizationTarget{
		ID:             record.ID,
		Name:           record.Name,
		Enable:         record.Enable,
		IsMust:         record.IsMust,
		SystemBaseline: record.ID == constants.PlatformTenantID,
	}
}

func normalizeTenantMenuAuthorizationSelections(
	input []TenantMenuAuthorizationSelection,
) ([]TenantMenuAuthorizationSelection, error) {
	result := make([]TenantMenuAuthorizationSelection, 0, len(input))
	seenMenus := make(map[string]struct{}, len(input))
	for _, item := range input {
		item.MenuID = strings.TrimSpace(item.MenuID)
		if item.MenuID == "" {
			return nil, errors.New("menu ID cannot be empty")
		}
		if _, exists := seenMenus[item.MenuID]; exists {
			return nil, fmt.Errorf("menu %q is selected more than once", item.MenuID)
		}
		seenMenus[item.MenuID] = struct{}{}
		item.Funcs = uniqueStrings(item.Funcs)
		sort.Strings(item.Funcs)
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].MenuID < result[j].MenuID
	})
	return result, nil
}

func tenantMenuAuthorizationRevision(selections []TenantMenuAuthorizationSelection) string {
	canonical, _ := json.Marshal(selections)
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func loadCurrentTenantMenuSelections(db *gorm.DB, tenantID string) ([]TenantMenuAuthorizationSelection, error) {
	var grants []tenantMenuAuthorizationGrant
	if err := db.Where("tenant_id = ?", tenantID).Find(&grants).Error; err != nil {
		return nil, err
	}
	funcsByMenu := make(map[string]map[string]struct{}, len(grants))
	for _, grant := range grants {
		menuID := strings.TrimSpace(grant.MenuID)
		if menuID == "" {
			continue
		}
		funcSet := funcsByMenu[menuID]
		if funcSet == nil {
			funcSet = make(map[string]struct{})
			funcsByMenu[menuID] = funcSet
		}
		for _, functionName := range strings.Split(grant.Funcs, ",") {
			if functionName = strings.TrimSpace(functionName); functionName != "" {
				funcSet[functionName] = struct{}{}
			}
		}
	}
	selections := make([]TenantMenuAuthorizationSelection, 0, len(funcsByMenu))
	for menuID, funcSet := range funcsByMenu {
		functions := make([]string, 0, len(funcSet))
		for functionName := range funcSet {
			functions = append(functions, functionName)
		}
		sort.Strings(functions)
		selections = append(selections, TenantMenuAuthorizationSelection{MenuID: menuID, Funcs: functions})
	}
	sort.Slice(selections, func(i, j int) bool {
		return selections[i].MenuID < selections[j].MenuID
	})
	return selections, nil
}

func loadTenantMenuAuthorizationCandidates(
	db *gorm.DB,
	targetTenantID string,
) ([]RoleAuthorizationMenu, map[string]*tenantMenuAuthorizationCandidate, error) {
	var menus []*Menu
	query := db.
		Where(
			"(tenant_id = ? OR tenant_id = ? OR tenant_id = '')",
			constants.PlatformTenantID,
			targetTenantID,
		).
		Order("sort ASC, title ASC, name ASC, id ASC").
		Preload("MenuFuncs.MenuFuncApis.API")
	if err := query.Find(&menus).Error; err != nil {
		return nil, nil, err
	}

	result := make([]RoleAuthorizationMenu, 0, len(menus))
	candidates := make(map[string]*tenantMenuAuthorizationCandidate, len(menus))
	for _, menu := range menus {
		allowedFuncs := make(map[string]struct{}, len(menu.MenuFuncs))
		funcByName := make(map[string]*MenuFunc, len(menu.MenuFuncs))
		functions := make([]RoleAuthorizationFunction, 0, len(menu.MenuFuncs))
		for _, function := range menu.MenuFuncs {
			allowedFuncs[function.Name] = struct{}{}
			funcByName[function.Name] = function
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
		candidates[menu.ID] = &tenantMenuAuthorizationCandidate{
			menu:         menu,
			allowedFuncs: allowedFuncs,
			funcByName:   funcByName,
		}
	}
	return result, candidates, nil
}

func sanitizeTenantMenuSelections(
	input []TenantMenuAuthorizationSelection,
	candidates map[string]*tenantMenuAuthorizationCandidate,
	strict bool,
) ([]TenantMenuAuthorizationSelection, []string, error) {
	normalized, err := normalizeTenantMenuAuthorizationSelections(input)
	if err != nil {
		return nil, nil, err
	}
	warnings := make([]string, 0)
	selected := make(map[string]TenantMenuAuthorizationSelection, len(normalized))
	for _, item := range normalized {
		candidate := candidates[item.MenuID]
		if candidate == nil {
			message := fmt.Sprintf("menu %q is outside the tenant authorization catalogue", item.MenuID)
			if strict {
				return nil, nil, errors.New(message)
			}
			warnings = append(warnings, message)
			continue
		}
		functions := make([]string, 0, len(item.Funcs))
		for _, functionName := range item.Funcs {
			if _, exists := candidate.allowedFuncs[functionName]; !exists {
				message := fmt.Sprintf("function %q does not belong to menu %q", functionName, item.MenuID)
				if strict {
					return nil, nil, errors.New(message)
				}
				warnings = append(warnings, message)
				continue
			}
			functions = append(functions, functionName)
		}
		item.Funcs = functions
		selected[item.MenuID] = item
	}

	invalidMenus := make(map[string]struct{})
	for menuID := range selected {
		candidate := candidates[menuID]
		for parentID := strings.TrimSpace(candidate.menu.ParentID); parentID != ""; {
			parent := candidates[parentID]
			if parent == nil {
				message := fmt.Sprintf("menu %q has an ancestor outside the tenant authorization catalogue", menuID)
				if strict {
					return nil, nil, errors.New(message)
				}
				warnings = append(warnings, message)
				invalidMenus[menuID] = struct{}{}
				break
			}
			if _, exists := selected[parentID]; !exists {
				message := fmt.Sprintf("menu %q requires ancestor menu %q", menuID, parentID)
				if strict {
					return nil, nil, errors.New(message)
				}
				warnings = append(warnings, message)
				invalidMenus[menuID] = struct{}{}
				break
			}
			parentID = strings.TrimSpace(parent.menu.ParentID)
		}
	}
	for menuID := range invalidMenus {
		delete(selected, menuID)
	}

	result := make([]TenantMenuAuthorizationSelection, 0, len(selected))
	for _, item := range selected {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].MenuID < result[j].MenuID
	})
	return result, warnings, nil
}

func platformBaselineSelections(
	menus []RoleAuthorizationMenu,
) []TenantMenuAuthorizationSelection {
	selections := make([]TenantMenuAuthorizationSelection, 0, len(menus))
	for _, menu := range menus {
		functions := make([]string, 0, len(menu.Functions))
		for _, function := range menu.Functions {
			functions = append(functions, function.Name)
		}
		sort.Strings(functions)
		selections = append(selections, TenantMenuAuthorizationSelection{
			MenuID: menu.ID,
			Funcs:  functions,
		})
	}
	sort.Slice(selections, func(i, j int) bool {
		return selections[i].MenuID < selections[j].MenuID
	})
	return selections
}

func selectedTenantMenuCounts(selections []TenantMenuAuthorizationSelection) (int, int) {
	functionCount := 0
	for _, selection := range selections {
		functionCount += len(selection.Funcs)
	}
	return len(selections), functionCount
}

func roleSelectionCounts(selections []RoleAuthorizationSelection) (int, int) {
	functionCount := 0
	for _, selection := range selections {
		functionCount += len(selection.Funcs)
	}
	return len(selections), functionCount
}

func pruneRoleSelectionsToTenantBoundary(
	current []RoleAuthorizationSelection,
	allowed map[string]TenantMenuAuthorizationSelection,
	candidates map[string]*tenantMenuAuthorizationCandidate,
) (
	[]RoleAuthorizationSelection,
	[]string,
	[]TenantMenuAuthorizationRemovedFunction,
) {
	projected := make([]RoleAuthorizationSelection, 0, len(current))
	removedMenus := make([]string, 0)
	removedFunctions := make([]TenantMenuAuthorizationRemovedFunction, 0)
	for _, selection := range current {
		grant, menuAllowed := allowed[selection.MenuID]
		candidate := candidates[selection.MenuID]
		if !menuAllowed || candidate == nil {
			removedMenus = append(removedMenus, selection.MenuID)
			continue
		}
		allowedFunctions := make(map[string]struct{}, len(grant.Funcs))
		for _, functionName := range grant.Funcs {
			allowedFunctions[functionName] = struct{}{}
		}
		kept := make([]string, 0, len(selection.Funcs))
		for _, functionName := range selection.Funcs {
			if _, exists := allowedFunctions[functionName]; exists {
				kept = append(kept, functionName)
				continue
			}
			functionTitle := functionName
			if function := candidate.funcByName[functionName]; function != nil && function.Title != "" {
				functionTitle = function.Title
			}
			removedFunctions = append(removedFunctions, TenantMenuAuthorizationRemovedFunction{
				MenuID:        selection.MenuID,
				MenuTitle:     candidate.menu.Title,
				FunctionName:  functionName,
				FunctionTitle: functionTitle,
			})
		}
		if !selection.Show && len(kept) == 0 {
			continue
		}
		projected = append(projected, RoleAuthorizationSelection{
			MenuID: selection.MenuID,
			Show:   selection.Show,
			Funcs:  kept,
		})
	}
	projected, _ = normalizeAuthorizationSelections(projected)
	sort.Strings(removedMenus)
	sort.Slice(removedFunctions, func(i, j int) bool {
		if removedFunctions[i].MenuTitle != removedFunctions[j].MenuTitle {
			return removedFunctions[i].MenuTitle < removedFunctions[j].MenuTitle
		}
		return removedFunctions[i].FunctionTitle < removedFunctions[j].FunctionTitle
	})
	return projected, removedMenus, removedFunctions
}

func projectedTenantRolePolicyCount(
	role *Role,
	selections []RoleAuthorizationSelection,
	candidates map[string]*tenantMenuAuthorizationCandidate,
) int {
	if role == nil || !role.Enable {
		return 0
	}
	keys := make(map[string]struct{})
	for _, selection := range selections {
		candidate := candidates[selection.MenuID]
		if candidate == nil {
			continue
		}
		for _, functionName := range selection.Funcs {
			function := candidate.funcByName[functionName]
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
	}
	return len(keys)
}

func loadTenantRoleProjections(
	db *gorm.DB,
	targetTenantID string,
	selections []TenantMenuAuthorizationSelection,
	candidates map[string]*tenantMenuAuthorizationCandidate,
) (
	[]TenantMenuAuthorizationRoleImpact,
	map[string]tenantMenuAuthorizationRoleProjection,
	TenantMenuAuthorizationSummary,
	error,
) {
	allowed := make(map[string]TenantMenuAuthorizationSelection, len(selections))
	for _, selection := range selections {
		allowed[selection.MenuID] = selection
	}
	var roles []*Role
	if err := db.
		Where("tenant_id = ? AND public = ?", targetTenantID, false).
		Order("name ASC, id ASC").
		Find(&roles).Error; err != nil {
		return nil, nil, TenantMenuAuthorizationSummary{}, err
	}
	roleIDs := make([]string, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
	}
	assignedCounts := make(map[string]int64, len(roles))
	if len(roleIDs) > 0 {
		var rows []struct {
			RoleID string
			Count  int64
		}
		if err := db.Table("user_roles").
			Select("role_id, COUNT(DISTINCT user_id) AS count").
			Where("role_id IN ?", roleIDs).
			Group("role_id").
			Scan(&rows).Error; err != nil {
			return nil, nil, TenantMenuAuthorizationSummary{}, err
		}
		for _, row := range rows {
			assignedCounts[row.RoleID] = row.Count
		}
	}

	summary := TenantMenuAuthorizationSummary{RoleCount: len(roles)}
	impacts := make([]TenantMenuAuthorizationRoleImpact, 0, len(roles))
	projections := make(map[string]tenantMenuAuthorizationRoleProjection, len(roles))
	affectedRoleIDs := make([]string, 0)
	for _, role := range roles {
		current, err := loadCurrentAuthorizationSelections(db, role.ID)
		if err != nil {
			return nil, nil, TenantMenuAuthorizationSummary{}, err
		}
		projected, removedMenus, removedFunctions := pruneRoleSelectionsToTenantBoundary(
			current,
			allowed,
			candidates,
		)
		changed := roleAuthorizationRevision(current) != roleAuthorizationRevision(projected)
		currentMenuCount, currentFunctionCount := roleSelectionCounts(current)
		retainedMenuCount, retainedFunctionCount := roleSelectionCounts(projected)
		policyCount := projectedTenantRolePolicyCount(role, projected, candidates)
		impact := TenantMenuAuthorizationRoleImpact{
			RoleID:                role.ID,
			RoleName:              role.Name,
			RoleEnabled:           role.Enable,
			Changed:               changed,
			CurrentMenuCount:      currentMenuCount,
			CurrentFunctionCount:  currentFunctionCount,
			RetainedMenuCount:     retainedMenuCount,
			RetainedFunctionCount: retainedFunctionCount,
			RemovedMenuIDs:        removedMenus,
			RemovedFunctions:      removedFunctions,
			GeneratedPolicyCount:  policyCount,
			AssignedUserCount:     assignedCounts[role.ID],
		}
		impacts = append(impacts, impact)
		projections[role.ID] = tenantMenuAuthorizationRoleProjection{
			role:      role,
			current:   current,
			projected: projected,
			changed:   changed,
		}
		summary.GeneratedPolicyCount += policyCount
		if changed {
			summary.AffectedRoleCount++
			summary.PrunedMenuCount += len(removedMenus)
			summary.PrunedFunctionCount += len(removedFunctions)
			affectedRoleIDs = append(affectedRoleIDs, role.ID)
			if role.Enable {
				summary.ActiveRoleCount++
			}
		}
	}
	if len(affectedRoleIDs) > 0 {
		if err := db.Table("user_roles").
			Where("role_id IN ?", affectedRoleIDs).
			Distinct("user_id").
			Count(&summary.AffectedUserCount).Error; err != nil {
			return nil, nil, TenantMenuAuthorizationSummary{}, err
		}
	}
	return impacts, projections, summary, nil
}

func buildTenantMenuAuthorizationPreview(
	db *gorm.DB,
	target *tenantAuthorizationRecord,
	input []TenantMenuAuthorizationSelection,
	strict bool,
) (
	*TenantMenuAuthorizationPreview,
	[]RoleAuthorizationMenu,
	map[string]tenantMenuAuthorizationRoleProjection,
	error,
) {
	menus, candidates, err := loadTenantMenuAuthorizationCandidates(db, target.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	current, err := loadCurrentTenantMenuSelections(db, target.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	selections, warnings, err := sanitizeTenantMenuSelections(input, candidates, strict)
	if err != nil {
		return nil, nil, nil, err
	}
	if target.ID == constants.PlatformTenantID {
		selections = platformBaselineSelections(menus)
		warnings = append(warnings, "platform tenant uses the unrestricted system menu baseline")
	}
	impacts, projections, summary, err := loadTenantRoleProjections(
		db,
		target.ID,
		selections,
		candidates,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	summary.CandidateMenuCount = len(menus)
	summary.SelectedMenuCount, summary.SelectedFunctionCount = selectedTenantMenuCounts(selections)
	if !target.Enable {
		warnings = append(warnings, "the target tenant is disabled; authorization is retained but no new login context can be issued")
	}
	if summary.AffectedRoleCount > 0 {
		warnings = append(
			warnings,
			fmt.Sprintf(
				"%d role authorizations will be narrowed and %d user login contexts will be refreshed",
				summary.AffectedRoleCount,
				summary.AffectedUserCount,
			),
		)
	}
	return &TenantMenuAuthorizationPreview{
		Tenant:           tenantAuthorizationTarget(target),
		CurrentRevision:  tenantMenuAuthorizationRevision(current),
		ProposedRevision: tenantMenuAuthorizationRevision(selections),
		Selections:       selections,
		Roles:            impacts,
		Summary:          summary,
		Warnings:         warnings,
	}, menus, projections, nil
}

func countTenantAuthorizationRolesAndUsers(db *gorm.DB, tenantID string) (int64, int64, error) {
	var roleCount int64
	if err := db.Model(&Role{}).
		Where("tenant_id = ? AND public = ?", tenantID, false).
		Count(&roleCount).Error; err != nil {
		return 0, 0, err
	}
	var assignedUserCount int64
	if err := db.Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id AND roles.deleted_at IS NULL").
		Where("roles.tenant_id = ? AND roles.public = ? AND user_roles.deleted_at IS NULL", tenantID, false).
		Distinct("user_roles.user_id").
		Count(&assignedUserCount).Error; err != nil {
		return 0, 0, err
	}
	return roleCount, assignedUserCount, nil
}

func ListTenantMenuAuthorizationTargets(
	actorTenantID string,
) (*TenantMenuAuthorizationTargetList, error) {
	actorTenantID = strings.TrimSpace(actorTenantID)
	if actorTenantID == "" {
		return nil, ErrTenantMenuAuthorizationForbidden
	}
	query := store.DB().Table("tenants").
		Select("id", "name", "enable", "is_must").
		Where("deleted_at IS NULL")
	if actorTenantID != constants.PlatformTenantID {
		query = query.Where("id = ?", actorTenantID)
	}
	var records []*tenantAuthorizationRecord
	if err := query.Order("name ASC, id ASC").Scan(&records).Error; err != nil {
		return nil, err
	}
	sort.SliceStable(records, func(i, j int) bool {
		leftPlatform := records[i].ID == constants.PlatformTenantID
		rightPlatform := records[j].ID == constants.PlatformTenantID
		if leftPlatform != rightPlatform {
			return !leftPlatform
		}
		if records[i].Name != records[j].Name {
			return records[i].Name < records[j].Name
		}
		return records[i].ID < records[j].ID
	})
	targets := make([]TenantMenuAuthorizationTarget, 0, len(records))
	for _, record := range records {
		target := tenantAuthorizationTarget(record)
		var selections []TenantMenuAuthorizationSelection
		if target.SystemBaseline {
			menus, _, err := loadTenantMenuAuthorizationCandidates(store.DB(), target.ID)
			if err != nil {
				return nil, err
			}
			selections = platformBaselineSelections(menus)
		} else {
			var err error
			selections, err = loadCurrentTenantMenuSelections(store.DB(), target.ID)
			if err != nil {
				return nil, err
			}
		}
		target.AuthorizedMenuCount, target.AuthorizedFunctionCount = selectedTenantMenuCounts(selections)
		roleCount, assignedUserCount, err := countTenantAuthorizationRolesAndUsers(store.DB(), target.ID)
		if err != nil {
			return nil, err
		}
		target.RoleCount = roleCount
		target.AssignedUserCount = assignedUserCount
		targets = append(targets, target)
	}
	return &TenantMenuAuthorizationTargetList{
		ActorTenantID:        actorTenantID,
		PlatformTenantID:     constants.PlatformTenantID,
		CanManageCrossTenant: actorTenantID == constants.PlatformTenantID,
		Targets:              targets,
	}, nil
}

func GetTenantMenuAuthorization(
	actorTenantID,
	targetTenantID string,
) (*TenantMenuAuthorizationDetail, error) {
	if err := authorizeTenantMenuTarget(actorTenantID, targetTenantID); err != nil {
		return nil, err
	}
	target, err := loadTenantAuthorizationRecord(store.DB(), targetTenantID, false)
	if err != nil {
		return nil, err
	}
	current, err := loadCurrentTenantMenuSelections(store.DB(), target.ID)
	if err != nil {
		return nil, err
	}
	preview, menus, _, err := buildTenantMenuAuthorizationPreview(
		store.DB(),
		target,
		current,
		false,
	)
	if err != nil {
		return nil, err
	}
	targetView := preview.Tenant
	targetView.AuthorizedMenuCount = preview.Summary.SelectedMenuCount
	targetView.AuthorizedFunctionCount = preview.Summary.SelectedFunctionCount
	roleCount, assignedUserCount, err := countTenantAuthorizationRolesAndUsers(store.DB(), target.ID)
	if err != nil {
		return nil, err
	}
	targetView.RoleCount = roleCount
	targetView.AssignedUserCount = assignedUserCount
	return &TenantMenuAuthorizationDetail{
		Tenant:         targetView,
		Revision:       preview.CurrentRevision,
		Selections:     preview.Selections,
		Menus:          menus,
		Roles:          preview.Roles,
		Summary:        preview.Summary,
		Warnings:       preview.Warnings,
		SystemBaseline: target.ID == constants.PlatformTenantID,
	}, nil
}

func PreviewTenantMenuAuthorization(
	actorTenantID,
	targetTenantID string,
	selections []TenantMenuAuthorizationSelection,
) (*TenantMenuAuthorizationPreview, error) {
	if err := authorizeTenantMenuTarget(actorTenantID, targetTenantID); err != nil {
		return nil, err
	}
	target, err := loadTenantAuthorizationRecord(store.DB(), targetTenantID, false)
	if err != nil {
		return nil, err
	}
	if target.ID == constants.PlatformTenantID {
		return nil, ErrTenantMenuAuthorizationSystemBaseline
	}
	preview, _, _, err := buildTenantMenuAuthorizationPreview(store.DB(), target, selections, true)
	return preview, err
}

func replaceTenantRoleProjection(
	tx *gorm.DB,
	projection tenantMenuAuthorizationRoleProjection,
) error {
	if !projection.changed {
		return nil
	}
	if err := tx.Unscoped().Where("role_id = ?", projection.role.ID).Delete(&RoleMenu{}).Error; err != nil {
		return err
	}
	for _, selection := range projection.projected {
		roleMenu := &RoleMenu{
			Model:  commonmodel.Model{},
			RoleID: projection.role.ID,
			MenuID: selection.MenuID,
			Funcs:  strings.Join(selection.Funcs, ","),
			Show:   selection.Show,
		}
		if err := tx.Create(roleMenu).Error; err != nil {
			return err
		}
	}
	return nil
}

func PublishTenantMenuAuthorization(
	actorTenantID,
	targetTenantID,
	baseRevision string,
	selections []TenantMenuAuthorizationSelection,
) (*TenantMenuAuthorizationPublishResult, error) {
	if err := authorizeTenantMenuTarget(actorTenantID, targetTenantID); err != nil {
		return nil, err
	}
	baseRevision = strings.TrimSpace(baseRevision)
	if baseRevision == "" {
		return nil, errors.New("base revision cannot be empty")
	}
	result := &TenantMenuAuthorizationPublishResult{}
	reloadPolicies := false
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		target, err := loadTenantAuthorizationRecord(tx, targetTenantID, true)
		if err != nil {
			return err
		}
		if target.ID == constants.PlatformTenantID {
			return ErrTenantMenuAuthorizationSystemBaseline
		}
		current, err := loadCurrentTenantMenuSelections(tx, target.ID)
		if err != nil {
			return err
		}
		currentRevision := tenantMenuAuthorizationRevision(current)
		if currentRevision != baseRevision {
			return fmt.Errorf(
				"%w: current revision is %s",
				ErrTenantMenuAuthorizationRevisionConflict,
				currentRevision,
			)
		}
		preview, _, projections, err := buildTenantMenuAuthorizationPreview(
			tx,
			target,
			selections,
			true,
		)
		if err != nil {
			return err
		}
		if err := tx.Unscoped().
			Where("tenant_id = ?", target.ID).
			Delete(&tenantMenuAuthorizationGrant{}).Error; err != nil {
			return err
		}
		for _, selection := range preview.Selections {
			grant := &tenantMenuAuthorizationGrant{
				Model:    commonmodel.Model{},
				TenantID: target.ID,
				MenuID:   selection.MenuID,
				Funcs:    strings.Join(selection.Funcs, ","),
			}
			if err := tx.Create(grant).Error; err != nil {
				return err
			}
		}

		affectedRoleIDs := make([]string, 0)
		for roleID, projection := range projections {
			if !projection.changed {
				continue
			}
			if err := replaceTenantRoleProjection(tx, projection); err != nil {
				return err
			}
			affectedRoleIDs = append(affectedRoleIDs, roleID)
		}
		if err := rebuildRoleAuthorizationPolicies(tx, affectedRoleIDs); err != nil {
			return err
		}
		reloadPolicies = len(affectedRoleIDs) > 0
		if len(affectedRoleIDs) > 0 {
			if err := tx.Table("user_roles").
				Where("role_id IN ?", affectedRoleIDs).
				Distinct("user_id").
				Pluck("user_id", &result.AffectedUserIDs).Error; err != nil {
				return err
			}
			result.AffectedUserIDs = uniqueStrings(result.AffectedUserIDs)
			if len(result.AffectedUserIDs) > 0 {
				update := tx.Table("user_session").
					Where("principal_id IN ? AND revoked = ?", result.AffectedUserIDs, false).
					Updates(map[string]interface{}{
						"revoked":        true,
						"revoked_reason": "tenant menu authorization published",
					})
				if update.Error != nil {
					return update.Error
				}
				result.SessionsRevoked = update.RowsAffected
			}
		}
		result.Revision = preview.ProposedRevision
		result.Summary = preview.Summary
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
