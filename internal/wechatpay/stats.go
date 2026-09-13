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
