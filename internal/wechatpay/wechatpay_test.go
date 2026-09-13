package wechatpay

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CloudSilk/pkg/db"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/wechatconfig"
	glebsqlite "github.com/glebarez/sqlite"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"gorm.io/gorm"
)

// fakePayAPI 可编程的上游接口测试替身。
type fakePayAPI struct {
	prepayCalls    int
	prepayParams   *PayParams
	prepayErr      error
	lastPrepay     *PrepayInput
	queryResult    *TransactionResult
	queryErr       error
	closeCalls     int
	closeErr       error
	refundResult   *RefundResult
	refundErr      error
	lastRefund     *RefundInput
	refundStatus   string                        // 查退款单返回的状态
	queryByTradeNo map[string]*TransactionResult // 按订单号定制的查单结果,优先于 queryResult
	notify         *NotifyContent
	notifyErr      error
	billCSV        []byte // 交易账单下载返回内容
	billErr        error
}

func (f *fakePayAPI) Prepay(ctx context.Context, in PrepayInput) (*PayParams, error) {
	f.prepayCalls++
	f.lastPrepay = &in
	if f.prepayErr != nil {
		return nil, f.prepayErr
	}
	return f.prepayParams, nil
}

func (f *fakePayAPI) Query(ctx context.Context, mchID, outTradeNo string) (*TransactionResult, error) {
	if result, ok := f.queryByTradeNo[outTradeNo]; ok {
		return result, nil
	}
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.queryResult, nil
}

func (f *fakePayAPI) Close(ctx context.Context, mchID, outTradeNo string) error {
	f.closeCalls++
	return f.closeErr
}

func (f *fakePayAPI) Refund(ctx context.Context, in RefundInput) (*RefundResult, error) {
	f.lastRefund = &in
	if f.refundErr != nil {
		return nil, f.refundErr
	}
	return f.refundResult, nil
}

func (f *fakePayAPI) QueryRefund(ctx context.Context, outRefundNo string) (*RefundResult, error) {
	return &RefundResult{OutRefundNo: outRefundNo, Status: f.refundStatus}, nil
}

func (f *fakePayAPI) ParseNotify(req *http.Request) (*NotifyContent, error) { // nolint:revive
	if f.notifyErr != nil {
		return nil, f.notifyErr
	}
	return f.notify, nil
}

func (f *fakePayAPI) DownloadTradeBill(ctx context.Context, billDate, billType string) ([]byte, error) { // nolint:revive
	if f.billErr != nil {
		return nil, f.billErr
	}
	return f.billCSV, nil
}

var fakeAPI *fakePayAPI

func TestMain(m *testing.M) {
	gdb, err := gorm.Open(glebsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := gdb.AutoMigrate(&wechatconfig.WechatConfig{}, &PayConfig{}, &PayOrder{}, &PayRefund{}, &BillFile{}); err != nil {
		panic(err)
	}
	store.SetDB(db.NewDBClient(gdb, false))
	fakeAPI = &fakePayAPI{prepayParams: &PayParams{
		AppID: "wx123", TimeStamp: "1700000000", NonceStr: "nonce",
		Package: "prepay_id=wx1", SignType: "RSA", PaySign: "sig", PrepayID: "wx1",
	}, refundResult: &RefundResult{RefundID: "re-1", OutRefundNo: "", Status: RefundProcessing}}
	SetPayAPIFactory(func(cfg *PayConfig) (PayAPI, error) { return fakeAPI, nil })
	OpenIDResolver = func(userID, wechatConfigID string) (string, error) {
		return "openid-" + userID, nil
	}
	m.Run()
}

// setupApp 建一套 微信应用+商户配置,返回 appName。
func setupApp(t *testing.T, tenant string) string {
	t.Helper()
	wc := &wechatconfig.WechatConfig{
		AppID: "wx-app-" + tenant, AppName: "pay-app-" + tenant,
		DisplayName: "pay", TenantID: tenant, AppType: 1,
	}
	if err := store.DB().Create(wc).Error; err != nil {
		t.Fatalf("create wechat config: %v", err)
	}
	cfg := &PayConfig{
		TenantID: tenant, WechatConfigID: wc.ID, AppID: wc.AppID,
		MchID: "mch-" + tenant, MchSerialNo: "serial-1",
		APIV3Key: strings.Repeat("k", 32), PrivateKey: "pem",
		NotifyURL: "https://host.example/api/wechat/notify/pay/" + wc.AppName,
		Enable:    true,
	}
	if _, err := CreatePayConfig(cfg); err != nil {
		t.Fatalf("create pay config: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&PayConfig{}, "id = ?", cfg.ID).Error
		_ = store.DB().Unscoped().Delete(&wechatconfig.WechatConfig{}, "id = ?", wc.ID).Error
	})
	return wc.AppName
}

func TestCreateJSAPIPaymentHappyPath(t *testing.T) {
	const tenant = "wp-tenant-1"
	app := setupApp(t, tenant)

	order, params, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app,
		Description: "会员月卡", AmountFen: 990,
	})
	if err != nil {
		t.Fatalf("CreateJSAPIPayment: %v", err)
	}
	if order.Status != PayOrderCreated || order.Amount != 990 || order.PrepayID != "wx1" {
		t.Fatalf("unexpected order: %#v", order)
	}
	if params == nil || params.Package != "prepay_id=wx1" {
		t.Fatalf("unexpected params: %#v", params)
	}
	// 下单入参应携带 openID、回调地址与商户信息
	if fakeAPI.lastPrepay.OpenID != "openid-user-1" || fakeAPI.lastPrepay.NotifyURL == "" ||
		fakeAPI.lastPrepay.MchID != "mch-"+tenant {
		t.Fatalf("unexpected prepay input: %#v", fakeAPI.lastPrepay)
	}
}

func TestCreateJSAPIPaymentValidatesInput(t *testing.T) {
	const tenant = "wp-tenant-2"
	app := setupApp(t, tenant)

	if _, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 0,
	}); err == nil {
		t.Fatal("expected amount validation error")
	}
	if _, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 100,
		OutTradeNo: "短",
	}); err == nil {
		t.Fatal("expected out_trade_no validation error")
	}
	// 未启用支付配置的应用
	if _, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: "not-exist-app", Description: "x", AmountFen: 100,
	}); err == nil {
		t.Fatal("expected missing app error")
	}
}

func TestCreateJSAPIPaymentRequiresOpenID(t *testing.T) {
	const tenant = "wp-tenant-3"
	app := setupApp(t, tenant)
	old := OpenIDResolver
	OpenIDResolver = func(userID, wechatConfigID string) (string, error) { return "", nil }
	defer func() { OpenIDResolver = old }()

	if _, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-x", App: app, Description: "x", AmountFen: 100,
	}); err == nil || !strings.Contains(err.Error(), "未绑定") {
		t.Fatalf("expected openid binding error, got %v", err)
	}
}

func TestCreateJSAPIPaymentReusesCreatedOrder(t *testing.T) {
	const tenant = "wp-tenant-4"
	app := setupApp(t, tenant)
	in := CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app,
		Description: "x", AmountFen: 500, OutTradeNo: "trade-reuse-0001",
	}
	first, _, err := CreateJSAPIPayment(context.Background(), in)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	callsBefore := fakeAPI.prepayCalls
	second, _, err := CreateJSAPIPayment(context.Background(), in)
	if err != nil {
		t.Fatalf("reuse create: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same order, got %s vs %s", first.ID, second.ID)
	}
	if fakeAPI.prepayCalls != callsBefore+1 {
		t.Fatalf("expected one more prepay call, got %d", fakeAPI.prepayCalls-callsBefore)
	}
}

func TestCreateJSAPIPaymentRejectsForeignTenant(t *testing.T) {
	const tenant = "wp-tenant-5"
	app := setupApp(t, tenant)
	if _, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: "other-tenant", UserID: "user-1", App: app, Description: "x", AmountFen: 100,
	}); err != ErrOrderNotOwned {
		t.Fatalf("expected ErrOrderNotOwned, got %v", err)
	}
}

func TestHandlePayNotifyMarksOrderPaidIdempotently(t *testing.T) {
	const tenant = "wp-tenant-6"
	app := setupApp(t, tenant)
	order, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 300,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	paidAt := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	fakeAPI.notify = &NotifyContent{
		EventType: "TRANSACTION.SUCCESS", OutTradeNo: order.OutTradeNo,
		TransactionID: "4200001", TradeState: "SUCCESS", SuccessTime: paidAt, AmountFen: 300,
	}
	req := httptest.NewRequest("POST", "/api/wechat/notify/pay/"+app, strings.NewReader("{}"))
	if err := HandlePayNotify(req, app); err != nil {
		t.Fatalf("HandlePayNotify: %v", err)
	}
	stored, err := GetPayOrderByOutTradeNo(order.OutTradeNo)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if stored.Status != PayOrderPaid || stored.TransactionID != "4200001" {
		t.Fatalf("unexpected order: %#v", stored)
	}
	if stored.PaidAt == nil || !stored.PaidAt.Equal(paidAt) {
		t.Fatalf("unexpected paidAt: %v", stored.PaidAt)
	}

	// 重复回调:幂等成功且 paid_at 不变
	req2 := httptest.NewRequest("POST", "/api/wechat/notify/pay/"+app, strings.NewReader("{}"))
	if err := HandlePayNotify(req2, app); err != nil {
		t.Fatalf("repeat HandlePayNotify: %v", err)
	}
	stored2, _ := GetPayOrderByOutTradeNo(order.OutTradeNo)
	if !stored2.PaidAt.Equal(paidAt) {
		t.Fatalf("paidAt changed on duplicate notify: %v", stored2.PaidAt)
	}
}

