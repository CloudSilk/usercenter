package auth

// 密钥轮换 JWKS + kid + grace period(REDESIGN #16)

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SigningKey JWT 签名密钥(含 kid)
type SigningKey struct {
	Kid       string    `json:"kid"`
	Key       string    `json:"-"`
	Algorithm string    `json:"alg"`
	CreatedAt time.Time `json:"createdAt"`
	IsActive  bool      `json:"isActive"`
}

// KeyManager 密钥管理器(新旧密钥共存,grace period)
type KeyManager struct {
	mu          sync.RWMutex
	keys        map[string]*SigningKey // kid → key
	activeKID   string
	gracePeriod time.Duration
}

var globalKeyManager = &KeyManager{
	keys:        make(map[string]*SigningKey),
	gracePeriod: 24 * time.Hour, // 旧密钥 grace period
}

// InitKeyManager 初始化密钥管理器(由 main.go 调用)
func InitKeyManager(secretKey string) {
	globalKeyManager.mu.Lock()
	defer globalKeyManager.mu.Unlock()
	kid := generateKID()
	globalKeyManager.keys[kid] = &SigningKey{
		Kid: kid, Key: secretKey, Algorithm: "HS256",
		CreatedAt: time.Now(), IsActive: true,
	}
	globalKeyManager.activeKID = kid
}

// GetActiveKey 获取当前签名密钥
func GetActiveKey() *SigningKey {
	globalKeyManager.mu.RLock()
	defer globalKeyManager.mu.RUnlock()
	return globalKeyManager.keys[globalKeyManager.activeKID]
}

// GetKeyByID 按 kid 获取密钥(验签用,含 grace period 内的旧密钥)
func GetKeyByID(kid string) *SigningKey {
	globalKeyManager.mu.RLock()
	defer globalKeyManager.mu.RUnlock()
	return globalKeyManager.keys[kid]
}

// RotateKey 轮换签名密钥:旧密钥标记非活跃(保留 grace period),新密钥成为 active
func RotateKey(newSecretKey string) string {
	globalKeyManager.mu.Lock()
	defer globalKeyManager.mu.Unlock()

	// 旧密钥降级(grace period 内仍可验签)
	if old := globalKeyManager.keys[globalKeyManager.activeKID]; old != nil {
		old.IsActive = false
	}

	// 新密钥
	kid := generateKID()
	globalKeyManager.keys[kid] = &SigningKey{
		Kid: kid, Key: newSecretKey, Algorithm: "HS256",
		CreatedAt: time.Now(), IsActive: true,
	}
	globalKeyManager.activeKID = kid

	// 清理过期旧密钥(beyond grace period)
	cutoff := time.Now().Add(-globalKeyManager.gracePeriod)
	for k, key := range globalKeyManager.keys {
		if !key.IsActive && key.CreatedAt.Before(cutoff) {
			delete(globalKeyManager.keys, k)
		}
	}

	return kid
}

// GetJWKS 返回 JWKS(公开密钥标识,不含密钥明文)
func GetJWKS(issuer string) *JWKS {
	globalKeyManager.mu.RLock()
	defer globalKeyManager.mu.RUnlock()

	var keys []JSONWebKey
	for _, key := range globalKeyManager.keys {
		// grace period 内的旧密钥也暴露在 JWKS(客户端可验签旧 token)
		if key.IsActive || time.Since(key.CreatedAt) < globalKeyManager.gracePeriod {
			keys = append(keys, JSONWebKey{
				Kty: "oct", Kid: key.Kid, Alg: key.Algorithm, Use: "sig",
			})
		}
	}
	return &JWKS{Keys: keys}
}

// SetGracePeriod 设置 grace period(测试用)
func SetGracePeriod(d time.Duration) {
	globalKeyManager.mu.Lock()
	defer globalKeyManager.mu.Unlock()
	globalKeyManager.gracePeriod = d
}

// generateKID 生成密钥 ID(16 字节 hex)
func generateKID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Suppress unused
var _ = fmt.Sprintf
var _ = strings.Builder{}
