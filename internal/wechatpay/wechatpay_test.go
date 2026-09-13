package wechatpay

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
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
	prepayCalls  int
	prepayParams *PayParams
	prepayErr    error
	lastPrepay   *PrepayInput
	queryResult  *TransactionResult
	queryErr     error
	closeCalls   int
	closeErr     error
	notify       *NotifyContent
	notifyErr    error
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
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.queryResult, nil
}

func (f *fakePayAPI) Close(ctx context.Context, mchID, outTradeNo string) error {
	f.closeCalls++
	return f.closeErr
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
	if err := gdb.AutoMigrate(&wechatconfig.WechatConfig{}, &PayConfig{}, &PayOrder{}); err != nil {
		panic(err)
	}
	store.SetDB(db.NewDBClient(gdb, false))
	fakeAPI = &fakePayAPI{prepayParams: &PayParams{
		AppID: "wx123", TimeStamp: "1700000000", NonceStr: "nonce",
		Package: "prepay_id=wx1", SignType: "RSA", PaySign: "sig", PrepayID: "wx1",
	}}
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
		EventType: "REFUND.SUCCESS", OutTradeNo: order.OutTradeNo,
		TradeState: "REFUND", TradeStateDesc: "退款成功",
	}
	req := httptest.NewRequest("POST", "/api/wechat/notify/pay/"+app, strings.NewReader("{}"))
	if err := HandlePayNotify(req, app); err != nil {
		t.Fatalf("HandlePayNotify: %v", err)
	}
	stored, _ := GetPayOrderByOutTradeNo(order.OutTradeNo)
	if stored.Status != PayOrderCreated {
		t.Fatalf("non-payment event should not change status, got %s", stored.Status)
	}
	if stored.LastEvent != "REFUND.SUCCESS" {
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

// 编译期约束:确保 fakePayAPI 始终实现 PayAPI。
var _ PayAPI = (*fakePayAPI)(nil)
