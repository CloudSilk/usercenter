package http_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/tenant"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func newTenantAuthorizationTestEngine(currentUser *apipb.CurrentUser) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", currentUser)
		c.Next()
	})
	userhttp.RegisterTenantRouter(r)
	return r
}

func ensureTenantAuthorizationTenant(t *testing.T, id, name string) *tenant.Tenant {
	t.Helper()
	target := &tenant.Tenant{}
	err := store.DB().Where("id = ?", id).First(target).Error
	if err == nil {
		return target
	}
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("load tenant %q: %v", id, err)
	}
	target = &tenant.Tenant{
		Model:     commonmodel.Model{ID: id},
		Name:      name,
		Enable:    true,
		Expired:   time.Now().Add(24 * time.Hour),
		UserCount: 100,
	}
	if err := store.DB().Create(target).Error; err != nil {
		t.Fatalf("create tenant %q: %v", id, err)
	}
	return target
}

func TestTenantMenuAuthorizationPublishesBoundaryAndNarrowsRoles(t *testing.T) {
	const prefix = "tenant-menu-authorization"
	fixture := createRoleAuthorizationFixture(t, prefix)
	ensureTenantAuthorizationTenant(t, platformTenant, "Platform tenant")
	target := ensureTenantAuthorizationTenant(t, prefix+"-target", "Tenant menu target")
	foreign := ensureTenantAuthorizationTenant(t, prefix+"-foreign", "Tenant menu foreign")

	foreignMenu := &permission.Menu{
		Model:     commonmodel.Model{ID: prefix + "-foreign-menu"},
		TenantID:  foreign.ID,
		ProjectID: "wenshu",
		Name:      prefix + "-foreign",
		Title:     "Foreign tenant menu",
		Path:      "/admin/" + prefix + "/foreign",
	}
	if err := permission.AddMenu(foreignMenu); err != nil {
		t.Fatalf("create foreign menu: %v", err)
	}

	initialDetail, err := permission.GetTenantMenuAuthorization(platformTenant, target.ID)
	if err != nil {
		t.Fatalf("load initial tenant authorization: %v", err)
	}
	initialSelections := []permission.TenantMenuAuthorizationSelection{
		{
			MenuID: fixture.ParentMenuID,
			Funcs: []string{
				fixture.ParentViewFunc,
				fixture.ParentManageFunc,
			},
		},
		{
			MenuID: fixture.ChildMenuID,
			Funcs:  []string{fixture.ChildEditFunc},
		},
	}
	initialPublish, err := permission.PublishTenantMenuAuthorization(
		platformTenant,
		target.ID,
		initialDetail.Revision,
		initialSelections,
	)
	if err != nil {
		t.Fatalf("publish initial tenant boundary: %v", err)
	}
	if initialPublish.Summary.SelectedMenuCount != 2 ||
		initialPublish.Summary.SelectedFunctionCount != 3 {
		t.Fatalf("unexpected initial tenant publish: %#v", initialPublish)
	}

	role := &permission.Role{
		Model:    commonmodel.Model{ID: prefix + "-role"},
		TenantID: target.ID,
		Name:     "Tenant document manager",
		CanDel:   true,
		Enable:   true,
	}
	if err := store.DB().Create(role).Error; err != nil {
		t.Fatalf("create target role: %v", err)
	}
	roleDetail, err := permission.GetRoleAuthorization(role.ID)
	if err != nil {
		t.Fatalf("load role authorization: %v", err)
	}
	if _, err := permission.PublishRoleAuthorization(
		role.ID,
		roleDetail.Revision,
		[]permission.RoleAuthorizationSelection{
			{
				MenuID: fixture.ParentMenuID,
				Show:   true,
				Funcs: []string{
					fixture.ParentViewFunc,
					fixture.ParentManageFunc,
				},
			},
			{
				MenuID: fixture.ChildMenuID,
				Show:   true,
				Funcs:  []string{fixture.ChildEditFunc},
			},
		},
	); err != nil {
		t.Fatalf("publish target role authorization: %v", err)
	}

	const affectedUserID = prefix + "-user"
	if err := store.DB().Create(&user.UserRole{
		Model:  commonmodel.Model{ID: prefix + "-user-role"},
		UserID: affectedUserID,
		RoleID: role.ID,
	}).Error; err != nil {
		t.Fatalf("assign target role: %v", err)
	}
	activeSession := &session.Session{
		Model:       commonmodel.Model{ID: prefix + "-session"},
		PrincipalID: affectedUserID,
		TenantID:    target.ID,
		TokenSig:    prefix + "-token-signature",
	}
	if err := store.DB().Create(activeSession).Error; err != nil {
		t.Fatalf("create active session: %v", err)
	}
	staleToken, err := token.EncodeToken(&apipb.CurrentUser{
		Id: affectedUserID, TenantID: target.ID, UserName: affectedUserID,
	})
	if err != nil {
		t.Fatalf("encode stale token: %v", err)
	}

	platformEngine := newTenantAuthorizationTestEngine(&apipb.CurrentUser{
		Id: prefix + "-platform-admin", TenantID: platformTenant, UserName: prefix + "-platform-admin",
	})
	targetsEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		platformEngine,
		http.MethodGet,
		"/api/core/auth/tenant/menu-authorization/targets",
		nil,
	))
	if targetsEnvelope.Code != apipb.Code_Success {
		t.Fatalf("load tenant authorization targets: %v (%s)", targetsEnvelope.Code, targetsEnvelope.Message)
	}
	var targets permission.TenantMenuAuthorizationTargetList
	if err := json.Unmarshal(targetsEnvelope.Data, &targets); err != nil {
		t.Fatalf("decode tenant targets: %v", err)
	}
	if !targets.CanManageCrossTenant {
		t.Fatalf("platform actor must expose cross-tenant management scope: %#v", targets)
	}
	foundTarget := false
	for _, item := range targets.Targets {
		if item.ID == target.ID {
			foundTarget = true
			if item.AuthorizedMenuCount != 2 || item.AuthorizedFunctionCount != 3 {
				t.Fatalf("unexpected target summary: %#v", item)
			}
		}
	}
	if !foundTarget {
		t.Fatalf("target tenant missing from platform scope: %#v", targets.Targets)
	}

	detailEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		platformEngine,
		http.MethodGet,
		"/api/core/auth/tenant/menu-authorization?tenantID="+target.ID,
		nil,
	))
	if detailEnvelope.Code != apipb.Code_Success {
		t.Fatalf("load tenant menu authorization: %v (%s)", detailEnvelope.Code, detailEnvelope.Message)
	}
	var detail permission.TenantMenuAuthorizationDetail
	if err := json.Unmarshal(detailEnvelope.Data, &detail); err != nil {
		t.Fatalf("decode tenant detail: %v", err)
	}
	if detail.Revision == "" ||
		detail.Summary.SelectedMenuCount != 2 ||
		detail.Summary.SelectedFunctionCount != 3 ||
		detail.Summary.RoleCount != 1 {
		t.Fatalf("unexpected tenant authorization detail: %#v", detail)
	}

	proposedSelections := []map[string]any{
		{
			"menuID": fixture.ParentMenuID,
			"funcs":  []string{fixture.ParentViewFunc},
		},
		{
			"menuID": fixture.ChildMenuID,
			"funcs":  []string{fixture.ChildEditFunc},
		},
	}
	previewEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		platformEngine,
		http.MethodPost,
		"/api/core/auth/tenant/menu-authorization/preview",
		map[string]any{
			"tenantID":   target.ID,
			"selections": proposedSelections,
		},
	))
	if previewEnvelope.Code != apipb.Code_Success {
		t.Fatalf("preview tenant authorization: %v (%s)", previewEnvelope.Code, previewEnvelope.Message)
	}
	var preview permission.TenantMenuAuthorizationPreview
	if err := json.Unmarshal(previewEnvelope.Data, &preview); err != nil {
		t.Fatalf("decode tenant preview: %v", err)
	}
	if preview.CurrentRevision != detail.Revision ||
		preview.Summary.AffectedRoleCount != 1 ||
		preview.Summary.PrunedFunctionCount != 1 ||
		preview.Summary.AffectedUserCount != 1 ||
		len(preview.Roles) != 1 ||
		!preview.Roles[0].Changed {
		t.Fatalf("unexpected tenant preview: %#v", preview)
	}

	publishEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		platformEngine,
		http.MethodPut,
		"/api/core/auth/tenant/menu-authorization",
		map[string]any{
			"tenantID":     target.ID,
			"baseRevision": detail.Revision,
			"selections":   proposedSelections,
		},
	))
	if publishEnvelope.Code != apipb.Code_Success {
		t.Fatalf("publish tenant authorization: %v (%s)", publishEnvelope.Code, publishEnvelope.Message)
	}
	var publishResult struct {
		Revision        string `json:"revision"`
		SessionsRevoked int64  `json:"sessionsRevoked"`
	}
	if err := json.Unmarshal(publishEnvelope.Data, &publishResult); err != nil {
		t.Fatalf("decode tenant publish result: %v", err)
	}
	if publishResult.Revision != preview.ProposedRevision || publishResult.SessionsRevoked != 1 {
		t.Fatalf("unexpected tenant publish result: %#v", publishResult)
	}

	var storedTenantMenus []*tenant.TenantMenu
	if err := store.DB().
		Where("tenant_id = ?", target.ID).
		Order("menu_id").
		Find(&storedTenantMenus).Error; err != nil {
		t.Fatalf("load tenant menu grants: %v", err)
	}
	if len(storedTenantMenus) != 2 {
		t.Fatalf("tenant grant count=%d, want 2", len(storedTenantMenus))
	}
	for _, item := range storedTenantMenus {
		if item.MenuID == fixture.ParentMenuID && item.Funcs != fixture.ParentViewFunc {
			t.Fatalf("parent grant was not narrowed: %#v", item)
		}
	}
	var storedRoleMenus []*permission.RoleMenu
	if err := store.DB().
		Where("role_id = ?", role.ID).
		Order("menu_id").
		Find(&storedRoleMenus).Error; err != nil {
		t.Fatalf("load narrowed role menus: %v", err)
	}
	if len(storedRoleMenus) != 2 {
		t.Fatalf("role menu count=%d, want 2", len(storedRoleMenus))
	}
	for _, item := range storedRoleMenus {
		if item.MenuID == fixture.ParentMenuID && item.Funcs != fixture.ParentViewFunc {
			t.Fatalf("role function remained outside tenant boundary: %#v", item)
		}
	}
	var policyCount int64
	if err := store.DB().Model(&permission.CasbinRule{}).
		Where("ptype = ? AND v0 = ?", "p", role.ID).
		Count(&policyCount).Error; err != nil {
		t.Fatalf("count rebuilt role policies: %v", err)
	}
	if policyCount != 2 {
		t.Fatalf("rebuilt policy count=%d, want 2", policyCount)
	}
	if err := store.DB().Where("id = ?", activeSession.ID).First(activeSession).Error; err != nil {
		t.Fatalf("reload revoked session: %v", err)
	}
	if !activeSession.Revoked || activeSession.RevokedReason != "tenant menu authorization published" {
		t.Fatalf("tenant publish did not revoke session: %#v", activeSession)
	}
	if exists, err := token.DefaultTokenCache.Exists(affectedUserID, staleToken); err != nil || exists {
		t.Fatalf("stale token must be cleared: exists=%v err=%v", exists, err)
	}

	conflictEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		platformEngine,
		http.MethodPut,
		"/api/core/auth/tenant/menu-authorization",
		map[string]any{
			"tenantID":     target.ID,
			"baseRevision": detail.Revision,
			"selections":   []map[string]any{},
		},
	))
	if conflictEnvelope.Code != apipb.Code_BadRequest {
		t.Fatalf("stale tenant revision should be rejected, got %v", conflictEnvelope.Code)
	}
	var conflict struct {
		Conflict bool `json:"conflict"`
	}
	if err := json.Unmarshal(conflictEnvelope.Data, &conflict); err != nil || !conflict.Conflict {
		t.Fatalf("tenant conflict flag missing: %#v err=%v", conflict, err)
	}

	missingAncestor := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		platformEngine,
		http.MethodPost,
		"/api/core/auth/tenant/menu-authorization/preview",
		map[string]any{
			"tenantID": target.ID,
			"selections": []map[string]any{{
				"menuID": fixture.ChildMenuID,
				"funcs":  []string{fixture.ChildEditFunc},
			}},
		},
	))
	if missingAncestor.Code != apipb.Code_BadRequest {
		t.Fatalf("child grant without ancestor should be rejected, got %v", missingAncestor.Code)
	}
	foreignMenuEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		platformEngine,
		http.MethodPost,
		"/api/core/auth/tenant/menu-authorization/preview",
		map[string]any{
			"tenantID": target.ID,
			"selections": []map[string]any{{
				"menuID": foreignMenu.ID,
				"funcs":  []string{},
			}},
		},
	))
	if foreignMenuEnvelope.Code != apipb.Code_BadRequest {
		t.Fatalf("foreign menu should be outside target catalogue, got %v", foreignMenuEnvelope.Code)
	}

	tenantEngine := newTenantAuthorizationTestEngine(&apipb.CurrentUser{
		Id: prefix + "-tenant-admin", TenantID: target.ID, UserName: prefix + "-tenant-admin",
	})
	ownTargetsEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		tenantEngine,
		http.MethodGet,
		"/api/core/auth/tenant/menu-authorization/targets",
		nil,
	))
	var ownTargets permission.TenantMenuAuthorizationTargetList
	if err := json.Unmarshal(ownTargetsEnvelope.Data, &ownTargets); err != nil {
		t.Fatalf("decode own tenant targets: %v", err)
	}
	if ownTargets.CanManageCrossTenant ||
		len(ownTargets.Targets) != 1 ||
		ownTargets.Targets[0].ID != target.ID {
		t.Fatalf("tenant actor scope leaked another tenant: %#v", ownTargets)
	}
	crossTenantEnvelope := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		tenantEngine,
		http.MethodGet,
		"/api/core/auth/tenant/menu-authorization?tenantID="+foreign.ID,
		nil,
	))
	if crossTenantEnvelope.Code != apipb.Code_NoPermission {
		t.Fatalf("cross-tenant detail should be denied, got %v", crossTenantEnvelope.Code)
	}

	platformDetail, err := permission.GetTenantMenuAuthorization(platformTenant, platformTenant)
	if err != nil {
		t.Fatalf("load platform baseline: %v", err)
	}
	lockedBaseline := decodeRoleAuthorizationEnvelope(t, doJSONRequest(
		t,
		platformEngine,
		http.MethodPut,
		"/api/core/auth/tenant/menu-authorization",
		map[string]any{
			"tenantID":     platformTenant,
			"baseRevision": platformDetail.Revision,
			"selections":   []map[string]any{},
		},
	))
	if lockedBaseline.Code != apipb.Code_BadRequest {
		t.Fatalf("platform baseline must be read-only, got %v", lockedBaseline.Code)
	}

	var auditCount int64
	if err := store.DB().Model(&audit.AuditLog{}).
		Where("action = ? AND target_id = ?", audit.AuditActionPublishTenantMenuAuth, target.ID).
		Count(&auditCount).Error; err != nil {
		t.Fatalf("count tenant authorization audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("tenant authorization audit count=%d, want 1", auditCount)
	}
}