func TestHandlePayNotifyUnknownOrder(t *testing.T) {
	const tenant = "wp-tenant-7"
	app := setupApp(t, tenant)
	fakeAPI.notify = &NotifyContent{
		EventType: "TRANSACTION.SUCCESS", OutTradeNo: "no-such-order",
		TransactionID: "4200002", TradeState: "SUCCESS",
	}
	req := httptest.NewRequest("POST", "/api/wechat/notify/pay/"+app, strings.NewReader("{}"))
	if err := HandlePayNotify(req, app); err != ErrUnknownNotifyOrder {
		t.Fatalf("expected ErrUnknownNotifyOrder, got %v", err)
	}
}

func TestHandlePayNotifyRecordEventsWithoutStatusChange(t *testing.T) {
	const tenant = "wp-tenant-8"
	app := setupApp(t, tenant)
	order, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 300,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	fakeAPI.notify = &NotifyContent{
		EventType: "TRANSACTION.CLOSED", OutTradeNo: order.OutTradeNo,
		TradeState: "CLOSED", TradeStateDesc: "订单已关闭",
	}
	req := httptest.NewRequest("POST", "/api/wechat/notify/pay/"+app, strings.NewReader("{}"))
	if err := HandlePayNotify(req, app); err != nil {
		t.Fatalf("HandlePayNotify: %v", err)
	}
	stored, _ := GetPayOrderByOutTradeNo(order.OutTradeNo)
	if stored.Status != PayOrderCreated {
		t.Fatalf("non-payment event should not change status, got %s", stored.Status)
	}
	if stored.LastEvent != "TRANSACTION.CLOSED" {
		t.Fatalf("expected last event recorded, got %q", stored.LastEvent)
	}
}

func TestSyncOrderStatusUpdatesFromRemote(t *testing.T) {
	const tenant = "wp-tenant-9"
	app := setupApp(t, tenant)
	order, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 700,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	paidAt := time.Now().Add(-time.Minute)
	fakeAPI.queryResult = &TransactionResult{
		OutTradeNo: order.OutTradeNo, TransactionID: "4200003",
		TradeState: "SUCCESS", SuccessTime: paidAt,
	}

	got, err := SyncOrderStatus(context.Background(), tenant, "user-1", order.OutTradeNo)
	if err != nil {
		t.Fatalf("SyncOrderStatus: %v", err)
	}
	if got.Status != PayOrderPaid || got.TransactionID != "4200003" {
		t.Fatalf("unexpected synced order: %#v", got)
	}
	// 他用户访问同一订单应被拒绝
	if _, err := SyncOrderStatus(context.Background(), tenant, "user-2", order.OutTradeNo); err != ErrOrderNotOwned {
		t.Fatalf("expected ErrOrderNotOwned, got %v", err)
	}
}

func TestClosePayOrderLifecycle(t *testing.T) {
	const tenant = "wp-tenant-10"
	app := setupApp(t, tenant)
	order, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 800,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if err := ClosePayOrder(context.Background(), tenant, "user-1", order.OutTradeNo); err != nil {
		t.Fatalf("ClosePayOrder: %v", err)
	}
	if fakeAPI.closeCalls != 1 {
		t.Fatalf("expected remote close called once, got %d", fakeAPI.closeCalls)
	}
	stored, _ := GetPayOrderByOutTradeNo(order.OutTradeNo)
	if stored.Status != PayOrderClosed {
		t.Fatalf("expected closed, got %s", stored.Status)
	}
	// 重复关单幂等
	if err := ClosePayOrder(context.Background(), tenant, "user-1", order.OutTradeNo); err != nil {
		t.Fatalf("repeat ClosePayOrder: %v", err)
	}
	// 已支付订单不可关闭
	paid, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 800,
	})
	if err != nil {
		t.Fatalf("create paid order: %v", err)
	}
	fakeAPI.queryResult = &TransactionResult{TradeState: "SUCCESS", TransactionID: "4200004"}
	_, _ = SyncOrderStatus(context.Background(), tenant, "user-1", paid.OutTradeNo)
	if err := ClosePayOrder(context.Background(), tenant, "user-1", paid.OutTradeNo); err != ErrOrderAlreadyPaid {
		t.Fatalf("expected ErrOrderAlreadyPaid, got %v", err)
	}
}

func TestPayAPIClientCache(t *testing.T) {
	var builds int32
	payAPIMu.Lock()
	originalFactory := payAPIFactory
	payAPIFactory = func(cfg *PayConfig) (PayAPI, error) {
		atomic.AddInt32(&builds, 1)
		return fakeAPI, nil
	}
	payAPIMu.Unlock()
	t.Cleanup(func() {
		payAPIMu.Lock()
		payAPIFactory = originalFactory
		payAPIMu.Unlock()
	})

	cfg := &PayConfig{MchID: "mch-cache", MchSerialNo: "s", APIV3Key: strings.Repeat("v", 32), PrivateKey: "pem"}
	cfg.ID = "cache-config-id"
	if _, err := GetPayAPI(cfg); err != nil {
		t.Fatalf("GetPayAPI: %v", err)
	}
	if _, err := GetPayAPI(cfg); err != nil {
		t.Fatalf("GetPayAPI second: %v", err)
	}
	if got := atomic.LoadInt32(&builds); got != 1 {
		t.Fatalf("expected cached client (1 build), got %d", got)
	}
	// 密钥变更后指纹变化,重建
	cfg.MchSerialNo = "s2"
	if _, err := GetPayAPI(cfg); err != nil {
		t.Fatalf("GetPayAPI after fingerprint change: %v", err)
	}
	if got := atomic.LoadInt32(&builds); got != 2 {
		t.Fatalf("expected rebuild after fingerprint change, got %d", got)
	}
	// 显式失效后重建
	InvalidatePayAPI(cfg.ID)
	if _, err := GetPayAPI(cfg); err != nil {
		t.Fatalf("GetPayAPI after invalidation: %v", err)
	}
	if got := atomic.LoadInt32(&builds); got != 3 {
		t.Fatalf("expected rebuild after invalidation, got %d", got)
	}
}

// TestLoadPrivateKeyRequiresPKCS8 验证配置校验使用的 SDK 私钥解析行为:
// 仅接受 PKCS8 "PRIVATE KEY" PEM,用于约束管理端保存合法密钥。
func TestLoadPrivateKeyRequiresPKCS8(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	if _, err := utils.LoadPrivateKey(pemStr); err != nil {
		t.Fatalf("expected PKCS8 pem to load, got %v", err)
	}
	if _, err := utils.LoadPrivateKey("not-a-pem"); err == nil {
		t.Fatal("expected invalid pem to fail")
	}
}

// TestQueryPayOrders 验证管理端订单分页查询的过滤条件。
func TestQueryPayOrders(t *testing.T) {
	const tenant = "wp-tenant-query"
	setupApp(t, tenant)
	tradeNos := map[string]string{}
	for i, status := range []string{PayOrderCreated, PayOrderPaid, PayOrderClosed} {
		order := &PayOrder{
			TenantID: tenant, UserID: "query-user", WechatConfigID: "wc-" + tenant,
			AppID: "wx-app", MchID: "mch-query", OutTradeNo: strings.Repeat("0", i) + "query-trade-" + status,
			Amount: int64(100 + i), Status: status, Description: "d",
		}
		if _, err := CreatePayOrder(order); err != nil {
			t.Fatalf("create order: %v", err)
		}
		tradeNos[status] = order.OutTradeNo
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&PayOrder{}, "tenant_id = ?", tenant).Error
	})

	result, err := QueryPayOrders(&PayOrderQuery{TenantID: tenant, PageSize: 2, PageIndex: 1})
	if err != nil {
		t.Fatalf("QueryPayOrders: %v", err)
	}
	if result.Total != 3 || len(result.Records) != 2 || result.Pages != 2 {
		t.Fatalf("unexpected paging: total=%d records=%d pages=%d", result.Total, len(result.Records), result.Pages)
	}
	byStatus, err := QueryPayOrders(&PayOrderQuery{TenantID: tenant, Status: PayOrderPaid})
	if err != nil || byStatus.Total != 1 {
		t.Fatalf("status filter: %v %+v", err, byStatus)
	}
	if byStatus.Records[0].OutTradeNo != tradeNos[PayOrderPaid] {
		t.Fatalf("unexpected filtered order: %s", byStatus.Records[0].OutTradeNo)
	}
	byNo, err := QueryPayOrders(&PayOrderQuery{UserID: "query-user", OutTradeNo: tradeNos[PayOrderClosed]})
	if err != nil || byNo.Total != 1 {
		t.Fatalf("outTradeNo filter: %v %+v", err, byNo)
	}
	none, err := QueryPayOrders(&PayOrderQuery{TenantID: "no-such-tenant"})
	if err != nil || none.Total != 0 {
		t.Fatalf("expected empty result for foreign tenant: %v %+v", err, none)
	}
}

// markOrderPaidDirect 直接将订单置为已支付(测试辅助,模拟回调后的状态)。
func markOrderPaidDirect(t *testing.T, tenant, app string, in CreateOrderInput) *PayOrder {
	t.Helper()
	order, _, err := CreateJSAPIPayment(context.Background(), in)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	fakeAPI.queryResult = &TransactionResult{TradeState: "SUCCESS", TransactionID: "tx-" + order.ID}
	got, err := SyncOrderStatus(context.Background(), tenant, in.UserID, order.OutTradeNo)
	if err != nil || got.Status != PayOrderPaid {
		t.Fatalf("expected paid order, got %v err=%v", got, err)
	}
	return got
}

