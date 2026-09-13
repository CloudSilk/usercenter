package wechatpay

import (
	"context"
	"sort"
	"time"

	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
)

// 对账日报统计:按自然日聚合支付与退款数据,供管理端报表与对账核对使用。
// 金额单位一律为分;日期为本地时区的 YYYY-MM-DD。

// DailyPayStat 单日支付对账汇总。
type DailyPayStat struct {
	// Date 统计日期(本地时区 YYYY-MM-DD)。
	Date string `json:"date"`
	// CreatedCount/CreatedAmount 当日下单笔数与金额。
	CreatedCount  int64 `json:"createdCount"`
	CreatedAmount int64 `json:"createdAmount"`
	// PaidCount/PaidAmount 当日支付成功笔数与金额(按支付完成时间)。
	PaidCount  int64 `json:"paidCount"`
	PaidAmount int64 `json:"paidAmount"`
	// ClosedCount 当日关单笔数(含过期兜底)。
	ClosedCount int64 `json:"closedCount"`
	// RefundCount/RefundAmount 当日有效退款申请笔数与金额
	// (待审核+受理中+已成功,不含已拒绝)。
	RefundCount  int64 `json:"refundCount"`
	RefundAmount int64 `json:"refundAmount"`
}

// dailyRow 聚合查询的原始行。
type dailyRow struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
	Total int64  `json:"total"`
}

// QueryDailyPayStats 查询最近 days 天(含今日)的按日汇总,缺数据的天补零。
// tenantID 为空表示全租户汇总(平台侧)。
func QueryDailyPayStats(tenantID string, days int) ([]*DailyPayStat, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	today := time.Now()
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location()).
		AddDate(0, 0, -(days - 1))
	dayKey := "date(created_at)"

	// 预填充日期轴,保证无数据的天也出现在报表中
	stats := make([]*DailyPayStat, 0, days)
	index := make(map[string]int, days)
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		index[d] = i
		stats = append(stats, &DailyPayStat{Date: d})
	}

	orderScope := store.DB().Model(&PayOrder{}).Where("created_at >= ?", start)
	if tenantID != "" {
		orderScope = orderScope.Where("tenant_id = ?", tenantID)
	}
	var rows []dailyRow
	if err := orderScope.Select("date(created_at) as date, count(*) as count, coalesce(sum(amount),0) as total").
		Group(dayKey).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		if i, ok := index[r.Date]; ok {
			stats[i].CreatedCount = r.Count
			stats[i].CreatedAmount = r.Total
		}
	}

	paidScope := store.DB().Model(&PayOrder{}).
		Where("status = ? AND paid_at >= ?", PayOrderPaid, start)
	if tenantID != "" {
		paidScope = paidScope.Where("tenant_id = ?", tenantID)
	}
	rows = nil
	if err := paidScope.Select("date(paid_at) as date, count(*) as count, coalesce(sum(amount),0) as total").
		Group("date(paid_at)").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		if i, ok := index[r.Date]; ok {
			stats[i].PaidCount = r.Count
			stats[i].PaidAmount = r.Total
		}
	}

	closedScope := store.DB().Model(&PayOrder{}).
		Where("status = ? AND closed_at >= ?", PayOrderClosed, start)
	if tenantID != "" {
		closedScope = closedScope.Where("tenant_id = ?", tenantID)
	}
	rows = nil
	if err := closedScope.Select("date(closed_at) as date, count(*) as count, coalesce(sum(amount),0) as total").
		Group("date(closed_at)").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		if i, ok := index[r.Date]; ok {
			stats[i].ClosedCount = r.Count
		}
	}

	refundScope := store.DB().Model(&PayRefund{}).
		Where("created_at >= ? AND status IN ?", start,
			[]string{RefundPending, RefundProcessing, RefundSuccess})
	if tenantID != "" {
		refundScope = refundScope.Where("tenant_id = ?", tenantID)
	}
	rows = nil
	if err := refundScope.Select("date(created_at) as date, count(*) as count, coalesce(sum(amount),0) as total").
		Group(dayKey).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		if i, ok := index[r.Date]; ok {
			stats[i].RefundCount = r.Count
			stats[i].RefundAmount = r.Total
		}
	}
	return stats, nil
}

