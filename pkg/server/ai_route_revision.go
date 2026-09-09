package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/CloudSilk/usercenter/internal/apikey"
	"github.com/CloudSilk/usercenter/internal/store"
	"gorm.io/gorm"
)

// AIModelRouteRevision is a credential-free snapshot of every configured route
// the gateway may use for an alias, including global fallback. Embedded products
// use it to invalidate model-dependent artifacts after routing changes.
type AIModelRouteRevision struct {
	Digest         string
	UpstreamModels []string
}

// GetAIModelRouteRevision is a trusted server composition API, not an HTTP
// authorization boundary. The host must derive tenantID from its authenticated
// actor. The digest excludes key material, key rotation, cooldown and transient
// health, but includes provider endpoints and routing configuration. This is a
// content digest, not a monotonic revision or proof of immutable model weights.
func GetAIModelRouteRevision(ctx context.Context, tenantID, alias string) (AIModelRouteRevision, error) {
	if store.DB() == nil || strings.TrimSpace(alias) == "" {
		return AIModelRouteRevision{}, errors.New("AI route revision is unavailable")
	}
	type binding struct {
		RouteID, RouteTenant, ProviderID, ProviderTenant, Endpoint, AuthType, Model string
		Priority                                                                    int32
	}
	var bindings []binding
	err := store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var routes []apikey.ModelRoute
		if err := tx.Where("tenant_id IN (?, '') AND model_alias = ? AND enable = ?", tenantID, alias, true).Order("id").Find(&routes).Error; err != nil {
			return err
		}
		for _, route := range routes {
			var provider apikey.AIProvider
			query := tx.Where("id = ?", route.ProviderID)
			if route.TenantID == "" {
				query = query.Where("tenant_id = ''")
			} else {
				query = query.Where("tenant_id IN (?, '')", tenantID)
			}
			if err := query.First(&provider).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			model := strings.TrimSpace(route.UpstreamModel)
			if model == "" {
				model = strings.TrimSpace(route.ModelAlias)
			}
			bindings = append(bindings, binding{route.ID, route.TenantID, provider.ID, provider.TenantID, provider.BaseURL, provider.AuthType, model, route.Priority})
		}
		return nil
	})
	if err != nil {
		return AIModelRouteRevision{}, err
	}
	if len(bindings) == 0 {
		return AIModelRouteRevision{}, errors.New("AI model has no configured route")
	}
	models := map[string]bool{}
	for _, b := range bindings {
		models[b.Model] = true
	}
	out := AIModelRouteRevision{}
	for model := range models {
		out.UpstreamModels = append(out.UpstreamModels, model)
	}
	sort.Strings(out.UpstreamModels)
	raw, err := json.Marshal(bindings)
	if err != nil {
		return AIModelRouteRevision{}, err
	}
	sum := sha256.Sum256(raw)
	out.Digest = hex.EncodeToString(sum[:])
	return out, nil
}