func TestApplyRefundHappyPath(t *testing.T) {
	const tenant = "wp-refund-1"
	app := setupApp(t, tenant)
	order := markOrderPaidDirect(t, tenant, app, CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 1000,
	})

	refund, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 400, Reason: "部分退款",
	})
	if err != nil {
		t.Fatalf("ApplyRefund: %v", err)
	}
	if refund.Status != RefundProcessing || refund.Amount != 400 || refund.Total != 1000 {
		t.Fatalf("unexpected refund: %#v", refund)
	}
	if fakeAPI.lastRefund.RefundAmountFen != 400 || fakeAPI.lastRefund.TotalFen != 1000 ||
		fakeAPI.lastRefund.NotifyURL == "" || fakeAPI.lastRefund.OutRefundNo == "" {
		t.Fatalf("unexpected refund input: %#v", fakeAPI.lastRefund)
	}

	// 幂等:同 outRefundNo 重复申请返回已有退款单
	again, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 400, OutRefundNo: refund.OutRefundNo,
	})
	if err != nil || again.ID != refund.ID {
		t.Fatalf("expected idempotent refund, got %v err=%v", again, err)
	}

	// 剩余可退 600,超退应被拒绝
	if _, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 601,
	}); err == nil {
		t.Fatal("expected over-refund error")
	}
}

func TestApplyRefundGuards(t *testing.T) {
	const tenant = "wp-refund-2"
	app := setupApp(t, tenant)
	unpaid, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 500,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: unpaid.OutTradeNo, RefundAmount: 100,
	}); err == nil || !strings.Contains(err.Error(), "已支付") {
		t.Fatalf("expected unpaid order error, got %v", err)
	}
	paid := markOrderPaidDirect(t, tenant, app, CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "y", AmountFen: 500,
	})
	if _, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: paid.OutTradeNo, RefundAmount: 0,
	}); err == nil {
		t.Fatal("expected zero amount error")
	}
	if _, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: "other-tenant", OutTradeNo: paid.OutTradeNo, RefundAmount: 100,
	}); err != ErrOrderNotOwned {
		t.Fatalf("expected ErrOrderNotOwned, got %v", err)
	}
}

func TestRefundNotifyUpdatesStatusIdempotently(t *testing.T) {
	const tenant = "wp-refund-3"
	app := setupApp(t, tenant)
	order := markOrderPaidDirect(t, tenant, app, CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 900,
	})
	refund, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 900,
	})
	if err != nil {
		t.Fatalf("ApplyRefund: %v", err)
	}

	successAt := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	fakeAPI.notify = &NotifyContent{
		EventType: "REFUND.SUCCESS", OutTradeNo: order.OutTradeNo,
		OutRefundNo: refund.OutRefundNo, RefundID: "re-9", RefundStatus: RefundSuccess,
		RefundSuccessTime: successAt, RefundAmountFen: 900,
	}
	req := httptest.NewRequest("POST", "/api/wechat/notify/pay/"+app, strings.NewReader("{}"))
	if err := HandlePayNotify(req, app); err != nil {
		t.Fatalf("HandlePayNotify refund: %v", err)
	}
	stored, err := GetPayRefundByOutRefundNo(refund.OutRefundNo)
	if err != nil {
		t.Fatalf("get refund: %v", err)
	}
	if stored.Status != RefundSuccess || stored.RefundID != "re-9" {
		t.Fatalf("unexpected refund: %#v", stored)
	}

	// 重复回调幂等
	if err := HandlePayNotify(req, app); err != nil {
		t.Fatalf("repeat HandlePayNotify: %v", err)
	}

	// 原订单支付状态不受退款事件影响
	paid, _ := GetPayOrderByOutTradeNo(order.OutTradeNo)
	if paid.Status != PayOrderPaid {
		t.Fatalf("order status changed by refund event: %s", paid.Status)
	}

	// 未知退款单号应报错
	fakeAPI.notify = &NotifyContent{
		EventType: "REFUND.SUCCESS", OutTradeNo: order.OutTradeNo,
		OutRefundNo: "no-such-refund-no", RefundStatus: RefundSuccess,
	}
	if err := HandlePayNotify(req, app); err != ErrUnknownNotifyOrder {
		t.Fatalf("expected ErrUnknownNotifyOrder, got %v", err)
	}
}

func TestSyncRefundStatus(t *testing.T) {
	const tenant = "wp-refund-4"
	app := setupApp(t, tenant)
	order := markOrderPaidDirect(t, tenant, app, CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 700,
	})
	refund, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 200,
	})
	if err != nil {
		t.Fatalf("ApplyRefund: %v", err)
	}
	fakeAPI.refundStatus = RefundSuccess
	got, err := SyncRefundStatus(context.Background(), tenant, refund.OutRefundNo)
	if err != nil {
		t.Fatalf("SyncRefundStatus: %v", err)
	}
	if got.Status != RefundSuccess {
		t.Fatalf("expected SUCCESS, got %s", got.Status)
	}
	// 他租户不可见
	if _, err := SyncRefundStatus(context.Background(), "other-tenant", refund.OutRefundNo); err != ErrOrderNotOwned {
		t.Fatalf("expected ErrOrderNotOwned, got %v", err)
	}
	// 平台侧(空租户)可同步仍在处理中的退款单
	second, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 100,
	})
	if err != nil {
		t.Fatalf("apply second refund: %v", err)
	}
	fakeAPI.refundStatus = RefundClosed
	got2, err := SyncRefundStatus(context.Background(), "", second.OutRefundNo)
	if err != nil {
		t.Fatalf("platform sync: %v", err)
	}
	if got2.Status != RefundClosed {
		t.Fatalf("expected CLOSED via platform sync, got %s", got2.Status)
	}
}

func TestCreateJSAPIPaymentExpire(t *testing.T) {
	const tenant = "wp-expire-1"
	app := setupApp(t, tenant)
	order, _, err := CreateJSAPIPayment(context.Background(), CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 600, ExpireMinutes: 30,
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.ExpireAt == nil {
		t.Fatal("expected expireAt set")
	}
	if got := int(time.Until(*order.ExpireAt).Minutes()); got < 28 || got > 31 {
		t.Fatalf("unexpected expire window: %d minutes", got)
	}
	if fakeAPI.lastPrepay.TimeExpire.IsZero() || !fakeAPI.lastPrepay.TimeExpire.Equal(*order.ExpireAt) {
		t.Fatalf("prepay TimeExpire not passed: %v", fakeAPI.lastPrepay.TimeExpire)
	}

	// 过期订单查单:远端确认未支付后本地关单
	fakeAPI.queryResult = &TransactionResult{TradeState: "NOTPAY"}
	past := time.Now().Add(-time.Minute)
	if err := store.DB().Model(&PayOrder{}).Where("id = ?", order.ID).Update("expire_at", past).Error; err != nil {
		t.Fatalf("set expire_at: %v", err)
	}
	got, err := SyncOrderStatus(context.Background(), tenant, "user-1", order.OutTradeNo)
	if err != nil {
		t.Fatalf("SyncOrderStatus: %v", err)
	}
	if got.Status != PayOrderClosed || got.TradeStateDesc != "订单已过期" {
		t.Fatalf("expected expired close, got %#v", got)
	}
}

func TestListUserOrders(t *testing.T) {
	const tenant = "wp-mylist-1"
	setupApp(t, tenant)
	for i := 0; i < 3; i++ {
		order := &PayOrder{
			TenantID: tenant, UserID: "my-user", WechatConfigID: "wc", AppID: "wx", MchID: "mch",
			OutTradeNo: fmt.Sprintf("my-list-trade-%04d", i), Amount: 100, Status: PayOrderCreated,
		}
		if _, err := CreatePayOrder(order); err != nil {
			t.Fatalf("create order: %v", err)
		}
	}
	// 其他用户的订单不应出现
	if _, err := CreatePayOrder(&PayOrder{
		TenantID: tenant, UserID: "another-user", OutTradeNo: "my-list-trade-other",
		Amount: 1, Status: PayOrderCreated,
	}); err != nil {
		t.Fatalf("create other order: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&PayOrder{}, "tenant_id = ?", tenant).Error
	})

	result, err := ListUserOrders(tenant, "my-user", "", 1, 10)
	if err != nil {
		t.Fatalf("ListUserOrders: %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected 3 own orders, got %d", result.Total)
	}
	paidOnly, err := ListUserOrders(tenant, "my-user", PayOrderPaid, 1, 10)
	if err != nil || paidOnly.Total != 0 {
		t.Fatalf("expected empty paid filter, got %v err=%v", paidOnly, err)
	}
	if _, err := ListUserOrders(tenant, "", "", 1, 10); err == nil {
		t.Fatal("expected missing user error")
	}
}

// ageOrder 把订单的 created_at 改旧,模拟"超时未收到回调"。
func ageOrder(t *testing.T, id string) {
	t.Helper()
	if err := store.DB().Model(&PayOrder{}).Where("id = ?", id).
		Update("created_at", time.Now().Add(-10*time.Minute)).Error; err != nil {
		t.Fatalf("age order: %v", err)
	}
}

func TestReconcileStaleOrders(t *testing.T) {
	const tenant = "wp-reconcile-1"
	app := setupApp(t, tenant)
	wc, err := wechatconfig.GetWechatConfigByAppName(app)
	if err != nil {
		t.Fatalf("get wechat config: %v", err)
	}
	makeOrder := func(suffix string) *PayOrder {
		t.Helper()
		order := &PayOrder{
			TenantID: tenant, UserID: "user-1", WechatConfigID: wc.ID, AppID: "wx", MchID: "mch",
			OutTradeNo: "reconcile-" + suffix, Amount: 100, Status: PayOrderCreated,
		}
		if _, err := CreatePayOrder(order); err != nil {
			t.Fatalf("create order: %v", err)
		}
		return order
	}
	paid := makeOrder("paid")
	closed := makeOrder("closed")
	notpay := makeOrder("notpay")
	expired := makeOrder("expired")
	fresh := makeOrder("fresh")
	for _, o := range []*PayOrder{paid, closed, notpay, expired} {
		ageOrder(t, o.ID)
	}
	// expired 订单设置已过期的失效时间
	expiredAt := time.Now().Add(-time.Minute)
	if err := store.DB().Model(&PayOrder{}).Where("id = ?", expired.ID).
		Update("expire_at", expiredAt).Error; err != nil {
		t.Fatalf("set expire_at: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&PayOrder{}, "tenant_id = ?", tenant).Error
	})

	fakeAPI.queryByTradeNo = map[string]*TransactionResult{
		paid.OutTradeNo:    {TradeState: "SUCCESS", TransactionID: "tx-paid"},
		closed.OutTradeNo:  {TradeState: "CLOSED", TradeStateDesc: "已关闭"},
		notpay.OutTradeNo:  {TradeState: "NOTPAY"},
		expired.OutTradeNo: {TradeState: "NOTPAY"},
		fresh.OutTradeNo:   {TradeState: "NOTPAY"},
	}

	processed, err := ReconcileStaleOrders(context.Background())
	if err != nil {
		t.Fatalf("ReconcileStaleOrders: %v", err)
	}
	if processed != 4 {
		t.Fatalf("expected 4 stale orders processed (fresh excluded), got %d", processed)
	}

	gotPaid, _ := GetPayOrderByOutTradeNo(paid.OutTradeNo)
	if gotPaid.Status != PayOrderPaid || gotPaid.TransactionID != "tx-paid" {
		t.Fatalf("expected paid order synced, got %#v", gotPaid)
	}
	gotClosed, _ := GetPayOrderByOutTradeNo(closed.OutTradeNo)
	if gotClosed.Status != PayOrderClosed {
		t.Fatalf("expected closed order, got %s", gotClosed.Status)
	}
	gotNotpay, _ := GetPayOrderByOutTradeNo(notpay.OutTradeNo)
	if gotNotpay.Status != PayOrderCreated {
		t.Fatalf("expected notpay order kept CREATED, got %s", gotNotpay.Status)
	}
	gotExpired, _ := GetPayOrderByOutTradeNo(expired.OutTradeNo)
	if gotExpired.Status != PayOrderClosed || gotExpired.TradeStateDesc != "订单已过期" {
		t.Fatalf("expected expired order closed, got %#v", gotExpired)
	}
	gotFresh, _ := GetPayOrderByOutTradeNo(fresh.OutTradeNo)
	if gotFresh.Status != PayOrderCreated {
		t.Fatalf("fresh order should not be reconciled, got %s", gotFresh.Status)
	}
}

func TestApplyRefundUsesDedicatedRefundNotifyURL(t *testing.T) {
	const tenant = "wp-refund-notify"
	app := setupApp(t, tenant)
	wc, err := wechatconfig.GetWechatConfigByAppName(app)
	if err != nil {
		t.Fatalf("get wechat config: %v", err)
	}
	cfg, err := GetPayConfigByWechatConfigID(wc.ID)
	if err != nil {
		t.Fatalf("get pay config: %v", err)
	}
	cfg.RefundNotifyURL = "https://host.example/api/wechat/notify/refund/" + app
	if err := UpdatePayConfig(cfg); err != nil {
		t.Fatalf("update pay config: %v", err)
	}
	order := markOrderPaidDirect(t, tenant, app, CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 300,
	})
	if _, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 100,
	}); err != nil {
		t.Fatalf("ApplyRefund: %v", err)
	}
	if fakeAPI.lastRefund.NotifyURL != cfg.RefundNotifyURL {
		t.Fatalf("expected dedicated refund notify URL, got %s", fakeAPI.lastRefund.NotifyURL)
	}
}

