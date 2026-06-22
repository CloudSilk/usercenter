package apikey

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/pkg/utils/log"
	"github.com/CloudSilk/usercenter/internal/store"
	apipb "github.com/CloudSilk/usercenter/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AIProvider LLM 服务商(OpenAI/Anthropic/本地 vLLM 等)
type AIProvider struct {
	commonmodel.Model
	TenantID    string `json:"tenantID" gorm:"index;size:36"`
	Name        string `json:"name" gorm:"size:100;index;comment:服务商名称"`
	BaseURL     string `json:"baseURL" gorm:"size:500;comment:API基础URL"`
	AuthType    string `json:"authType" gorm:"size:50;comment:鉴权方式(bearer/header/apikey)"`
	Healthy     bool   `json:"healthy" gorm:"index;default:true;comment:健康状态"`
	Description string `json:"description" gorm:"size:500"`
	IsMust      bool   `json:"isMust" gorm:"index;comment:系统必须要有的数据"`
}

func (AIProvider) TableName() string { return "ai_provider" }

// AIKey LLM API Key(明文加密存储,AES-GCM)
type AIKey struct {
	commonmodel.Model
	TenantID    string `json:"tenantID" gorm:"index;size:36"`
	ProviderID  string `json:"providerID" gorm:"index;size:36;comment:所属服务商"`
	Name        string `json:"name" gorm:"size:100;comment:Key名称"`
	APIKeyEnc   string `json:"-" gorm:"column:api_key_enc;size:1000;comment:AES-GCM加密后的Key"`
	KeyHint     string `json:"keyHint" gorm:"size:20;comment:Key前4后4位,用于识别"`
	Priority    int32  `json:"priority" gorm:"default:0;comment:优先级(主从池)"`
	Enable      bool   `json:"enable" gorm:"index;default:true"`
	CooldownEnd int64  `json:"cooldownEnd" gorm:"default:0;comment:故障冷却结束时间(unix)"`
	Last429     int64  `json:"last429" gorm:"default:0;comment:上次429时间"`
	IsMust      bool   `json:"isMust" gorm:"index"`
}

func (AIKey) TableName() string { return "ai_key" }

// ModelRoute 模型路由策略(request → model alias → provider+key)
type ModelRoute struct {
	commonmodel.Model
	TenantID    string `json:"tenantID" gorm:"index;size:36"`
	ModelAlias  string `json:"modelAlias" gorm:"size:100;index;comment:模型别名(gpt-4/claude-3等)"`
	ProviderID  string `json:"providerID" gorm:"size:36;index;comment:目标服务商"`
	Priority    int32  `json:"priority" gorm:"default:0;comment:路由优先级"`
	Enable      bool   `json:"enable" gorm:"index;default:true"`
	Description string `json:"description" gorm:"size:500"`
}

func (ModelRoute) TableName() string { return "model_route" }

// --- 加密/解密(AES-GCM) ---

var encryptionKey []byte

// SetEncryptionKey 设置 API Key 加密密钥(32 字节,由 main.go 启动时注入)
func SetEncryptionKey(key []byte) {
	if len(key) >= 32 {
		encryptionKey = key[:32]
	}
}

// SetEncryptionKeyFrom derives the 32-byte AES-GCM key from an arbitrary input
// via SHA-256. Accepts the raw token key / configured apiKeyEncKey so callers
// don't need to manage exact byte length. Empty input returns false and leaves
// the key unset (decrypt/encrypt will then error explicitly).
func SetEncryptionKeyFrom(input string) bool {
	if input == "" {
		return false
	}
	sum := sha256.Sum256([]byte(input))
	encryptionKey = sum[:]
	return true
}

