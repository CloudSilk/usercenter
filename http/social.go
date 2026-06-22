package http

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/identity"
	"github.com/CloudSilk/usercenter/internal/store"
	"github.com/CloudSilk/usercenter/internal/user"
	apipb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 社交登录画廊（GitHub / Google）：UserCenter 作为社交 IdP 聚合登录。
//
//   GET  /api/oauth/:provider/login?redirect=<front>   跳转 provider 授权页（带 state）
//   GET  /api/oauth/:provider/callback?code=&state=    换 token、取 profile、匹配/创建用户、签发 token、回跳前端
//
// provider 的 authorize/token/profile URL 与 scope 在 socialProviders 中声明，client 凭据由配置注入。

// SocialLoginConfig 单个社交登录 provider 配置。
type SocialLoginConfig struct {
	Provider     string `json:"provider" yaml:"provider"`
	ClientID     string `json:"clientID" yaml:"clientID"`
	ClientSecret string `json:"clientSecret" yaml:"clientSecret"`
	RedirectURI  string `json:"redirectURI" yaml:"redirectURI"`
}

type socialProviderSpec struct {
	AuthorizeURL string
	TokenURL     string
	ProfileURL   string
	Scope        string
	// profile → {sub, email, name, avatar} 的映射
	Map func(profile map[string]interface{}) (sub, email, name, avatar string)
}

var socialProviders = map[string]socialProviderSpec{
	"github": {
		AuthorizeURL: "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		ProfileURL:   "https://api.github.com/user",
		Scope:        "read:user user:email",
		Map: func(p map[string]interface{}) (string, string, string, string) {
			sub := fmt.Sprintf("%v", p["id"])
			email, _ := p["email"].(string)
			name, _ := p["name"].(string)
			if name == "" {
				name, _ = p["login"].(string)
			}
			avatar, _ := p["avatar_url"].(string)
			return sub, email, name, avatar
		},
	},
	"google": {
		AuthorizeURL: "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		ProfileURL:   "https://www.googleapis.com/oauth2/v2/userinfo",
		Scope:        "openid email profile",
		Map: func(p map[string]interface{}) (string, string, string, string) {
			sub, _ := p["id"].(string)
			email, _ := p["email"].(string)
			name, _ := p["name"].(string)
			avatar, _ := p["picture"].(string)
			return sub, email, name, avatar
		},
	},
}

// 配置注入（main.go 启动时调用 SetSocialLogins）。
var (
	socialMu      sync.RWMutex
	socialConfigs = map[string]SocialLoginConfig{}
)

// SetSocialLogins 注入社交登录 provider 配置。
func SetSocialLogins(configs []SocialLoginConfig) {
	socialMu.Lock()
	defer socialMu.Unlock()
	socialConfigs = map[string]SocialLoginConfig{}
	for _, c := range configs {
		if c.Provider != "" {
			socialConfigs[c.Provider] = c
		}
	}
}

// GetSocialProviders 返回已配置的 provider 列表（供面板展示「画廊」）。
func GetSocialProviders() []string {
	socialMu.RLock()
	defer socialMu.RUnlock()
	out := make([]string, 0, len(socialConfigs))
	for k := range socialConfigs {
		out = append(out, k)
	}
	return out
}

// state 短期缓存（防 CSRF，10 分钟）。
var socialStates = newSocialStateCache()

type stateEntry struct{ redirect string; expire time.Time }

type socialStateCache struct {
	mu sync.Mutex
	m  map[string]stateEntry
}

func newSocialStateCache() *socialStateCache {
	c := &socialStateCache{m: map[string]stateEntry{}}
	go func() {
		for range time.Tick(time.Minute) {
			c.mu.Lock()
			now := time.Now()
			for k, v := range c.m {
				if now.After(v.expire) {
					delete(c.m, k)
				}
			}
			c.mu.Unlock()
		}
	}()
	return c
}

func (c *socialStateCache) set(state, redirect string) {
	c.mu.Lock()
	c.m[state] = stateEntry{redirect: redirect, expire: time.Now().Add(10 * time.Minute)}
	c.mu.Unlock()
}

func (c *socialStateCache) take(state string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[state]
	if !ok {
		return "", false
	}
	delete(c.m, state)
	return e.redirect, true
}

// RegisterSocialLoginRouter 挂载社交登录端点（公开，无需登录）。
func RegisterSocialLoginRouter(r *gin.Engine) {
	r.GET("/api/oauth/:provider/login", socialLoginRedirect)
	r.GET("/api/oauth/:provider/callback", socialCallback)
	r.GET("/api/social/providers", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 20000, "data": GetSocialProviders()})
	})
}

// registerSocialAdminRoutes 管理端：已配置的社交 provider 列表 + 当前用户外部身份。
func registerSocialAdminRoutes(g *gin.RouterGroup) {
	g.GET("/social/providers", func(c *gin.Context) {
		writeOK(c, gin.H{"data": GetSocialProviders()})
	})
	ident := g.Group("/identities")
	ident.GET("", func(c *gin.Context) {
		list, err := identity.ListByUser(ucm.GetUserID(c))
		if err != nil {
			writeErr(c, err)
			return
		}
		writeOK(c, gin.H{"data": list})
	})
	ident.DELETE("/:id", func(c *gin.Context) {
		if err := identity.Unbind(c.Param("id"), ucm.GetUserID(c)); err != nil {
			writeErr(c, err)
			return
		}
		recordAudit(c, "social_unbind", c.Param("id"), "")
		writeOK(c, nil)
	})
}

