package config

import (
	"fmt"
	"os"
	"time"

	"github.com/dubbogo/gost/encoding/yaml"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

var DefaultConfig = &Config{}

// retryIntervals 退避间隔（秒）
var retryIntervals = []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second}

// Init 从 Nacos 加载配置。
// 先尝试 Nacos，最多重试 3 次（5s/10s/30s 退避）；全部失败后尝试从 /etc/usercenter/config.yaml 本地文件回退；
// 如果全都失败则 panic。
func Init(nacosNamespace, nacosAddr string, port uint64, nacosUserName, nacosPwd string) {
	// 空配置中心地址时直接尝试本地文件回退
	if nacosAddr == "" {
		fmt.Println("[config] Nacos 地址为空，尝试本地文件回退")
		if err := InitFromFile("/etc/usercenter/config.yaml"); err != nil {
			panic(fmt.Sprintf("[config] Nacos 地址为空且本地文件回退失败: %v", err))
		}
		fmt.Println("[config] 成功从本地文件加载配置")
		return
	}

	sc := []constant.ServerConfig{
		{
			IpAddr: nacosAddr,
			Port:   port,
		},
	}

	var lastErr error
	for i, interval := range retryIntervals {
		cc := constant.ClientConfig{
			NamespaceId:         nacosNamespace,
			NotLoadCacheAtStart: false, // 重启时加载 Nacos 本地磁盘缓存，提升 Nacos 不可用时的恢复能力
			LogDir:              "./log",
			CacheDir:            "./cache",
			LogLevel:            "debug",
			Username:            nacosUserName,
			Password:            nacosPwd,
		}

		client, err := clients.NewConfigClient(
			vo.NacosClientParam{
				ClientConfig:  &cc,
				ServerConfigs: sc,
			},
		)
		if err != nil {
			lastErr = err
			fmt.Printf("[config] 创建 Nacos 客户端失败(第%d次): %v\n", i+1, err)
			if i < len(retryIntervals)-1 {
				fmt.Printf("[config] %v 后重试...\n", interval)
				time.Sleep(interval)
			}
			continue
		}

		content, err := client.GetConfig(vo.ConfigParam{
			DataId: "usercenter-config",
			Group:  "nooocode",
		})
		if err != nil {
			lastErr = err
			fmt.Printf("[config] 从 Nacos 获取配置失败(第%d次): %v\n", i+1, err)
			if i < len(retryIntervals)-1 {
				fmt.Printf("[config] %v 后重试...\n", interval)
				time.Sleep(interval)
			}
			continue
		}

		err = yaml.UnmarshalYML([]byte(content), DefaultConfig)
		if err != nil {
			lastErr = err
			fmt.Printf("[config] 解析 Nacos 配置失败(第%d次): %v\n", i+1, err)
			if i < len(retryIntervals)-1 {
				fmt.Printf("[config] %v 后重试...\n", interval)
				time.Sleep(interval)
			}
			continue
		}

		fmt.Println("[config] 成功从 Nacos 加载配置")
		return
	}

	// 全部 Nacos 重试失败，尝试本地文件回退
	fmt.Printf("[config] Nacos 重试全部失败(%v)，尝试本地文件回退...\n", lastErr)
	if err := InitFromFile("/etc/usercenter/config.yaml"); err != nil {
		panic(fmt.Sprintf("[config] Nacos 与本地文件回退均失败: Nacos(%v), 本地(%v)", lastErr, err))
	}
	fmt.Println("[config] 成功从本地文件加载配置")
}

func InitFromFile(fileName string) error {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}
	return yaml.UnmarshalYML(data, DefaultConfig)
}