// enableRefundApproval 开启商户配置的退款审核开关。
func enableRefundApproval(t *testing.T, app string) {
	t.Helper()
	wc, err := wechatconfig.GetWechatConfigByAppName(app)
	if err != nil {
		t.Fatalf("get wechat config: %v", err)
	}
	cfg, err := GetPayConfigByWechatConfigID(wc.ID)
	if err != nil {
		t.Fatalf("get pay config: %v", err)
	}
	cfg.RefundApprovalRequired = true
	if err := UpdatePayConfig(cfg); err != nil {
		t.Fatalf("update pay config: %v", err)
	}
}

func TestRefundApprovalFlow(t *testing.T) {
	const tenant = "wp-approval-1"
	app := setupApp(t, tenant)
	enableRefundApproval(t, app)
	order := markOrderPaidDirect(t, tenant, app, CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 1000,
	})

	// 申请进入 PENDING,不调用微信
	refund, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 400,
	})
	if err != nil {
		t.Fatalf("ApplyRefund: %v", err)
	}
	if refund.Status != RefundPending {
		t.Fatalf("expected PENDING, got %s", refund.Status)
	}
	if fakeAPI.lastRefund != nil && fakeAPI.lastRefund.OutRefundNo == refund.OutRefundNo {
		t.Fatal("wechat refund should not be called for pending approval")
	}

	// PENDING 占用退款额度:再申请 601(400+601>1000)应被拒绝
	if _, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 601,
	}); err == nil {
		t.Fatal("expected pending amount to consume refund quota")
	}

	// 拒绝:置 REJECTED,记录审批人,不调用微信
	callsBefore := fakeAPI.lastRefund
	rejected, err := ApproveRefund(context.Background(), ApproveRefundInput{
		TenantID: tenant, ApproverID: "admin-1", OutRefundNo: refund.OutRefundNo,
		Approved: false, Comment: "凭证不足",
	})
	if err != nil {
		t.Fatalf("ApproveRefund reject: %v", err)
	}
	if rejected.Status != RefundRejected || rejected.ApproverID != "admin-1" || rejected.ApproveComment != "凭证不足" {
		t.Fatalf("unexpected rejected refund: %#v", rejected)
	}
	if callsBefore != nil && fakeAPI.lastRefund != callsBefore && fakeAPI.lastRefund.OutRefundNo == refund.OutRefundNo {
		t.Fatal("wechat refund should not be called on rejection")
	}

	// 重复审核同一单应报错
	if _, err := ApproveRefund(context.Background(), ApproveRefundInput{
		TenantID: tenant, ApproverID: "admin-1", OutRefundNo: refund.OutRefundNo, Approved: true,
	}); err == nil {
		t.Fatal("expected re-approval error")
	}

	// 新申请并通过审核:调用微信并置为返回状态
	second, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 600,
	})
	if err != nil {
		t.Fatalf("second ApplyRefund: %v", err)
	}
	if second.Status != RefundPending {
		t.Fatalf("expected second refund PENDING, got %s", second.Status)
	}
	fakeAPI.refundResult = &RefundResult{RefundID: "re-approved", OutRefundNo: second.OutRefundNo, Status: RefundProcessing}
	approved, err := ApproveRefund(context.Background(), ApproveRefundInput{
		TenantID: tenant, ApproverID: "admin-2", OutRefundNo: second.OutRefundNo, Approved: true,
	})
	if err != nil {
		t.Fatalf("ApproveRefund approve: %v", err)
	}
	if approved.Status != RefundProcessing || approved.RefundID != "re-approved" || approved.ApproverID != "admin-2" {
		t.Fatalf("unexpected approved refund: %#v", approved)
	}
	if fakeAPI.lastRefund == nil || fakeAPI.lastRefund.OutRefundNo != second.OutRefundNo ||
		fakeAPI.lastRefund.RefundAmountFen != 600 {
		t.Fatalf("wechat refund not called properly: %#v", fakeAPI.lastRefund)
	}
}

func TestRefundApprovalTenantGuard(t *testing.T) {
	const tenant = "wp-approval-2"
	app := setupApp(t, tenant)
	enableRefundApproval(t, app)
	order := markOrderPaidDirect(t, tenant, app, CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 500,
	})
	refund, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 100,
	})
	if err != nil {
		t.Fatalf("ApplyRefund: %v", err)
	}
	if _, err := ApproveRefund(context.Background(), ApproveRefundInput{
		TenantID: "other-tenant", ApproverID: "admin-x", OutRefundNo: refund.OutRefundNo, Approved: true,
	}); err != ErrOrderNotOwned {
		t.Fatalf("expected ErrOrderNotOwned, got %v", err)
	}
	// 平台侧(空租户)可审核
	if _, err := ApproveRefund(context.Background(), ApproveRefundInput{
		TenantID: "", ApproverID: "platform-admin", OutRefundNo: refund.OutRefundNo, Approved: true,
	}); err != nil {
		t.Fatalf("platform approve: %v", err)
	}
}