func socialLoginRedirect(c *gin.Context) {
	provider := c.Param("provider")
	spec, ok := socialProviders[provider]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider"})
		return
	}
	socialMu.RLock()
	cfg, ok := socialConfigs[provider]
	socialMu.RUnlock()
	if !ok || cfg.ClientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider 未配置"})
		return
	}
	state := randToken()
	socialStates.set(state, c.Query("redirect"))
	q := fmt.Sprintf("?client_id=%s&redirect_uri=%s&scope=%s&state=%s&response_type=code",
		cfg.ClientID, cfg.RedirectURI, spec.Scope, state)
	c.Redirect(http.StatusFound, spec.AuthorizeURL+q)
}

func socialCallback(c *gin.Context) {
	provider := c.Param("provider")
	spec, ok := socialProviders[provider]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider"})
		return
	}
	socialMu.RLock()
	cfg, ok := socialConfigs[provider]
	socialMu.RUnlock()
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider 未配置"})
		return
	}
	front, ok := socialStates.take(c.Query("state"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired state"})
		return
	}
	code := c.Query("code")
	accessToken, err := exchangeSocialCode(spec.TokenURL, cfg, code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "token 交换失败: " + err.Error()})
		return
	}
	profile, err := fetchSocialProfile(spec.ProfileURL, accessToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "获取用户信息失败: " + err.Error()})
		return
	}
	sub, email, name, avatar := spec.Map(profile)

	userID, err := matchOrCreateSocialUser(provider, sub, email, name, avatar)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tok, err := issueSocialToken(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if front == "" {
		front = "/web/admin"
	}
	sep := "?"
	if strings.Contains(front, "?") {
		sep = "&"
	}
	c.Redirect(http.StatusFound, front+sep+"social_token="+tok)
}

// matchOrCreateSocialUser 按 (provider,sub) → email → 新建 的顺序匹配/绑定用户。
func matchOrCreateSocialUser(provider, sub, email, name, avatar string) (string, error) {
	if bind, err := identity.FindByProvider(provider, sub); err == nil && bind != nil {
		return bind.UserID, nil
	}
	// 按 email 匹配已有用户
	var u user.User
	emailMatch := false
	if email != "" {
		if err := store.DB().Where("email = ? OR user_name = ?", email, email).First(&u).Error; err == nil {
			emailMatch = true
		}
	}
	if !emailMatch {
		// 新建用户（社交注册）：随机强密码，用户名=provider+sub 或 email
		uname := email
		if uname == "" {
			uname = provider + "_" + sub
		}
		// 避免用户名冲突：追加短后缀
		if exists(uname) {
			uname = uname + "_" + randToken()[:6]
		}
		u = user.User{}
		u.UserName = strings.ToLower(uname)
		u.Nickname = name
		u.Email = email
		u.Avatar = avatar
		u.Enable = true
		u.TenantID = platformTenant()
		if err := user.CreateUser(&u, false); err != nil && err != gorm.ErrRecordNotFound {
			return "", fmt.Errorf("创建用户失败: %w", err)
		}
	}
	// 绑定身份
	if _, err := identity.Bind(u.ID, provider, sub, email); err != nil {
		return "", err
	}
	return u.ID, nil
}

func exists(userName string) bool {
	var count int64
	_ = store.DB().Model(&user.User{}).Where("user_name = ?", strings.ToLower(userName)).Count(&count).Error
	return count > 0
}

func platformTenant() string {
	// 取第一个租户作为社交注册用户的归宿；具体策略可后续细化
	var t struct{ ID string }
	_ = store.DB().Table("tenant").Limit(1).Scan(&t).Error
	return t.ID
}

func issueSocialToken(userID string) (string, error) {
	var u user.User
	if err := store.DB().Preload("UserRoles").First(&u, "id = ?", userID).Error; err != nil {
		return "", err
	}
	cu := &apipb.CurrentUser{
		Id: u.ID, UserName: u.UserName, Nickname: u.Nickname, Avatar: u.Avatar,
		TenantID: u.TenantID, RoleIDs: u.GetRoleIDs(),
	}
	// 复用 token.EncodeToken（同包 http 之外，经 user 包不直接可达，这里用领域 token）
	return encodeUserToken(cu)
}

// --- HTTP 工具 ---

func exchangeSocialCode(tokenURL string, cfg SocialLoginConfig, code string) (string, error) {
	body := fmt.Sprintf("client_id=%s&client_secret=%s&code=%s&redirect_uri=%s&grant_type=authorization_code",
		cfg.ClientID, cfg.ClientSecret, code, cfg.RedirectURI)
	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("解析 token 响应失败: %s", string(raw))
	}
	if out.Error != "" {
		return "", fmt.Errorf("%s", out.Error)
	}
	return out.AccessToken, nil
}

func fetchSocialProfile(profileURL, accessToken string) (map[string]interface{}, error) {
	req, _ := http.NewRequest(http.MethodGet, profileURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("解析 profile 失败: %s", string(raw))
	}
	return m, nil
}

func randToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// encodeUserToken 复用 token.EncodeToken 为社交登录用户签发 JWT。
func encodeUserToken(cu *apipb.CurrentUser) (string, error) {
	return token.EncodeToken(cu)
}
