package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
)

func TestMenuFunctionBindingsAtomicallyRebuildPoliciesAndPruneSelections(t *testing.T) {
	const prefix = "menu-function-bindings"
	const projectID = "wenshu"

	apis := []*permission.API{
		{
			Model:       commonmodel.Model{ID: prefix + "-api-view"},
			TenantID:    platformTenant,
			ProjectID:   projectID,
			Path:        "/api/" + prefix + "/documents",
			Method:      http.MethodGet,
			Description: "view documents",
			Enable:      true,
			CheckAuth:   true,
			CheckLogin:  true,
		},
		{
			Model:       commonmodel.Model{ID: prefix + "-api-update"},
			TenantID:    platformTenant,
			ProjectID:   projectID,
			Path:        "/api/" + prefix + "/documents/:id",
			Method:      http.MethodPut,
			Description: "update document",
			Enable:      true,
			CheckAuth:   true,
			CheckLogin:  true,
		},
		{
			Model:       commonmodel.Model{ID: prefix + "-api-disabled"},
			TenantID:    platformTenant,
			ProjectID:   projectID,
			Path:        "/api/" + prefix + "/documents/export",
			Method:      http.MethodPost,
			Description: "disabled export",
			Enable:      false,
			CheckAuth:   true,
			CheckLogin:  true,
		},
		{
			Model:       commonmodel.Model{ID: prefix + "-api-foreign"},
			TenantID:    prefix + "-foreign-tenant",
			ProjectID:   projectID,
			Path:        "/api/" + prefix + "/foreign",
			Method:      http.MethodGet,
			Description: "foreign tenant API",
			Enable:      true,
			CheckAuth:   true,
			CheckLogin:  true,
		},
	}
	if err := store.DB().Create(&apis).Error; err != nil {
		t.Fatalf("create binding APIs: %v", err)
	}

	menu := &permission.Menu{
		Model:     commonmodel.Model{ID: prefix + "-menu"},
		TenantID:  platformTenant,
		ProjectID: projectID,
		Name:      prefix,
		Title:     "Document approvals",
		Path:      "/admin/" + prefix,
	}
	if err := permission.AddMenu(menu); err != nil {
		t.Fatalf("create binding menu: %v", err)
	}
	initialFunctions := []*permission.MenuFunc{
		{
			Model:  commonmodel.Model{ID: prefix + "-function-view"},
			MenuID: menu.ID,
			Name:   "view",
			Title:  "View documents",
		},
		{
			Model:  commonmodel.Model{ID: prefix + "-function-legacy"},
			MenuID: menu.ID,
			Name:   "legacy",
			Title:  "Legacy action",
		},
	}
	if err := store.DB().Create(&initialFunctions).Error; err != nil {
		t.Fatalf("create initial menu functions: %v", err)
	}
	initialLinks := []*permission.MenuFuncApi{
		{
			Model:      commonmodel.Model{ID: prefix + "-link-view"},
			MenuFuncID: initialFunctions[0].ID,
			APIID:      apis[0].ID,
		},
		{
			Model:      commonmodel.Model{ID: prefix + "-link-legacy"},
			MenuFuncID: initialFunctions[1].ID,
			APIID:      apis[0].ID,
		},
	}
	if err := store.DB().Create(&initialLinks).Error; err != nil {
		t.Fatalf("create initial function links: %v", err)
	}

	role := &permission.Role{
		Model:    commonmodel.Model{ID: prefix + "-role"},
		TenantID: platformTenant,
		Name:     "Document approver",
		CanDel:   true,
		Enable:   true,
	}
	if err := store.DB().Create(role).Error; err != nil {
		t.Fatalf("create affected role: %v", err)
	}
	roleDetail, err := permission.GetRoleAuthorization(role.ID)
	if err != nil {
		t.Fatalf("load role authorization: %v", err)
	}
	if _, err := permission.PublishRoleAuthorization(
		role.ID,
		roleDetail.Revision,
		[]permission.RoleAuthorizationSelection{{
			MenuID: menu.ID,
			Show:   true,
			Funcs:  []string{"view", "legacy"},
		}},
	); err != nil {
		t.Fatalf("publish initial role authorization: %v", err)
	}
	tenantMenu := &tenant.TenantMenu{
		Model:    commonmodel.Model{ID: prefix + "-tenant-menu"},
		TenantID: prefix + "-consumer-tenant",
		MenuID:   menu.ID,
		Funcs:    "view,legacy",
	}
	if err := store.DB().Create(tenantMenu).Error; err != nil {
		t.Fatalf("create tenant menu selection: %v", err)
	}
	const affectedUserID = prefix + "-user"
	if err := store.DB().Create(&user.UserRole{
		Model:  commonmodel.Model{ID: prefix + "-user-role"},
		UserID: affectedUserID,
		RoleID: role.ID,
	}).Error; err != nil {
		t.Fatalf("create affected user role: %v", err)
	}
	activeSession := &session.Session{
		Model:       commonmodel.Model{ID: prefix + "-session"},
		PrincipalID: affectedUserID,
		TenantID:    platformTenant,
		TokenSig:    prefix + "-token",
	}
	if err := store.DB().Create(activeSession).Error; err != nil {
		t.Fatalf("create affected session: %v", err)
	}

	engine := newMenuTestEngine(&apipb.CurrentUser{
		Id: prefix + "-admin", TenantID: platformTenant, UserName: prefix + "-admin",
	})
	detailEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		engine,
		http.MethodGet,
		"/api/core/auth/menu/functions?id="+menu.ID,
		nil,
	))
	if detailEnvelope.Code != apipb.Code_Success {
		t.Fatalf("load menu function bindings failed: %v (%s)", detailEnvelope.Code, detailEnvelope.Message)
	}
	var detail permission.MenuFunctionBindingDetail
	if err := json.Unmarshal(detailEnvelope.Data, &detail); err != nil {
		t.Fatalf("decode binding detail: %v", err)
	}
	if detail.MenuID != menu.ID ||
		detail.Summary.FunctionCount != 2 ||
		detail.Summary.APIBindingCount != 2 ||
		detail.Summary.ActiveRoleCount != 1 ||
		detail.Summary.AffectedUserCount != 1 ||
		detail.Revision == "" {
		t.Fatalf("unexpected initial binding detail: %#v", detail)
	}

	updateEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		engine,
		http.MethodPut,
		"/api/core/auth/menu/functions",
		map[string]any{
			"menuID":       menu.ID,
			"baseRevision": detail.Revision,
			"functions": []map[string]any{
				{
					"id":     initialFunctions[0].ID,
					"name":   "view",
					"title":  "View and update documents",
					"hidden": false,
					"apiIDs": []string{apis[1].ID},
				},
				{
					"name":   "export",
					"title":  "Export documents",
					"hidden": true,
					"apiIDs": []string{apis[2].ID},
				},
			},
		},
	))
	if updateEnvelope.Code != apipb.Code_Success {
		t.Fatalf("update menu function bindings failed: %v (%s)", updateEnvelope.Code, updateEnvelope.Message)
	}
	var updateResult struct {
		Revision        string                                `json:"revision"`
		Summary         permission.MenuFunctionBindingSummary `json:"summary"`
		SessionsRevoked int64                                 `json:"sessionsRevoked"`
	}
	if err := json.Unmarshal(updateEnvelope.Data, &updateResult); err != nil {
		t.Fatalf("decode binding update result: %v", err)
	}
	if updateResult.Revision == "" ||
		updateResult.Revision == detail.Revision ||
		updateResult.Summary.FunctionCount != 2 ||
		updateResult.Summary.APIBindingCount != 2 ||
		updateResult.Summary.DisabledAPIBindingCount != 1 ||
		updateResult.Summary.GeneratedPolicyCount != 1 ||
		updateResult.SessionsRevoked != 1 {
		t.Fatalf("unexpected binding update result: %#v", updateResult)
	}

	var storedFunctions []*permission.MenuFunc
	if err := store.DB().
		Preload("MenuFuncApis.API").
		Where("menu_id = ?", menu.ID).
		Order("name").
		Find(&storedFunctions).Error; err != nil {
		t.Fatalf("load stored menu functions: %v", err)
	}
	if len(storedFunctions) != 2 ||
		storedFunctions[0].Name != "export" ||
		storedFunctions[1].Name != "view" ||
		len(storedFunctions[1].MenuFuncApis) != 1 ||
		storedFunctions[1].MenuFuncApis[0].APIID != apis[1].ID {
		t.Fatalf("unexpected stored menu function mapping: %#v", storedFunctions)
	}
	var storedRoleMenu permission.RoleMenu
	if err := store.DB().
		Where("role_id = ? AND menu_id = ?", role.ID, menu.ID).
		First(&storedRoleMenu).Error; err != nil {
		t.Fatalf("load pruned role menu: %v", err)
	}
	if storedRoleMenu.Funcs != "view" {
		t.Fatalf("removed function remained in role selection: %#v", storedRoleMenu)
	}
	if err := store.DB().Where("id = ?", tenantMenu.ID).First(tenantMenu).Error; err != nil {
		t.Fatalf("reload tenant menu selection: %v", err)
	}
	if tenantMenu.Funcs != "view" {
		t.Fatalf("removed function remained in tenant selection: %#v", tenantMenu)
	}
	var policies []*permission.CasbinRule
	if err := store.DB().
		Where("ptype = ? AND v0 = ?", "p", role.ID).
		Find(&policies).Error; err != nil {
		t.Fatalf("load rebuilt role policies: %v", err)
	}
	if len(policies) != 1 ||
		policies[0].Path != apis[1].Path ||
		policies[0].Method != apis[1].Method {
		t.Fatalf("unexpected rebuilt role policies: %#v", policies)
	}
	if err := store.DB().Where("id = ?", activeSession.ID).First(activeSession).Error; err != nil {
		t.Fatalf("reload affected session: %v", err)
	}
	if !activeSession.Revoked || activeSession.RevokedReason != "menu function bindings updated" {
		t.Fatalf("affected session was not revoked: %#v", activeSession)
	}

	updatedDetailEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		engine,
		http.MethodGet,
		"/api/core/auth/menu/functions?id="+menu.ID,
		nil,
	))
	var updatedDetail permission.MenuFunctionBindingDetail
	if err := json.Unmarshal(updatedDetailEnvelope.Data, &updatedDetail); err != nil {
		t.Fatalf("decode updated binding detail: %v", err)
	}
	if updatedDetail.Revision != updateResult.Revision ||
		len(updatedDetail.Roles) != 1 ||
		len(updatedDetail.Roles[0].SelectedFunctions) != 1 ||
		updatedDetail.Roles[0].SelectedFunctions[0] != "view" {
		t.Fatalf("updated binding projection is inconsistent: %#v", updatedDetail)
	}

	conflictEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		engine,
		http.MethodPut,
		"/api/core/auth/menu/functions",
		map[string]any{
			"menuID":       menu.ID,
			"baseRevision": detail.Revision,
			"functions":    []map[string]any{},
		},
	))
	if conflictEnvelope.Code != apipb.Code_BadRequest {
		t.Fatalf("stale binding revision should be rejected, got %v", conflictEnvelope.Code)
	}
	var conflict struct {
		Conflict bool `json:"conflict"`
	}
	if err := json.Unmarshal(conflictEnvelope.Data, &conflict); err != nil || !conflict.Conflict {
		t.Fatalf("binding conflict flag missing: %#v err=%v", conflict, err)
	}

	crossTenantAPI := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		engine,
		http.MethodPut,
		"/api/core/auth/menu/functions",
		map[string]any{
			"menuID":       menu.ID,
			"baseRevision": updatedDetail.Revision,
			"functions": []map[string]any{{
				"id":     initialFunctions[0].ID,
				"name":   "view",
				"title":  "View documents",
				"apiIDs": []string{apis[3].ID},
			}},
		},
	))
	if crossTenantAPI.Code != apipb.Code_BadRequest {
		t.Fatalf("cross-tenant API binding should be rejected, got %v", crossTenantAPI.Code)
	}

	foreignEngine := newMenuTestEngine(&apipb.CurrentUser{
		Id: prefix + "-foreign-admin", TenantID: prefix + "-foreign-tenant",
	})
	foreignDetail := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		foreignEngine,
		http.MethodGet,
		"/api/core/auth/menu/functions?id="+menu.ID,
		nil,
	))
	if foreignDetail.Code != apipb.Code_NoPermission {
		t.Fatalf("cross-tenant binding detail should be denied, got %v", foreignDetail.Code)
	}

	systemMenu := &permission.Menu{
		Model:     commonmodel.Model{ID: prefix + "-system-menu"},
		TenantID:  platformTenant,
		ProjectID: projectID,
		Name:      prefix + "-system",
		Title:     "System menu",
		Path:      "/admin/" + prefix + "/system",
		IsMust:    true,
	}
	if err := permission.AddMenu(systemMenu); err != nil {
		t.Fatalf("create protected system menu: %v", err)
	}
	systemDetail, err := permission.GetMenuFunctionBindings(systemMenu.ID)
	if err != nil {
		t.Fatalf("load protected system menu bindings: %v", err)
	}
	protectedUpdate := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		engine,
		http.MethodPut,
		"/api/core/auth/menu/functions",
		map[string]any{
			"menuID":       systemMenu.ID,
			"baseRevision": systemDetail.Revision,
			"functions":    []map[string]any{},
		},
	))
	if protectedUpdate.Code != apipb.Code_BadRequest {
		t.Fatalf("protected menu function update should be rejected, got %v", protectedUpdate.Code)
	}

	var auditCount int64
	if err := store.DB().Model(&audit.AuditLog{}).
		Where("action = ? AND target_id = ?", "update_menu_function_bindings", menu.ID).
		Count(&auditCount).Error; err != nil {
		t.Fatalf("count menu function binding audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("menu function binding audit count=%d, want 1", auditCount)
	}
}
