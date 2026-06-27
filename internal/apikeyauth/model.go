package apikeyauth

import "github.com/CloudSilk/pkg/model"

// APIKeyAuth represents a long-lived API key for external service authentication.
// The plaintext key is shown only once at creation time; only the SHA-256 hash
// is stored in the database.
type APIKeyAuth struct {
	model.Model
	TenantID    string `json:"tenantID" gorm:"index;size:36"`
	PrincipalID string `json:"principalID" gorm:"size:36"`
	Name        string `json:"name" gorm:"size:100"`
	KeyHash     string `json:"-" gorm:"size:64;uniqueIndex"` // SHA-256 of the key
	KeyPrefix   string `json:"keyPrefix" gorm:"size:8"`      // First 8 chars for display
	Roles       string `json:"roles" gorm:"size:200"`        // comma-separated role IDs
	Enable      bool   `json:"enable" gorm:"index;default:true"`
	LastUsedAt  int64  `json:"lastUsedAt" gorm:"default:0"`
}

func (APIKeyAuth) TableName() string { return "api_key_auth" }
