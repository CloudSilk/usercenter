package permission

import (
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

var (
	ErrInvalidAPIResource   = errors.New("invalid API resource")
	ErrProtectedAPIResource = errors.New("system API resource is protected")
	ErrBoundAPIResource     = errors.New("API resource is still bound to menu functions")
)

var supportedHTTPMethods = map[string]struct{}{
	"GET": {}, "POST": {}, "PUT": {}, "PATCH": {}, "DELETE": {}, "HEAD": {}, "OPTIONS": {},
}

type APIResourceBinding struct {
	LinkID        string `json:"linkID"`
	MenuID        string `json:"menuID"`
	MenuTitle     string `json:"menuTitle"`
	FunctionID    string `json:"functionID"`
	FunctionName  string `json:"functionName"`
	FunctionTitle string `json:"functionTitle"`
}

type APIResourceImpact struct {
	APIID                    string               `json:"apiID"`
	MenuFunctionBindingCount int                  `json:"menuFunctionBindingCount"`
	ActiveRoleCount          int                  `json:"activeRoleCount"`
	ActivePolicyCount        int64                `json:"activePolicyCount"`
	AffectedUserCount        int64                `json:"affectedUserCount"`
	CanDelete                bool                 `json:"canDelete"`
	DeleteBlockReason        string               `json:"deleteBlockReason,omitempty"`
	Bindings                 []APIResourceBinding `json:"bindings"`
}

func normalizeAPIResource(api *API) error {
	if api == nil {
		return fmt.Errorf("%w: request body is required", ErrInvalidAPIResource)
	}
	api.ID = strings.TrimSpace(api.ID)
	api.TenantID = strings.TrimSpace(api.TenantID)
	api.ProjectID = strings.TrimSpace(api.ProjectID)
	api.Path = strings.TrimSpace(api.Path)
	api.Group = strings.TrimSpace(api.Group)
	api.Method = strings.ToUpper(strings.TrimSpace(api.Method))
	api.Description = strings.TrimSpace(api.Description)
	if api.Path == "" || !strings.HasPrefix(api.Path, "/") || strings.ContainsAny(api.Path, " \t\r\n?#") {
		return fmt.Errorf("%w: path must start with / and cannot contain spaces, query strings or fragments", ErrInvalidAPIResource)
	}
	if len(api.Path) > 200 {
		return fmt.Errorf("%w: path cannot exceed 200 characters", ErrInvalidAPIResource)
	}
	if _, supported := supportedHTTPMethods[api.Method]; !supported {
		return fmt.Errorf("%w: unsupported HTTP method %q", ErrInvalidAPIResource, api.Method)
	}
	if len(api.Group) > 50 {
		return fmt.Errorf("%w: group cannot exceed 50 characters", ErrInvalidAPIResource)
	}
	if len(api.Description) > 200 {
		return fmt.Errorf("%w: description cannot exceed 200 characters", ErrInvalidAPIResource)
	}
	if api.CheckAuth {
		api.CheckLogin = true
	}
	return nil
}

func apiResourceExists(tx *gorm.DB, api *API) (bool, error) {
	var count int64
	err := tx.Model(&API{}).
		Where("id <> ? AND path = ? AND method = ? AND tenant_id = ? AND project_id = ?",
			api.ID, api.Path, api.Method, api.TenantID, api.ProjectID).
		Count(&count).Error
	return count > 0, err
}

func uniqueStrings(values []string) []string {
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
	return result
}

func apiRoleIDsFromBindings(tx *gorm.DB, apiID string) ([]string, []APIResourceBinding, error) {
	var bindings []APIResourceBinding
	if err := tx.Table("menu_func_apis AS link").
		Select(`link.id AS link_id,
			menu.id AS menu_id,
			menu.title AS menu_title,
			function.id AS function_id,
			function.name AS function_name,
			function.title AS function_title`).
		Joins("JOIN menu_funcs AS function ON function.id = link.menu_func_id AND function.deleted_at IS NULL").
		Joins("JOIN menus AS menu ON menu.id = function.menu_id AND menu.deleted_at IS NULL").
		Where("link.api_id = ? AND link.deleted_at IS NULL", apiID).
		Order("menu.sort, menu.title, function.title").
		Scan(&bindings).Error; err != nil {
		return nil, nil, err
	}
	if len(bindings) == 0 {
		return []string{}, []APIResourceBinding{}, nil
	}
	menuIDs := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		menuIDs = append(menuIDs, binding.MenuID)
	}
	var roleMenus []*RoleMenu
	if err := tx.Where("menu_id IN ?", uniqueStrings(menuIDs)).Find(&roleMenus).Error; err != nil {
		return nil, nil, err
	}
	roleSet := make(map[string]struct{})
	for _, roleMenu := range roleMenus {
		selected := make(map[string]struct{})
		for _, name := range strings.Split(roleMenu.Funcs, ",") {
			if name = strings.TrimSpace(name); name != "" {
				selected[name] = struct{}{}
			}
		}
		for _, binding := range bindings {
			if binding.MenuID != roleMenu.MenuID {
				continue
			}
			if _, ok := selected[binding.FunctionName]; ok {
				roleSet[roleMenu.RoleID] = struct{}{}
			}
		}
	}
	roleIDs := make([]string, 0, len(roleSet))
	for roleID := range roleSet {
		roleIDs = append(roleIDs, roleID)
	}
	return roleIDs, bindings, nil
}

