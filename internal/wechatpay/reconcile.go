package wechatpay

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/alert"
	"github.com/CloudSilk/usercenter/internal/audit"
	"github.com/CloudSilk/usercenter/internal/store"
)

// 对账兜底:微信支付回调可能因网络抖动丢失,定时扫描超时未决(CREATED)订单,
// 主动向微信侧查单同步状态,避免订单长期滞留在"已下单"状态。
// 循环由宿主在配置加载后显式启动(main → StartReconcileLoop),参数可经配置中心覆盖。

var (
	// ReconcileScanAge 只扫描创建时间早于该时长的订单,给回调留出到达窗口。
	ReconcileScanAge = 5 * time.Minute
	// ReconcileBatchSize 单轮对账的最大订单数。
	ReconcileBatchSize = 200
	// ReconcileLoopInterval 对账轮询间隔。
	ReconcileLoopInterval = time.Minute
	// ReconcileAlertAge 订单滞留 CREATED 超过该时长触发告警。
	ReconcileAlertAge = 24 * time.Hour
	// ReconcileAlertSilence 同类告警的静默窗口:发送后窗口内的重复事件被抑制。
	// <=0 表示禁用去重(每轮都发)。
	ReconcileAlertSilence = 2 * time.Hour

	// alertFire 告警推送函数,测试可替换。未配置 webhook URL 时为空操作。
	alertFire = alert.FireWebhook

	// auditRecorder 审计写入函数,测试可替换。
	auditRecorder = audit.RecordAuditWithKind

	// alertSilenceLast 各事件类型上次发送时间(静默窗口去重)。
	alertSilenceLast = map[string]time.Time{}

	reconcileMu      sync.Mutex
	reconcileRunning bool
	reconcileStop    chan struct{}

	// 最近一轮对账执行情况(供状态查询端点展示)。
	lastReconcileAt  time.Time
	lastReconcileOK  bool
	lastProcessedCnt int
)

// LoopStatus 对账循环运行状态快照。
type LoopStatus struct {
	Running           bool      `json:"running"`
	IntervalSeconds   int       `json:"intervalSeconds"`
	ScanAgeMinutes    int       `json:"scanAgeMinutes"`
	BatchSize         int       `json:"batchSize"`
	AlertAgeHours     int       `json:"alertAgeHours"`
	BillRetentionDays int       `json:"billRetentionDays"`
	LastReconcileAt   time.Time `json:"lastReconcileAt,omitempty"`
	LastReconcileOK   bool      `json:"lastReconcileOK"`
	LastProcessed     int       `json:"lastProcessed"`
	LastDailyReport   string    `json:"lastDailyReport,omitempty"`
	LastBillDownload  string    `json:"lastBillDownload,omitempty"`
}

// GetLoopStatus 返回循环状态快照。
func GetLoopStatus() LoopStatus {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	return LoopStatus{
		Running:           reconcileRunning,
		IntervalSeconds:   int(ReconcileLoopInterval / time.Second),
		ScanAgeMinutes:    int(ReconcileScanAge / time.Minute),
		BatchSize:         ReconcileBatchSize,
		AlertAgeHours:     int(ReconcileAlertAge / time.Hour),
		BillRetentionDays: BillRetentionDays,
		LastReconcileAt:   lastReconcileAt,
		LastReconcileOK:   lastReconcileOK,
		LastProcessed:     lastProcessedCnt,
		LastDailyReport:   dailyReportLastDate,
		LastBillDownload:  billDownloadLastDate,
	}
}

// shouldAlert 静默窗口判定:该事件类型上次发送在窗口内则返回 false(不发送)。
func shouldAlert(eventType string) bool {
	if ReconcileAlertSilence <= 0 {
		return true
	}
	if last, ok := alertSilenceLast[eventType]; ok && time.Since(last) < ReconcileAlertSilence {
		return false
	}
	alertSilenceLast[eventType] = time.Now()
	return true
}

