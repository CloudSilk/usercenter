package http_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

type adminAPIResponse struct {
	Code    int32           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type adminProviderInfo struct {
	ID                string `json:"id"`
	TenantID          string `json:"tenantID"`
	Name              string `json:"name"`
	BaseURL           string `json:"baseURL"`
	AuthType          string `json:"authType"`
	Healthy           bool   `json:"healthy"`
	Description       string `json:"description"`
	IsMust            bool   `json:"isMust"`
	KeyCount          int64  `json:"keyCount"`
	RouteCount        int64  `json:"routeCount"`
	CanDelete         bool   `json:"canDelete"`
	DeleteBlockReason string `json:"deleteBlockReason"`
}

type adminKeyInfo struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenantID"`
	ProviderID string `json:"providerID"`
	Name       string `json:"name"`
	KeyHint    string `json:"keyHint"`
	Priority   int32  `json:"priority"`
	Enable     bool   `json:"enable"`
	IsMust     bool   `json:"isMust"`
}

func newAdminTestEngine(currentUser *apipb.CurrentUser) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("User", currentUser)
		c.Next()
	})
	userhttp.RegisterAdminRouter(r)
	return r
}

func decodeAdminAPIResponse(t *testing.T, body []byte) adminAPIResponse {
	t.Helper()
	var response adminAPIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode admin response: %v (body=%s)", err, string(body))
	}
	return response
}

func requireAdminSuccess(t *testing.T, body []byte) adminAPIResponse {
	t.Helper()
	response := decodeAdminAPIResponse(t, body)
	if response.Code != int32(apipb.Code_Success) {
		t.Fatalf("admin request failed: code=%d message=%q body=%s", response.Code, response.Message, string(body))
	}
	return response
}

func requireAdminFailure(t *testing.T, body []byte) adminAPIResponse {
	t.Helper()
	response := decodeAdminAPIResponse(t, body)
	if response.Code == int32(apipb.Code_Success) {
		t.Fatalf("admin request unexpectedly succeeded: body=%s", string(body))
	}
	return response
}

func cleanupAITenant(t *testing.T, tenantID string) {
	t.Helper()
	db := store.DB().Unscoped()
	if err := db.Where("tenant_id = ?", tenantID).Delete(&apikey.ModelRoute{}).Error; err != nil {
		t.Fatalf("cleanup routes: %v", err)
	}
	if err := db.Where("tenant_id = ?", tenantID).Delete(&apikey.AIKey{}).Error; err != nil {
		t.Fatalf("cleanup keys: %v", err)
	}
	if err := db.Where("tenant_id = ?", tenantID).Delete(&apikey.AIProvider{}).Error; err != nil {
		t.Fatalf("cleanup providers: %v", err)
	}
}

func createProviderThroughAdmin(t *testing.T, r *gin.Engine, body map[string]any) string {
	t.Helper()
	response := requireAdminSuccess(t, doJSONRequest(
		t,
		r,
		http.MethodPost,
		"/admin/api/ai-providers",
		body,
	).Body.Bytes())
	var id string
	if err := json.Unmarshal(response.Data, &id); err != nil || id == "" {
		t.Fatalf("decode provider id: id=%q err=%v data=%s", id, err, string(response.Data))
	}
	return id
}

func createKeyThroughAdmin(t *testing.T, r *gin.Engine, body map[string]any) string {
	t.Helper()
	response := requireAdminSuccess(t, doJSONRequest(
		t,
		r,
		http.MethodPost,
		"/admin/api/ai-keys",
		body,
	).Body.Bytes())
	var id string
	if err := json.Unmarshal(response.Data, &id); err != nil || id == "" {
		t.Fatalf("decode key id: id=%q err=%v data=%s", id, err, string(response.Data))
	}
	return id
}

func createRouteThroughAdmin(t *testing.T, r *gin.Engine, body map[string]any) string {
	t.Helper()
	response := requireAdminSuccess(t, doJSONRequest(
		t,
		r,
		http.MethodPost,
		"/admin/api/model-routes",
		body,
	).Body.Bytes())
	var id string
	if err := json.Unmarshal(response.Data, &id); err != nil || id == "" {
		t.Fatalf("decode route id: id=%q err=%v data=%s", id, err, string(response.Data))
	}
	return id
}

