// Package accesseffect projects the native usercenter login and authorization
// state into one evidence model. It deliberately reuses user, role, menu, API,
// Casbin, session and audit storage instead of maintaining another permission
// graph.
package accesseffect

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/CloudSilk/pkg/constants"
	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	"gorm.io/gorm"
)

const (
	DetailPath = "/api/core/auth/permission/effectiveness"
	CheckPath  = "/api/core/auth/permission/effectiveness/check"
	SelfPath   = "/api/core/auth/permission/effectiveness/self"
)

type Identity struct {
	ID         string `json:"id"`
	UserName   string `json:"userName"`
	Nickname   string `json:"nickname"`
	TenantID   string `json:"tenantID"`
	TenantName string `json:"tenantName"`
	Mobile     string `json:"mobile"`
	Enable     bool   `json:"enable"`
	Type       int32  `json:"type"`
}

type RoleSnapshot struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	TenantID       string `json:"tenantID"`
	Enable         bool   `json:"enable"`
	Public         bool   `json:"public"`
	InLoginContext bool   `json:"inLoginContext"`
}

type PersistedPolicy struct {
	Path      string `json:"path"`
	Method    string `json:"method"`
	CheckAuth string `json:"checkAuth"`
}

type RoleProjection struct {
	Role              RoleSnapshot                         `json:"role"`
	Revision          string                               `json:"revision"`
	Menus             []permission.RoleAuthorizationMenu   `json:"menus"`
	ProjectedPolicies []permission.RoleAuthorizationPolicy `json:"projectedPolicies"`
	PersistedPolicies []PersistedPolicy                    `json:"persistedPolicies"`
	Warnings          []string                             `json:"warnings"`
}

type APISource struct {
	RoleID         string `json:"roleID"`
	RoleName       string `json:"roleName"`
	RoleEnabled    bool   `json:"roleEnabled"`
	InLoginContext bool   `json:"inLoginContext"`
	MenuID         string `json:"menuID"`
	MenuTitle      string `json:"menuTitle"`
	FunctionID     string `json:"functionID"`
	FunctionName   string `json:"functionName"`
	FunctionTitle  string `json:"functionTitle"`
	APIID          string `json:"apiID"`
	Path           string `json:"path"`
	Method         string `json:"method"`
	Description    string `json:"description"`
}

type AuditEvidence struct {
	ID            string         `json:"id"`
	OperatorID    string         `json:"operatorID"`
	OperatorName  string         `json:"operatorName"`
	PrincipalKind int32          `json:"principalKind"`
	Action        string         `json:"action"`
	TargetID      string         `json:"targetID"`
	IP            string         `json:"ip"`
	Detail        map[string]any `json:"detail"`
	CreatedAt     time.Time      `json:"createdAt"`
}

type Summary struct {
	AssignedRoleCount    int `json:"assignedRoleCount"`
	LoginRoleCount       int `json:"loginRoleCount"`
	EffectiveMenuCount   int `json:"effectiveMenuCount"`
	EffectiveFuncCount   int `json:"effectiveFunctionCount"`
	EffectiveAPICount    int `json:"effectiveAPICount"`
	ProjectedPolicyCount int `json:"projectedPolicyCount"`
	PersistedPolicyCount int `json:"persistedPolicyCount"`
	ActiveSessionCount   int `json:"activeSessionCount"`
	RecentCheckCount     int `json:"recentCheckCount"`
}

type Detail struct {
	Identity        Identity         `json:"identity"`
	AssignedRoles   []RoleSnapshot   `json:"assignedRoles"`
	LoginRoleIDs    []string         `json:"loginRoleIDs"`
	RoleProjections []RoleProjection `json:"roleProjections"`
	APISources      []APISource      `json:"apiSources"`
	RecentChecks    []AuditEvidence  `json:"recentChecks"`
	Summary         Summary          `json:"summary"`
	Warnings        []string         `json:"warnings"`
	GeneratedAt     time.Time        `json:"generatedAt"`
}

