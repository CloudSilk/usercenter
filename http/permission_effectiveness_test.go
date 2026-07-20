package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	commonmodel "github.com/CloudSilk/pkg/model"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/accesseffect"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/session"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

type permissionEffectivenessEnvelope[T any] struct {
	Code    apipb.Code `json:"code"`
	Message string     `json:"message"`
	Data    T          `json:"data"`
}

func newPermissionEffectivenessTestEngine(currentUser *apipb.CurrentUser) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", currentUser)
		c.Set("Principal", principal.NewHuman(currentUser.Id, currentUser.TenantID, currentUser.RoleIDs))
		c.Next()
	})
	userhttp.RegisterPermissionEffectivenessRouter(r)
	return r
}

func decodePermissionEffectivenessEnvelope[T any](
	t *testing.T,
	body []byte,
) permissionEffectivenessEnvelope[T] {
	t.Helper()
	envelope := permissionEffectivenessEnvelope[T]{}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode permission effectiveness response: %v (body=%s)", err, string(body))
	}
	return envelope
}

func createPermissionEffectivenessFixture(t *testing.T, prefix, tenantID string) (string, string, string, string) {
	t.Helper()
	userID := mustCreateUser(t, prefix+"-user", tenantID, "Abc12345")
	allowedAPI := &permission.API{
		Model:       commonmodel.Model{ID: prefix + "-api-allowed"},
		TenantID:    tenantID,
		Path:        "/" + prefix + "/documents",
		Method:      http.MethodGet,
		Description: "allowed documents",
		Enable:      true,
		CheckAuth:   true,
		CheckLogin:  true,
	}
	deniedAPI := &permission.API{
		Model:       commonmodel.Model{ID: prefix + "-api-denied"},
		TenantID:    tenantID,
		Path:        "/" + prefix + "/documents/export",
		Method:      http.MethodPost,
		Description: "unbound export",
		Enable:      true,
		CheckAuth:   true,
		CheckLogin:  true,
	}
	if err := store.DB().Create([]*permission.API{allowedAPI, deniedAPI}).Error; err != nil {
		t.Fatalf("create effectiveness APIs: %v", err)
	}
	menu := &permission.Menu{
		Model:    commonmodel.Model{ID: prefix + "-menu"},
		TenantID: tenantID,
		Name:     prefix + "-menu",
		Title:    "权限核验文档",
		Path:     "/" + prefix,
	}
	if err := store.DB().Create(menu).Error; err != nil {
		t.Fatalf("create effectiveness menu: %v", err)
	}
	function := &permission.MenuFunc{
		Model:  commonmodel.Model{ID: prefix + "-function"},
		MenuID: menu.ID,
		Name:   prefix + "-view",
		Title:  "查看文档",
	}
	if err := store.DB().Create(function).Error; err != nil {
		t.Fatalf("create effectiveness function: %v", err)
	}
	if err := store.DB().Create(&permission.MenuFuncApi{
		Model:      commonmodel.Model{ID: prefix + "-function-api"},
		MenuFuncID: function.ID,
		APIID:      allowedAPI.ID,
	}).Error; err != nil {
		t.Fatalf("create effectiveness function API link: %v", err)
	}

	enabledRole := &permission.Role{
		Model:    commonmodel.Model{ID: prefix + "-role-enabled"},
		TenantID: tenantID,
		Name:     "已启用核验角色",
		CanDel:   true,
		Enable:   true,
	}
	disabledRole := &permission.Role{
		Model:    commonmodel.Model{ID: prefix + "-role-disabled"},
		TenantID: tenantID,
		Name:     "已停用核验角色",
		CanDel:   true,
		Enable:   false,
	}
	if err := store.DB().Create([]*permission.Role{enabledRole, disabledRole}).Error; err != nil {
		t.Fatalf("create effectiveness roles: %v", err)
	}
	if err := store.DB().Model(&permission.Role{}).
		Where("id = ?", disabledRole.ID).
		Update("enable", false).Error; err != nil {
		t.Fatalf("disable effectiveness role: %v", err)
	}
	detail, err := permission.GetRoleAuthorization(enabledRole.ID)
	if err != nil {
		t.Fatalf("load enabled role authorization: %v", err)
	}
	if _, err := permission.PublishRoleAuthorization(
		enabledRole.ID,
		detail.Revision,
		[]permission.RoleAuthorizationSelection{{
			MenuID: menu.ID,
			Show:   true,
			Funcs:  []string{function.Name},
		}},
	); err != nil {
		t.Fatalf("publish enabled role authorization: %v", err)
	}
	if err := store.DB().Create([]*user.UserRole{
		{
			Model:  commonmodel.Model{ID: prefix + "-user-role-enabled"},
			UserID: userID,
			RoleID: enabledRole.ID,
		},
		{
			Model:  commonmodel.Model{ID: prefix + "-user-role-disabled"},
			UserID: userID,
			RoleID: disabledRole.ID,
		},
	}).Error; err != nil {
		t.Fatalf("assign effectiveness roles: %v", err)
	}
	if err := store.DB().Create(&session.Session{
		Model:       commonmodel.Model{ID: prefix + "-session"},
		PrincipalID: userID,
		TenantID:    tenantID,
		TokenSig:    prefix + "-token",
	}).Error; err != nil {
		t.Fatalf("create effectiveness session: %v", err)
	}
	return userID, enabledRole.ID, allowedAPI.ID, deniedAPI.ID
}

