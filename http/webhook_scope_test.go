package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/webhook"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

func newWebhookScopeEngine(currentUser *apipb.CurrentUser) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", currentUser)
		c.Next()
	})
	userhttp.RegisterAdminRouter(r)
	return r
}

func decodeWebhookList(t *testing.T, body []byte) struct {
	Code  apipb.Code             `json:"code"`
	Data  []webhook.Subscription `json:"data"`
	Total int64                  `json:"total"`
} {
	t.Helper()
	var response struct {
		Code  apipb.Code             `json:"code"`
		Data  []webhook.Subscription `json:"data"`
		Total int64                  `json:"total"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode webhook list: %v (body=%s)", err, body)
	}
	return response
}

func TestWebhookAdminRoutesEnforceTenantScope(t *testing.T) {
	const (
		tenantA = "webhook-scope-tenant-a"
		tenantB = "webhook-scope-tenant-b"
	)
	subA := &webhook.Subscription{TenantID: tenantA, Name: "tenant-a", URL: "https://a.example.test/hook", Events: "order.created", Secret: "secret-a", Enable: true}
	subB := &webhook.Subscription{TenantID: tenantB, Name: "tenant-b", URL: "https://b.example.test/hook", Events: "order.created", Secret: "secret-b", Enable: true}
	if _, err := webhook.CreateSub(subA); err != nil {
		t.Fatalf("create tenant A webhook: %v", err)
	}
	if _, err := webhook.CreateSub(subB); err != nil {
		t.Fatalf("create tenant B webhook: %v", err)
	}

	r := newWebhookScopeEngine(&apipb.CurrentUser{Id: "webhook-admin-a", TenantID: tenantA})
	list := decodeWebhookList(t, doJSONRequest(t, r, http.MethodGet, "/admin/api/webhooks?tenantID="+tenantB, nil).Body.Bytes())
	if list.Code != apipb.Code_Success || list.Total != 1 || len(list.Data) != 1 || list.Data[0].ID != subA.ID {
		t.Fatalf("webhook list escaped tenant scope: %#v", list)
	}

	created := doJSONRequest(t, r, http.MethodPost, "/admin/api/webhooks", map[string]any{
		"tenantID": tenantB, "name": "forged", "url": "https://forged.example.test/hook",
		"events": "order.created", "secret": "secret-forged", "enable": true,
	})
	var createResponse struct {
		Code apipb.Code `json:"code"`
		Data string     `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createResponse); err != nil || createResponse.Code != apipb.Code_Success || createResponse.Data == "" {
		t.Fatalf("create webhook failed: response=%#v err=%v body=%s", createResponse, err, created.Body.String())
	}
	createdSub, err := webhook.GetSub(createResponse.Data)
	if err != nil || createdSub.TenantID != tenantA {
		t.Fatalf("forged webhook tenant accepted: subscription=%#v err=%v", createdSub, err)
	}

	update := doJSONRequest(t, r, http.MethodPut, "/admin/api/webhooks/"+subB.ID+"?tenantID="+tenantB, map[string]any{
		"name": "hacked", "url": "https://hacked.example.test/hook", "events": "order.deleted", "enable": false,
	})
	if response := decodeCommonResponse(t, update); response.Code != apipb.Code_NoPermission {
		t.Fatalf("cross-tenant webhook update was not rejected: %#v", response)
	}
	remove := doJSONRequest(t, r, http.MethodDelete, "/admin/api/webhooks/"+subB.ID+"?tenantID="+tenantB, nil)
	if response := decodeCommonResponse(t, remove); response.Code != apipb.Code_NoPermission {
		t.Fatalf("cross-tenant webhook delete was not rejected: %#v", response)
	}
	remaining, err := webhook.GetSub(subB.ID)
	if err != nil || remaining.Name != "tenant-b" || !remaining.Enable {
		t.Fatalf("tenant B webhook was changed: subscription=%#v err=%v", remaining, err)
	}
}

func TestPlatformTenantCanExplicitlyManageTargetWebhook(t *testing.T) {
	const targetTenant = "webhook-scope-platform-target"
	sub := &webhook.Subscription{TenantID: targetTenant, Name: "platform-target", URL: "https://target.example.test/hook", Events: "order.created", Secret: "secret", Enable: true}
	if _, err := webhook.CreateSub(sub); err != nil {
		t.Fatalf("create target webhook: %v", err)
	}
	r := newWebhookScopeEngine(&apipb.CurrentUser{Id: "platform-admin", TenantID: platformTenant})
	list := decodeWebhookList(t, doJSONRequest(t, r, http.MethodGet, "/admin/api/webhooks?tenantID="+targetTenant, nil).Body.Bytes())
	if list.Code != apipb.Code_Success || list.Total != 1 || len(list.Data) != 1 || list.Data[0].ID != sub.ID {
		t.Fatalf("platform target webhook query failed: %#v", list)
	}
	update := doJSONRequest(t, r, http.MethodPut, "/admin/api/webhooks/"+sub.ID+"?tenantID="+targetTenant, map[string]any{
		"name": "platform-updated", "url": sub.URL, "events": sub.Events, "secret": sub.Secret, "enable": true,
	})
	var updateResponse apipb.CommonResponse
	if err := json.Unmarshal(update.Body.Bytes(), &updateResponse); err != nil || updateResponse.Code != apipb.Code_Success {
		t.Fatalf("platform target webhook update failed: response=%#v err=%v body=%s", updateResponse, err, update.Body.String())
	}
	updated, err := webhook.GetSub(sub.ID)
	if err != nil || updated.Name != "platform-updated" {
		t.Fatalf("platform update not persisted: subscription=%#v err=%v", updated, err)
	}
	remove := doJSONRequest(t, r, http.MethodDelete, "/admin/api/webhooks/"+sub.ID+"?tenantID="+targetTenant, nil)
	var deleteResponse apipb.CommonResponse
	if err := json.Unmarshal(remove.Body.Bytes(), &deleteResponse); err != nil || deleteResponse.Code != apipb.Code_Success {
		t.Fatalf("platform target webhook delete failed: response=%#v err=%v body=%s", deleteResponse, err, remove.Body.String())
	}
}