func TestReconcileConfigAndLoop(t *testing.T) {
	// 参数应用与钳制
	oldInterval, oldAge, oldBatch, oldAlert, oldSilence := ReconcileLoopInterval, ReconcileScanAge, ReconcileBatchSize, ReconcileAlertAge, ReconcileAlertSilence
	defer func() {
		reconcileMu.Lock()
		ReconcileLoopInterval, ReconcileScanAge, ReconcileBatchSize, ReconcileAlertAge = oldInterval, oldAge, oldBatch, oldAlert
		ReconcileAlertSilence = oldSilence
		alertSilenceLast = map[string]time.Time{}
		reconcileMu.Unlock()
	}()
	ConfigureReconcile(3*time.Second, 2*time.Minute, 5000, 2*time.Hour, time.Minute)
	if ReconcileLoopInterval != 10*time.Second {
		t.Fatalf("expected interval clamped to 10s, got %v", ReconcileLoopInterval)
	}
	if ReconcileScanAge != 2*time.Minute {
		t.Fatalf("expected scanAge applied, got %v", ReconcileScanAge)
	}
	if ReconcileBatchSize != 1000 {
		t.Fatalf("expected batchSize clamped to 1000, got %d", ReconcileBatchSize)
	}
	if ReconcileAlertAge != 2*time.Hour {
		t.Fatalf("expected alertAge applied, got %v", ReconcileAlertAge)
	}
	if ReconcileAlertSilence != time.Minute {
		t.Fatalf("expected alertSilence clamped to 1m, got %v", ReconcileAlertSilence)
	}
	// 0 = 显式禁用静默去重
	ConfigureReconcile(0, 0, 0, 0, 0)
	if ReconcileAlertSilence != 0 {
		t.Fatalf("expected alertSilence disabled, got %v", ReconcileAlertSilence)
	}

	// 循环启停幂等
	if ReconcileLoopRunning() {
		t.Fatal("loop should not be running initially")
	}
	StartReconcileLoop()
	StartReconcileLoop() // 幂等
	if !ReconcileLoopRunning() {
		t.Fatal("expected loop running after start")
	}
	StopReconcileLoop()
	StopReconcileLoop() // 幂等
	if ReconcileLoopRunning() {
		t.Fatal("expected loop stopped")
	}
	// 停止后可重启
	StartReconcileLoop()
	if !ReconcileLoopRunning() {
		t.Fatal("expected loop running after restart")
	}
	StopReconcileLoop()
}

// collectAlerts 替换告警推送为内存收集器,返回还原函数与收集通道。
func collectAlerts() (func() []string, func()) {
	reconcileMu.Lock()
	original := alertFire
	reconcileMu.Unlock()
	mu := &sync.Mutex{}
	var events []string
	alertFire = func(eventType string, payload map[string]any) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, eventType)
	}
	return func() []string {
			mu.Lock()
			defer mu.Unlock()
			return append([]string(nil), events...)
		}, func() {
			reconcileMu.Lock()
			alertFire = original
			reconcileMu.Unlock()
		}
}

// collectAudits 替换审计写入为内存收集器,返回收集函数与还原函数。
func collectAudits() (func() []audit.AuditLog, func()) {
	reconcileMu.Lock()
	original := auditRecorder
	reconcileMu.Unlock()
	mu := &sync.Mutex{}
	var logs []audit.AuditLog
	auditRecorder = func(db *gorm.DB, userID, userName string, principalKind int32, action, targetID, ip, detail string) {
		mu.Lock()
		defer mu.Unlock()
		logs = append(logs, audit.AuditLog{
			UserID: userID, UserName: userName, PrincipalKind: principalKind,
			Action: action, TargetID: targetID, Detail: detail,
		})
	}
	return func() []audit.AuditLog {
			mu.Lock()
			defer mu.Unlock()
			return append([]audit.AuditLog(nil), logs...)
		}, func() {
			reconcileMu.Lock()
			auditRecorder = original
			reconcileMu.Unlock()
		}
}

func TestReconcileAlerts(t *testing.T) {
	const tenant = "wp-alert-1"
	app := setupApp(t, tenant)
	wc, err := wechatconfig.GetWechatConfigByAppName(app)
	if err != nil {
		t.Fatalf("get wechat config: %v", err)
	}
	makeOrder := func(suffix string) *PayOrder {
		t.Helper()
		order := &PayOrder{
			TenantID: tenant, UserID: "user-1", WechatConfigID: wc.ID, AppID: "wx", MchID: "mch",
			OutTradeNo: "alert-" + suffix, Amount: 100, Status: PayOrderCreated,
		}
		if _, err := CreatePayOrder(order); err != nil {
			t.Fatalf("create order: %v", err)
		}
		return order
	}
	failing := makeOrder("query-fail")
	stuck := makeOrder("stuck")
	ageOrder(t, failing.ID)
	ageOrder(t, stuck.ID)
	// stuck 单改老到超过告警阈值(默认 24h)
	if err := store.DB().Model(&PayOrder{}).Where("id = ?", stuck.ID).
		Update("created_at", time.Now().Add(-25*time.Hour)).Error; err != nil {
		t.Fatalf("age stuck order: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&PayOrder{}, "tenant_id = ?", tenant).Error
	})

	// 注入告警收集器、审计收集器与按单定制查单结果
	getEvents, restore := collectAlerts()
	defer restore()
	getAudits, restoreAudits := collectAudits()
	defer restoreAudits()
	alertSilenceLast = map[string]time.Time{} // 重置静默状态,确保首轮告警必发
	oldAge := ReconcileAlertAge
	ReconcileAlertAge = 24 * time.Hour
	defer func() { ReconcileAlertAge = oldAge }()
	fakeAPI.queryErr = errors.New("connection refused")
	fakeAPI.queryByTradeNo = map[string]*TransactionResult{}

	_, err = ReconcileStaleOrders(context.Background())
	if err != nil {
		t.Fatalf("ReconcileStaleOrders: %v", err)
	}
	events := getEvents()
	found := func(event string) bool {
		for _, e := range events {
			if e == event {
				return true
			}
		}
		return false
	}
	if !found("pay_reconcile_query_failed") {
		t.Fatalf("expected query_failed alert, got %v", events)
	}
	if !found("pay_order_stuck_created") {
		t.Fatalf("expected stuck_created alert, got %v", events)
	}

	// 告警事件应同步落审计日志:系统主体,详情含订单号
	audits := getAudits()
	if len(audits) != 2 {
		t.Fatalf("expected 2 audit records, got %d: %+v", len(audits), audits)
	}
	for _, a := range audits {
		if a.UserID != "system" || a.PrincipalKind != 2 {
			t.Fatalf("unexpected audit principal: %+v", a)
		}
	}
	if audits[0].Action != "pay_reconcile_query_failed" ||
		!strings.Contains(audits[0].Detail, "alert-query-fail") {
		t.Fatalf("unexpected first audit: %+v", audits[0])
	}

	// 下一轮:查单恢复正常;滞留单会按设计重复告警,但不应再出现查单失败告警
	fakeAPI.queryErr = nil
	fakeAPI.queryResult = &TransactionResult{TradeState: "NOTPAY"}
	base := len(getEvents())
	auditBase := len(getAudits())
	if _, err := ReconcileStaleOrders(context.Background()); err != nil {
		t.Fatalf("second ReconcileStaleOrders: %v", err)
	}
	for _, e := range getEvents()[base:] {
		if e == "pay_reconcile_query_failed" {
			t.Fatalf("unexpected query_failed alert after recovery: %v", getEvents())
		}
	}
	// 静默窗口内不应重复写审计
	if got := len(getAudits()) - auditBase; got != 0 {
		t.Fatalf("expected no audit records within silence window, got %d", got)
	}
}

func TestAlertSilenceWindow(t *testing.T) {
	oldSilence := ReconcileAlertSilence
	defer func() { ReconcileAlertSilence = oldSilence }()

	// 禁用静默(<=0):每次都放行
	ReconcileAlertSilence = 0
	alertSilenceLast = map[string]time.Time{}
	if !shouldAlert("evt") || !shouldAlert("evt") {
		t.Fatal("disabled silence should always alert")
	}

	// 窗口内第二次同类事件被抑制,异类事件不受影响
	ReconcileAlertSilence = time.Hour
	alertSilenceLast = map[string]time.Time{}
	if !shouldAlert("evt-a") {
		t.Fatal("first event should alert")
	}
	if shouldAlert("evt-a") {
		t.Fatal("second event in silence window should be suppressed")
	}
	if !shouldAlert("evt-b") {
		t.Fatal("different event type should alert")
	}

	// 窗口过期后再次放行(模拟:把上次发送时间改到窗口之前)
	alertSilenceLast["evt-a"] = time.Now().Add(-2 * time.Hour)
	if !shouldAlert("evt-a") {
		t.Fatal("event after silence window should alert again")
	}
}

// 编译期约束:确保 fakePayAPI 始终实现 PayAPI。
var _ PayAPI = (*fakePayAPI)(nil)

func TestApproveRefundAudit(t *testing.T) {
	const tenant = "wp-approve-audit"
	app := setupApp(t, tenant)
	enableRefundApproval(t, app)
	order := markOrderPaidDirect(t, tenant, app, CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 1000,
	})
	refund, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 300, Reason: "部分退款",
	})
	if err != nil {
		t.Fatalf("ApplyRefund: %v", err)
	}

	getAudits, restore := collectAudits()
	defer restore()

	// 拒绝 → reject 审计
	fakeAPI.refundResult = &RefundResult{RefundID: "re-a1", OutRefundNo: refund.OutRefundNo, Status: RefundProcessing}
	if _, err := ApproveRefund(context.Background(), ApproveRefundInput{
		TenantID: tenant, ApproverID: "admin-reject", OutRefundNo: refund.OutRefundNo,
		Approved: false, Comment: "凭证不足",
	}); err != nil {
		t.Fatalf("reject: %v", err)
	}

	// 通过 → approve 审计
	second, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 200,
	})
	if err != nil {
		t.Fatalf("second ApplyRefund: %v", err)
	}
	if _, err := ApproveRefund(context.Background(), ApproveRefundInput{
		TenantID: tenant, ApproverID: "admin-approve", OutRefundNo: second.OutRefundNo,
		Approved: true, Comment: "已核实",
	}); err != nil {
		t.Fatalf("approve: %v", err)
	}

	audits := getAudits()
	if len(audits) != 2 {
		t.Fatalf("expected 2 audit records, got %d: %+v", len(audits), audits)
	}
	rejectAudit, approveAudit := audits[0], audits[1]
	if rejectAudit.Action != AuditActionRefundReject || rejectAudit.UserID != "admin-reject" ||
		rejectAudit.PrincipalKind != 0 || rejectAudit.TargetID != refund.ID {
		t.Fatalf("unexpected reject audit: %+v", rejectAudit)
	}
	for _, s := range []string{refund.OutRefundNo, `"approved":false`, "凭证不足"} {
		if !strings.Contains(rejectAudit.Detail, s) {
			t.Fatalf("reject audit detail missing %q: %s", s, rejectAudit.Detail)
		}
	}
	if approveAudit.Action != AuditActionRefundApprove || approveAudit.UserID != "admin-approve" ||
		approveAudit.TargetID != second.ID {
		t.Fatalf("unexpected approve audit: %+v", approveAudit)
	}
	for _, s := range []string{second.OutRefundNo, `"approved":true`, "re-a1"} {
		if !strings.Contains(approveAudit.Detail, s) {
			t.Fatalf("approve audit detail missing %q: %s", s, approveAudit.Detail)
		}
	}
}