func TestPermissionEffectivenessProjectionCheckAndAudit(t *testing.T) {
	const prefix = "permission-effectiveness"
	userID, enabledRoleID, allowedAPIID, deniedAPIID := createPermissionEffectivenessFixture(t, prefix, platformTenant)
	engine := newPermissionEffectivenessTestEngine(&apipb.CurrentUser{
		Id:       prefix + "-admin",
		UserName: prefix + "-admin",
		TenantID: platformTenant,
		RoleIDs:  []string{"1"},
	})
	if allowed, err := permission.EnforceCached("0", accesseffect.SelfPath, http.MethodGet); err != nil || !allowed {
		t.Fatalf("self endpoint must be backed by native login-only Casbin policy: allow=%v err=%v", allowed, err)
	}
	if allowed, err := permission.EnforceCached("0", accesseffect.DetailPath, http.MethodGet); err != nil || allowed {
		t.Fatalf("management projection endpoint must remain role protected: allow=%v err=%v", allowed, err)
	}

	initial := decodePermissionEffectivenessEnvelope[*accesseffect.Detail](
		t,
		doJSONRequest(t, engine, http.MethodGet, accesseffect.DetailPath+"?userID="+userID, nil).Body.Bytes(),
	)
	if initial.Code != apipb.Code_Success || initial.Data == nil {
		t.Fatalf("load initial effectiveness projection: code=%v message=%s", initial.Code, initial.Message)
	}
	if initial.Data.Summary.AssignedRoleCount != 2 ||
		initial.Data.Summary.LoginRoleCount != 1 ||
		len(initial.Data.LoginRoleIDs) != 1 ||
		initial.Data.LoginRoleIDs[0] != enabledRoleID {
		t.Fatalf("disabled role leaked into login context: %#v", initial.Data)
	}
	if initial.Data.Summary.ActiveSessionCount != 1 ||
		initial.Data.Summary.EffectiveMenuCount != 1 ||
		initial.Data.Summary.EffectiveFuncCount != 1 ||
		initial.Data.Summary.EffectiveAPICount != 1 {
		t.Fatalf("unexpected effectiveness summary: %#v", initial.Data.Summary)
	}
	if len(initial.Data.APISources) != 1 ||
		initial.Data.APISources[0].APIID != allowedAPIID ||
		initial.Data.APISources[0].RoleID != enabledRoleID {
		t.Fatalf("authorization source chain is incomplete: %#v", initial.Data.APISources)
	}

	allowed := decodePermissionEffectivenessEnvelope[*accesseffect.CheckResult](
		t,
		doJSONRequest(t, engine, http.MethodPost, accesseffect.CheckPath, map[string]string{
			"userID": userID,
			"apiID":  allowedAPIID,
		}).Body.Bytes(),
	)
	if allowed.Code != apipb.Code_Success ||
		allowed.Data == nil ||
		!allowed.Data.Allow ||
		allowed.Data.Reason != "role_policy" ||
		len(allowed.Data.MatchedRoleIDs) != 1 ||
		allowed.Data.MatchedRoleIDs[0] != enabledRoleID ||
		!allowed.Data.PolicyMetadataConsistent {
		t.Fatalf("expected role policy allow evidence, got %#v", allowed)
	}

	denied := decodePermissionEffectivenessEnvelope[*accesseffect.CheckResult](
		t,
		doJSONRequest(t, engine, http.MethodPost, accesseffect.CheckPath, map[string]string{
			"userID": userID,
			"apiID":  deniedAPIID,
		}).Body.Bytes(),
	)
	if denied.Code != apipb.Code_Success ||
		denied.Data == nil ||
		denied.Data.Allow ||
		denied.Data.Reason != "no_matching_policy" ||
		len(denied.Data.MatchedRoleIDs) != 0 ||
		!denied.Data.PolicyMetadataConsistent {
		t.Fatalf("expected unbound API deny evidence, got %#v", denied)
	}
	if denied.Data.LoginRoleIDs == nil || denied.Data.MatchedRoleIDs == nil {
		t.Fatalf("role collections must use stable empty arrays instead of null: %#v", denied.Data)
	}

	refreshed := decodePermissionEffectivenessEnvelope[*accesseffect.Detail](
		t,
		doJSONRequest(t, engine, http.MethodGet, accesseffect.DetailPath+"?userID="+userID, nil).Body.Bytes(),
	)
	if refreshed.Code != apipb.Code_Success ||
		refreshed.Data == nil ||
		refreshed.Data.Summary.RecentCheckCount != 2 ||
		len(refreshed.Data.RecentChecks) != 2 {
		t.Fatalf("expected two native audit records in projection, got %#v", refreshed)
	}
	decisions := map[string]bool{}
	for _, evidence := range refreshed.Data.RecentChecks {
		decision, _ := evidence.Detail["decision"].(string)
		decisions[decision] = true
		if evidence.Action != audit.AuditActionVerifyPermissionEffect || evidence.TargetID != userID {
			t.Fatalf("unexpected native audit evidence: %#v", evidence)
		}
		if evidence.OperatorName != prefix+"-admin" {
			t.Fatalf("audit evidence must use the human-readable username: %#v", evidence)
		}
	}
	if !decisions["allow"] || !decisions["deny"] {
		t.Fatalf("allow and deny audit evidence must both be present: %#v", refreshed.Data.RecentChecks)
	}
}

