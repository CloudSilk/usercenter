package http_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/dictionaries"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/systemconfig"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

func newTenantConfigTestEngine(currentUser *apipb.CurrentUser) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", currentUser)
		c.Next()
	})
	userhttp.RegisterSystemConfigRouter(r)
	userhttp.RegisterDictionariesRouter(r)
	return r
}

func decodeQuerySystemConfigResponse(t *testing.T, body []byte) *apipb.QuerySystemConfigResponse {
	t.Helper()
	resp := &apipb.QuerySystemConfigResponse{}
	if err := json.Unmarshal(body, resp); err != nil {
		t.Fatalf("decode system config response: %v (body=%s)", err, body)
	}
	return resp
}

func decodeQueryDictionariesResponse(t *testing.T, body []byte) *apipb.QueryDictionariesResponse {
	t.Helper()
	resp := &apipb.QueryDictionariesResponse{}
	if err := json.Unmarshal(body, resp); err != nil {
		t.Fatalf("decode dictionaries response: %v (body=%s)", err, body)
	}
	return resp
}

func TestTenantConfigurationRoutesRejectCrossTenantAccess(t *testing.T) {
	const (
		tenantA = "config-scope-tenant-a"
		tenantB = "config-scope-tenant-b"
	)
	configA := &systemconfig.SystemConfig{Key: "scope.config.a", Value: "a", TenantID: tenantA}
	configB := &systemconfig.SystemConfig{Key: "scope.config.b", Value: "b", TenantID: tenantB}
	if _, err := systemconfig.CreateSystemConfig(configA); err != nil {
		t.Fatalf("create tenant A system config: %v", err)
	}
	if _, err := systemconfig.CreateSystemConfig(configB); err != nil {
		t.Fatalf("create tenant B system config: %v", err)
	}
	dictA := &dictionaries.Dictionaries{Name: "scope-dict-a", Type: "scope", Value: "a", TenantID: tenantA}
	dictB := &dictionaries.Dictionaries{Name: "scope-dict-b", Type: "scope", Value: "b", TenantID: tenantB}
	if _, err := dictionaries.CreateDictionaries(dictA); err != nil {
		t.Fatalf("create tenant A dictionary: %v", err)
	}
	if _, err := dictionaries.CreateDictionaries(dictB); err != nil {
		t.Fatalf("create tenant B dictionary: %v", err)
	}

	r := newTenantConfigTestEngine(&apipb.CurrentUser{Id: "tenant-a-admin", TenantID: tenantA})
	systemQuery := doJSONRequest(t, r, http.MethodGet, "/api/core/system/config/query?pageIndex=1&pageSize=100&tenantID="+tenantB, nil)
	systemResp := decodeQuerySystemConfigResponse(t, systemQuery.Body.Bytes())
	if systemResp.Code != apipb.Code_Success || len(systemResp.Data) != 1 || systemResp.Data[0].Id != configA.ID {
		t.Fatalf("system config query escaped tenant scope: code=%v data=%#v", systemResp.Code, systemResp.Data)
	}
	dictQuery := doJSONRequest(t, r, http.MethodGet, "/api/core/dictionaries/query?pageIndex=1&pageSize=100&tenantID="+tenantB, nil)
	dictResp := decodeQueryDictionariesResponse(t, dictQuery.Body.Bytes())
	if dictResp.Code != apipb.Code_Success || len(dictResp.Data) != 1 || dictResp.Data[0].Id != dictA.ID {
		t.Fatalf("dictionary query escaped tenant scope: code=%v data=%#v", dictResp.Code, dictResp.Data)
	}

	addSystem := doJSONRequest(t, r, http.MethodPost, "/api/core/system/config/add", &apipb.SystemConfigInfo{
		Key: "scope.config.forged", Value: "forged", TenantID: tenantB,
	})
	addedSystemID := decodeCommonResponse(t, addSystem).Message
	addedSystem, err := systemconfig.GetSystemConfigByID(addedSystemID)
	if err != nil || addedSystem.TenantID != tenantA {
		t.Fatalf("forged system config tenant accepted: config=%#v err=%v", addedSystem, err)
	}
	addDict := doJSONRequest(t, r, http.MethodPost, "/api/core/dictionaries/add", &apipb.DictionariesInfo{
		Name: "scope-dict-forged", Type: "scope", Value: "forged", TenantID: tenantB,
	})
	addedDictID := decodeCommonResponse(t, addDict).Message
	addedDict, err := dictionaries.GetDictionariesByID(addedDictID)
	if err != nil || addedDict.TenantID != tenantA {
		t.Fatalf("forged dictionary tenant accepted: dictionary=%#v err=%v", addedDict, err)
	}

	assertNoPermission := func(name string, responseBody []byte) {
		t.Helper()
		var resp apipb.CommonResponse
		if err := json.Unmarshal(responseBody, &resp); err != nil {
			t.Fatalf("decode %s response: %v (body=%s)", name, err, responseBody)
		}
		if resp.Code != apipb.Code_NoPermission {
			t.Fatalf("%s must return no permission, got code=%v body=%s", name, resp.Code, responseBody)
		}
	}
	assertNoPermission("system update", doJSONRequest(t, r, http.MethodPut, "/api/core/system/config/update", &apipb.SystemConfigInfo{
		Id: configB.ID, Key: configB.Key, Value: "hacked", TenantID: tenantB,
	}).Body.Bytes())
	assertNoPermission("system delete", doJSONRequest(t, r, http.MethodDelete, "/api/core/system/config/delete?tenantID="+tenantB, &apipb.DelRequest{Id: configB.ID}).Body.Bytes())
	assertNoPermission("dictionary update", doJSONRequest(t, r, http.MethodPut, "/api/core/dictionaries/update", &apipb.DictionariesInfo{
		Id: dictB.ID, Name: dictB.Name, Type: dictB.Type, Value: "hacked", TenantID: tenantB,
	}).Body.Bytes())
	assertNoPermission("dictionary delete", doJSONRequest(t, r, http.MethodDelete, "/api/core/dictionaries/delete?tenantID="+tenantB, &apipb.DelRequest{Id: dictB.ID}).Body.Bytes())

	for name, path := range map[string]string{
		"system detail":     "/api/core/system/config/detail?id=" + url.QueryEscape(configB.ID) + "&tenantID=" + tenantB,
		"dictionary detail": "/api/core/dictionaries/detail?id=" + url.QueryEscape(dictB.ID) + "&tenantID=" + tenantB,
	} {
		w := doJSONRequest(t, r, http.MethodGet, path, nil)
		var envelope struct {
			Code apipb.Code `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("decode %s response: %v", name, err)
		}
		if envelope.Code != apipb.Code_NoPermission {
			t.Fatalf("%s must return no permission, got code=%v body=%s", name, envelope.Code, w.Body.String())
		}
	}

	var remainingSystem systemconfig.SystemConfig
	if err := store.DB().First(&remainingSystem, "id = ?", configB.ID).Error; err != nil || remainingSystem.Value != "b" {
		t.Fatalf("tenant B system config was changed: config=%#v err=%v", remainingSystem, err)
	}
	var remainingDict dictionaries.Dictionaries
	if err := store.DB().First(&remainingDict, "id = ?", dictB.ID).Error; err != nil || remainingDict.Value != "b" {
		t.Fatalf("tenant B dictionary was changed: dictionary=%#v err=%v", remainingDict, err)
	}
}

func TestPlatformTenantCanExplicitlyManageTargetTenantConfiguration(t *testing.T) {
	const targetTenant = "config-scope-platform-target"
	r := newTenantConfigTestEngine(&apipb.CurrentUser{Id: "platform-admin", TenantID: platformTenant})

	addSystem := doJSONRequest(t, r, http.MethodPost, "/api/core/system/config/add", &apipb.SystemConfigInfo{
		Key: "scope.config.platform", Value: "platform", TenantID: targetTenant,
	})
	systemID := decodeCommonResponse(t, addSystem).Message
	addDict := doJSONRequest(t, r, http.MethodPost, "/api/core/dictionaries/add", &apipb.DictionariesInfo{
		Name: "scope-dict-platform", Type: "scope", Value: "platform", TenantID: targetTenant,
	})
	dictID := decodeCommonResponse(t, addDict).Message

	systemQuery := decodeQuerySystemConfigResponse(t, doJSONRequest(t, r, http.MethodGet,
		"/api/core/system/config/query?pageIndex=1&pageSize=100&tenantID="+targetTenant, nil).Body.Bytes())
	if systemQuery.Code != apipb.Code_Success || len(systemQuery.Data) != 1 || systemQuery.Data[0].Id != systemID {
		t.Fatalf("platform system config query failed: code=%v data=%#v", systemQuery.Code, systemQuery.Data)
	}
	dictQuery := decodeQueryDictionariesResponse(t, doJSONRequest(t, r, http.MethodGet,
		"/api/core/dictionaries/query?pageIndex=1&pageSize=100&tenantID="+targetTenant, nil).Body.Bytes())
	if dictQuery.Code != apipb.Code_Success || len(dictQuery.Data) != 1 || dictQuery.Data[0].Id != dictID {
		t.Fatalf("platform dictionary query failed: code=%v data=%#v", dictQuery.Code, dictQuery.Data)
	}

	if resp := decodeCommonResponse(t, doJSONRequest(t, r, http.MethodPut, "/api/core/system/config/update", &apipb.SystemConfigInfo{
		Id: systemID, Key: "scope.config.platform", Value: "updated", TenantID: targetTenant,
	})); resp.Code != apipb.Code_Success {
		t.Fatalf("platform system config update failed: %#v", resp)
	}
	if resp := decodeCommonResponse(t, doJSONRequest(t, r, http.MethodPut, "/api/core/dictionaries/update", &apipb.DictionariesInfo{
		Id: dictID, Name: "scope-dict-platform", Type: "scope", Value: "updated", TenantID: targetTenant,
	})); resp.Code != apipb.Code_Success {
		t.Fatalf("platform dictionary update failed: %#v", resp)
	}

	var updatedSystem systemconfig.SystemConfig
	if err := store.DB().First(&updatedSystem, "id = ?", systemID).Error; err != nil || updatedSystem.Value != "updated" || updatedSystem.TenantID != targetTenant {
		t.Fatalf("platform system config update not persisted: config=%#v err=%v", updatedSystem, err)
	}
	var updatedDict dictionaries.Dictionaries
	if err := store.DB().First(&updatedDict, "id = ?", dictID).Error; err != nil || updatedDict.Value != "updated" || updatedDict.TenantID != targetTenant {
		t.Fatalf("platform dictionary update not persisted: dictionary=%#v err=%v", updatedDict, err)
	}
}
