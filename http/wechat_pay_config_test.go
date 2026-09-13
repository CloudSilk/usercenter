package http_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/wechatpay"
	apipb "github.com/CloudSilk/usercenter/proto"
	"github.com/gin-gonic/gin"
)

// newTestPrivateKeyPEM 生成 PKCS8 "PRIVATE KEY" PEM,满足微信支付 SDK 的解析要求。
func newTestPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestWechatPayConfigReadIsRedactedAndBlankUpdatePreservesSecrets(t *testing.T) {
	privateKey := newTestPrivateKeyPEM(t)
	apiV3Key := strings.Repeat("z", 32)
	config := &wechatpay.PayConfig{
		TenantID: "paycfg-tenant", WechatConfigID: "paycfg-wc", AppID: "wx-paycfg",
		MchID: "1900000000", MchSerialNo: "serial-paycfg",
		APIV3Key: apiV3Key, PrivateKey: privateKey,
		NotifyURL: "https://host.example/api/wechat/notify/pay/app", Enable: true,
	}
	if _, err := wechatpay.CreatePayConfig(config); err != nil {
		t.Fatalf("create config: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&wechatpay.PayConfig{}, "id = ?", config.ID).Error
	})

	router := gin.New()
	userhttp.RegisterWechatPayConfigRouter(router)

	detail := httptest.NewRecorder()
	router.ServeHTTP(detail, httptest.NewRequest(
		http.MethodGet, "/api/core/wechat/pay/config/detail?id="+config.ID, nil))
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(
		http.MethodGet, "/api/core/wechat/pay/config/query?tenantID=paycfg-tenant", nil))
	for _, resp := range []*httptest.ResponseRecorder{detail, list} {
		for _, secret := range []string{apiV3Key, "PRIVATE KEY", "BEGIN"} {
			if strings.Contains(resp.Body.String(), secret) {
				t.Fatalf("response leaked secret %q: %s", secret, resp.Body.String())
			}
		}
	}

	// 留空 apiV3Key/privateKey 更新,应保持原值
	update := map[string]any{
		"id": config.ID, "tenantID": "paycfg-tenant", "wechatConfigID": "paycfg-wc",
		"appID": "wx-paycfg", "mchID": "1900000000", "mchSerialNo": "serial-paycfg-2",
		"notifyURL": "https://host.example/api/wechat/notify/pay/app", "enable": false,
		"description": "updated",
	}
	recorder := performJSON(router, http.MethodPut, "/api/core/wechat/pay/config/update", update)
	var response struct {
		Code    apipb.Code `json:"code"`
		Message string     `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Code != apipb.Code_Success {
		t.Fatalf("update response=%s err=%v", recorder.Body.String(), err)
	}
	var stored wechatpay.PayConfig
	if err := store.DB().First(&stored, "id = ?", config.ID).Error; err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if stored.MchSerialNo != "serial-paycfg-2" || stored.APIV3Key != apiV3Key || stored.PrivateKey != privateKey {
		t.Fatalf("unexpected stored config: mchSerialNo=%s, secrets preserved=%v/%v",
			stored.MchSerialNo, stored.APIV3Key == apiV3Key, stored.PrivateKey == privateKey)
	}
}

func TestWechatPayConfigAddRejectsInvalidPrivateKey(t *testing.T) {
	router := gin.New()
	userhttp.RegisterWechatPayConfigRouter(router)

	add := map[string]any{
		"tenantID": "paycfg-tenant-bad", "wechatConfigID": "wc-bad", "appID": "wx-bad",
		"mchID": "1900000001", "mchSerialNo": "serial-bad",
		"apiV3Key": strings.Repeat("k", 32), "privateKey": "not-a-valid-pem",
	}
	recorder := performJSON(router, http.MethodPost, "/api/core/wechat/pay/config/add", add)
	if !strings.Contains(recorder.Body.String(), "decode private key") &&
		!strings.Contains(recorder.Body.String(), "parse private key") &&
		!strings.Contains(recorder.Body.String(), "Bad Request") {
		t.Fatalf("expected private key validation failure, got: %s", recorder.Body.String())
	}

	// 合法私钥应保存成功
	add["privateKey"] = newTestPrivateKeyPEM(t)
	recorder = performJSON(router, http.MethodPost, "/api/core/wechat/pay/config/add", add)
	var response struct {
		Code apipb.Code `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Code != apipb.Code_Success {
		t.Fatalf("expected success, got: %s err=%v", recorder.Body.String(), err)
	}
}
