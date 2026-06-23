package auth

// RSA 签名密钥管理器(REDESIGN S2):id_token 签名为 RS256 + JWKS 暴露公钥 n/e。
//
// OIDC id_token 必须用非对称密钥——私钥在 IdP 签发,公钥经 JWKS 公开给 Client 验签。
// HS256 对称密钥一旦泄露即可伪造任意 id_token,RS256 把"签发权"留在一方。
//
// 设计:
//   - InitKeyManager 生成或注入 RSA-2048 密钥对(传入 PEM,空串则自动生成)
//   - SigningKey 持有 *rsa.PrivateKey / *rsa.PublicKey
//   - RotateKey 轮换:旧 key 保留 grace period 供 Client 验签旧 id_token
//   - GetJWKS 返回 RSA 公钥的 JWK(n/e)数组
//   - GetActiveKey / GetKeyByID 返回签发/验签用密钥

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

const defaultRSAKeyBits = 2048

// SigningKey RSA 签名密钥(含 kid)
type SigningKey struct {
	Kid       string         `json:"kid"`
	Private   *rsa.PrivateKey `json:"-"`
	Public    *rsa.PublicKey  `json:"-"`
	Algorithm string         `json:"alg"`
	CreatedAt time.Time      `json:"createdAt"`
	IsActive  bool           `json:"isActive"`
}

// KeyManager RSA 密钥管理器(新旧密钥共存,grace period)
type KeyManager struct {
	mu          sync.RWMutex
	keys        map[string]*SigningKey // kid → key
	activeKID   string
	gracePeriod time.Duration
}

var globalKeyManager = &KeyManager{
	keys:        make(map[string]*SigningKey),
	gracePeriod: 24 * time.Hour,
}

// InitKeyManager 初始化密钥管理器(由 main.go 调用)。
// pemPrivateKey 为空时自动生成 RSA-2048 密钥对。
// 返回 kid。
func InitKeyManager(pemPrivateKey string) (string, error) {
	priv, err := parseRSAPrivatePEM(pemPrivateKey)
	if err != nil {
		return "", err
	}
	kid := kidFromPublic(&priv.PublicKey)

	globalKeyManager.mu.Lock()
	defer globalKeyManager.mu.Unlock()
	globalKeyManager.keys[kid] = &SigningKey{
		Kid: kid, Private: priv, Public: &priv.PublicKey,
		Algorithm: "RS256", CreatedAt: time.Now(), IsActive: true,
	}
	globalKeyManager.activeKID = kid
	return kid, nil
}

// GetActiveKey 获取当前签名密钥(含私钥,用于签发)
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

// RotateKey 轮换签名密钥:生成新 RSA 密钥对,旧密钥标记非活跃(保留 grace period)。
// 返回新 kid。
func RotateKey(pemPrivateKey string) (string, error) {
	priv, err := parseRSAPrivatePEM(pemPrivateKey)
	if err != nil {
		return "", err
	}

	globalKeyManager.mu.Lock()
	defer globalKeyManager.mu.Unlock()

	// 旧密钥降级(grace period 内仍可验签)
	if old := globalKeyManager.keys[globalKeyManager.activeKID]; old != nil {
		old.IsActive = false
	}

	kid := kidFromPublic(&priv.PublicKey)
	globalKeyManager.keys[kid] = &SigningKey{
		Kid: kid, Private: priv, Public: &priv.PublicKey,
		Algorithm: "RS256", CreatedAt: time.Now(), IsActive: true,
	}
	globalKeyManager.activeKID = kid

	// 清理过期旧密钥(beyond grace period)
	cutoff := time.Now().Add(-globalKeyManager.gracePeriod)
	for k, key := range globalKeyManager.keys {
		if !key.IsActive && key.CreatedAt.Before(cutoff) {
			delete(globalKeyManager.keys, k)
		}
	}

	return kid, nil
}

// RSAJWK JWKS 中的单个 RSA 公钥条目(RFC 7517 / 7518)
type RSAJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// GetJWKS 返回 JWKS 公钥数组(活跃 + grace period 内的旧密钥)
func GetJWKS() []RSAJWK {
	globalKeyManager.mu.RLock()
	defer globalKeyManager.mu.RUnlock()

	var out []RSAJWK
	for _, key := range globalKeyManager.keys {
		if !key.IsActive && time.Since(key.CreatedAt) >= globalKeyManager.gracePeriod {
			continue
		}
		out = append(out, rsaPublicToJWK(key.Public, key.Kid))
	}
	return out
}

// GetJWKSMap 返回 JWKS 作为 gin-compatible map(兼容旧调用方)
func GetJWKSMap() map[string]interface{} {
	keys := GetJWKS()
	if len(keys) == 0 {
		return map[string]interface{}{"keys": []RSAJWK{}}
	}
	return map[string]interface{}{"keys": keys}
}

// SetGracePeriod 设置 grace period(测试用)
func SetGracePeriod(d time.Duration) {
	globalKeyManager.mu.Lock()
	defer globalKeyManager.mu.Unlock()
	globalKeyManager.gracePeriod = d
}

// ResetKeys 清空所有密钥(测试用,避免跨用例污染)
func ResetKeys() {
	globalKeyManager.mu.Lock()
	defer globalKeyManager.mu.Unlock()
	globalKeyManager.keys = make(map[string]*SigningKey)
	globalKeyManager.activeKID = ""
}

// --- 内部工具 ---

func parseRSAPrivatePEM(s string) (*rsa.PrivateKey, error) {
	if strings.TrimSpace(s) == "" {
		return rsa.GenerateKey(rand.Reader, defaultRSAKeyBits)
	}
	block, _ := pem.Decode([]byte(s))
	if block == nil {
		// 容错:裸 base64 串视为 PKCS1/8 DER 的 base64
		if raw, err := base64.StdEncoding.DecodeString(s); err == nil {
			return parseRSADER(raw)
		}
		return nil, errors.New("invalid PEM private key")
	}
	return parseRSADER(block.Bytes)
}

func parseRSADER(der []byte) (*rsa.PrivateKey, error) {
	if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("PKCS8 key is not RSA")
	}
	return rk, nil
}

// kidFromPublic 用公钥的 SPKI DER 做 SHA-256,取前 16 字节 hex 作为 kid。
// 确定性:同一密钥每次 kid 相同。
func kidFromPublic(pub *rsa.PublicKey) string {
	spki, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		// 退化路径:用 (n,e) 直接哈希
		h := sha256.New()
		h.Write(pub.N.Bytes())
		b := make([]byte, 4)
		b[0] = byte(pub.E >> 24)
		b[1] = byte(pub.E >> 16)
		b[2] = byte(pub.E >> 8)
		b[3] = byte(pub.E)
		h.Write(b)
		sum := h.Sum(nil)
		return encodeHex(sum[:16])
	}
	sum := sha256.Sum256(spki)
	return encodeHex(sum[:16])
}

func rsaPublicToJWK(pub *rsa.PublicKey, kid string) RSAJWK {
	return RSAJWK{
		Kty: "RSA", Kid: kid, Use: "sig", Alg: "RS256",
		N: base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString(bigEndBytes(pub.E)),
	}
}

// bigEndBytes 把 int 编成大端字节(RSA 公钥指数 e 通常为 65537 = 0x010001)
func bigEndBytes(e int) []byte {
	return new(big.Int).SetInt64(int64(e)).Bytes()
}

func encodeHex(b []byte) string {
	const hexc = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexc[v>>4]
		out[i*2+1] = hexc[v&0x0f]
	}
	return string(out)
}

// Suppress unused import
var _ = fmt.Sprintf
