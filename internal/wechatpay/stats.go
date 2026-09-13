package wechatpay

import (
	"time"

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
