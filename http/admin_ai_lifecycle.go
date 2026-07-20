package http

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/store"
	"gorm.io/gorm"
)

const (
	maxAIProviderNameLength        = 100
	maxAIProviderURLLength         = 500
	maxAIProviderDescriptionLength = 500
	maxAIKeyNameLength             = 100
	maxAIKeySecretLength           = 4096
	maxAIModelAliasLength          = 100
	maxAIRouteDescriptionLength    = 500
	minAIRoutePriority             = -1000
	maxAIRoutePriority             = 1000
)

type aiProviderImpact struct {
	KeyCount   int64
	RouteCount int64
}

func normalizeAIProviderInput(provider *apikey.AIProvider) error {
	if provider == nil {
		return errors.New("provider is required")
	}
	provider.Name = strings.TrimSpace(provider.Name)
	provider.BaseURL = strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	provider.AuthType = strings.ToLower(strings.TrimSpace(provider.AuthType))
	provider.Description = strings.TrimSpace(provider.Description)
	if provider.AuthType == "" {
		provider.AuthType = "bearer"
	}
	if provider.Name == "" || utf8.RuneCountInString(provider.Name) > maxAIProviderNameLength {
		return fmt.Errorf("服务商名称不能为空且不能超过 %d 个字符", maxAIProviderNameLength)
	}
	if provider.BaseURL == "" || len(provider.BaseURL) > maxAIProviderURLLength {
		return fmt.Errorf("Base URL 不能为空且不能超过 %d 个字符", maxAIProviderURLLength)
	}
	parsed, err := url.Parse(provider.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return errors.New("Base URL 必须是无账号信息的 http 或 https 地址")
	}
	switch provider.AuthType {
	case "bearer", "header", "apikey", "query":
	default:
		return errors.New("鉴权方式必须是 bearer、header、apikey 或 query")
	}
	if utf8.RuneCountInString(provider.Description) > maxAIProviderDescriptionLength {
		return fmt.Errorf("服务商说明不能超过 %d 个字符", maxAIProviderDescriptionLength)
	}
	return nil
}

func normalizeAIKeyInput(name, secret string, priority int32) (string, string, error) {
	name = strings.TrimSpace(name)
	secret = strings.TrimSpace(secret)
	if name == "" || utf8.RuneCountInString(name) > maxAIKeyNameLength {
		return "", "", fmt.Errorf("Key 名称不能为空且不能超过 %d 个字符", maxAIKeyNameLength)
	}
	if secret == "" || len(secret) > maxAIKeySecretLength {
		return "", "", fmt.Errorf("API Key 不能为空且不能超过 %d 个字符", maxAIKeySecretLength)
	}
	if priority < minAIRoutePriority || priority > maxAIRoutePriority {
		return "", "", fmt.Errorf("Key 优先级必须在 %d 到 %d 之间", minAIRoutePriority, maxAIRoutePriority)
	}
	return name, secret, nil
}

func normalizeAIKeyMetadata(name string, priority int32) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxAIKeyNameLength {
		return "", fmt.Errorf("Key 名称不能为空且不能超过 %d 个字符", maxAIKeyNameLength)
	}
	if priority < minAIRoutePriority || priority > maxAIRoutePriority {
		return "", fmt.Errorf("Key 优先级必须在 %d 到 %d 之间", minAIRoutePriority, maxAIRoutePriority)
	}
	return name, nil
}

func normalizeAIRouteInput(route *apikey.ModelRoute) error {
	if route == nil {
		return errors.New("model route is required")
	}
	route.ModelAlias = strings.TrimSpace(route.ModelAlias)
	route.ProviderID = strings.TrimSpace(route.ProviderID)
	route.Description = strings.TrimSpace(route.Description)
	if route.ModelAlias == "" || utf8.RuneCountInString(route.ModelAlias) > maxAIModelAliasLength {
		return fmt.Errorf("模型别名不能为空且不能超过 %d 个字符", maxAIModelAliasLength)
	}
	if route.ProviderID == "" {
		return errors.New("请选择目标服务商")
	}
	if route.Priority < minAIRoutePriority || route.Priority > maxAIRoutePriority {
		return fmt.Errorf("路由优先级必须在 %d 到 %d 之间", minAIRoutePriority, maxAIRoutePriority)
	}
	if utf8.RuneCountInString(route.Description) > maxAIRouteDescriptionLength {
		return fmt.Errorf("路由说明不能超过 %d 个字符", maxAIRouteDescriptionLength)
	}
	return nil
}

func findAIProviderForTenant(tenantID, id string) (*apikey.AIProvider, error) {
	var provider apikey.AIProvider
	err := store.DB().
		Where("id = ? AND tenant_id = ?", strings.TrimSpace(id), strings.TrimSpace(tenantID)).
		First(&provider).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("服务商不存在或不属于当前租户")
	}
	return &provider, err
}