// --- 昨日日报定时推送:复用对账循环的 tick,每日到达推送时刻后推送一次 ---

var (
	// DailyReportEnabled 是否启用每日对账日报推送,默认启用。
	DailyReportEnabled = true
	// DailyReportHour 每日推送时刻(本地时区小时,0-23),默认 8。
	DailyReportHour = 8
	// dailyReportLastDate 最近一次推送的基准日(YYYY-MM-DD),进程内去重。
	dailyReportLastDate string
)

// buildDailyReportPayload 汇总 reportDate 当日指定范围(租户ID留空为平台全量)的对账数据。
func buildDailyReportPayload(tenantID, reportDate string) (map[string]any, error) {
	stats, err := QueryDailyPayStats(tenantID, 90)
	if err != nil {
		return nil, err
	}
	for _, s := range stats {
		if s.Date == reportDate {
			return map[string]any{
				"date":         s.Date,
				"createdCount": s.CreatedCount, "createdAmount": s.CreatedAmount,
				"paidCount": s.PaidCount, "paidAmount": s.PaidAmount,
				"closedCount": s.ClosedCount,
				"refundCount": s.RefundCount, "refundAmount": s.RefundAmount,
			}, nil
		}
	}
	return nil, nil
}

// maybePushDailyReport 对账循环 tick 时调用:到达每日推送时刻且当日未推送,
// 则推送昨日对账日报(webhook + 审计留痕,不受告警静默窗口影响)。
// 推送范围:平台全量汇总一条 + 各有支付活动租户分别一条(payload 携带 tenantID)。
func maybePushDailyReport(ctx context.Context, now time.Time) {
	if !DailyReportEnabled || now.Hour() < DailyReportHour {
		return
	}
	today := now.Format("2006-01-02")
	if dailyReportLastDate == today {
		return
	}
	dailyReportLastDate = today
	reportDate := now.AddDate(0, 0, -1).Format("2006-01-02")

	pushOne := func(tenantID, date string) {
		payload, err := buildDailyReportPayload(tenantID, date)
		if err != nil {
			log.Errorf(ctx, "支付对账日报汇总失败(租户 %s):%v", tenantID, err)
			return
		}
		if payload == nil {
			return
		}
		if tenantID != "" {
			payload["tenantID"] = tenantID
			payload["scope"] = "tenant"
		} else {
			payload["scope"] = "platform"
		}
		alertFire("pay_daily_report", payload)
		recordAlertAudit("pay_daily_report", payload)
	}
	pushOne("", reportDate) // 平台汇总
	tenantIDs, err := ListActivePayTenantIDs(2)
	if err != nil {
		log.Errorf(ctx, "对账日报租户列表查询失败:%v", err)
		return
	}
	pushed := 0
	for _, tid := range tenantIDs {
		if tid == "" {
			continue
		}
		pushOne(tid, reportDate)
		pushed++
	}
	log.Infof(ctx, "支付对账日报已推送(%s):平台 1 条,分租户 %d 条", reportDate, pushed)
}

// ListActivePayTenantIDs 取最近 days 天有下单活动的租户ID列表。
func ListActivePayTenantIDs(days int) ([]string, error) {
	start := time.Now().AddDate(0, 0, -(days - 1))
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	var ids []string
	err := store.DB().Model(&PayOrder{}).
		Where("created_at >= ?", start).
		Distinct("tenant_id").Pluck("tenant_id", &ids).Error
	return ids, err
}

// SetDailyReportConfig 配置日报推送开关与推送时刻(启动时调用一次)。
// hour 越界时钳制到 0-23;enabled=false 时禁用推送。
func SetDailyReportConfig(enabled bool, hour int) {
	if hour < 0 {
		hour = 0
	}
	if hour > 23 {
		hour = 23
	}
	DailyReportEnabled = enabled
	DailyReportHour = hour
}