func rebuildRoleAuthorizationPolicies(tx *gorm.DB, roleIDs []string) error {
	for _, roleID := range uniqueStrings(roleIDs) {
		role := &Role{}
		if err := tx.Where("id = ?", roleID).First(role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		selections, err := loadCurrentAuthorizationSelections(tx, role.ID)
		if err != nil {
			return err
		}
		preview, _, err := buildRoleAuthorizationPreview(tx, role, selections, false)
		if err != nil {
			return err
		}
		if err := replaceRoleAuthorizationPolicies(tx, role, preview.Policies); err != nil {
			return err
		}
	}
	return nil
}

func revokeAPIResourceUsers(tx *gorm.DB, roleIDs []string, reason string) ([]string, error) {
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

func clearAPIResourceTokenCache(userIDs []string) error {
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

func replaceSpecialAPIPolicies(tx *gorm.DB) error {
	if err := tx.Where("ptype = ? AND v0 IN ?", "p", []string{"0", "-1"}).Delete(&CasbinRule{}).Error; err != nil {
		return err
	}
	var apis []API
	if err := tx.Where("enable = ?", true).Find(&apis).Error; err != nil {
		return err
	}
	ruleByKey := make(map[string]*CasbinRule)
	for _, api := range apis {
		roleID := ""
		switch {
		case !api.CheckLogin:
			roleID = "-1"
		case !api.CheckAuth:
			roleID = "0"
		default:
			continue
		}
		key := strings.Join([]string{roleID, api.Path, api.Method}, "\x00")
		ruleByKey[key] = &CasbinRule{
			Ptype: "p", RoleID: roleID, Path: api.Path, Method: api.Method, CheckAuth: "false",
		}
	}
	if len(ruleByKey) == 0 {
		return nil
	}
	rules := make([]*CasbinRule, 0, len(ruleByKey))
	for _, rule := range ruleByKey {
		rules = append(rules, rule)
	}
	return tx.Create(&rules).Error
}

func createAPIResource(api *API) error {
	if err := normalizeAPIResource(api); err != nil {
		return err
	}
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		duplicate, err := apiResourceExists(tx, api)
		if err != nil {
			return err
		}
		if duplicate {
			return fmt.Errorf("%w: an API with the same path and method already exists in this tenant and project", ErrInvalidAPIResource)
		}
		if err := tx.Create(api).Error; err != nil {
			return err
		}
		return replaceSpecialAPIPolicies(tx)
	})
	if err != nil {
		return err
	}
	return ReloadCasbinPolicy()
}

func CreateAPIResource(api *API) error {
	return createAPIResource(api)
}

func deleteAPIResource(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: API ID cannot be empty", ErrInvalidAPIResource)
	}
	var affectedUserIDs []string
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		api := &API{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(api).Error; err != nil {
			return err
		}
		if api.IsMust {
			return ErrProtectedAPIResource
		}
		roleIDs, bindings, err := apiRoleIDsFromBindings(tx, api.ID)
		if err != nil {
			return err
		}
		if len(bindings) > 0 {
			return fmt.Errorf("%w: remove %d menu function bindings first", ErrBoundAPIResource, len(bindings))
		}
		var policyRoleIDs []string
		if err := tx.Model(&CasbinRule{}).
			Where("ptype = ? AND v1 = ? AND v2 = ? AND v0 NOT IN ?", "p", api.Path, api.Method, []string{"0", "-1"}).
			Distinct("v0").
			Pluck("v0", &policyRoleIDs).Error; err != nil {
			return err
		}
		roleIDs = uniqueStrings(append(roleIDs, policyRoleIDs...))
		if err := tx.Delete(api).Error; err != nil {
			return err
		}
		if err := rebuildRoleAuthorizationPolicies(tx, roleIDs); err != nil {
			return err
		}
		if err := replaceSpecialAPIPolicies(tx); err != nil {
			return err
		}
		affectedUserIDs, err = revokeAPIResourceUsers(tx, roleIDs, "API resource deleted")
		return err
	})
	if err != nil {
		return err
	}
	if err := ReloadCasbinPolicy(); err != nil {
		return err
	}
	return clearAPIResourceTokenCache(affectedUserIDs)
}