// recordAlertAudit 告警事件落审计日志:系统主体(PrincipalKind=2),
// 与 webhook 推送同受静默窗口控制;detail 截断至 900 字节防超长。
func recordAlertAudit(eventType string, payload map[string]any) {
	b, err := json.Marshal(payload)
	if err != nil {
		b = []byte("{}")
	}
	if len(b) > 900 {
		b = b[:900]
	}
	auditRecorder(store.DB(), "system", "wechatpay-reconcile", 2, eventType, "", "", string(b))
}

// ConfigureReconcile 应用对账参数(须在 StartReconcileLoop 前调用)。
// 非法值被钳制:interval 最小 10s,batchSize 范围 1-1000,alertAge 最小 1h,
// alertSilence 最小 1m(传 0 表示禁用静默去重)。
func ConfigureReconcile(interval time.Duration, scanAge time.Duration, batchSize int, alertAge time.Duration, alertSilence time.Duration) {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	ReconcileLoopInterval = interval
	if scanAge > 0 {
		ReconcileScanAge = scanAge
	}
	if batchSize < 1 {
		batchSize = 1
	}
	if batchSize > 1000 {
		batchSize = 1000
	}
	ReconcileBatchSize = batchSize
	if alertAge < time.Hour {
		alertAge = time.Hour
	}
	ReconcileAlertAge = alertAge
	switch {
	case alertSilence == 0:
		ReconcileAlertSilence = 0 // 显式禁用静默去重
	case alertSilence < time.Minute:
		ReconcileAlertSilence = time.Minute
	default:
		ReconcileAlertSilence = alertSilence
	}
}

// ReconcileLoopRunning 对账循环是否在运行。
func ReconcileLoopRunning() bool {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	return reconcileRunning
}

// StartReconcileLoop 启动对账兜底循环;幂等,重复调用无副作用。
func StartReconcileLoop() {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	if reconcileRunning {
		return
	}
	reconcileRunning = true
	reconcileStop = make(chan struct{})
	stop := reconcileStop
	interval := ReconcileLoopInterval
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx := context.Background()
				if n, err := ReconcileStaleOrders(ctx); err != nil {
					log.Errorf(ctx, "支付订单对账失败:%v", err)
				} else if n > 0 {
					log.Infof(ctx, "支付订单对账完成:同步 %d 笔", n)
				}
				maybePushDailyReport(ctx, time.Now())
				maybeDownloadDailyBills(ctx, time.Now())
				maybeCleanupExpiredBills(ctx, time.Now())
			case <-stop:
				return
			}
		}
	}()
}

// StopReconcileLoop 停止对账循环;未启动时调用无副作用。
func StopReconcileLoop() {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	if !reconcileRunning {
		return
	}
	close(reconcileStop)
	reconcileRunning = false
}

// ListStaleCreatedOrders 取超时未决的 CREATED 订单(按创建时间升序)。
func ListStaleCreatedOrders(before time.Time, limit int) ([]*PayOrder, error) {
	var list []*PayOrder
	err := store.DB().
		Where("status = ? AND created_at < ?", PayOrderCreated, before).
		Order("created_at ASC").Limit(limit).Find(&list).Error
	return list, err
}

// reconcileOrder 对单笔订单查单同步;远端确认未支付且已过失效时间则关单兜底。
// 查单出错时返回错误供告警聚合(订单状态保持 CREATED,等待下一轮)。
func reconcileOrder(ctx context.Context, mchID string, api PayAPI, order *PayOrder) error {
	if tx, err := api.Query(ctx, mchID, order.OutTradeNo); err != nil {
		return err
	} else if tx != nil {
		applyTransaction(order, tx)
	}
	if order.Status != PayOrderCreated {
		return nil
	}
	if order.ExpireAt != nil && time.Now().After(*order.ExpireAt) {
		// 远端关单尽力而为:订单实际已支付时微信会拒绝关单,此时以上一次查单结果为准
		_ = api.Close(ctx, mchID, order.OutTradeNo)
		if err := MarkOrderClosed(order.ID, "订单已过期"); err != nil {
			log.Errorf(ctx, "对账关单失败 订单 %s:%v", order.OutTradeNo, err)
			return nil
		}
		order.Status = PayOrderClosed
		order.TradeStateDesc = "订单已过期"
	}
	return nil
}