func TestPermissionEffectivenessTenantBoundaryAndSelfContext(t *testing.T) {
	const prefix = "permission-effectiveness-boundary"
	userID, enabledRoleID, _, _ := createPermissionEffectivenessFixture(t, prefix, platformTenant)

	foreignEngine := newPermissionEffectivenessTestEngine(&apipb.CurrentUser{
		Id:       prefix + "-foreign-admin",
		UserName: prefix + "-foreign-admin",
		TenantID: "effectiveness-tenant-b",
	})
	foreign := decodePermissionEffectivenessEnvelope[json.RawMessage](
		t,
		doJSONRequest(t, foreignEngine, http.MethodGet, accesseffect.DetailPath+"?userID="+userID, nil).Body.Bytes(),
	)
	if foreign.Code != apipb.Code_NoPermission {
		t.Fatalf("cross-tenant effectiveness lookup must be denied, got %#v", foreign)
	}

	selfEngine := newPermissionEffectivenessTestEngine(&apipb.CurrentUser{
		Id:       userID,
		UserName: prefix + "-user",
		TenantID: platformTenant,
		RoleIDs:  []string{enabledRoleID},
	})
	self := decodePermissionEffectivenessEnvelope[struct {
		UserID   string   `json:"userID"`
		TenantID string   `json:"tenantID"`
		RoleIDs  []string `json:"roleIDs"`
	}](
		t,
		doJSONRequest(t, selfEngine, http.MethodGet, accesseffect.SelfPath, nil).Body.Bytes(),
	)
	if self.Code != apipb.Code_Success ||
		self.Data.UserID != userID ||
		self.Data.TenantID != platformTenant ||
		len(self.Data.RoleIDs) != 1 ||
		self.Data.RoleIDs[0] != enabledRoleID {
		t.Fatalf("self context does not match injected JWT snapshot: %#v", self)
	}
}
