package webhook

import (
	"strings"

	"github.com/CloudSilk/usercenter/internal/store"
	"gorm.io/gorm/clause"
)

// CreateSub 创建一条 Webhook 订阅。
func CreateSub(s *Subscription) (string, error) {
	err := store.DB().Create(s).Error
	if err != nil {
		return "", err
	}
	return s.ID, nil
}

// GetSub 根据 ID 获取一条订阅。
func GetSub(id string) (*Subscription, error) {
	var s Subscription
	err := store.DB().Preload(clause.Associations).Where("id = ?", id).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSubs 按租户分页查询订阅列表。tenantID 为空时不过滤。
func ListSubs(tenantID string, limit, offset int) ([]Subscription, int64, error) {
	db := store.DB().Model(&Subscription{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Subscription
	if err := db.Order("created_at desc").Limit(limit).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateSub 更新一条订阅（允许修改 Name / URL / Events / Secret / Enable）。
func UpdateSub(s *Subscription) error {
	return store.DB().Model(s).Select("name", "url", "events", "secret", "enable").Where("id = ?", s.ID).Updates(s).Error
}

// UpdateSubForTenant updates a subscription only when it belongs to tenantID.
func UpdateSubForTenant(s *Subscription, tenantID string) (bool, error) {
	result := store.DB().Model(&Subscription{}).
		Where("id = ? AND tenant_id = ?", s.ID, tenantID).
		Select("name", "url", "events", "secret", "enable").
		Updates(s)
	return result.RowsAffected == 1, result.Error
}

// DeleteSub 删除一条订阅。
func DeleteSub(id string) error {
	return store.DB().Delete(&Subscription{}, "id = ?", id).Error
}

// DeleteSubForTenant deletes a subscription only when it belongs to tenantID.
func DeleteSubForTenant(id, tenantID string) (bool, error) {
	result := store.DB().Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&Subscription{})
	return result.RowsAffected == 1, result.Error
}

// GetEnabledSubsForEvent 查询所有启用的、匹配给定事件的订阅。
// events 字段为逗号分隔的字符串，包含匹配即返回。
func GetEnabledSubsForEvent(event string) []*Subscription {
	var list []*Subscription
	err := store.DB().Where("enable = ?", true).Find(&list).Error
	if err != nil {
		return nil
	}
	var matched []*Subscription
	for _, s := range list {
		for _, e := range strings.Split(s.Events, ",") {
			if strings.TrimSpace(e) == event {
				matched = append(matched, s)
				break
			}
		}
	}
	return matched
}
