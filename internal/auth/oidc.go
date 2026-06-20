package auth

// OIDC 端点(REDESIGN #6):标准 OAuth2/OIDC 端点骨架
// /.well-known/openid-configuration
// /oauth/authorize, /oauth/token, /oauth/revoke, /oauth/userinfo

// OIDCDiscovery OIDC 发现文档
type OIDCDiscovery struct {
	Issuer                 string   `json:"issuer"`
	AuthorizationEndpoint  string   `json:"authorization_endpoint"`
	TokenEndpoint          string   `json:"token_endpoint"`
	UserInfoEndpoint       string   `json:"userinfo_endpoint"`
	RevocationEndpoint     string   `json:"revocation_endpoint"`
	JWKSURI                string   `json:"jwks_uri"`
	ResponseTypes          []string `json:"response_types_supported"`
	SubjectTypes           []string `json:"subject_types_supported"`
	IDTokenSigningAlgs     []string `json:"id_token_signing_alg_values_supported"`
	Scopes                 []string `json:"scopes_supported"`
	TokenEndpointAuthMethods []string `json:"token_endpoint_auth_methods_supported"`
	Claims                 []string `json:"claims_supported"`
}

// GetDiscovery 返回 OIDC 发现文档
func GetDiscovery(issuer string) *OIDCDiscovery {
	return &OIDCDiscovery{
		Issuer:                issuer,
		AuthorizationEndpoint: issuer + "/oauth/authorize",
		TokenEndpoint:         issuer + "/oauth/token",
		UserInfoEndpoint:      issuer + "/oauth/userinfo",
		RevocationEndpoint:    issuer + "/oauth/revoke",
		JWKSURI:               issuer + "/.well-known/jwks.json",
		ResponseTypes:         []string{"code", "token", "id_token"},
		SubjectTypes:          []string{"public"},
		IDTokenSigningAlgs:    []string{"HS256"},
		Scopes:                []string{"openid", "profile", "email", "read", "write"},
		TokenEndpointAuthMethods: []string{"client_secret_basic", "client_secret_post"},
		Claims: []string{"sub", "iss", "aud", "exp", "iat", "name", "email", "tenant_id", "role_ids"},
	}
}

// JWKS JSON Web Key Set(REDESIGN #16 密钥轮换)
type JWKS struct {
	Keys []JSONWebKey `json:"keys"`
}

type JSONWebKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

// TokenResponse OAuth2 token 响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// RefreshToken refresh token 存储结构
type RefreshToken struct {
	Token        string `json:"token" gorm:"primaryKey;size:64"`
	PrincipalID  string `json:"principalID" gorm:"index;size:36"`
	TenantID     string `json:"tenantID" gorm:"index;size:36"`
	ClientID     string `json:"clientID" gorm:"size:100"`
	Scope        string `json:"scope" gorm:"size:500"`
	ExpiresAt    int64  `json:"expiresAt"`
	Revoked      bool   `json:"revoked" gorm:"index;default:false"`
}

func (RefreshToken) TableName() string { return "refresh_token" }

// OAuthClient OAuth2 客户端应用(第三方接入)
type OAuthClient struct {
	ID           string `json:"id" gorm:"primaryKey;size:64"`
	Secret       string `json:"-" gorm:"size:200"`
	Name         string `json:"name" gorm:"size:200"`
	RedirectURIs string `json:"redirectURIs" gorm:"size:1000;comment:逗号分隔"`
	GrantTypes   string `json:"grantTypes" gorm:"size:200;comment:逗号分隔"`
	Scopes       string `json:"scopes" gorm:"size:500"`
	Enable       bool   `json:"enable" gorm:"index;default:true"`
}

func (OAuthClient) TableName() string { return "oauth_client" }

// ConsentRecord 用户授权同意记录(Agent 委派 #10)
type ConsentRecord struct {
	ID           uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	PrincipalID  string `json:"principalID" gorm:"index;size:36;comment:授权人"`
	ClientID     string `json:"clientID" gorm:"index;size:64;comment:被授权的Agent/应用"`
	Scope        string `json:"scope" gorm:"size:500;comment:授权范围"`
	GrantedAt    int64  `json:"grantedAt"`
	ExpiresAt    int64  `json:"expiresAt" gorm:"comment:0=永不过期"`
	Revoked      bool   `json:"revoked" gorm:"index;default:false"`
}

func (ConsentRecord) TableName() string { return "oauth_consent" }