type CheckRequest struct {
	UserID string `json:"userID" binding:"required"`
	APIID  string `json:"apiID" binding:"required"`
}

type APIReference struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenantID"`
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description"`
	Enable      bool   `json:"enable"`
	CheckAuth   bool   `json:"checkAuth"`
	CheckLogin  bool   `json:"checkLogin"`
}

type CheckTrace struct {
	Subject string `json:"subject"`
	Source  string `json:"source"`
	Allow   bool   `json:"allow"`
}

type CheckResult struct {
	UserID                   string       `json:"userID"`
	UserName                 string       `json:"userName"`
	TenantID                 string       `json:"tenantID"`
	UserEnabled              bool         `json:"userEnabled"`
	API                      APIReference `json:"api"`
	Allow                    bool         `json:"allow"`
	Decision                 string       `json:"decision"`
	Reason                   string       `json:"reason"`
	PolicyClass              string       `json:"policyClass"`
	LoginRoleIDs             []string     `json:"loginRoleIDs"`
	MatchedRoleIDs           []string     `json:"matchedRoleIDs"`
	Sources                  []APISource  `json:"sources"`
	Trace                    []CheckTrace `json:"trace"`
	PolicyMetadataConsistent bool         `json:"policyMetadataConsistent"`
	MetadataMessage          string       `json:"metadataMessage"`
	CheckedAt                time.Time    `json:"checkedAt"`
}

func roleSnapshot(role *permission.Role, loginRoles map[string]struct{}) RoleSnapshot {
	if role == nil {
		return RoleSnapshot{}
	}
	_, inLoginContext := loginRoles[role.ID]
	return RoleSnapshot{
		ID:             role.ID,
		Name:           role.Name,
		TenantID:       role.TenantID,
		Enable:         role.Enable,
		Public:         role.Public,
		InLoginContext: inLoginContext,
	}
}

func selectedMenus(
	menus []permission.RoleAuthorizationMenu,
	selections []permission.RoleAuthorizationSelection,
) []permission.RoleAuthorizationMenu {
	selected := make(map[string]map[string]struct{}, len(selections))
	for _, selection := range selections {
		functions := make(map[string]struct{}, len(selection.Funcs))
		for _, functionName := range selection.Funcs {
			functions[functionName] = struct{}{}
		}
		selected[selection.MenuID] = functions
	}
	result := make([]permission.RoleAuthorizationMenu, 0, len(selections))
	for _, menu := range menus {
		functionNames, ok := selected[menu.ID]
		if !ok {
			continue
		}
		filteredFunctions := make([]permission.RoleAuthorizationFunction, 0, len(menu.Functions))
		for _, function := range menu.Functions {
			if _, selected := functionNames[function.Name]; selected {
				filteredFunctions = append(filteredFunctions, function)
			}
		}
		menu.Functions = filteredFunctions
		result = append(result, menu)
	}
	return result
}

func persistedRolePolicies(roleID string) ([]PersistedPolicy, error) {
	var rules []permission.CasbinRule
	if err := store.DB().
		Where("ptype = ? AND v0 = ?", "p", roleID).
		Order("v1 asc, v2 asc").
		Find(&rules).Error; err != nil {
		return nil, err
	}
	result := make([]PersistedPolicy, 0, len(rules))
	for _, rule := range rules {
		result = append(result, PersistedPolicy{
			Path:      rule.Path,
			Method:    rule.Method,
			CheckAuth: rule.CheckAuth,
		})
	}
	return result, nil
}

func sourcesFromProjection(projection RoleProjection) []APISource {
	result := make([]APISource, 0)
	for _, menu := range projection.Menus {
		for _, function := range menu.Functions {
			for _, api := range function.APIs {
				result = append(result, APISource{
					RoleID:         projection.Role.ID,
					RoleName:       projection.Role.Name,
					RoleEnabled:    projection.Role.Enable,
					InLoginContext: projection.Role.InLoginContext,
					MenuID:         menu.ID,
					MenuTitle:      menu.Title,
					FunctionID:     function.ID,
					FunctionName:   function.Name,
					FunctionTitle:  function.Title,
					APIID:          api.ID,
					Path:           api.Path,
					Method:         api.Method,
					Description:    api.Description,
				})
			}
		}
	}
	return result
}