// ReasonCodeStat 按退款原因类别聚合的统计行。
type ReasonCodeStat struct {
	// ReasonCode 退款原因类别(见 ValidReasonCodes)。
	ReasonCode string `json:"reasonCode"`
	// Count 有效退款申请笔数(待审核+受理中+已成功)。
	Count int64 `json:"count"`
	// Amount 退款金额合计,单位:分。
	Amount int64 `json:"amount"`
}

// QueryRefundReasonStats 按退款原因类别聚合最近 days 天(含今日)的有效退款申请,
// 按金额降序。tenantID 为空表示全租户汇总(平台侧)。
func QueryRefundReasonStats(tenantID string, days int) ([]*ReasonCodeStat, error) {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	start := time.Now().AddDate(0, 0, -(days - 1))
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())

	scope := store.DB().Model(&PayRefund{}).
		Where("created_at >= ? AND status IN ?", start,
			[]string{RefundPending, RefundProcessing, RefundSuccess})
	if tenantID != "" {
		scope = scope.Where("tenant_id = ?", tenantID)
	}
	var stats []*ReasonCodeStat
	err := scope.Select("reason_code as reason_code, count(*) as count, coalesce(sum(amount),0) as amount").
		Group("reason_code").Order("amount DESC").Find(&stats).Error
	return stats, err
}

// RefundReasonTrendPoint 退款原因月度趋势点。
type RefundReasonTrendPoint struct {
	// Month 统计月份(YYYY-MM,本地时区)。
	Month string `json:"month"`
	// ReasonCode 退款原因类别。
	ReasonCode string `json:"reasonCode"`
	// Count 有效退款申请笔数。
	Count int64 `json:"count"`
	// Amount 退款金额合计,单位:分。
	Amount int64 `json:"amount"`
}

// QueryRefundReasonTrend 按月聚合各类别退款趋势,最近 months 个月(含当月)。
// 月内按类别一行,按月升序、类别金额降序;tenantID 留空为平台全量。
// 日期分组在 Go 侧完成(而非 SQL date 函数),兼容 SQLite 与 MySQL。
func QueryRefundReasonTrend(tenantID string, months int) ([]*RefundReasonTrendPoint, error) {
	if months <= 0 {
		months = 6
	}
	if months > 24 {
		months = 24
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -(months - 1), 0)

	scope := store.DB().Model(&PayRefund{}).
		Where("created_at >= ? AND status IN ?", start,
				[]string{RefundPending, RefundProcessing, RefundSuccess}).
		Limit(20000) // 防御性上限:聚合在内存完成,避免异常数据量拖垮服务
	if tenantID != "" {
		scope = scope.Where("tenant_id = ?", tenantID)
	}
	var rows []struct {
		CreatedAt  time.Time
		ReasonCode string
		Amount     int64
	}
	if err := scope.Select("created_at, reason_code, amount").Find(&rows).Error; err != nil {
		return nil, err
	}

	type key struct {
		month string
		code  string
	}
	agg := map[key]*RefundReasonTrendPoint{}
	var order []key
	for _, r := range rows {
		month := r.CreatedAt.Format("2006-01")
		k := key{month, r.ReasonCode}
		p, ok := agg[k]
		if !ok {
			p = &RefundReasonTrendPoint{Month: month, ReasonCode: r.ReasonCode}
			agg[k] = p
			order = append(order, k)
		}
		p.Count++
		p.Amount += r.Amount
	}
	// 稳定输出:月升序,月内按类别金额降序
	sort.Slice(order, func(i, j int) bool {
		if order[i].month != order[j].month {
			return order[i].month < order[j].month
		}
		return agg[order[i]].Amount > agg[order[j]].Amount
	})
	result := make([]*RefundReasonTrendPoint, 0, len(order))
	for _, k := range order {
		result = append(result, agg[k])
	}
	return result, nil
}

// --- 每日账单任务化下载:自动拉取昨日账单入库,管理端可离线查询/下载 ---

var (
	// BillDownloadEnabled 是否启用每日账单自动下载,默认启用。
	BillDownloadEnabled = true
	// billDownloadLastDate 进程内按日去重。
	billDownloadLastDate string
)