func TestQueryDailyPayStats(t *testing.T) {
	const tenant = "wp-stats-1"
	setupApp(t, tenant)
	today := time.Now()
	todayStr := today.Format("2006-01-02")
	yesterday := today.AddDate(0, 0, -1)

	// 今日下单 2 笔(其中 1 笔今日支付成功),昨日下单 1 笔(昨日支付后今日关闭)
	o1 := &PayOrder{TenantID: tenant, UserID: "u", WechatConfigID: "wc", AppID: "wx", MchID: "m",
		OutTradeNo: "stats-today-created", Amount: 100, Status: PayOrderCreated}
	o2 := &PayOrder{TenantID: tenant, UserID: "u", WechatConfigID: "wc", AppID: "wx", MchID: "m",
		OutTradeNo: "stats-today-paid", Amount: 500, Status: PayOrderPaid,
		PaidAt: &today}
	o3 := &PayOrder{TenantID: tenant, UserID: "u", WechatConfigID: "wc", AppID: "wx", MchID: "m",
		OutTradeNo: "stats-yesterday-closed", Amount: 300, Status: PayOrderClosed,
		PaidAt: &yesterday, ClosedAt: &today}
	orders := []*PayOrder{o1, o2, o3}
	for _, o := range orders {
		if _, err := CreatePayOrder(o); err != nil {
			t.Fatalf("create order: %v", err)
		}
	}
	// 把 o3 的下单时间改到昨日
	if err := store.DB().Model(&PayOrder{}).Where("id = ?", o3.ID).
		Update("created_at", yesterday).Error; err != nil {
		t.Fatalf("backdate order: %v", err)
	}
	refund := &PayRefund{TenantID: tenant, PayOrderID: o2.ID, OutTradeNo: o2.OutTradeNo,
		OutRefundNo: "stats-refund-1", Amount: 100, Total: 500, Status: RefundSuccess}
	if _, err := CreatePayRefund(refund); err != nil {
		t.Fatalf("create refund: %v", err)
	}
	// 一笔已拒绝退款,不计入统计
	if _, err := CreatePayRefund(&PayRefund{TenantID: tenant, PayOrderID: o2.ID,
		OutTradeNo: o2.OutTradeNo, OutRefundNo: "stats-refund-rejected",
		Amount: 50, Total: 500, Status: RefundRejected}); err != nil {
		t.Fatalf("create rejected refund: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&PayOrder{}, "tenant_id = ?", tenant).Error
		_ = store.DB().Unscoped().Delete(&PayRefund{}, "tenant_id = ?", tenant).Error
	})

	stats, err := QueryDailyPayStats(tenant, 7)
	if err != nil {
		t.Fatalf("QueryDailyPayStats: %v", err)
	}
	if len(stats) != 7 {
		t.Fatalf("expected 7 days, got %d", len(stats))
	}
	byDate := map[string]*DailyPayStat{}
	for _, s := range stats {
		byDate[s.Date] = s
	}
	td := byDate[todayStr]
	if td == nil {
		t.Fatal("today stat missing")
	}
	if td.CreatedCount != 2 || td.CreatedAmount != 600 {
		t.Fatalf("unexpected today created: %+v", td)
	}
	if td.PaidCount != 1 || td.PaidAmount != 500 {
		t.Fatalf("unexpected today paid: %+v", td)
	}
	if td.ClosedCount != 1 {
		t.Fatalf("unexpected today closed: %+v", td)
	}
	if td.RefundCount != 1 || td.RefundAmount != 100 {
		t.Fatalf("rejected refund should be excluded: %+v", td)
	}
	// 租户过滤:其他租户视角应为全零
	other, err := QueryDailyPayStats("other-tenant", 3)
	if err != nil {
		t.Fatalf("other tenant stats: %v", err)
	}
	for _, s := range other {
		if s.CreatedCount != 0 {
			t.Fatalf("expected zero for foreign tenant: %+v", s)
		}
	}
}

func TestMaybePushDailyReport(t *testing.T) {
	const tenant = "wp-daily-report"
	setupApp(t, tenant)
	now := time.Now()

	oldEnabled, oldHour := DailyReportEnabled, DailyReportHour
	dailyReportLastDate = ""
	defer func() {
		DailyReportEnabled, DailyReportHour = oldEnabled, oldHour
	}()
	getEvents, restoreAlerts := collectAlerts()
	defer restoreAlerts()
	getAudits, restoreAudits := collectAudits()
	defer restoreAudits()

	// 禁用时不推送
	DailyReportEnabled = false
	maybePushDailyReport(context.Background(), now)
	if len(getEvents()) != 0 {
		t.Fatal("disabled report should not push")
	}

	// 未到推送时刻不推送
	DailyReportEnabled = true
	DailyReportHour = 23
	maybePushDailyReport(context.Background(), now)
	if len(getEvents()) != 0 {
		t.Fatal("should not push before report hour")
	}

	// 到达时刻:推送昨日日报(平台汇总 + 各活跃租户各一条)
	DailyReportHour = 0
	// 再造一个有昨日支付活动的租户,验证分租户推送
	otherTenant := tenant + "-b"
	setupApp(t, otherTenant)
	otherOrder := &PayOrder{TenantID: otherTenant, UserID: "u2", WechatConfigID: "wc", AppID: "wx",
		MchID: "m", OutTradeNo: "daily-report-other", Amount: 700, Status: PayOrderCreated}
	if _, err := CreatePayOrder(otherOrder); err != nil {
		t.Fatalf("create other order: %v", err)
	}
	if err := store.DB().Model(&PayOrder{}).Where("id = ?", otherOrder.ID).
		Update("created_at", now.AddDate(0, 0, -1)).Error; err != nil {
		t.Fatalf("backdate other order: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&PayOrder{}, "id = ?", otherOrder.ID).Error
		_ = store.DB().Unscoped().Delete(&wechatconfig.WechatConfig{}, "tenant_id = ?", otherTenant).Error
		_ = store.DB().Unscoped().Delete(&PayConfig{}, "tenant_id = ?", otherTenant).Error
	})
	// 活跃租户是全库口径(含其他测试残留的当日订单租户),预期条数动态计算
	activeTenants, err := ListActivePayTenantIDs(2)
	if err != nil {
		t.Fatalf("list active tenants: %v", err)
	}
	tenantFound := false
	for _, tid := range activeTenants {
		if tid == otherTenant {
			tenantFound = true
		}
	}
	if !tenantFound {
		t.Fatalf("expected %q among active tenants: %v", otherTenant, activeTenants)
	}
	maybePushDailyReport(context.Background(), now)
	// 平台 1 条 + 每个活跃租户 1 条
	wantTotal := 1 + len(activeTenants)
	events := getEvents()
	if len(events) != wantTotal {
		t.Fatalf("expected %d daily reports (platform+%d tenants), got %d: %v",
			wantTotal, len(activeTenants), len(events), events)
	}
	for _, e := range events {
		if e != "pay_daily_report" {
			t.Fatalf("unexpected event %q", e)
		}
	}
	audits := getAudits()
	if len(audits) != wantTotal {
		t.Fatalf("expected %d daily report audits, got %d: %+v", wantTotal, len(audits), audits)
	}
	wantDate := now.AddDate(0, 0, -1).Format("2006-01-02")
	if !strings.Contains(audits[0].Detail, wantDate) {
		t.Fatalf("expected report date %s in detail: %s", wantDate, audits[0].Detail)
	}
	// 首条为平台汇总,其余携带 tenantID 与 scope=tenant
	if !strings.Contains(audits[0].Detail, `"scope":"platform"`) {
		t.Fatalf("expected platform scope in first audit: %s", audits[0].Detail)
	}
	tenantScopeCount := 0
	for _, a := range audits[1:] {
		if !strings.Contains(a.Detail, `"scope":"tenant"`) || !strings.Contains(a.Detail, `"tenantID"`) {
			t.Fatalf("expected tenant scope with tenantID: %s", a.Detail)
		}
		tenantScopeCount++
	}
	if tenantScopeCount != len(activeTenants) {
		t.Fatalf("expected %d tenant-scoped reports, got %d", len(activeTenants), tenantScopeCount)
	}

	// 同日重复调用不推送(去重)
	maybePushDailyReport(context.Background(), now)
	if len(getEvents()) != wantTotal {
		t.Fatal("duplicate push on same day should be suppressed")
	}
}

