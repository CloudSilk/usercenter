package apikeyauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/CloudSilk/usercenter/internal/store"
)

const (
	keyPrefix     = "uc_"
	keyRandomLen  = 32 // 32 hex chars = 16 bytes
	prefixDisplay = 8  // First 8 chars of full key for KeyPrefix
)

// GenerateKey creates a new random API key with uc_ prefix.
func GenerateKey() (string, error) {
	b := make([]byte, keyRandomLen/2) // 16 bytes -> 32 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random key: %w", err)
	}
	return keyPrefix + hex.EncodeToString(b), nil
}

// CreateKey hashes the plaintext and stores it. Returns the plaintext (one-time show).
func CreateKey(k *APIKeyAuth) (string, error) {
	plaintext, err := GenerateKey()
	if err != nil {
		return "", err
	}
	k.KeyHash = hashKey(plaintext)
	k.KeyPrefix = plaintext[:prefixDisplay]
	k.LastUsedAt = 0
	if err := store.DB().Create(k).Error; err != nil {
		return "", fmt.Errorf("create api key: %w", err)
	}
	return plaintext, nil
}

// ValidateKey looks up by hash, checks enable/lastUsed, returns the APIKeyAuth record.
func ValidateKey(plaintext string) (*APIKeyAuth, error) {
	hash := hashKey(plaintext)
	k := &APIKeyAuth{}
	if err := store.DB().Where("key_hash = ? AND `enable` = ?", hash, true).First(k).Error; err != nil {
		return nil, fmt.Errorf("api key not found or disabled: %w", err)
	}
	// Update LastUsedAt asynchronously (best-effort, don't block auth)
	now := time.Now().Unix()
	if k.LastUsedAt < now-60 {
		_ = store.DB().Model(k).Where("id = ? AND last_used_at = ?", k.ID, k.LastUsedAt).
			Update("last_used_at", now).Error
	}
	return k, nil
}

// DeleteKey removes a key by ID.
func DeleteKey(id string) error {
	return store.DB().Delete(&APIKeyAuth{}, "id = ?", id).Error
}

// GetKeys returns all API keys.
func GetKeys() ([]APIKeyAuth, error) {
	var list []APIKeyAuth
	if err := store.DB().Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// GetKeyByID returns a single API key by ID.
func GetKeyByID(id string) (*APIKeyAuth, error) {
	k := &APIKeyAuth{}
	if err := store.DB().Where("id = ?", id).First(k).Error; err != nil {
		return nil, err
	}
	return k, nil
}

// hashKey returns the SHA-256 hex digest of the plaintext key.
func hashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// Suppress unused import warnings in tests
var (
	_ = errors.New
	_ = strings.Builder{}
)
