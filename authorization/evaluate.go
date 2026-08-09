package authorization

import (
	"errors"
	"net/http"
	"strings"

	"github.com/CloudSilk/usercenter/internal/permission"
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

func EvaluateRoles(roleIDs []string, path, method string) (Decision, error) {
	path = strings.TrimSpace(path)
	method = strings.ToUpper(strings.TrimSpace(method))
	if path == "" || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, " \t\r\n?#") {
		return Decision{}, errors.New("authorization resource path is invalid")
	}
	if _, ok := supportedRuntimeMethods[method]; !ok {
		return Decision{}, errors.New("authorization resource method is invalid")
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