type Config struct {
	Mysql            string          `yaml:"mysql"`
	Sqlite           string          `yaml:"sqlite"`
	DBType           string          `yaml:"dbType"`
	Debug            bool            `yaml:"debug"`
	Token            TokenConfig     `yaml:"token"`
	SuperAdminRoleID string          `yaml:"superAdminRoleID"`
	PlatformTenantID string          `yaml:"platformTenantID"`
	DefaultRoleID    string          `yaml:"defaultRoleID"`
	DefaultPwd       string          `yaml:"defaultPwd"`
	MiniApp          []MiniAppConfig `yaml:"miniApp"`
	EnableTenant     bool            `yaml:"enableTenant"`
	LoginLock        LoginLockConfig `yaml:"loginLock"`
	// APIKeyEncKey 用于 AES-GCM 加密 AI Key 明文（32 字节十六进制/base64 或任意长度，取 SHA-256 派生）。
	// 留空时从 token.key 派生（SHA-256），保证部署内确定且唯一。
	APIKeyEncKey string `yaml:"apiKeyEncKey"`
	// PIIEncKey 用于 AES-GCM 加密 PII 敏感字段（身份证/手机/邮箱）。
	// 留空时从 token.key 派生（SHA-256），保证部署内确定。
	PIIEncKey string `yaml:"piiEncKey"`
	// OIDCSigningKey PEM 编码的 RSA 私钥,用于 id_token RS256 签名。
	// 留空则启动时自动生成 2048-bit RSA 密钥(日志打印 kid,重启后旧 id_token 失效)。
	OIDCSigningKey string `yaml:"oidcSigningKey"`
	// AlertWebhookURL 告警 Webhook（Slack/钉钉/飞书/自建平台）。空则禁用推送。
	AlertWebhookURL string `yaml:"alertWebhookURL"`
	// SocialLogins 社交登录 provider 配置（GitHub/Google 等）。
	SocialLogins []httpSocialLogin `yaml:"socialLogins"`
	// AICache AI 网关语义缓存配置。全部留空 = 启用默认值。
	AICache AICacheConfig `yaml:"aiCache"`
	// AIAuxModels AI 网关辅助任务（标题生成/嵌入/内容审核）模型别名。留空使用 OpenAI 默认。
	AIAuxModels AIAuxModelsConfig `yaml:"aiAuxModels"`
	// ModerationFailOpen 内容审核服务不可用时的策略。
	// nil（未配置）= fail-open（放行，保证可用性）；false = fail-close（拒绝，合规场景）。
	ModerationFailOpen *bool `yaml:"moderationFailOpen"`
	// PwdExpiredDays 密码过期天数（0=永不过期），详见 internal/auth/pwdexpiry.go。
	PwdExpiredDays int `yaml:"pwdExpiredDays"`
	// SCIMToken SCIM 2.0 同步使用的 Bearer token（空=不启用 SCIM）。
	SCIMToken string `yaml:"scimToken"`
}

// AICacheConfig AI 网关语义缓存配置。
type AICacheConfig struct {
	// Enabled 是否启用缓存。未配置任何字段时默认启用。
	Enabled bool `yaml:"enabled"`
	// SimilarityThreshold 语义相似度阈值（0-1），0=默认 0.95。
	SimilarityThreshold float64 `yaml:"similarityThreshold"`
	// TTLSeconds 缓存有效期（秒），0=默认 86400（24h）。
	TTLSeconds int `yaml:"ttlSeconds"`
	// MaxEntries 最大缓存条目数，0=默认 10000。
	MaxEntries int `yaml:"maxEntries"`
}

// AIAuxModelsConfig AI 网关辅助任务模型别名。
type AIAuxModelsConfig struct {
	Title      string `yaml:"title"`      // 标题生成模型，默认 gpt-3.5-turbo
	Embedding  string `yaml:"embedding"`  // 嵌入模型，默认 text-embedding-3-small
	Moderation string `yaml:"moderation"` // 内容审核模型，默认 text-moderation-latest
}

// httpSocialLogin 与 http.SocialLoginConfig 结构一致（避免 config 反向依赖 http 包）。
type httpSocialLogin struct {
	Provider     string `yaml:"provider"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
	RedirectURI  string `yaml:"redirectURI"`
}

// LoginLockConfig 登录失败锁定策略
type LoginLockConfig struct {
	// MaxErrCount 连续失败次数上限，达到后锁定账号（默认 5）
	MaxErrCount int `yaml:"maxErrCount"`
	// LockMinutes 锁定时长（分钟，默认 15）
	LockMinutes int `yaml:"lockMinutes"`
}

type MiniAppConfig struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Secret   string `yaml:"secret"`
	TenantID string `yaml:"tenantID"`
}

type TokenConfig struct {
	Key       string `yaml:"key"`
	RedisAddr string `yaml:"redisAddr"`
	RedisName string `yaml:"redisName"`
	RedisPwd  string `yaml:"redisPwd"`
	Expired   int    `yaml:"expired"`
}