func DeleteAPIResource(id string) error {
	return deleteAPIResource(id)
}

func updateAPIResource(api *API) error {
	if err := normalizeAPIResource(api); err != nil {
		return err
	}
	if api.ID == "" {
		return fmt.Errorf("%w: API ID cannot be empty", ErrInvalidAPIResource)
	}
	var affectedUserIDs []string
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		current := &API{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", api.ID).First(current).Error; err != nil {
			return err
		}
		if current.IsMust &&
			(current.Path != api.Path ||
				current.Method != api.Method ||
				current.Enable != api.Enable ||
				current.CheckAuth != api.CheckAuth ||
				current.CheckLogin != api.CheckLogin) {
			return ErrProtectedAPIResource
		}
		authorizationChanged := current.Path != api.Path ||
			current.Method != api.Method ||
			current.Enable != api.Enable ||
			current.CheckAuth != api.CheckAuth ||
			current.CheckLogin != api.CheckLogin
		api.TenantID = current.TenantID
		api.ProjectID = current.ProjectID
		api.IsMust = current.IsMust
		duplicate, err := apiResourceExists(tx, api)
		if err != nil {
			return err
		}
		if duplicate {
			return fmt.Errorf("%w: an API with the same path and method already exists in this tenant and project", ErrInvalidAPIResource)
		}
		roleIDs, _, err := apiRoleIDsFromBindings(tx, api.ID)
		if err != nil {
			return err
		}
		if err := tx.Model(&API{}).Where("id = ?", api.ID).Select(
			"path", "group", "method", "description", "enable", "check_auth", "check_login",
		).Updates(api).Error; err != nil {
			return err
		}
		if authorizationChanged {
			if err := rebuildRoleAuthorizationPolicies(tx, roleIDs); err != nil {
				return err
			}
		}
		if err := replaceSpecialAPIPolicies(tx); err != nil {
			return err
		}
		if authorizationChanged {
			affectedUserIDs, err = revokeAPIResourceUsers(tx, roleIDs, "API resource updated")
		}
		return err
	})
	if err != nil {
		return err
	}
	if err := ReloadCasbinPolicy(); err != nil {
		return err
	}
	return clearAPIResourceTokenCache(affectedUserIDs)
}

func UpdateAPIResource(api *API) error {
	return updateAPIResource(api)
}