func auditEvidence(logs []*audit.AuditLog) []AuditEvidence {
	result := make([]AuditEvidence, 0, len(logs))
	for _, log := range logs {
		if log == nil {
			continue
		}
		detail := map[string]any{}
		if err := json.Unmarshal([]byte(log.Detail), &detail); err != nil && strings.TrimSpace(log.Detail) != "" {
			detail["raw"] = log.Detail
		}
		result = append(result, AuditEvidence{
			ID:            log.ID,
			OperatorID:    log.UserID,
			OperatorName:  log.UserName,
			PrincipalKind: log.PrincipalKind,
			Action:        log.Action,
			TargetID:      log.TargetID,
			IP:            log.IP,
			Detail:        detail,
			CreatedAt:     log.CreatedAt,
		})
	}
	return result
}

func projectionWarning(projected []permission.RoleAuthorizationPolicy, persisted []PersistedPolicy) string {
	projectedKeys := make(map[string]struct{}, len(projected))
	for _, policy := range projected {
		projectedKeys[policy.Method+" "+policy.Path] = struct{}{}
	}
	persistedKeys := make(map[string]struct{}, len(persisted))
	for _, policy := range persisted {
		persistedKeys[policy.Method+" "+policy.Path] = struct{}{}
	}
	if len(projectedKeys) != len(persistedKeys) {
		return "projected role policies and persisted Casbin policies differ"
	}
	for key := range projectedKeys {
		if _, ok := persistedKeys[key]; !ok {
			return "projected role policies and persisted Casbin policies differ"
		}
	}
	return ""
}