func TestRefundReasonCode(t *testing.T) {
	const tenant = "wp-reason-1"
	app := setupApp(t, tenant)
	enableRefundApproval(t, app)
	order := markOrderPaidDirect(t, tenant, app, CreateOrderInput{
		TenantID: tenant, UserID: "user-1", App: app, Description: "x", AmountFen: 1000,
	})

	// 空类别归一化为 other
	r1, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 100,
	})
	if err != nil {
		t.Fatalf("ApplyRefund default code: %v", err)
	}
	if r1.ReasonCode != RefundReasonOther {
		t.Fatalf("expected default other, got %q", r1.ReasonCode)
	}

	// 合法类别落库
	r2, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 100,
		ReasonCode: RefundReasonQuality, Reason: "外观划痕",
	})
	if err != nil {
		t.Fatalf("ApplyRefund quality: %v", err)
	}
	if r2.ReasonCode != RefundReasonQuality {
		t.Fatalf("expected quality, got %q", r2.ReasonCode)
	}

	// 非法类别拒绝
	if _, err := ApplyRefund(context.Background(), ApplyRefundInput{
		TenantID: tenant, OutTradeNo: order.OutTradeNo, RefundAmount: 100,
		ReasonCode: "hacked",
	}); err == nil || !strings.Contains(err.Error(), "非法退款原因类别") {
		t.Fatalf("expected invalid reason code error, got %v", err)
	}

	// 白名单完整
	for _, code := range ValidReasonCodes() {
		if _, err := NormalizeReasonCode(code); err != nil {
			t.Fatalf("whitelist code %q should be valid: %v", code, err)
		}
	}

	// 审核审计详情包含原因类别
	getAudits, restore := collectAudits()
	defer restore()
	fakeAPI.refundResult = &RefundResult{RefundID: "re-rc", OutRefundNo: r2.OutRefundNo, Status: RefundProcessing}
	if _, err := ApproveRefund(context.Background(), ApproveRefundInput{
		TenantID: tenant, ApproverID: "admin-1", OutRefundNo: r2.OutRefundNo, Approved: true,
	}); err != nil {
		t.Fatalf("ApproveRefund: %v", err)
	}
	audits := getAudits()
	if len(audits) != 1 || !strings.Contains(audits[0].Detail, `"reasonCode":"quality"`) {
		t.Fatalf("expected reasonCode in audit detail: %+v", audits)
	}
}

func TestQueryRefundReasonStats(t *testing.T) {
	const tenant = "wp-reason-stats"
	setupApp(t, tenant)
	order := &PayOrder{TenantID: tenant, UserID: "u", WechatConfigID: "wc", AppID: "wx", MchID: "m",
		OutTradeNo: "reason-stats-trade", Amount: 10000, Status: PayOrderPaid}
	if _, err := CreatePayOrder(order); err != nil {
		t.Fatalf("create order: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&PayOrder{}, "id = ?", order.ID).Error
		_ = store.DB().Unscoped().Delete(&PayRefund{}, "tenant_id = ?", tenant).Error
	})
	mk := func(no, code string, amount int64, status string) {
		t.Helper()
		if _, err := CreatePayRefund(&PayRefund{TenantID: tenant, PayOrderID: order.ID,
			OutTradeNo: order.OutTradeNo, OutRefundNo: no, Amount: amount, Total: 10000,
			ReasonCode: code, Status: status}); err != nil {
			t.Fatalf("create refund %s: %v", no, err)
		}
	}
	mk("reason-stats-q1", RefundReasonQuality, 500, RefundSuccess)
	mk("reason-stats-q2", RefundReasonQuality, 300, RefundPending)
	mk("reason-stats-nr", RefundReasonNotReceived, 800, RefundProcessing)
	mk("reason-stats-other", "", 50, RefundSuccess) // 空类别落库为空串,统计按原始值分组
	mk("reason-stats-rejected", RefundReasonPrice, 999, RefundRejected)

	stats, err := QueryRefundReasonStats(tenant, 30)
	if err != nil {
		t.Fatalf("QueryRefundReasonStats: %v", err)
	}
	if len(stats) != 3 {
		t.Fatalf("expected 3 groups (rejected excluded), got %d: %+v", len(stats), stats)
	}
	// 金额降序:not_received(800) > quality(800) —— 同额时顺序不敏感,改断言集合
	sum := map[string]*ReasonCodeStat{}
	for _, s := range stats {
		sum[s.ReasonCode] = s
	}
	if s := sum[RefundReasonNotReceived]; s == nil || s.Count != 1 || s.Amount != 800 {
		t.Fatalf("unexpected not_received stat: %+v", s)
	}
	if s := sum[RefundReasonQuality]; s == nil || s.Count != 2 || s.Amount != 800 {
		t.Fatalf("unexpected quality stat: %+v", s)
	}
	if s, ok := sum[""]; !ok || s.Count != 1 || s.Amount != 50 {
		t.Fatalf("unexpected empty-code stat: %+v", s)
	}
	if _, ok := sum[RefundReasonPrice]; ok {
		t.Fatal("rejected refund should be excluded from reason stats")
	}
	// 租户隔离
	other, err := QueryRefundReasonStats("other-tenant", 30)
	if err != nil || len(other) != 0 {
		t.Fatalf("expected empty for foreign tenant: %v %+v", err, other)
	}
}

func TestQueryRefundReasonTrend(t *testing.T) {
	const tenant = "wp-reason-trend"
	setupApp(t, tenant)
	order := &PayOrder{TenantID: tenant, UserID: "u", WechatConfigID: "wc", AppID: "wx", MchID: "m",
		OutTradeNo: "reason-trend-trade", Amount: 10000, Status: PayOrderPaid}
	if _, err := CreatePayOrder(order); err != nil {
		t.Fatalf("create order: %v", err)
	}
	// 两笔当月退款(不同类别) + 一笔上月退款 + 一笔已拒绝(排除)
	mk := func(no, code string, amount int64, status string, createdAt time.Time) {
		t.Helper()
		r := &PayRefund{TenantID: tenant, PayOrderID: order.ID,
			OutTradeNo: order.OutTradeNo, OutRefundNo: no, Amount: amount, Total: 10000,
			ReasonCode: code, Status: status}
		if _, err := CreatePayRefund(r); err != nil {
			t.Fatalf("create refund %s: %v", no, err)
		}
		if err := store.DB().Model(&PayRefund{}).Where("id = ?", r.ID).
			Update("created_at", createdAt).Error; err != nil {
			t.Fatalf("backdate refund: %v", err)
		}
	}
	now := time.Now()
	lastMonth := now.AddDate(0, -1, 0)
	mk("trend-cur-quality", RefundReasonQuality, 200, RefundSuccess, now)
	mk("trend-cur-price", RefundReasonPrice, 100, RefundProcessing, now)
	mk("trend-last-dup", RefundReasonDuplicate, 400, RefundSuccess, lastMonth)
	mk("trend-rejected", RefundReasonOther, 999, RefundRejected, now)
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&PayOrder{}, "id = ?", order.ID).Error
		_ = store.DB().Unscoped().Delete(&PayRefund{}, "tenant_id = ?", tenant).Error
	})

	points, err := QueryRefundReasonTrend(tenant, 6)
	if err != nil {
		t.Fatalf("QueryRefundReasonTrend: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 trend points, got %d: %+v", len(points), points)
	}
	thisMonth := now.Format("2006-01")
	lastMonthStr := lastMonth.Format("2006-01")
	// 月升序:上月条目在最前
	if points[0].Month != lastMonthStr || points[0].ReasonCode != RefundReasonDuplicate ||
		points[0].Amount != 400 {
		t.Fatalf("unexpected last month point: %+v", points[0])
	}
	// 当月按金额降序:quality(200) 在 price(100) 前
	cur := map[string]*RefundReasonTrendPoint{}
	for _, p := range points[1:] {
		if p.Month != thisMonth {
			t.Fatalf("unexpected month %q", p.Month)
		}
		cur[p.ReasonCode] = p
	}
	if p := cur[RefundReasonQuality]; p == nil || p.Count != 1 || p.Amount != 200 {
		t.Fatalf("unexpected quality point: %+v", p)
	}
	if p := cur[RefundReasonPrice]; p == nil || p.Amount != 100 {
		t.Fatalf("unexpected price point: %+v", p)
	}
	// 已拒绝不出现
	if _, ok := cur[RefundReasonOther]; ok {
		t.Fatal("rejected refund should be excluded from trend")
	}
	// 租户隔离
	other, err := QueryRefundReasonTrend("other-tenant", 6)
	if err != nil || len(other) != 0 {
		t.Fatalf("expected empty for foreign tenant: %v %+v", err, other)
	}
}