// maybeDownloadDailyBills 对账循环 tick 时调用:到达下载时刻(日报时刻+1 小时)且当日未执行,
// 为每个启用中的商户配置下载昨日账单(ALL)入库;已存在的账单自动跳过。
// 单配置失败推送告警并写审计,不中断其他配置。
func maybeDownloadDailyBills(ctx context.Context, now time.Time) {
	if !BillDownloadEnabled || now.Hour() < DailyReportHour+1 {
		return
	}
	today := now.Format("2006-01-02")
	if billDownloadLastDate == today {
		return
	}
	billDownloadLastDate = today
	billDate := now.AddDate(0, 0, -1).Format("2006-01-02")

	configs, err := ListEnabledPayConfigs()
	if err != nil {
		log.Errorf(ctx, "账单任务化下载获取配置失败:%v", err)
		return
	}
	failed := 0
	for i := range configs {
		cfg := configs[i]
		api, err := GetPayAPI(cfg)
		if err != nil {
			failed++
			log.Errorf(ctx, "账单下载初始化客户端失败(租户 %s):%v", cfg.TenantID, err)
			continue
		}
		csvData, err := api.DownloadTradeBill(ctx, billDate, BillTypeAll)
		if err != nil {
			failed++
			log.Errorf(ctx, "账单下载失败(租户 %s, %s):%v", cfg.TenantID, billDate, err)
			continue
		}
		if _, err := UpsertBillFile(&BillFile{
			TenantID: cfg.TenantID, ConfigID: cfg.ID,
			BillDate: billDate, BillType: BillTypeAll, Content: csvData,
		}); err != nil {
			failed++
			log.Errorf(ctx, "账单入库失败(租户 %s):%v", cfg.TenantID, err)
		}
	}
	if failed > 0 {
		payload := map[string]any{"date": billDate, "failed": failed}
		alertFire("pay_bill_download_failed", payload)
		recordAlertAudit("pay_bill_download_failed", payload)
	}
	log.Infof(ctx, "每日账单任务完成(%s):配置 %d 个,失败 %d", billDate, len(configs), failed)
}

// ListEnabledPayConfigs 取所有启用中的商户配置。
func ListEnabledPayConfigs() ([]*PayConfig, error) {
	var list []*PayConfig
	err := store.DB().Where("enable = ?", true).Find(&list).Error
	return list, err
}

// --- 归档账单保留期清理:随对账循环每日执行一次 ---

var (
	// BillRetentionDays 归档账单保留天数,默认 90。
	BillRetentionDays = 90
	// billCleanupLastDate 进程内按日去重。
	billCleanupLastDate string
)

// CleanupExpiredBills 删除 bill_date 早于保留期起点(BillRetentionDays)的归档账单,
// 返回删除行数。
func CleanupExpiredBills(ctx context.Context) (int64, error) {
	if BillRetentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -BillRetentionDays).Format("2006-01-02")
	res := store.DB().Where("bill_date < ?", cutoff).Delete(&BillFile{})
	if res.Error == nil && res.RowsAffected > 0 {
		log.Infof(ctx, "已清理超期归档账单 %d 条(保留 %d 天)", res.RowsAffected, BillRetentionDays)
	}
	return res.RowsAffected, res.Error
}

// maybeCleanupExpiredBills 对账循环 tick 调用:每日在账单下载完成后清理一次超期归档。
func maybeCleanupExpiredBills(ctx context.Context, now time.Time) {
	if !BillDownloadEnabled || now.Hour() < DailyReportHour+2 {
		return
	}
	today := now.Format("2006-01-02")
	if billCleanupLastDate == today {
		return
	}
	billCleanupLastDate = today
	if _, err := CleanupExpiredBills(ctx); err != nil {
		log.Errorf(ctx, "归档账单清理失败:%v", err)
	}
}

// SetBillRetentionDays 运行时更新归档账单保留期(最小 7 天),加锁保证并发安全。
func SetBillRetentionDays(days int) {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	if days < 7 {
		days = 7
	}
	BillRetentionDays = days
}
