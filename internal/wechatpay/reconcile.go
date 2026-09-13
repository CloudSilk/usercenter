package wechatpay

import (
	"context"
	"sync"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
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

	reconcileMu      sync.Mutex
	reconcileRunning bool
	reconcileStop    chan struct{}
)

// ConfigureReconcile 应用对账参数(须在 StartReconcileLoop 前调用)。
// 非法值被钳制:interval 最小 10s,batchSize 范围 1-1000。
func ConfigureReconcile(interval time.Duration, scanAge time.Duration, batchSize int) {
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
func reconcileOrder(ctx context.Context, mchID string, api PayAPI, order *PayOrder) {
	if tx, err := api.Query(ctx, mchID, order.OutTradeNo); err == nil && tx != nil {
		applyTransaction(order, tx)
	}
	if order.Status != PayOrderCreated {
		return
	}
	if order.ExpireAt != nil && time.Now().After(*order.ExpireAt) {
		// 远端关单尽力而为:订单实际已支付时微信会拒绝关单,此时以上一次查单结果为准
		_ = api.Close(ctx, mchID, order.OutTradeNo)
		if err := MarkOrderClosed(order.ID, "订单已过期"); err != nil {
			log.Errorf(ctx, "对账关单失败 订单 %s:%v", order.OutTradeNo, err)
			return
		}
		order.Status = PayOrderClosed
		order.TradeStateDesc = "订单已过期"
	}
}

// ReconcileStaleOrders 执行一轮对账,返回处理的订单数。单笔失败不中断整轮。
func ReconcileStaleOrders(ctx context.Context) (int, error) {
	before := time.Now().Add(-ReconcileScanAge)
	orders, err := ListStaleCreatedOrders(before, ReconcileBatchSize)
	if err != nil {
		return 0, err
	}
	// 按微信应用配置缓存客户端,避免同一商户重复构建
	apis := make(map[string]PayAPI, 4)
	processed := 0
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
				continue
			}
			apis[cfg.ID] = api
		}
		reconcileOrder(ctx, cfg.MchID, api, order)
		processed++
	}
	return processed, nil
}