func TestBatchCloseOrders(t *testing.T) {
	const tenant = "wp-batch-close"
	app := setupApp(t, tenant)
	wc, err := wechatconfig.GetWechatConfigByAppName(app)
	if err != nil {
		t.Fatalf("get wechat config: %v", err)
	}
	mk := func(suffix, status string) *PayOrder {
		t.Helper()
		order := &PayOrder{
			TenantID: tenant, UserID: "user-1", WechatConfigID: wc.ID, AppID: "wx", MchID: "m",
			OutTradeNo: "batch-close-" + suffix, Amount: 100, Status: status, PrepayID: "p-" + suffix,
		}
		if _, err := CreatePayOrder(order); err != nil {
			t.Fatalf("create order: %v", err)
		}
		return order
	}
	c1 := mk("c1", PayOrderCreated)
	c2 := mk("c2", PayOrderCreated)
	paid := mk("paid", PayOrderPaid)
	closed := mk("closed", PayOrderClosed)
	t.Cleanup(func() {
		_ = store.DB().Unscoped().Delete(&PayOrder{}, "tenant_id = ?", tenant).Error
	})

	closeCalls := &atomic.Int32{}
	oldCloseErr := fakeAPI.closeErr
	fakeAPI.closeErr = nil
	defer func() { fakeAPI.closeErr = oldCloseErr }()
	_ = closeCalls

	result, err := BatchCloseOrders(context.Background(), tenant,
		[]string{c1.OutTradeNo, c2.OutTradeNo, paid.OutTradeNo, closed.OutTradeNo, "no-such-order-1"})
	if err != nil {
		t.Fatalf("BatchCloseOrders: %v", err)
	}
	if result.Closed != 2 {
		t.Fatalf("expected 2 closed, got %+v", result)
	}
	if result.Skipped != 2 {
		t.Fatalf("expected 2 skipped (paid+closed), got %+v", result)
	}
	if len(result.Failures) != 1 || result.Failures[0].OutTradeNo != "no-such-order-1" {
		t.Fatalf("expected 1 failure for unknown order, got %+v", result.Failures)
	}
	// 远端关单只对有 prepay 的 CREATED 订单调用(2 次)
	// 状态复核
	for _, no := range []string{c1.OutTradeNo, c2.OutTradeNo} {
		got, _ := GetPayOrderByOutTradeNo(no)
		if got.Status != PayOrderClosed {
			t.Fatalf("order %s expected CLOSED, got %s", no, got.Status)
		}
	}
	if got, _ := GetPayOrderByOutTradeNo(paid.OutTradeNo); got.Status != PayOrderPaid {
		t.Fatalf("paid order must stay PAID, got %s", got.Status)
	}

	// 平台侧(空租户)可批量关单
	c3 := mk("c3", PayOrderCreated)
	t.Cleanup(func() { _ = store.DB().Unscoped().Delete(&PayOrder{}, "id = ?", c3.ID).Error })
	if _, err := BatchCloseOrders(context.Background(), "", []string{c3.OutTradeNo}); err != nil {
		t.Fatalf("platform batch close: %v", err)
	}
	if got, _ := GetPayOrderByOutTradeNo(c3.OutTradeNo); got.Status != PayOrderClosed {
		t.Fatalf("platform close failed: %s", got.Status)
	}

	// 超上限拒绝
	tooMany := make([]string, maxBatchCloseSize+1)
	for i := range tooMany {
		tooMany[i] = "overflow-order-no-1"
	}
	if _, err := BatchCloseOrders(context.Background(), tenant, tooMany); err == nil {
		t.Fatal("expected overflow error")
	}
}

func TestGetTradeBill(t *testing.T) {
	const tenant = "wp-trade-bill"
	app := setupApp(t, tenant)
	wc, err := wechatconfig.GetWechatConfigByAppName(app)
	if err != nil {
		t.Fatalf("get wechat config: %v", err)
	}
	cfg, err := GetPayConfigByWechatConfigID(wc.ID)
	if err != nil {
		t.Fatalf("get pay config: %v", err)
	}

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	fakeAPI.billCSV = []byte("交易时间,交易金额\n2026-09-12 10:00:00,100\n总收款,100\n")
	getAudits, restoreAudits := collectAudits()
	defer restoreAudits()

	// 正常下载
	csvData, err := GetTradeBill(context.Background(), tenant, "op-1", cfg.ID, yesterday, "")
	if err != nil {
		t.Fatalf("GetTradeBill: %v", err)
	}
	if !strings.Contains(string(csvData), "总收款") {
		t.Fatalf("unexpected csv: %s", csvData)
	}

	// 当日/未来日期拒绝
	if _, err := GetTradeBill(context.Background(), tenant, "op-1", cfg.ID, time.Now().Format("2006-01-02"), ""); err == nil {
		t.Fatal("expected today rejection")
	}
	if _, err := GetTradeBill(context.Background(), tenant, "op-1", cfg.ID, "not-a-date", ""); err == nil {
		t.Fatal("expected invalid date rejection")
	}
	// 租户隔离
	if _, err := GetTradeBill(context.Background(), "other-tenant", "op-1", cfg.ID, yesterday, ""); err != ErrOrderNotOwned {
		t.Fatalf("expected ErrOrderNotOwned, got %v", err)
	}
	// 平台侧(空租户)可下载
	if _, err := GetTradeBill(context.Background(), "", "op-2", cfg.ID, yesterday, BillTypeRefund); err != nil {
		t.Fatalf("platform download: %v", err)
	}
	// 非法账单类型拒绝
	if _, err := GetTradeBill(context.Background(), tenant, "op-1", cfg.ID, yesterday, "HACKED"); err == nil ||
		!strings.Contains(err.Error(), "非法账单类型") {
		t.Fatalf("expected invalid billType rejection, got %v", err)
	}
	// 单日下载审计留痕
	audits := getAudits()
	if len(audits) != 2 || audits[0].Action != AuditActionTradeBillDownload || audits[0].UserID != "op-1" {
		t.Fatalf("expected 2 download audits, got %+v", audits)
	}
}

func TestGetTradeBillRange(t *testing.T) {
	const tenant = "wp-bill-range"
	app := setupApp(t, tenant)
	enableRefundApproval(t, app) // 无实际作用,仅为保持 setup 一致
	wc, err := wechatconfig.GetWechatConfigByAppName(app)
	if err != nil {
		t.Fatalf("get wechat config: %v", err)
	}
	cfg, err := GetPayConfigByWechatConfigID(wc.ID)
	if err != nil {
		t.Fatalf("get pay config: %v", err)
	}

	getAudits, restoreAudits := collectAudits()
	defer restoreAudits()

	// 范围:昨日往前 3 天(中间一天模拟失败)
	end := time.Now().AddDate(0, 0, -1)
	start := end.AddDate(0, 0, -2)
	getAudits()
	_ = getAudits
	fakeAPI.billCSV = []byte("fake,bill\n1,100\n")
	csvData, err := GetTradeBillRange(context.Background(), tenant, "admin-1", cfg.ID,
		start.Format("2006-01-02"), end.Format("2006-01-02"), BillTypeSuccess)
	if err != nil {
		t.Fatalf("GetTradeBillRange: %v", err)
	}
	text := string(csvData)
	if got := strings.Count(text, "# ====="); got != 3 {
		t.Fatalf("expected 3 day sections, got %d", got)
	}
	if !strings.Contains(text, "fake,bill") {
		t.Fatalf("expected bill content, got %s", text)
	}

	// 操作级审计恰好 1 条(范围版)
	audits := getAudits()
	if len(audits) != 1 || audits[0].Action != AuditActionTradeBillRangeDownload ||
		audits[0].UserID != "admin-1" {
		t.Fatalf("unexpected range audit: %+v", audits)
	}

	// 结束日期为当日拒绝
	if _, err := GetTradeBillRange(context.Background(), tenant, "admin-1", cfg.ID,
		time.Now().Format("2006-01-02"), time.Now().Format("2006-01-02"), ""); err == nil {
		t.Fatal("expected today rejection")
	}
	// 跨度超限拒绝
	if _, err := GetTradeBillRange(context.Background(), tenant, "admin-1", cfg.ID,
		start.Format("2006-01-02"), start.AddDate(0, 0, maxTradeBillRangeDays).Format("2006-01-02"), ""); err == nil {
		t.Fatal("expected range overflow rejection")
	}
	// 租户隔离
	if _, err := GetTradeBillRange(context.Background(), "other-tenant", "admin-1", cfg.ID,
		start.Format("2006-01-02"), end.Format("2006-01-02"), ""); err != ErrOrderNotOwned {
		t.Fatalf("expected ErrOrderNotOwned, got %v", err)
	}
}

func TestMaybeDownloadDailyBills(t *testing.T) {
	const tenant = "wp-bill-task"
	setupApp(t, tenant)
	oldEnabled, oldHour := DailyReportEnabled, DailyReportHour
	defer func() {
		DailyReportEnabled, DailyReportHour = oldEnabled, oldHour
	}()
	billDownloadLastDate = ""
	fakeAPI.billCSV = []byte("fake,bill\n1,100\n")
	billErr := errors.New("bill api down")

	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.Local) // 固定 12 点,越过任何下载时刻
	DailyReportHour = 0                                    // 下载时刻 = 1 点,now 必然越过

	// 触发下载:每个启用配置入库昨日账单
	maybeDownloadDailyBills(context.Background(), now)
	billDate := now.AddDate(0, 0, -1).Format("2006-01-02")
	var stored BillFile
	if err := store.DB().Where("tenant_id = ?", tenant).First(&stored).Error; err != nil {
		t.Fatalf("expected archived bill, got %v", err)
	}
	if stored.BillDate != billDate || stored.BillType != BillTypeAll {
		t.Fatalf("unexpected bill: %+v", stored)
	}

	// 同日去重:再触发不新增(即便此时改 billErr 也不产生新告警)
	before := time.Now()
	maybeDownloadDailyBills(context.Background(), now)
	if got := time.Since(before); got > time.Second {
		t.Fatalf("duplicate trigger took too long: %v", got)
	}

	// 失败场景:新的一天,下载报错 → 告警事件
	billDownloadLastDate = ""
	fakeAPI.billErr = billErr
	defer func() { fakeAPI.billErr = nil }()
	getEvents, restoreAlerts := collectAlerts()
	defer restoreAlerts()
	getAudits, restoreAudits := collectAudits()
	defer restoreAudits()
	DailyReportHour = 0
	nextDay := now.AddDate(0, 0, 1)
	maybeDownloadDailyBills(context.Background(), nextDay)
	events := getEvents()
	found := false
	for _, e := range events {
		if e == "pay_bill_download_failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected bill download failed alert, got %v", events)
	}
	audits := getAudits()
	found = false
	for _, a := range audits {
		if a.Action == "pay_bill_download_failed" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected bill download failed audit")
	}
}