func findAIKeyForTenant(tenantID, id string) (*apikey.AIKey, *apikey.AIProvider, error) {
	var key apikey.AIKey
	err := store.DB().
		Where("id = ? AND tenant_id = ?", strings.TrimSpace(id), strings.TrimSpace(tenantID)).
		First(&key).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, errors.New("API Key 不存在或不属于当前租户")
	}
	if err != nil {
		return nil, nil, err
	}
	provider, err := findAIProviderForTenant(tenantID, key.ProviderID)
	return &key, provider, err
}

func findAIRouteForTenant(tenantID, id string) (*apikey.ModelRoute, *apikey.AIProvider, error) {
	var route apikey.ModelRoute
	err := store.DB().
		Where("id = ? AND tenant_id = ?", strings.TrimSpace(id), strings.TrimSpace(tenantID)).
		First(&route).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, errors.New("模型路由不存在或不属于当前租户")
	}
	if err != nil {
		return nil, nil, err
	}
	provider, err := findAIProviderForTenant(tenantID, route.ProviderID)
	return &route, provider, err
}

func isReservedAIProviderName(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "banlu-mock")
}

func isSystemAIProvider(provider *apikey.AIProvider) bool {
	return provider != nil && (provider.IsMust || isReservedAIProviderName(provider.Name))
}

func requireMutableAIProvider(provider *apikey.AIProvider) error {
	if isSystemAIProvider(provider) {
		return errors.New("系统 Mock 兜底由启动流程维护，不能在管理端修改或删除")
	}
	return nil
}

func aiProviderImpactForTenant(tenantID, providerID string) (aiProviderImpact, error) {
	var impact aiProviderImpact
	db := store.DB()
	if err := db.Model(&apikey.AIKey{}).
		Where("tenant_id = ? AND provider_id = ?", tenantID, providerID).
		Count(&impact.KeyCount).
		Error; err != nil {
		return impact, err
	}
	if err := db.Model(&apikey.ModelRoute{}).
		Where("tenant_id = ? AND provider_id = ?", tenantID, providerID).
		Count(&impact.RouteCount).
		Error; err != nil {
		return impact, err
	}
	return impact, nil
}

func aiProviderNameExists(tenantID, name, excludeID string) (bool, error) {
	query := store.DB().Model(&apikey.AIProvider{}).
		Where("tenant_id = ? AND name = ?", tenantID, name)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func createAIProviderForTenant(tenantID string, request apikey.AIProvider) (string, error) {
	request.TenantID = strings.TrimSpace(tenantID)
	request.IsMust = false
	if request.TenantID == "" {
		return "", errors.New("当前身份缺少租户信息")
	}
	if err := normalizeAIProviderInput(&request); err != nil {
		return "", err
	}
	if isReservedAIProviderName(request.Name) {
		return "", errors.New("banlu-mock 是系统保留服务商名称")
	}
	exists, err := aiProviderNameExists(request.TenantID, request.Name, "")
	if err != nil {
		return "", err
	}
	if exists {
		return "", errors.New("当前租户已存在同名服务商")
	}
	return apikey.CreateProvider(&request)
}

func updateAIProviderForTenant(tenantID, id string, request apikey.AIProvider) error {
	current, err := findAIProviderForTenant(tenantID, id)
	if err != nil {
		return err
	}
	if err := requireMutableAIProvider(current); err != nil {
		return err
	}
	request.TenantID = current.TenantID
	if err := normalizeAIProviderInput(&request); err != nil {
		return err
	}
	if isReservedAIProviderName(request.Name) {
		return errors.New("banlu-mock 是系统保留服务商名称")
	}
	exists, err := aiProviderNameExists(current.TenantID, request.Name, current.ID)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("当前租户已存在同名服务商")
	}
	return store.DB().Model(current).Updates(map[string]interface{}{
		"name":        request.Name,
		"base_url":    request.BaseURL,
		"auth_type":   request.AuthType,
		"healthy":     request.Healthy,
		"description": request.Description,
	}).Error
}

func deleteAIProviderForTenant(tenantID, id string) error {
	provider, err := findAIProviderForTenant(tenantID, id)
	if err != nil {
		return err
	}
	if err := requireMutableAIProvider(provider); err != nil {
		return err
	}
	impact, err := aiProviderImpactForTenant(tenantID, provider.ID)
	if err != nil {
		return err
	}
	if impact.KeyCount > 0 || impact.RouteCount > 0 {
		return fmt.Errorf("请先删除该服务商下的 %d 个 Key 和 %d 条模型路由", impact.KeyCount, impact.RouteCount)
	}
	return store.DB().Delete(provider).Error
}

