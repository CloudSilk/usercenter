package wechatpay

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CloudSilk/pkg/db"
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

var fakeAPI *fakePayAPI

func TestMain(m *testing.M) {
	gdb, err := gorm.Open(glebsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := gdb.AutoMigrate(&wechatconfig.WechatConfig{}, &PayConfig{}, &PayOrder{}, &PayRefund{}); err != nil {
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

// 编译期约束:确保 fakePayAPI 始终实现 PayAPI。
var _ PayAPI = (*fakePayAPI)(nil)