func TestAdminAIManagementLifecycle(t *testing.T) {
	const tenantID = "admin-ai-lifecycle"
	const forgedTenantID = "admin-ai-forged"
	const firstSecret = "sk-lifecycle-1234567890"
	const rotatedSecret = "sk-rotated-0987654321"

	apikey.SetEncryptionKeyFrom("admin-ai-lifecycle-test-key")
	cleanupAITenant(t, tenantID)
	t.Cleanup(func() { cleanupAITenant(t, tenantID) })

	current := &apipb.CurrentUser{Id: "admin-ai-user", TenantID: tenantID, UserName: "admin-ai-user"}
	r := newAdminTestEngine(current)
	providerID := createProviderThroughAdmin(t, r, map[string]any{
		"tenantID": forgedTenantID,
		"name":     " Lifecycle Provider ",
		"baseURL":  "https://models.example.com/v1/",
		"authType": "bearer",
		"healthy":  true,
	})

	var storedProvider apikey.AIProvider
	if err := store.DB().Where("id = ?", providerID).First(&storedProvider).Error; err != nil {
		t.Fatalf("load stored provider: %v", err)
	}
	if storedProvider.TenantID != tenantID {
		t.Fatalf("request body tenant escaped current tenant: got=%q want=%q", storedProvider.TenantID, tenantID)
	}
	if storedProvider.Name != "Lifecycle Provider" || storedProvider.BaseURL != "https://models.example.com/v1" {
		t.Fatalf("provider input was not normalized: %#v", storedProvider)
	}

	keyID := createKeyThroughAdmin(t, r, map[string]any{
		"tenantID":   forgedTenantID,
		"providerID": providerID,
		"name":       "Primary Key",
		"apiKey":     firstSecret,
		"priority":   10,
		"enable":     true,
	})
	routeID := createRouteThroughAdmin(t, r, map[string]any{
		"tenantID":    forgedTenantID,
		"modelAlias":  "document-generation",
		"providerID":  providerID,
		"priority":    20,
		"enable":      true,
		"description": "公文生成主路由",
	})

	var storedKey apikey.AIKey
	if err := store.DB().Where("id = ?", keyID).First(&storedKey).Error; err != nil {
		t.Fatalf("load stored key: %v", err)
	}
	if storedKey.TenantID != tenantID || storedKey.APIKeyEnc == "" || storedKey.APIKeyEnc == firstSecret {
		t.Fatalf("key was not tenant-scoped and encrypted: %#v", storedKey)
	}

	keyList := doJSONRequest(
		t,
		r,
		http.MethodGet,
		"/admin/api/ai-keys?providerID="+providerID,
		nil,
	)
	requireAdminSuccess(t, keyList.Body.Bytes())
	keyJSON := keyList.Body.String()
	if strings.Contains(keyJSON, firstSecret) || strings.Contains(keyJSON, storedKey.APIKeyEnc) ||
		strings.Contains(keyJSON, "apiKeyEnc") {
		t.Fatalf("key plaintext or ciphertext leaked from API: %s", keyJSON)
	}
	var keyPayload struct {
		Code int32          `json:"code"`
		Data []adminKeyInfo `json:"data"`
	}
	if err := json.Unmarshal(keyList.Body.Bytes(), &keyPayload); err != nil {
		t.Fatalf("decode key list: %v", err)
	}
	if len(keyPayload.Data) != 1 || keyPayload.Data[0].KeyHint != "sk-l...7890" {
		t.Fatalf("unexpected redacted key list: %#v", keyPayload.Data)
	}

	providerList := doJSONRequest(t, r, http.MethodGet, "/admin/api/ai-providers", nil)
	requireAdminSuccess(t, providerList.Body.Bytes())
	var providerPayload struct {
		Code int32               `json:"code"`
		Data []adminProviderInfo `json:"data"`
	}
	if err := json.Unmarshal(providerList.Body.Bytes(), &providerPayload); err != nil {
		t.Fatalf("decode provider list: %v", err)
	}
	if len(providerPayload.Data) != 1 {
		t.Fatalf("unexpected provider list: %#v", providerPayload.Data)
	}
	info := providerPayload.Data[0]
	if info.KeyCount != 1 || info.RouteCount != 1 || info.CanDelete || info.DeleteBlockReason == "" {
		t.Fatalf("provider impact projection is wrong: %#v", info)
	}

	blockedDelete := doJSONRequest(t, r, http.MethodDelete, "/admin/api/ai-providers/"+providerID, nil)
	failure := requireAdminFailure(t, blockedDelete.Body.Bytes())
	if !strings.Contains(failure.Message, "1 个 Key") || !strings.Contains(failure.Message, "1 条模型路由") {
		t.Fatalf("dependency error missing impact counts: %#v", failure)
	}

	requireAdminSuccess(t, doJSONRequest(t, r, http.MethodPut, "/admin/api/ai-keys/"+keyID, map[string]any{
		"providerID": providerID,
		"name":       "Primary Key Renamed",
		"priority":   -5,
		"enable":     false,
	}).Body.Bytes())
	requireAdminSuccess(t, doJSONRequest(t, r, http.MethodPost, "/admin/api/ai-keys/"+keyID+"/rotate", map[string]any{
		"newAPIKey": rotatedSecret,
	}).Body.Bytes())

	var rotatedKey apikey.AIKey
	if err := store.DB().Where("id = ?", keyID).First(&rotatedKey).Error; err != nil {
		t.Fatalf("load rotated key: %v", err)
	}
	if rotatedKey.Enable || rotatedKey.Name != "Primary Key Renamed" ||
		rotatedKey.KeyHint != "sk-r...4321" || rotatedKey.APIKeyEnc == storedKey.APIKeyEnc {
		t.Fatalf("key update or rotation did not persist: %#v", rotatedKey)
	}

	requireAdminSuccess(t, doJSONRequest(t, r, http.MethodDelete, "/admin/api/model-routes/"+routeID, nil).Body.Bytes())
	requireAdminSuccess(t, doJSONRequest(t, r, http.MethodDelete, "/admin/api/ai-keys/"+keyID, nil).Body.Bytes())
	requireAdminSuccess(t, doJSONRequest(t, r, http.MethodDelete, "/admin/api/ai-providers/"+providerID, nil).Body.Bytes())

	var providerCount, keyCount, routeCount int64
	store.DB().Model(&apikey.AIProvider{}).Where("tenant_id = ?", tenantID).Count(&providerCount)
	store.DB().Model(&apikey.AIKey{}).Where("tenant_id = ?", tenantID).Count(&keyCount)
	store.DB().Model(&apikey.ModelRoute{}).Where("tenant_id = ?", tenantID).Count(&routeCount)
	if providerCount != 0 || keyCount != 0 || routeCount != 0 {
		t.Fatalf("lifecycle cleanup incomplete: providers=%d keys=%d routes=%d", providerCount, keyCount, routeCount)
	}
}