// ReconcileStaleOrders 执行一轮对账,返回处理的订单数。单笔失败不中断整轮,
// 异常(查单失败/客户端初始化失败/订单长期滞留)按轮聚合推送告警。
func ReconcileStaleOrders(ctx context.Context) (int, error) {
	before := time.Now().Add(-ReconcileScanAge)
	orders, err := ListStaleCreatedOrders(before, ReconcileBatchSize)
	if err != nil {
		reconcileMu.Lock()
		lastReconcileAt, lastReconcileOK, lastProcessedCnt = time.Now(), false, 0
		reconcileMu.Unlock()
		return 0, err
	}
	// 按微信应用配置缓存客户端,避免同一商户重复构建
	apis := make(map[string]PayAPI, 4)
	var (
		processed      int
		queryFailures  []map[string]any // 查单失败
		clientFailures []map[string]any // 客户端初始化失败
		stuckOrders    []map[string]any // 长期滞留 CREATED
	)
	stuckBefore := time.Now().Add(-ReconcileAlertAge)
	for _, order := range orders {
		cfg, err := GetPayConfigByWechatConfigID(order.WechatConfigID)
		if err != nil {
			log.Warnf(ctx, "对账跳过订单 %s:支付配置不可用(%v)", order.OutTradeNo, err)
			continue
		}
		api, ok := apis[cfg.ID]
		if !ok {
			api, err = GetPayAPI(cfg)
			if err != nil {
				log.Errorf(ctx, "对账跳过订单 %s:初始化支付客户端失败(%v)", order.OutTradeNo, err)
				clientFailures = append(clientFailures, map[string]any{
					"mchID": cfg.MchID, "orderNo": order.OutTradeNo, "error": err.Error(),
				})
				continue
			}
			apis[cfg.ID] = api
		}
		if qErr := reconcileOrder(ctx, cfg.MchID, api, order); qErr != nil {
			queryFailures = append(queryFailures, map[string]any{
				"orderNo": order.OutTradeNo, "error": qErr.Error(),
			})
		}
		processed++
		if order.Status == PayOrderCreated && order.CreatedAt.Before(stuckBefore) {
			stuckOrders = append(stuckOrders, map[string]any{
				"orderNo":    order.OutTradeNo,
				"tenantID":   order.TenantID,
				"ageMinutes": int(time.Since(order.CreatedAt).Minutes()),
			})
		}
	}
	if len(queryFailures) > 0 && shouldAlert("pay_reconcile_query_failed") {
		alertFire("pay_reconcile_query_failed", map[string]any{"count": len(queryFailures), "orders": queryFailures})
		recordAlertAudit("pay_reconcile_query_failed", map[string]any{"count": len(queryFailures), "orders": queryFailures})
	}
	if len(clientFailures) > 0 && shouldAlert("pay_reconcile_client_failed") {
		alertFire("pay_reconcile_client_failed", map[string]any{"count": len(clientFailures), "failures": clientFailures})
		recordAlertAudit("pay_reconcile_client_failed", map[string]any{"count": len(clientFailures), "failures": clientFailures})
	}
	if len(stuckOrders) > 0 && shouldAlert("pay_order_stuck_created") {
		payload := map[string]any{
			"count": len(stuckOrders), "orders": stuckOrders, "thresholdHours": int(ReconcileAlertAge.Hours()),
		}
		alertFire("pay_order_stuck_created", payload)
		recordAlertAudit("pay_order_stuck_created", payload)
	}
	reconcileMu.Lock()
	lastReconcileAt, lastReconcileOK, lastProcessedCnt = time.Now(), true, processed
	reconcileMu.Unlock()
	return processed, nil
}