func enableAPIResource(id string, enable bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: API ID cannot be empty", ErrInvalidAPIResource)
	}
	var affectedUserIDs []string
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		api := &API{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(api).Error; err != nil {
			return err
		}
		if api.Enable == enable {
			return nil
		}
		if api.IsMust && !enable {
			return ErrProtectedAPIResource
		}
		roleIDs, _, err := apiRoleIDsFromBindings(tx, api.ID)
		if err != nil {
			return err
		}
		if err := tx.Model(&API{}).Where("id = ?", api.ID).Update("enable", enable).Error; err != nil {
			return err
		}
		if err := rebuildRoleAuthorizationPolicies(tx, roleIDs); err != nil {
			return err
		}
		if err := replaceSpecialAPIPolicies(tx); err != nil {
			return err
		}
		affectedUserIDs, err = revokeAPIResourceUsers(tx, roleIDs, "API resource state updated")
		return err
	})
	if err != nil {
		return err
	}
	if err := ReloadCasbinPolicy(); err != nil {
		return err
	}
	return clearAPIResourceTokenCache(affectedUserIDs)
}

func EnableAPIResource(id string, enable bool) error {
	return enableAPIResource(id, enable)
}

func QueryAPIResources(req *apipb.QueryAPIRequest, resp *apipb.QueryAPIResponse, keyword string, enable *bool) {
	db := store.DB().Model(&API{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("path LIKE ? OR description LIKE ? OR `group` LIKE ?", like, like, like)
	}
	if req.Path != "" {
		db = db.Where("path LIKE ?", "%"+req.Path+"%")
	}
	if req.Method != "" {
		db = db.Where("method = ?", strings.ToUpper(strings.TrimSpace(req.Method)))
	}
	if req.Group != "" {
		db = db.Where("`group` = ?", req.Group)
	}
	if req.CheckAuth > 0 {
		db = db.Where("check_auth = ?", req.CheckAuth == 1)
	}
	if req.CheckLogin > 0 {
		db = db.Where("check_login = ?", req.CheckLogin == 1)
	}
	if enable != nil {
		db = db.Where("enable = ?", *enable)
	}
	if len(req.Ids) > 0 {
		db = db.Where("id IN ?", req.Ids)
	}
	if req.TenantID != "" {
		db = db.Where("tenant_id = ?", req.TenantID)
	}
	if req.ProjectID != "" {
		db = db.Where("project_id = ?", req.ProjectID)
	}
	if req.IsMust {
		db = db.Where("is_must = ?", true)
	}
	orderStr, err := utils.GenerateOrderString(req.SortConfig, "`path`")
	if err != nil {
		resp.Code = apipb.Code_BadRequest
		resp.Message = err.Error()
		return
	}
	var apis []API
	resp.Records, resp.Pages, err = store.Client().PageQuery(db, req.PageSize, req.PageIndex, orderStr, &apis, nil)
	if err != nil {
		resp.Code = commonmodel.InternalServerError
		resp.Message = err.Error()
		return
	}
	resp.Data = APIsToPB(apis)
	resp.Total = resp.Records
}

func GetAPIResourceImpact(id string) (*APIResourceImpact, error) {
	api, err := GetAPIById(strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	roleIDs, bindings, err := apiRoleIDsFromBindings(store.DB(), api.ID)
	if err != nil {
		return nil, err
	}
	impact := &APIResourceImpact{
		APIID:                    api.ID,
		MenuFunctionBindingCount: len(bindings),
		ActiveRoleCount:          len(roleIDs),
		CanDelete:                !api.IsMust && len(bindings) == 0,
		Bindings:                 bindings,
	}
	if api.IsMust {
		impact.DeleteBlockReason = "system API resources cannot be deleted"
	} else if len(bindings) > 0 {
		impact.DeleteBlockReason = fmt.Sprintf("remove %d menu function bindings before deletion", len(bindings))
	}
	if len(roleIDs) > 0 {
		if err := store.DB().Model(&CasbinRule{}).
			Where("ptype = ? AND v0 IN ? AND v1 = ? AND v2 = ?", "p", roleIDs, api.Path, api.Method).
			Count(&impact.ActivePolicyCount).Error; err != nil {
			return nil, err
		}
		if err := store.DB().Table("user_roles").
			Where("role_id IN ?", roleIDs).
			Distinct("user_id").
			Count(&impact.AffectedUserCount).Error; err != nil {
			return nil, err
		}
	}
	return impact, nil
}