func TestAdminAIManagementTenantAndSystemBoundaries(t *testing.T) {
	const tenantA = "admin-ai-boundary-a"
	const tenantB = "admin-ai-boundary-b"

	apikey.SetEncryptionKeyFrom("admin-ai-boundary-test-key")
	cleanupAITenant(t, tenantA)
	cleanupAITenant(t, tenantB)
	t.Cleanup(func() {
		cleanupAITenant(t, tenantA)
		cleanupAITenant(t, tenantB)
	})

	engineA := newAdminTestEngine(&apipb.CurrentUser{Id: "admin-a", TenantID: tenantA, UserName: "admin-a"})
	engineB := newAdminTestEngine(&apipb.CurrentUser{Id: "admin-b", TenantID: tenantB, UserName: "admin-b"})
	providerID := createProviderThroughAdmin(t, engineA, map[string]any{
		"name": "Tenant A Provider", "baseURL": "https://tenant-a.example.com/v1",
		"authType": "query", "healthy": true,
	})
	keyID := createKeyThroughAdmin(t, engineA, map[string]any{
		"providerID": providerID, "name": "Tenant A Key",
		"apiKey": "tenant-a-secret-1234", "priority": 0, "enable": true,
	})
	routeID := createRouteThroughAdmin(t, engineA, map[string]any{
		"modelAlias": "tenant-a-model", "providerID": providerID,
		"priority": 0, "enable": true,
	})

	for name, response := range map[string][]byte{
		"list keys": doJSONRequest(t, engineB, http.MethodGet, "/admin/api/ai-keys?providerID="+providerID, nil).Body.Bytes(),
		"update provider": doJSONRequest(t, engineB, http.MethodPut, "/admin/api/ai-providers/"+providerID, map[string]any{
			"name": "Hijacked", "baseURL": "https://hijacked.example.com", "authType": "bearer",
		}).Body.Bytes(),
		"delete provider": doJSONRequest(t, engineB, http.MethodDelete, "/admin/api/ai-providers/"+providerID, nil).Body.Bytes(),
		"rotate key": doJSONRequest(t, engineB, http.MethodPost, "/admin/api/ai-keys/"+keyID+"/rotate", map[string]any{
			"newAPIKey": "hijacked-secret-1234",
		}).Body.Bytes(),
		"test key":   doJSONRequest(t, engineB, http.MethodPost, "/admin/api/ai-keys/"+keyID+"/test", nil).Body.Bytes(),
		"delete key": doJSONRequest(t, engineB, http.MethodDelete, "/admin/api/ai-keys/"+keyID, nil).Body.Bytes(),
		"update route": doJSONRequest(t, engineB, http.MethodPut, "/admin/api/model-routes/"+routeID, map[string]any{
			"modelAlias": "hijacked", "providerID": providerID, "enable": true,
		}).Body.Bytes(),
		"delete route": doJSONRequest(t, engineB, http.MethodDelete, "/admin/api/model-routes/"+routeID, nil).Body.Bytes(),
	} {
		t.Run(name, func(t *testing.T) {
			failure := requireAdminFailure(t, response)
			if !strings.Contains(failure.Message, "当前租户") {
				t.Fatalf("tenant boundary error is unclear: %#v", failure)
			}
		})
	}

	systemProvider := &apikey.AIProvider{
		TenantID: tenantA, Name: "banlu-mock", BaseURL: "http://127.0.0.1:19000/v1",
		AuthType: "bearer", Healthy: true, IsMust: true,
	}
	if _, err := apikey.CreateProvider(systemProvider); err != nil {
		t.Fatalf("create system provider: %v", err)
	}
	systemKey := &apikey.AIKey{
		TenantID: tenantA, ProviderID: systemProvider.ID, Name: "banlu-mock-key",
		Priority: 0, Enable: true, IsMust: true,
	}
	if _, err := apikey.CreateKey(systemKey, "banlu-mock-secret"); err != nil {
		t.Fatalf("create system key: %v", err)
	}
	systemRoute := &apikey.ModelRoute{
		TenantID: tenantA, ModelAlias: "banlu-mock-model", ProviderID: systemProvider.ID,
		Priority: 999, Enable: true,
	}
	if _, err := apikey.CreateRoute(systemRoute); err != nil {
		t.Fatalf("create system route: %v", err)
	}

	providerList := doJSONRequest(t, engineA, http.MethodGet, "/admin/api/ai-providers", nil)
	requireAdminSuccess(t, providerList.Body.Bytes())
	var providerPayload struct {
		Data []adminProviderInfo `json:"data"`
	}
	if err := json.Unmarshal(providerList.Body.Bytes(), &providerPayload); err != nil {
		t.Fatalf("decode system provider list: %v", err)
	}
	var systemInfo *adminProviderInfo
	for i := range providerPayload.Data {
		if providerPayload.Data[i].ID == systemProvider.ID {
			systemInfo = &providerPayload.Data[i]
			break
		}
	}
	if systemInfo == nil || !systemInfo.IsMust || systemInfo.CanDelete ||
		systemInfo.KeyCount != 1 || systemInfo.RouteCount != 1 {
		t.Fatalf("system provider projection is wrong: %#v", systemInfo)
	}

	for name, response := range map[string][]byte{
		"create reserved provider": doJSONRequest(t, engineA, http.MethodPost, "/admin/api/ai-providers", map[string]any{
			"name": "BANLU-MOCK", "baseURL": "http://127.0.0.1:19000/v1", "authType": "bearer",
		}).Body.Bytes(),
		"update system provider": doJSONRequest(t, engineA, http.MethodPut, "/admin/api/ai-providers/"+systemProvider.ID, map[string]any{
			"name": "Changed Mock", "baseURL": "http://127.0.0.1:19000/v1", "authType": "bearer",
		}).Body.Bytes(),
		"delete system provider": doJSONRequest(t, engineA, http.MethodDelete, "/admin/api/ai-providers/"+systemProvider.ID, nil).Body.Bytes(),
		"create key under system provider": doJSONRequest(t, engineA, http.MethodPost, "/admin/api/ai-keys", map[string]any{
			"providerID": systemProvider.ID, "name": "blocked-key",
			"apiKey": "blocked-secret", "enable": true,
		}).Body.Bytes(),
		"rotate system key": doJSONRequest(t, engineA, http.MethodPost, "/admin/api/ai-keys/"+systemKey.ID+"/rotate", map[string]any{
			"newAPIKey": "blocked-rotation",
		}).Body.Bytes(),
		"delete system key": doJSONRequest(t, engineA, http.MethodDelete, "/admin/api/ai-keys/"+systemKey.ID, nil).Body.Bytes(),
		"create route under system provider": doJSONRequest(t, engineA, http.MethodPost, "/admin/api/model-routes", map[string]any{
			"modelAlias": "blocked-route", "providerID": systemProvider.ID, "enable": true,
		}).Body.Bytes(),
		"delete system route": doJSONRequest(t, engineA, http.MethodDelete, "/admin/api/model-routes/"+systemRoute.ID, nil).Body.Bytes(),
	} {
		t.Run(name, func(t *testing.T) {
			requireAdminFailure(t, response)
		})
	}
}