// GetDetail joins the native login identity, enabled JWT role snapshot,
// selected menu/function/API graph, persisted Casbin rules, sessions and recent
// verification audits.
func GetDetail(userID string) (*Detail, error) {
	target, err := user.GetUserById(strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	tenantName := target.TenantID
	if value, tenantErr := tenant.GetTenantByID(target.TenantID); tenantErr == nil {
		tenantName = value.Name
	}

	loginRoleIDs := target.GetEnabledRoleIDs()
	loginRoleSet := make(map[string]struct{}, len(loginRoleIDs))
	for _, roleID := range loginRoleIDs {
		loginRoleSet[roleID] = struct{}{}
	}

	assignedRoles := make([]RoleSnapshot, 0, len(target.UserRoles))
	projections := make([]RoleProjection, 0, len(target.UserRoles))
	allSources := make([]APISource, 0)
	warnings := make([]string, 0)
	for _, userRole := range target.UserRoles {
		if userRole == nil {
			continue
		}
		role := userRole.Role
		if role == nil {
			role, err = permission.GetRoleByID(userRole.RoleID)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("assigned role %s no longer exists", userRole.RoleID))
				continue
			}
		}
		snapshot := roleSnapshot(role, loginRoleSet)
		assignedRoles = append(assignedRoles, snapshot)
		authorization, authErr := permission.GetRoleAuthorization(role.ID)
		if authErr != nil {
			warnings = append(warnings, fmt.Sprintf("cannot project role %s: %v", role.ID, authErr))
			continue
		}
		persisted, policyErr := persistedRolePolicies(role.ID)
		if policyErr != nil {
			return nil, policyErr
		}
		projection := RoleProjection{
			Role:              snapshot,
			Revision:          authorization.Revision,
			Menus:             selectedMenus(authorization.Menus, authorization.Selections),
			ProjectedPolicies: authorization.Policies,
			PersistedPolicies: persisted,
			Warnings:          append([]string(nil), authorization.Warnings...),
		}
		if warning := projectionWarning(projection.ProjectedPolicies, projection.PersistedPolicies); warning != "" {
			projection.Warnings = append(projection.Warnings, warning)
			warnings = append(warnings, fmt.Sprintf("%s: %s", role.Name, warning))
		}
		if !snapshot.InLoginContext {
			projection.Warnings = append(projection.Warnings, "role is assigned but excluded from a new login context")
		}
		projections = append(projections, projection)
		allSources = append(allSources, sourcesFromProjection(projection)...)
	}
	sort.Slice(assignedRoles, func(i, j int) bool {
		if assignedRoles[i].Name != assignedRoles[j].Name {
			return assignedRoles[i].Name < assignedRoles[j].Name
		}
		return assignedRoles[i].ID < assignedRoles[j].ID
	})
	sort.Slice(projections, func(i, j int) bool {
		if projections[i].Role.Name != projections[j].Role.Name {
			return projections[i].Role.Name < projections[j].Role.Name
		}
		return projections[i].Role.ID < projections[j].Role.ID
	})
	sort.Slice(allSources, func(i, j int) bool {
		left := allSources[i].RoleName + allSources[i].MenuTitle + allSources[i].FunctionTitle + allSources[i].Method + allSources[i].Path
		right := allSources[j].RoleName + allSources[j].MenuTitle + allSources[j].FunctionTitle + allSources[j].Method + allSources[j].Path
		return left < right
	})

	activeSessions, err := session.ListSessions(target.ID)
	if err != nil {
		return nil, err
	}
	logs, _, err := audit.QueryAuditLogs(&audit.AuditQuery{
		Action:        audit.AuditActionVerifyPermissionEffect,
		TargetID:      target.ID,
		PrincipalKind: -1,
		PageIndex:     1,
		PageSize:      10,
	})
	if err != nil {
		return nil, err
	}

	menuIDs := map[string]struct{}{}
	functionIDs := map[string]struct{}{}
	apiIDs := map[string]struct{}{}
	projectedPolicyCount := 0
	persistedPolicyCount := 0
	for _, projection := range projections {
		if !projection.Role.InLoginContext {
			continue
		}
		projectedPolicyCount += len(projection.ProjectedPolicies)
		persistedPolicyCount += len(projection.PersistedPolicies)
		for _, menu := range projection.Menus {
			menuIDs[menu.ID] = struct{}{}
			for _, function := range menu.Functions {
				functionIDs[function.ID] = struct{}{}
				for _, api := range function.APIs {
					apiIDs[api.ID] = struct{}{}
				}
			}
		}
	}

	recentChecks := auditEvidence(logs)
	return &Detail{
		Identity: Identity{
			ID:         target.ID,
			UserName:   target.UserName,
			Nickname:   target.Nickname,
			TenantID:   target.TenantID,
			TenantName: tenantName,
			Mobile:     target.Mobile,
			Enable:     target.Enable,
			Type:       target.Type,
		},
		AssignedRoles:   assignedRoles,
		LoginRoleIDs:    loginRoleIDs,
		RoleProjections: projections,
		APISources:      allSources,
		RecentChecks:    recentChecks,
		Summary: Summary{
			AssignedRoleCount:    len(assignedRoles),
			LoginRoleCount:       len(loginRoleIDs),
			EffectiveMenuCount:   len(menuIDs),
			EffectiveFuncCount:   len(functionIDs),
			EffectiveAPICount:    len(apiIDs),
			ProjectedPolicyCount: projectedPolicyCount,
			PersistedPolicyCount: persistedPolicyCount,
			ActiveSessionCount:   len(activeSessions),
			RecentCheckCount:     len(recentChecks),
		},
		Warnings:    warnings,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func policyClass(api permission.API) string {
	if !api.Enable {
		return "disabled"
	}
	if !api.CheckLogin {
		return "public"
	}
	if !api.CheckAuth {
		return "authenticated"
	}
	return "role"
}

func metadataConsistency(api permission.API) (bool, string, error) {
	var rules []permission.CasbinRule
	if err := store.DB().
		Where("ptype = ? AND v1 = ? AND v2 = ?", "p", api.Path, api.Method).
		Find(&rules).Error; err != nil {
		return false, "", err
	}
	hasPublic := false
	hasAuthenticated := false
	for _, rule := range rules {
		switch rule.RoleID {
		case "-1":
			hasPublic = true
		case "0":
			hasAuthenticated = true
		}
	}
	switch policyClass(api) {
	case "disabled":
		if len(rules) == 0 {
			return true, "API 已停用，且不存在生效中的 Casbin 策略", nil
		}
		return false, "API 已停用，但仍存在生效中的 Casbin 策略", nil
	case "public":
		if hasPublic {
			return true, "API 元数据与公开访问 Casbin 策略一致", nil
		}
		return false, "API 元数据标记为公开访问，但缺少公开访问 Casbin 策略", nil
	case "authenticated":
		if hasAuthenticated && !hasPublic {
			return true, "API 元数据与仅登录访问 Casbin 策略一致", nil
		}
		return false, "API 元数据与仅登录访问 Casbin 策略不一致", nil
	default:
		if !hasPublic && !hasAuthenticated {
			return true, "API 受角色权限保护，不存在公开或仅登录绕过策略", nil
		}
		return false, "角色权限 API 存在公开或仅登录绕过策略", nil
	}
}

func sourcesForAPI(roleIDs []string, apiID string) ([]APISource, error) {
	result := make([]APISource, 0)
	for _, roleID := range roleIDs {
		authorization, err := permission.GetRoleAuthorization(roleID)
		if err != nil {
			return nil, err
		}
		role, err := permission.GetRoleByID(roleID)
		if err != nil {
			return nil, err
		}
		projection := RoleProjection{
			Role:  roleSnapshot(role, map[string]struct{}{roleID: {}}),
			Menus: selectedMenus(authorization.Menus, authorization.Selections),
		}
		for _, source := range sourcesFromProjection(projection) {
			if source.APIID == apiID {
				result = append(result, source)
			}
		}
	}
	return result, nil
}

// Check follows the same Casbin order as AuthenticatePrincipal: public (-1),
// login-only (0), then every enabled role included in a newly issued JWT.
func Check(userID, apiID string) (*CheckResult, error) {
	target, err := user.GetUserById(strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	api, err := permission.GetAPIById(strings.TrimSpace(apiID))
	if err != nil {
		return nil, err
	}
	loginRoleIDs := target.GetEnabledRoleIDs()
	result := &CheckResult{
		UserID:         target.ID,
		UserName:       target.UserName,
		TenantID:       target.TenantID,
		UserEnabled:    target.Enable,
		LoginRoleIDs:   append([]string{}, loginRoleIDs...),
		MatchedRoleIDs: make([]string, 0),
		API: APIReference{
			ID:          api.ID,
			TenantID:    api.TenantID,
			Path:        api.Path,
			Method:      api.Method,
			Description: api.Description,
			Enable:      api.Enable,
			CheckAuth:   api.CheckAuth,
			CheckLogin:  api.CheckLogin,
		},
		Decision:    "deny",
		Reason:      "no_matching_policy",
		PolicyClass: policyClass(api),
		CheckedAt:   time.Now().UTC(),
	}

	publicAllow, err := permission.EnforceCached("-1", api.Path, api.Method)
	if err != nil {
		return nil, err
	}
	result.Trace = append(result.Trace, CheckTrace{Subject: "-1", Source: "public", Allow: publicAllow})
	if publicAllow {
		result.Allow = true
		result.Decision = "allow"
		result.Reason = "public_policy"
	} else if !target.Enable {
		result.Trace = append(result.Trace, CheckTrace{Subject: target.ID, Source: "account", Allow: false})
		result.Reason = "user_disabled"
	} else {
		loginAllow, enforceErr := permission.EnforceCached("0", api.Path, api.Method)
		if enforceErr != nil {
			return nil, enforceErr
		}
		result.Trace = append(result.Trace, CheckTrace{Subject: "0", Source: "authenticated", Allow: loginAllow})
		if loginAllow {
			result.Allow = true
			result.Decision = "allow"
			result.Reason = "authenticated_policy"
		} else {
			for _, roleID := range loginRoleIDs {
				roleAllow, roleErr := permission.EnforceCached(roleID, api.Path, api.Method)
				if roleErr != nil {
					return nil, roleErr
				}
				result.Trace = append(result.Trace, CheckTrace{Subject: roleID, Source: "role", Allow: roleAllow})
				if roleAllow {
					result.Allow = true
					result.Decision = "allow"
					result.Reason = "role_policy"
					result.MatchedRoleIDs = append(result.MatchedRoleIDs, roleID)
				}
			}
		}
	}
	result.Sources, err = sourcesForAPI(result.MatchedRoleIDs, api.ID)
	if err != nil {
		return nil, err
	}
	result.PolicyMetadataConsistent, result.MetadataMessage, err = metadataConsistency(api)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// AuditDetail returns a compact but complete decision envelope that fits the
// native audit detail column and can be compared with a later detail projection.
func AuditDetail(result *CheckResult) string {
	if result == nil {
		return "{}"
	}
	sourceIDs := make([]string, 0, len(result.Sources))
	for _, source := range result.Sources {
		sourceIDs = append(sourceIDs, source.RoleID+"/"+source.MenuID+"/"+source.FunctionID)
	}
	detail, _ := json.Marshal(map[string]any{
		"apiID":                    result.API.ID,
		"path":                     result.API.Path,
		"method":                   result.API.Method,
		"allow":                    result.Allow,
		"decision":                 result.Decision,
		"reason":                   result.Reason,
		"policyClass":              result.PolicyClass,
		"loginRoleIDs":             result.LoginRoleIDs,
		"matchedRoleIDs":           result.MatchedRoleIDs,
		"sourceIDs":                sourceIDs,
		"policyMetadataConsistent": result.PolicyMetadataConsistent,
		"checkedAt":                result.CheckedAt,
	})
	return string(detail)
}

type systemAPI struct {
	id          string
	path        string
	method      string
	description string
	checkAuth   bool
}

// EnsureAPIResources keeps the three workflow endpoints in usercenter's native
// API catalogue so self can use the login-only policy and tenant roles can bind
// the management endpoints through the normal menu-function workflow.
func EnsureAPIResources() error {
	definitions := []systemAPI{
		{
			id:          "permission-effectiveness-detail",
			path:        DetailPath,
			method:      http.MethodGet,
			description: "查看用户权限生效投影",
			checkAuth:   true,
		},
		{
			id:          "permission-effectiveness-check",
			path:        CheckPath,
			method:      http.MethodPost,
			description: "核验用户 API 权限并记录审计",
			checkAuth:   true,
		},
		{
			id:          "permission-effectiveness-self",
			path:        SelfPath,
			method:      http.MethodGet,
			description: "查看当前登录身份与 JWT 角色",
			checkAuth:   false,
		},
	}
	for _, definition := range definitions {
		current := &permission.API{}
		err := store.DB().Unscoped().
			Where("path = ? AND method = ?", definition.path, definition.method).
			First(current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			current = &permission.API{
				Model:       commonmodel.Model{ID: definition.id},
				TenantID:    constants.PlatformTenantID,
				Group:       "权限生效闭环",
				Path:        definition.path,
				Method:      definition.method,
				Description: definition.description,
				Enable:      true,
				CheckAuth:   definition.checkAuth,
				CheckLogin:  true,
				IsMust:      true,
			}
			if createErr := store.DB().Create(current).Error; createErr != nil {
				return createErr
			}
			continue
		}
		if err != nil {
			return err
		}
		if updateErr := store.DB().Unscoped().Model(current).Updates(map[string]any{
			"tenant_id":   constants.PlatformTenantID,
			"group":       "权限生效闭环",
			"description": definition.description,
			"enable":      true,
			"check_auth":  definition.checkAuth,
			"check_login": true,
			"is_must":     true,
			"deleted_at":  nil,
		}).Error; updateErr != nil {
			return updateErr
		}
	}
	permission.UpdateNotCheckAuthRule()
	permission.UpdateNotCheckLoginRule()
	return nil
}
