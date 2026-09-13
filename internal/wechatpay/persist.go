package wechatpay

// 对账参数持久化:管理端运行时调整的参数保存到 SystemConfig 键值存储,
// 进程重启后优先于 Nacos 基线生效,保证临时调整不被重启重置。

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/systemconfig"
	"gorm.io/gorm"
)

const statsConfigKey = "wechatpay.stats_config"

// StatsConfigSnapshot 对账参数持久化快照(全部字段为生效值,非增量)。
type StatsConfigSnapshot struct {
	IntervalSeconds     int `json:"intervalSeconds,omitempty"`
	ScanAgeMinutes      int `json:"scanAgeMinutes,omitempty"`
	BatchSize           int `json:"batchSize,omitempty"`
	AlertAgeHours       int `json:"alertAgeHours,omitempty"`
	AlertSilenceMinutes int `json:"alertSilenceMinutes,omitempty"`
	BillRetentionDays   int `json:"billRetentionDays,omitempty"`
}

// snapshotFromLoopStatus 从当前运行值构造快照。
func SnapshotFromLoopStatus(s LoopStatus) StatsConfigSnapshot {
	return StatsConfigSnapshot{
		IntervalSeconds:     s.IntervalSeconds,
		ScanAgeMinutes:      s.ScanAgeMinutes,
		BatchSize:           s.BatchSize,
		AlertAgeHours:       s.AlertAgeHours,
		AlertSilenceMinutes: int(ReconcileAlertSilence / time.Minute),
		BillRetentionDays:   s.BillRetentionDays,
	}
}

// SaveStatsConfigSnapshot 幂等保存参数快照(key 唯一,存在即更新)。
func SaveStatsConfigSnapshot(s StatsConfigSnapshot) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var exist systemconfig.SystemConfig
	err = store.DB().Where("`key` = ?", statsConfigKey).First(&exist).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return store.DB().Create(&systemconfig.SystemConfig{
			Key: statsConfigKey, Value: string(b), IsMust: true,
		}).Error
	}
	if err != nil {
		return err
	}
	exist.Value = string(b)
	return store.DB().Save(&exist).Error
}

// LoadStatsConfigSnapshot 读取参数快照;不存在时返回 (nil, nil)。
func LoadStatsConfigSnapshot() (*StatsConfigSnapshot, error) {
	var rec systemconfig.SystemConfig
	err := store.DB().Where("`key` = ?", statsConfigKey).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s StatsConfigSnapshot
	if err := json.Unmarshal([]byte(rec.Value), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ApplyPersistedStatsConfig 启动时调用:存在已保存快照则覆盖 Nacos 基线,
// 各字段经既有钳制逻辑生效。无快照时为无操作。
func ApplyPersistedStatsConfig() error {
	s, err := LoadStatsConfigSnapshot()
	if err != nil || s == nil {
		return err
	}
	ConfigureReconcile(
		time.Duration(s.IntervalSeconds)*time.Second,
		time.Duration(s.ScanAgeMinutes)*time.Minute,
		s.BatchSize,
		time.Duration(s.AlertAgeHours)*time.Hour,
		time.Duration(s.AlertSilenceMinutes)*time.Minute,
	)
	if s.BillRetentionDays > 0 {
		SetBillRetentionDays(s.BillRetentionDays)
	}
	return nil
}
