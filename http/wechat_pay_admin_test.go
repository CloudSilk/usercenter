package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	userhttp "github.com/CloudSilk/usercenter/http"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/wechatpay"
	"github.com/gin-gonic/gin"
)

func TestWechatPayOrderAdminQuery(t *testing.T) {
	const tenant = "payorder-admin-tenant"
	order := &wechatpay.PayOrder{
		TenantID: tenant, UserID: "admin-query-user", WechatConfigID: "wc-x",
		AppID: "wx-x", MchID: "1900000099", OutTradeNo: "admin-query-trade-0001",
		Amount: 2560, Status: wechatpay.PayOrderPaid, Description: "年卡",
	}
	if _, err := wechatpay.CreatePayOrder(order); err != nil {
		t.Fatalf("create order: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&wechatpay.PayOrder{}, "id = ?", order.ID).Error
	})

	router := gin.New()
	userhttp.RegisterWechatPayOrderRouter(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet, "/api/core/wechat/pay/order/query?tenantID="+tenant+"&status=PAID", nil))
	var response struct {
		Code    int   `json:"code"`
		Records int64 `json:"records"`
		Data    []struct {
			OutTradeNo    string `json:"outTradeNo"`
			TransactionID string `json:"transactionID"`
			Amount        int64  `json:"amount"`
			Status        string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}
	if response.Code != 20000 || response.Records != 1 || len(response.Data) != 1 {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
	if response.Data[0].OutTradeNo != order.OutTradeNo || response.Data[0].Amount != 2560 ||
		response.Data[0].Status != "PAID" {
		t.Fatalf("unexpected order item: %+v", response.Data[0])
	}

	// 非法状态参数应被校验拦截
	bad := httptest.NewRecorder()
	router.ServeHTTP(bad, httptest.NewRequest(
		http.MethodGet, "/api/core/wechat/pay/order/query?status=HACKED", nil))
	if bad.Code != http.StatusOK || !strings.Contains(bad.Body.String(), "Status") {
		t.Fatalf("expected validation error, got: %d %s", bad.Code, bad.Body.String())
	}
}