func aiKeyNameExists(tenantID, providerID, name, excludeID string) (bool, error) {
	query := store.DB().Model(&apikey.AIKey{}).
		Where("tenant_id = ? AND provider_id = ? AND name = ?", tenantID, providerID, name)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func createAIKeyForTenant(
	tenantID, providerID, name, secret string,
	priority int32,
	enable bool,
) (string, error) {
	provider, err := findAIProviderForTenant(tenantID, providerID)
	if err != nil {
		return "", err
	}
	if err := requireMutableAIProvider(provider); err != nil {
		return "", err
	}
	name, secret, err = normalizeAIKeyInput(name, secret, priority)
	if err != nil {
		return "", err
	}
	exists, err := aiKeyNameExists(tenantID, provider.ID, name, "")
	if err != nil {
		return "", err
	}
	if exists {
		return "", errors.New("该服务商已存在同名 Key")
	}
	return apikey.CreateKey(&apikey.AIKey{
		TenantID: tenantID, ProviderID: provider.ID, Name: name,
		Priority: priority, Enable: enable, IsMust: false,
	}, secret)
}

func updateAIKeyForTenant(
	tenantID, id, providerID, name string,
	priority int32,
	enable bool,
) error {
	key, currentProvider, err := findAIKeyForTenant(tenantID, id)
	if err != nil {
		return err
	}
	if key.IsMust {
		return errors.New("系统 Key 不能在管理端修改")
	}
	if err := requireMutableAIProvider(currentProvider); err != nil {
		return err
	}
	if strings.TrimSpace(providerID) == "" {
		providerID = key.ProviderID
	}
	targetProvider, err := findAIProviderForTenant(tenantID, providerID)
	if err != nil {
		return err
	}
	if err := requireMutableAIProvider(targetProvider); err != nil {
		return err
	}
	name, err = normalizeAIKeyMetadata(name, priority)
	if err != nil {
		return err
	}
	exists, err := aiKeyNameExists(tenantID, targetProvider.ID, name, key.ID)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("该服务商已存在同名 Key")
	}
	return store.DB().Model(key).Updates(map[string]interface{}{
		"provider_id": targetProvider.ID,
		"name":        name,
		"priority":    priority,
		"enable":      enable,
	}).Error
}

func rotateAIKeyForTenant(tenantID, id, secret string) error {
	key, provider, err := findAIKeyForTenant(tenantID, id)
	if err != nil {
		return err
	}
	if key.IsMust {
		return errors.New("系统 Key 不能在管理端轮转")
	}
	if err := requireMutableAIProvider(provider); err != nil {
		return err
	}
	_, secret, err = normalizeAIKeyInput(key.Name, secret, key.Priority)
	if err != nil {
		return err
	}
	return apikey.RotateKey(key.ID, secret)
}

func deleteAIKeyForTenant(tenantID, id string) error {
	key, provider, err := findAIKeyForTenant(tenantID, id)
	if err != nil {
		return err
	}
	if key.IsMust {
		return errors.New("系统 Key 不能在管理端删除")
	}
	if err := requireMutableAIProvider(provider); err != nil {
		return err
	}
	return store.DB().Delete(key).Error
}

func aiRouteExists(tenantID, modelAlias, providerID, excludeID string) (bool, error) {
	query := store.DB().Model(&apikey.ModelRoute{}).
		Where("tenant_id = ? AND model_alias = ? AND provider_id = ?", tenantID, modelAlias, providerID)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

func createAIRouteForTenant(tenantID string, request apikey.ModelRoute) (string, error) {
	request.TenantID = strings.TrimSpace(tenantID)
	if request.TenantID == "" {
		return "", errors.New("当前身份缺少租户信息")
	}
	if err := normalizeAIRouteInput(&request); err != nil {
		return "", err
	}
	provider, err := findAIProviderForTenant(tenantID, request.ProviderID)
	if err != nil {
		return "", err
	}
	if err := requireMutableAIProvider(provider); err != nil {
		return "", err
	}
	exists, err := aiRouteExists(tenantID, request.ModelAlias, request.ProviderID, "")
	if err != nil {
		return "", err
	}
	if exists {
		return "", errors.New("该服务商已存在相同模型别名的路由")
	}
	return apikey.CreateRoute(&request)
}

func updateAIRouteForTenant(tenantID, id string, request apikey.ModelRoute) error {
	route, currentProvider, err := findAIRouteForTenant(tenantID, id)
	if err != nil {
		return err
	}
	if err := requireMutableAIProvider(currentProvider); err != nil {
		return err
	}
	request.TenantID = tenantID
	if err := normalizeAIRouteInput(&request); err != nil {
		return err
	}
	targetProvider, err := findAIProviderForTenant(tenantID, request.ProviderID)
	if err != nil {
		return err
	}
	if err := requireMutableAIProvider(targetProvider); err != nil {
		return err
	}
	exists, err := aiRouteExists(tenantID, request.ModelAlias, request.ProviderID, route.ID)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("该服务商已存在相同模型别名的路由")
	}
	return store.DB().Model(route).Updates(map[string]interface{}{
		"model_alias": request.ModelAlias,
		"provider_id": request.ProviderID,
		"priority":    request.Priority,
		"enable":      request.Enable,
		"description": request.Description,
	}).Error
}

func deleteAIRouteForTenant(tenantID, id string) error {
	route, provider, err := findAIRouteForTenant(tenantID, id)
	if err != nil {
		return err
	}
	if err := requireMutableAIProvider(provider); err != nil {
		return err
	}
	return store.DB().Delete(route).Error
}
