package authorization

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/store"
)

const (
	RuntimeCheckPath = "/api/core/auth/authorization/check"
	CatalogApplyPath = "/api/core/auth/authorization/catalog/apply"
)

var supportedRuntimeMethods = map[string]struct{}{
	http.MethodGet: {}, http.MethodHead: {}, http.MethodPost: {}, http.MethodPut: {},
	http.MethodPatch: {}, http.MethodDelete: {}, http.MethodOptions: {},
}

type Decision struct {
	Allow          bool     `json:"allow"`
	Reason         string   `json:"reason"`
	MatchedRoleIDs []string `json:"matchedRoleIDs"`
}

type DataScopeRule struct {
	RoleID    string `json:"roleID,omitempty"`
	DataScope int32  `json:"dataScope"`
	Condition string `json:"condition,omitempty"`
	Priority  int32  `json:"priority,omitempty"`
	Source    string `json:"source"`
}

type DataScopeDecision struct {
	Rules []DataScopeRule `json:"rules"`
}

const (
	DataScopeAll              = int32(permission.DataScopeAll)
	DataScopeTenant           = int32(permission.DataScopeTenant)
	DataScopeOrganization     = int32(permission.DataScopeDept)
	DataScopeSelf             = int32(permission.DataScopeSelf)
	DataScopeCustom           = int32(permission.DataScopeCustom)
	DataScopeTeam             = int32(permission.DataScopeTeam)
	DataScopeOrganizationTree = int32(permission.DataScopeDeptAndChildren)
	DataScopeProject          = int32(permission.DataScopeProject)
)

func EvaluateRoles(roleIDs []string, path, method string) (Decision, error) {
	path, method, err := normalizeRuntimeResource(path, method)
	if err != nil {
		return Decision{}, err
	}

	allowed, err := permission.EnforceCached("-1", path, method)
	if err != nil {
		return Decision{}, err
	}
	if allowed {
		return Decision{Allow: true, Reason: "public_policy", MatchedRoleIDs: []string{}}, nil
	}

	allowed, err = permission.EnforceCached("0", path, method)
	if err != nil {
		return Decision{}, err
	}
	if allowed {
		return Decision{Allow: true, Reason: "authenticated_policy", MatchedRoleIDs: []string{}}, nil
	}

	matched := make([]string, 0, 1)
	seen := make(map[string]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		if _, exists := seen[roleID]; exists {
			continue
		}
		seen[roleID] = struct{}{}
		allowed, err = permission.EnforceCached(roleID, path, method)
		if err != nil {
			return Decision{}, err
		}
		if allowed {
			matched = append(matched, roleID)
		}
	}
	if len(matched) > 0 {
		return Decision{Allow: true, Reason: "role_policy", MatchedRoleIDs: matched}, nil
	}
	return Decision{Allow: false, Reason: "no_matching_policy", MatchedRoleIDs: []string{}}, nil
}

func EvaluateDataScopes(roleIDs []string, tenantID, path, method string) (DataScopeDecision, error) {
	path, method, err := normalizeRuntimeResource(path, method)
	if err != nil {
		return DataScopeDecision{}, err
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return DataScopeDecision{}, errors.New("authorization data scope tenant ID is required")
	}

	roleIDs = normalizeRoleIDs(roleIDs)
	for _, roleID := range roleIDs {
		if roleID == "1" || roleID == "super_admin" {
			return DataScopeDecision{Rules: []DataScopeRule{{
				RoleID: roleID, DataScope: DataScopeAll, Source: "super_admin",
			}}}, nil
		}
	}
	if len(roleIDs) == 0 {
		return tenantFallback(""), nil
	}
	if store.DB() == nil {
		return DataScopeDecision{}, errors.New("usercenter store is not initialized")
	}

	var policies []permission.ABACPolicy
	if err := store.DB().
		Where("enable = ? AND resource = ? AND action = ? AND role_id IN ? AND tenant_id IN (?, '')",
			true, path, method, roleIDs, tenantID).
		Order("priority DESC, id ASC").
		Find(&policies).Error; err != nil {
		return DataScopeDecision{}, fmt.Errorf("load authorization data scopes: %w", err)
	}

	byRole := make(map[string][]permission.ABACPolicy, len(roleIDs))
	for _, policy := range policies {
		if !validDataScope(policy.DataScope) {
			return DataScopeDecision{}, fmt.Errorf("authorization data scope %d is invalid", policy.DataScope)
		}
		byRole[policy.RoleID] = append(byRole[policy.RoleID], policy)
	}

	decision := DataScopeDecision{Rules: make([]DataScopeRule, 0, len(policies)+len(roleIDs))}
	for _, roleID := range roleIDs {
		rolePolicies := byRole[roleID]
		if len(rolePolicies) == 0 {
			decision.Rules = append(decision.Rules, DataScopeRule{
				RoleID: roleID, DataScope: DataScopeTenant, Source: "tenant_fallback",
			})
			continue
		}
		for _, policy := range rolePolicies {
			decision.Rules = append(decision.Rules, DataScopeRule{
				RoleID: roleID, DataScope: policy.DataScope, Condition: strings.TrimSpace(policy.Condition),
				Priority: policy.Priority, Source: "abac_policy",
			})
		}
	}
	return decision, nil
}

func normalizeRuntimeResource(path, method string) (string, string, error) {
	path = strings.TrimSpace(path)
	method = strings.ToUpper(strings.TrimSpace(method))
	if path == "" || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, " \t\r\n?#") {
		return "", "", errors.New("authorization resource path is invalid")
	}
	if _, ok := supportedRuntimeMethods[method]; !ok {
		return "", "", errors.New("authorization resource method is invalid")
	}
	return path, method, nil
}

func normalizeRoleIDs(roleIDs []string) []string {
	result := make([]string, 0, len(roleIDs))
	seen := make(map[string]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		if _, exists := seen[roleID]; exists {
			continue
		}
		seen[roleID] = struct{}{}
		result = append(result, roleID)
	}
	sort.Strings(result)
	return result
}

func tenantFallback(roleID string) DataScopeDecision {
	return DataScopeDecision{Rules: []DataScopeRule{{
		RoleID: roleID, DataScope: DataScopeTenant, Source: "tenant_fallback",
	}}}
}

func validDataScope(scope int32) bool {
	return scope >= DataScopeAll && scope <= DataScopeProject
}