func encryptAPIKey(plaintext string) (string, error) {
	if len(encryptionKey) == 0 {
		return "", errors.New("encryption key not set")
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aesgcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptAPIKey(enc string) (string, error) {
	if len(encryptionKey) == 0 {
		return "", errors.New("encryption key not set")
	}
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aesgcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	plaintext, err := aesgcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// makeKeyHint 生成 Key 提示(前4后4)
func makeKeyHint(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// --- Provider CRUD ---

func CreateProvider(p *AIProvider) (string, error) {
	err := store.DB().Create(p).Error
	return p.ID, err
}

func UpdateProvider(p *AIProvider) error {
	return store.DB().Omit("created_at").Save(p).Error
}

func DeleteProvider(id string) error {
	return store.DB().Delete(&AIProvider{}, "id=?", id).Error
}

func GetProviderByID(id string) (*AIProvider, error) {
	p := &AIProvider{}
	err := store.DB().Where("id = ?", id).First(p).Error
	return p, err
}

func GetAllProviders(tenantID string) (list []*AIProvider, err error) {
	db := store.DB()
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	err = db.Find(&list).Error
	return
}

// --- Key CRUD ---

// CreateKey 创建 API Key(明文加密存储)
func CreateKey(k *AIKey, plaintext string) (string, error) {
	enc, err := encryptAPIKey(plaintext)
	if err != nil {
		return "", fmt.Errorf("encrypt api key: %w", err)
	}
	k.APIKeyEnc = enc
	k.KeyHint = makeKeyHint(plaintext)
	err = store.DB().Create(k).Error
	return k.ID, err
}

// GetDecryptedKey 解密获取明文 API Key
func GetDecryptedKey(id string) (string, error) {
	k := &AIKey{}
	if err := store.DB().Where("id = ?", id).First(k).Error; err != nil {
		return "", err
	}
	return decryptAPIKey(k.APIKeyEnc)
}

func UpdateKey(k *AIKey) error {
	return store.DB().Omit("created_at", "api_key_enc").Save(k).Error
}

// GetKeysByProvider 列出某服务商下的 Key(可选租户隔离)。
// tenantID 为空时返回该服务商全部 Key(管理端跨租户视图)。
// 返回的 AIKey.APIKeyEnc 为加密密文,调用方不应下发到前端(用 KeyHint 展示)。
func GetKeysByProvider(providerID, tenantID string) (list []*AIKey, err error) {
	db := store.DB().Where("provider_id = ?", providerID)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	err = db.Order("priority, created_at").Find(&list).Error
	return
}

func DeleteKey(id string) error {
	return store.DB().Delete(&AIKey{}, "id=?", id).Error
}

// RotateKey 轮转 API Key(旧 key 立即失效,新 key 写入)
func RotateKey(id, newPlaintext string) error {
	enc, err := encryptAPIKey(newPlaintext)
	if err != nil {
		return err
	}
	return store.DB().Model(&AIKey{}).Where("id=?", id).Updates(map[string]interface{}{
		"api_key_enc": enc,
		"key_hint":    makeKeyHint(newPlaintext),
		"cooldown_end": 0,
		"last429":     0,
	}).Error
}

// MarkCooldown 标记 Key 冷却(429/401 时调用)
func MarkCooldown(id string, duration time.Duration) {
	_ = store.DB().Model(&AIKey{}).Where("id=?", id).Updates(map[string]interface{}{
		"cooldown_end": time.Now().Add(duration).Unix(),
		"last429":      time.Now().Unix(),
	}).Error
}

// --- Key 池选择(故障转移) ---

// KeySelection 选中的 Key + Provider
type KeySelection struct {
	Key      *AIKey
	Provider *AIProvider
	APIKey   string // 解密后的明文
}

// SelectKey 根据模型路由选择可用 Key(主从池 + 故障转移)
// 策略:按 priority 排序,跳过 enable=false 和 cooldown 未过期的
func SelectKey(tenantID, modelAlias string) (*KeySelection, error) {
	// 1. 查路由
	var routes []*ModelRoute
	routeDB := store.DB().Where("enable = ? AND tenant_id IN (?, '')", true, tenantID)
	if modelAlias != "" {
		routeDB = routeDB.Where("model_alias = ?", modelAlias)
	}
	if err := routeDB.Order("priority").Find(&routes).Error; err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, errors.New("no model route found")
	}

	now := time.Now().Unix()
	// 2. 按路由优先级遍历,选第一个有可用 Key 的 Provider
	for _, route := range routes {
		provider, err := GetProviderByID(route.ProviderID)
		if err != nil || !provider.Healthy {
			continue
		}
		// 3. 选该 Provider 下最高优先级的可用 Key
		var key AIKey
		err = store.DB().Where("provider_id = ? AND enable = ? AND cooldown_end <= ?",
			route.ProviderID, true, now).
			Order("priority").
			First(&key).Error
		if err != nil {
			continue
		}
		// 4. 解密
		plaintext, err := decryptAPIKey(key.APIKeyEnc)
		if err != nil {
			log.Errorf(context.Background(), "decrypt api key %s failed: %v", key.ID, err)
			continue
		}
		return &KeySelection{Key: &key, Provider: provider, APIKey: plaintext}, nil
	}

	return nil, errors.New("no available api key (all keys disabled or in cooldown)")
}

// --- ModelRoute CRUD ---

func CreateRoute(r *ModelRoute) (string, error) {
	err := store.DB().Create(r).Error
	return r.ID, err
}

func UpdateRoute(r *ModelRoute) error {
	return store.DB().Omit("created_at").Save(r).Error
}

func DeleteRoute(id string) error {
	return store.DB().Delete(&ModelRoute{}, "id=?", id).Error
}

func GetRoutes(tenantID string) (list []*ModelRoute, err error) {
	db := store.DB().Where("tenant_id IN (?, '')", tenantID)
	err = db.Preload(clause.Associations).Find(&list).Error
	return
}

// --- PB 转换 ---

func PBToProvider(in *apipb.UserInfo) *AIProvider {
	if in == nil {
		return nil
	}
	return &AIProvider{
		Model:       commonmodel.Model{ID: in.Id},
		TenantID:    in.TenantID,
		Name:        in.Nickname,
		BaseURL:     in.Avatar,
		Description: in.Description,
	}
}

// Suppress unused import warnings
var (
	_ = strings.Builder{}
	_ = gorm.ErrRecordNotFound
	_ = apipb.Code_Success
)
