package http_test

import (
	"bytes"
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

func TestWechatPayOrderExport(t *testing.T) {
	const tenant = "payorder-export-tenant"
	orders := []*wechatpay.PayOrder{
		{TenantID: tenant, UserID: "u1", WechatConfigID: "wc-e", AppID: "wx-e", MchID: "1900000777",
			OutTradeNo: "export-trade-0001", TransactionID: "wx-tx-1", Amount: 1000,
			Status: wechatpay.PayOrderPaid, Description: "含,逗号与\"引号\""},
		{TenantID: tenant, UserID: "u2", WechatConfigID: "wc-e", AppID: "wx-e", MchID: "1900000777",
			OutTradeNo: "export-trade-0002", Amount: 2000, Status: wechatpay.PayOrderCreated},
	}
	for _, o := range orders {
		if _, err := wechatpay.CreatePayOrder(o); err != nil {
			t.Fatalf("create order: %v", err)
		}
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&wechatpay.PayOrder{}, "tenant_id = ?", tenant).Error
	})

	router := gin.New()
	userhttp.RegisterWechatPayOrderRouter(router)

	// 全量导出:UTF-8 BOM + 表头 + 2 行数据,CSV 特殊字符被正确转义
	all := httptest.NewRecorder()
	router.ServeHTTP(all, httptest.NewRequest(
		http.MethodGet, "/api/core/wechat/pay/order/export?tenantID="+tenant, nil))
	body := all.Body.Bytes()
	if all.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", all.Code)
	}
	if !bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("expected UTF-8 BOM prefix")
	}
	if ct := all.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("unexpected content type %q", ct)
	}
	if cd := all.Header().Get("Content-Disposition"); !strings.Contains(cd, ".csv") {
		t.Fatalf("unexpected content disposition %q", cd)
	}
	text := string(body[3:])
	if got := strings.Count(text, "\n"); got < 3 {
		t.Fatalf("expected header+2 rows, got %d lines", got)
	}
	for _, want := range []string{"export-trade-0001", "export-trade-0002", `"含,逗号与""引号"""`, "订单ID"} {
		if !strings.Contains(text, want) {
			t.Fatalf("csv missing %q", want)
		}
	}

	// 状态过滤导出:仅含 PAID 订单
	paid := httptest.NewRecorder()
	router.ServeHTTP(paid, httptest.NewRequest(
		http.MethodGet, "/api/core/wechat/pay/order/export?tenantID="+tenant+"&status=PAID", nil))
	if got := strings.Count(string(paid.Body.Bytes()), "\n"); got != 2 {
		t.Fatalf("expected header+1 row for PAID filter, got %d lines", got)
	}
	if strings.Contains(paid.Body.String(), "export-trade-0002") {
		t.Fatal("CREATED order should be excluded by status filter")
	}

	// 非法状态参数
	bad := httptest.NewRecorder()
	router.ServeHTTP(bad, httptest.NewRequest(
		http.MethodGet, "/api/core/wechat/pay/order/export?status=HACKED", nil))
	if !strings.Contains(bad.Body.String(), "Status") {
		t.Fatalf("expected validation error, got: %s", bad.Body.String())
	}
}
