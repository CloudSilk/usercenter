package server

import (
	"errors"
	"strings"
	"time"

	"github.com/CloudSilk/usercenter/internal/systemconfig"
	"gorm.io/gorm"
)

// SystemConfigValue is the narrow public value contract for embedded
// products. It deliberately exposes neither the UserCenter GORM model nor
// table details.
type SystemConfigValue struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}

// GetSystemConfigValue reads one namespaced configuration value. found=false
// is returned for a missing key so callers do not need to depend on GORM
// errors.
func GetSystemConfigValue(key string) (
	value SystemConfigValue,
	found bool,
	err error,
) {
	key = strings.TrimSpace(key)
	if key == "" {
		return SystemConfigValue{}, false, errors.New(
			"system config key is required",
		)
	}
	item, err := systemconfig.GetSystemConfigByKey(key)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SystemConfigValue{}, false, nil
	}
	if err != nil {
		return SystemConfigValue{}, false, err
	}
	return SystemConfigValue{
		Key:       item.Key,
		Value:     item.Value,
		UpdatedAt: item.UpdatedAt,
	}, true, nil
}

// CompareAndSwapSystemConfigValue atomically writes one namespaced value.
// expectedValue=nil creates only when the key is absent. A non-nil value
// updates only when the complete stored value still matches, preventing a
// stale embedded-product settings page from overwriting a newer revision.
func CompareAndSwapSystemConfigValue(
	key string,
	expectedValue *string,
	value string,
) (bool, error) {
	return systemconfig.CompareAndSwapSystemConfigByKey(
		key,
		expectedValue,
		value,
	)
}
