package config

import (
	"os"

	"github.com/dubbogo/gost/encoding/yaml"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

var DefaultConfig = &Config{}

func Init(nacosNamespace, nacosAddr string, port uint64, nacosUserName, nacosPwd string) {
	sc := []constant.ServerConfig{
		{
			IpAddr: nacosAddr,
			Port:   port,
		},
	}

	cc := constant.ClientConfig{
		NamespaceId:         nacosNamespace,
		NotLoadCacheAtStart: true,
		LogDir:              "./log",
		CacheDir:            "./cache",
		LogLevel:            "debug",
		Username:            nacosUserName,
		Password:            nacosPwd,
	}

	// a more graceful way to create config client
	client, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)

	if err != nil {
		panic(err)
	}

	//get config
	content, err := client.GetConfig(vo.ConfigParam{
		DataId: "usercenter-config",
		Group:  "nooocode",
	})
	if err != nil {
		panic(err)
	}
	err = yaml.UnmarshalYML([]byte(content), DefaultConfig)
	if err != nil {
		panic(err)
	}
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
	// AlertWebhookURL 告警 Webhook（Slack/钉钉/飞书/自建平台）。空则禁用推送。
	AlertWebhookURL string `yaml:"alertWebhookURL"`
	// SocialLogins 社交登录 provider 配置（GitHub/Google 等）。
	SocialLogins []httpSocialLogin `yaml:"socialLogins"`
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
