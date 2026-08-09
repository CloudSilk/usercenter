package http_test

import (
	"net/http"
	"testing"

	"github.com/CloudSilk/usercenter/authorization"
	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/permission"
	"github.com/CloudSilk/usercenter/internal/principal"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

func newAuthorizationTestEngine(user *apipb.CurrentUser) *gin.Engine {
	return newAuthorizationTestEngineWithPrincipal(user, principal.NewHuman(user.Id, user.TenantID, user.RoleIDs))
}

func newAuthorizationTestEngineWithPrincipal(user *apipb.CurrentUser, authenticated principal.Principal) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("User", user)
		c.Set("Principal", authenticated)
		c.Next()
	})
	userhttp.RegisterAuthorizationRouter(router)
	return router
}

func TestRuntimeAuthorizationUsesCurrentPrincipalRoles(t *testing.T) {
	engine := newAuthorizationTestEngine(&apipb.CurrentUser{
		Id: "runtime-user", TenantID: platformTenant, UserName: "Runtime User", RoleIDs: []string{"1"},
	})
	allowed := decodePermissionEffectivenessEnvelope[struct {
		Allow     bool                            `json:"allow"`
		Reason    string                          `json:"reason"`
		DataScope authorization.DataScopeDecision `json:"dataScope"`
		Identity  struct {
			SubjectID     string   `json:"subjectID"`
			TenantID      string   `json:"tenantID"`
			RoleIDs       []string `json:"roleIDs"`
			PrincipalKind int32    `json:"principalKind"`
		} `json:"identity"`
	}](t, doJSONRequest(t, engine, http.MethodPost, authorization.RuntimeCheckPath, gin.H{
		"path": "/api/v1/runtime/documents/document-1", "method": http.MethodGet,
	}).Body.Bytes())
	if allowed.Code != apipb.Code_Success || !allowed.Data.Allow || allowed.Data.Reason != "role_policy" ||
		allowed.Data.Identity.SubjectID != "runtime-user" || allowed.Data.Identity.TenantID != platformTenant ||
		allowed.Data.Identity.PrincipalKind != int32(principal.KindHuman) ||
		len(allowed.Data.Identity.RoleIDs) != 1 || len(allowed.Data.DataScope.Rules) != 1 ||
		allowed.Data.DataScope.Rules[0].DataScope != authorization.DataScopeAll {
		t.Fatalf("unexpected runtime allow decision: %#v", allowed)
	}

	deniedEngine := newAuthorizationTestEngine(&apipb.CurrentUser{
		Id: "runtime-denied-user", TenantID: platformTenant, UserName: "Denied User", RoleIDs: []string{},
	})
	denied := decodePermissionEffectivenessEnvelope[struct {
		Allow  bool   `json:"allow"`
		Reason string `json:"reason"`
	}](t, doJSONRequest(t, deniedEngine, http.MethodPost, authorization.RuntimeCheckPath, gin.H{
		"path": "/api/v1/runtime/documents/document-1", "method": http.MethodDelete,
	}).Body.Bytes())
	if denied.Code != apipb.Code_Success || denied.Data.Allow || denied.Data.Reason != "no_matching_policy" {
		t.Fatalf("unexpected runtime deny decision: %#v", denied)
	}
}

func TestRuntimeAuthorizationExposesTrustedMachineIdentity(t *testing.T) {
	for _, test := range []struct {
		name          string
		authenticated principal.Principal
		wantKind      principal.Kind
		wantOwner     string
	}{
		{
			name:          "agent",
			authenticated: principal.NewAgent("agent-1", "owner-1", platformTenant, []string{"1"}),
			wantKind:      principal.KindAgent,
			wantOwner:     "owner-1",
		},
		{
			name:          "service",
			authenticated: principal.NewService("service-1", platformTenant, []string{"1"}),
			wantKind:      principal.KindService,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := newAuthorizationTestEngineWithPrincipal(&apipb.CurrentUser{
				Id: test.authenticated.Subject(), TenantID: platformTenant, RoleIDs: []string{"1"},
			}, test.authenticated)
			result := decodePermissionEffectivenessEnvelope[struct {
				Identity struct {
					SubjectID     string `json:"subjectID"`
					PrincipalKind int32  `json:"principalKind"`
					OwnerUserID   string `json:"ownerUserID"`
				} `json:"identity"`
			}](t, doJSONRequest(t, engine, http.MethodPost, authorization.RuntimeCheckPath, gin.H{
				"path": "/api/v1/runtime/documents/document-1", "method": http.MethodGet,
			}).Body.Bytes())
			if result.Code != apipb.Code_Success || result.Data.Identity.SubjectID != test.authenticated.Subject() ||
				result.Data.Identity.PrincipalKind != int32(test.wantKind) || result.Data.Identity.OwnerUserID != test.wantOwner {
				t.Fatalf("unexpected machine identity: %#v", result)
			}
		})
	}
}

func TestApplyAuthorizationCatalogRequiresSuperAdministrator(t *testing.T) {
	defer store.DB().Unscoped().Delete(&permission.API{}, "id = ?", "remote-catalog-api")
	catalog := authorization.Catalog{
		TenantID: "remote-catalog-tenant",
		APIs: []authorization.API{{
			ID: "remote-catalog-api", Path: "/api/v1/remote/catalog", Method: http.MethodGet,
			CheckAuth: true, CheckLogin: true,
		}},
	}

	nonAdmin := newAuthorizationTestEngine(&apipb.CurrentUser{
		Id: "catalog-user", TenantID: platformTenant, RoleIDs: []string{"catalog-reader"},
	})
	forbidden := decodePermissionEffectivenessEnvelope[authorization.Summary](
		t,
		doJSONRequest(t, nonAdmin, http.MethodPost, authorization.CatalogApplyPath, catalog).Body.Bytes(),
	)
	if forbidden.Code != apipb.Code_NoPermission {
		t.Fatalf("non-super-admin catalog apply must be denied: %#v", forbidden)
	}

	admin := newAuthorizationTestEngine(&apipb.CurrentUser{
		Id: "catalog-admin", TenantID: platformTenant, RoleIDs: []string{"1"},
	})
	applied := decodePermissionEffectivenessEnvelope[authorization.Summary](
		t,
		doJSONRequest(t, admin, http.MethodPost, authorization.CatalogApplyPath, catalog).Body.Bytes(),
	)
	if applied.Code != apipb.Code_Success || applied.Data.APICount != 1 || applied.Data.CasbinRuleCount != 0 {
		t.Fatalf("unexpected catalog apply response: %#v", applied)
	}
	api, err := permission.GetAPIById("remote-catalog-api")
	if err != nil || api.Path != "/api/v1/remote/catalog" || api.Method != http.MethodGet {
		t.Fatalf("catalog API resource was not registered: api=%#v err=%v", api, err)
	}
}
